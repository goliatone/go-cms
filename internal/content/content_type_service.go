package content

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/permissions"
	"github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-cms/internal/validation"
	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/goliatone/go-slug"
	"github.com/google/uuid"
)

// ContentTypeService provides CRUD operations for content types.
type ContentTypeService interface {
	Create(ctx context.Context, req CreateContentTypeRequest) (*ContentType, error)
	Update(ctx context.Context, req UpdateContentTypeRequest) (*ContentType, error)
	Delete(ctx context.Context, req DeleteContentTypeRequest) error
	Get(ctx context.Context, id uuid.UUID) (*ContentType, error)
	GetBySlug(ctx context.Context, slug string) (*ContentType, error)
	List(ctx context.Context) ([]*ContentType, error)
	Search(ctx context.Context, query string) ([]*ContentType, error)
}

// CreateContentTypeRequest captures required fields to create a content type.
type CreateContentTypeRequest struct {
	Name         string
	Slug         string
	Description  *string
	Schema       map[string]any
	UISchema     map[string]any
	Capabilities map[string]any
	Icon         *string
	Status       string
	CreatedBy    uuid.UUID
	UpdatedBy    uuid.UUID
}

// UpdateContentTypeRequest captures mutable fields for a content type.
type UpdateContentTypeRequest struct {
	ID                   uuid.UUID
	Name                 *string
	Slug                 *string
	Description          *string
	Schema               map[string]any
	UISchema             map[string]any
	Capabilities         map[string]any
	Icon                 *string
	Status               *string
	UpdatedBy            uuid.UUID
	AllowBreakingChanges bool
}

// DeleteContentTypeRequest captures details required to delete a content type.
type DeleteContentTypeRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}

var (
	ErrContentTypeNameRequired   = errors.New("content type: name is required")
	ErrContentTypeSchemaRequired = errors.New("content type: schema is required")
	ErrContentTypeSchemaInvalid  = errors.New("content type: schema is invalid")
	ErrContentTypeIDRequired     = errors.New("content type: id required")
	ErrContentTypeSlugInvalid    = errors.New("content type: slug contains invalid characters")
	ErrContentTypeSchemaVersion  = errors.New("content type: schema version invalid")
	ErrContentTypeSchemaBreaking = errors.New("content type: schema has breaking changes")
	ErrContentTypeStatusInvalid  = errors.New("content type: status invalid")
	ErrContentTypeStatusChange   = errors.New("content type: status transition invalid")
)

// ContentTypeOption mutates the content type service.
type ContentTypeOption func(*contentTypeService)

// ContentTypeValidator runs custom validation logic for content types.
type ContentTypeValidator func(ctx context.Context, record *ContentType) error

// WithContentTypeClock overrides the clock used by the service.
func WithContentTypeClock(clock func() time.Time) ContentTypeOption {
	return func(s *contentTypeService) {
		if clock != nil {
			s.now = clock
		}
	}
}

// WithContentTypeIDGenerator overrides the ID generator used by the service.
func WithContentTypeIDGenerator(generator IDGenerator) ContentTypeOption {
	return func(s *contentTypeService) {
		if generator != nil {
			s.id = generator
		}
	}
}

// WithContentTypeSlugNormalizer overrides the slug normalizer used by the service.
func WithContentTypeSlugNormalizer(normalizer slug.Normalizer) ContentTypeOption {
	return func(s *contentTypeService) {
		if normalizer != nil {
			s.slugger = normalizer
		}
	}
}

// WithContentTypeActivityEmitter overrides the activity emitter for content types.
func WithContentTypeActivityEmitter(emitter *activity.Emitter) ContentTypeOption {
	return func(s *contentTypeService) {
		if emitter != nil {
			s.activity = emitter
		}
	}
}

// WithContentTypeValidators appends validators applied on create/update.
func WithContentTypeValidators(validators ...ContentTypeValidator) ContentTypeOption {
	return func(s *contentTypeService) {
		for _, validator := range validators {
			if validator == nil {
				continue
			}
			s.validators = append(s.validators, validator)
		}
	}
}

// NewContentTypeService constructs a content type service.
func NewContentTypeService(repo ContentTypeRepository, opts ...ContentTypeOption) ContentTypeService {
	svc := &contentTypeService{
		repo:       repo,
		now:        func() time.Time { return time.Now().UTC() },
		id:         uuid.New,
		slugger:    slug.Default(),
		activity:   activity.NewEmitter(nil, activity.Config{}),
		validators: []ContentTypeValidator{validateContentTypeSchema},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

type contentTypeService struct {
	repo       ContentTypeRepository
	now        func() time.Time
	id         IDGenerator
	slugger    slug.Normalizer
	activity   *activity.Emitter
	validators []ContentTypeValidator
}

func (s *contentTypeService) Create(ctx context.Context, req CreateContentTypeRequest) (*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	if err := permissions.Require(ctx, permissions.ContentTypesCreate); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrContentTypeNameRequired
	}
	if req.Schema == nil {
		return nil, ErrContentTypeSchemaRequired
	}

	status, err := normalizeContentTypeStatus(req.Status)
	if err != nil {
		return nil, err
	}

	record := &ContentType{
		ID:           s.id(),
		Name:         name,
		Slug:         strings.TrimSpace(req.Slug),
		Description:  req.Description,
		Schema:       cloneMap(req.Schema),
		UISchema:     cloneMap(req.UISchema),
		Capabilities: cloneMap(req.Capabilities),
		Icon:         req.Icon,
		Status:       status,
		CreatedAt:    s.now(),
		UpdatedAt:    s.now(),
	}

	record.Slug, err = s.normalizeSlug(record)
	if err != nil {
		return nil, err
	}

	if err := s.ensureSlugAvailable(ctx, record.Slug, record.ID); err != nil {
		return nil, err
	}

	normalizedSchema, version, err := schema.EnsureSchemaVersion(record.Schema, record.Slug)
	if err != nil {
		return nil, ErrContentTypeSchemaVersion
	}
	record.Schema = normalizedSchema
	record.SchemaVersion = version.String()
	record.SchemaHistory = appendSchemaHistory(record.SchemaHistory, newContentTypeSchemaSnapshot(record, pickActor(req.CreatedBy, req.UpdatedBy), record.UpdatedAt))

	if err := s.validate(ctx, record); err != nil {
		return nil, err
	}

	created, err := s.repo.Create(ctx, record)
	if err != nil {
		return nil, err
	}
	s.emitActivity(ctx, pickActor(req.CreatedBy, req.UpdatedBy), "create", created)
	return created, nil
}

func (s *contentTypeService) Update(ctx context.Context, req UpdateContentTypeRequest) (*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	if err := permissions.Require(ctx, permissions.ContentTypesUpdate); err != nil {
		return nil, err
	}
	if req.ID == uuid.Nil {
		return nil, ErrContentTypeIDRequired
	}

	record, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	previousSchema := cloneMap(record.Schema)
	previousUISchema := cloneMap(record.UISchema)
	previousCapabilities := cloneMap(record.Capabilities)
	previousStatus := record.Status
	previousVersion := record.SchemaVersion
	previousUpdatedAt := record.UpdatedAt

	if req.Name != nil {
		record.Name = strings.TrimSpace(*req.Name)
	}
	if req.Slug != nil {
		record.Slug = strings.TrimSpace(*req.Slug)
	}
	if req.Description != nil {
		record.Description = req.Description
	}
	if req.Schema != nil {
		record.Schema = cloneMap(req.Schema)
	}
	if req.UISchema != nil {
		record.UISchema = cloneMap(req.UISchema)
	}
	if req.Capabilities != nil {
		record.Capabilities = cloneMap(req.Capabilities)
	}
	if req.Icon != nil {
		record.Icon = req.Icon
	}

	record.Name = strings.TrimSpace(record.Name)
	if record.Name == "" {
		return nil, ErrContentTypeNameRequired
	}
	if record.Schema == nil {
		return nil, ErrContentTypeSchemaRequired
	}

	currentStatus, err := normalizeContentTypeStatus(record.Status)
	if err != nil {
		return nil, err
	}
	record.Status = currentStatus
	if req.Status != nil {
		nextStatus, err := normalizeContentTypeStatus(*req.Status)
		if err != nil {
			return nil, err
		}
		if nextStatus != currentStatus {
			if err := permissions.Require(ctx, permissions.ContentTypesPublish); err != nil {
				return nil, err
			}
		}
		if err := validateContentTypeStatusTransition(currentStatus, nextStatus); err != nil {
			return nil, err
		}
		record.Status = nextStatus
	}

	record.Slug, err = s.normalizeSlug(record)
	if err != nil {
		return nil, err
	}

	if err := s.ensureSlugAvailable(ctx, record.Slug, record.ID); err != nil {
		return nil, err
	}

	compatibility := schema.CompatibilityResult{Compatible: true, ChangeLevel: schema.ChangeNone}
	if req.Schema != nil {
		if _, _, err := schema.EnsureSchemaVersion(cloneMap(record.Schema), record.Slug); err != nil {
			return nil, ErrContentTypeSchemaVersion
		}
		compatibility = schema.CheckSchemaCompatibility(previousSchema, record.Schema)
		if len(compatibility.BreakingChanges) > 0 && record.Status == ContentTypeStatusActive && !req.AllowBreakingChanges {
			return nil, &schemaCompatibilityError{Result: compatibility}
		}
	}

	changeLevel := compatibility.ChangeLevel
	if changeLevel == schema.ChangeNone && !reflect.DeepEqual(previousUISchema, record.UISchema) {
		changeLevel = schema.ChangePatch
	}
	if changeLevel == schema.ChangeNone && !reflect.DeepEqual(previousCapabilities, record.Capabilities) {
		changeLevel = schema.ChangePatch
	}

	baseVersion, err := resolveContentTypeVersion(previousSchema, record.Slug, previousVersion)
	if err != nil {
		return nil, ErrContentTypeSchemaVersion
	}
	nextVersion, err := schema.BumpVersion(baseVersion, changeLevel)
	if err != nil {
		return nil, ErrContentTypeSchemaVersion
	}
	record.SchemaVersion = nextVersion.String()
	meta := schema.ExtractMetadata(record.Schema)
	meta.Slug = record.Slug
	meta.SchemaVersion = record.SchemaVersion
	record.Schema = schema.ApplyMetadata(record.Schema, meta)

	if err := s.validate(ctx, record); err != nil {
		return nil, err
	}

	record.UpdatedAt = s.now()
	if record.SchemaVersion != previousVersion {
		if len(record.SchemaHistory) == 0 && previousVersion != "" {
			record.SchemaHistory = appendSchemaHistory(record.SchemaHistory, ContentTypeSchemaSnapshot{
				Version:      previousVersion,
				Schema:       cloneMap(previousSchema),
				UISchema:     cloneMap(previousUISchema),
				Capabilities: cloneMap(previousCapabilities),
				Status:       previousStatus,
				UpdatedAt:    previousUpdatedAt,
			})
		}
		record.SchemaHistory = appendSchemaHistory(record.SchemaHistory, newContentTypeSchemaSnapshot(record, req.UpdatedBy, record.UpdatedAt))
	}
	updated, err := s.repo.Update(ctx, record)
	if err != nil {
		return nil, err
	}
	verb := "update"
	if currentStatus != ContentTypeStatusActive && updated.Status == ContentTypeStatusActive {
		verb = "publish"
	}
	s.emitActivity(ctx, req.UpdatedBy, verb, updated)
	return updated, nil
}

func (s *contentTypeService) Delete(ctx context.Context, req DeleteContentTypeRequest) error {
	if s == nil || s.repo == nil {
		return errors.New("content type service unavailable")
	}
	if err := permissions.Require(ctx, permissions.ContentTypesDelete); err != nil {
		return err
	}
	if req.ID == uuid.Nil {
		return ErrContentTypeIDRequired
	}

	if req.HardDelete {
		record, err := s.repo.GetByID(ctx, req.ID)
		if err != nil {
			var notFound *NotFoundError
			if errors.As(err, &notFound) {
				if err := s.repo.Delete(ctx, req.ID, true); err != nil {
					return err
				}
				s.emitActivity(ctx, pickActor(req.DeletedBy), "delete", &ContentType{ID: req.ID})
				return nil
			}
			return err
		}
		if err := s.repo.Delete(ctx, req.ID, true); err != nil {
			return err
		}
		s.emitActivity(ctx, pickActor(req.DeletedBy), "delete", record)
		return nil
	}

	record, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		return err
	}

	now := s.now()
	record.DeletedAt = &now
	record.Status = ContentTypeStatusDeprecated
	record.UpdatedAt = now
	if err := s.repo.Delete(ctx, req.ID, false); err != nil {
		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			return err
		}
	}
	s.emitActivity(ctx, pickActor(req.DeletedBy), "delete", record)
	return nil
}

func (s *contentTypeService) Get(ctx context.Context, id uuid.UUID) (*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	if err := permissions.Require(ctx, permissions.ContentTypesRead); err != nil {
		return nil, err
	}
	if id == uuid.Nil {
		return nil, ErrContentTypeIDRequired
	}
	return s.repo.GetByID(ctx, id)
}

func (s *contentTypeService) GetBySlug(ctx context.Context, rawSlug string) (*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	if err := permissions.Require(ctx, permissions.ContentTypesRead); err != nil {
		return nil, err
	}
	slugValue := strings.TrimSpace(rawSlug)
	if slugValue == "" {
		return nil, ErrContentTypeSlugRequired
	}
	if s.slugger == nil {
		s.slugger = slug.Default()
	}
	normalized, err := s.slugger.Normalize(slugValue)
	if err != nil || normalized == "" {
		return nil, ErrContentTypeSlugInvalid
	}
	slugValue = normalized
	return s.repo.GetBySlug(ctx, slugValue)
}

func (s *contentTypeService) List(ctx context.Context) ([]*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	if err := permissions.Require(ctx, permissions.ContentTypesRead); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *contentTypeService) Search(ctx context.Context, query string) ([]*ContentType, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("content type service unavailable")
	}
	if err := permissions.Require(ctx, permissions.ContentTypesRead); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return s.repo.List(ctx)
	}
	return s.repo.Search(ctx, query)
}

func (s *contentTypeService) emitActivity(ctx context.Context, actor uuid.UUID, verb string, record *ContentType) {
	if s.activity == nil || !s.activity.Enabled() || record == nil || record.ID == uuid.Nil {
		return
	}
	meta := map[string]any{
		"slug":           record.Slug,
		"status":         record.Status,
		"schema_version": record.SchemaVersion,
		"name":           record.Name,
	}
	_ = s.activity.Emit(ctx, activity.Event{
		Verb:       verb,
		ActorID:    actor.String(),
		ObjectType: "content_type",
		ObjectID:   record.ID.String(),
		Metadata:   meta,
	})
}

func (s *contentTypeService) validate(ctx context.Context, record *ContentType) error {
	if len(s.validators) == 0 {
		return nil
	}
	for _, validator := range s.validators {
		if validator == nil {
			continue
		}
		if err := validator(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (s *contentTypeService) normalizeSlug(ct *ContentType) (string, error) {
	if ct == nil {
		return "", ErrContentTypeSlugRequired
	}
	if s.slugger == nil {
		s.slugger = slug.Default()
	}
	candidate := strings.TrimSpace(ct.Slug)
	if candidate == "" {
		candidate = strings.TrimSpace(extractSchemaSlug(ct.Schema))
	}
	if candidate == "" {
		candidate = strings.TrimSpace(ct.Name)
	}
	if candidate == "" {
		return "", ErrContentTypeSlugRequired
	}
	normalized, err := s.slugger.Normalize(candidate)
	if err != nil || normalized == "" {
		return "", ErrContentTypeSlugRequired
	}
	if !slug.IsValid(normalized) {
		return "", ErrContentTypeSlugInvalid
	}
	return normalized, nil
}

func (s *contentTypeService) ensureSlugAvailable(ctx context.Context, slug string, currentID uuid.UUID) error {
	existing, err := s.repo.GetBySlug(ctx, slug)
	if err == nil && existing != nil {
		if existing.ID != currentID {
			return ErrContentTypeSlugExists
		}
		return nil
	}
	var notFound *NotFoundError
	if err != nil && !errors.As(err, &notFound) {
		return err
	}
	return nil
}

func normalizeContentTypeStatus(value string) (string, error) {
	status := strings.ToLower(strings.TrimSpace(value))
	if status == "" {
		return ContentTypeStatusDraft, nil
	}
	switch status {
	case ContentTypeStatusDraft, ContentTypeStatusActive, ContentTypeStatusDeprecated:
		return status, nil
	default:
		return "", ErrContentTypeStatusInvalid
	}
}

func validateContentTypeStatusTransition(fromStatus, toStatus string) error {
	if fromStatus == toStatus {
		return nil
	}
	switch fromStatus {
	case ContentTypeStatusDraft:
		if toStatus == ContentTypeStatusActive || toStatus == ContentTypeStatusDeprecated {
			return nil
		}
	case ContentTypeStatusActive:
		if toStatus == ContentTypeStatusDeprecated {
			return nil
		}
	case ContentTypeStatusDeprecated:
		if toStatus == ContentTypeStatusActive {
			return nil
		}
	}
	return ErrContentTypeStatusChange
}

func validateContentTypeSchema(_ context.Context, record *ContentType) error {
	if record == nil {
		return nil
	}
	if err := validation.ValidateSchema(record.Schema); err != nil {
		return fmt.Errorf("%w: %s", ErrContentTypeSchemaInvalid, err)
	}
	return nil
}

type schemaCompatibilityError struct {
	Result schema.CompatibilityResult
}

func (e *schemaCompatibilityError) Error() string {
	if e == nil || len(e.Result.BreakingChanges) == 0 {
		return ErrContentTypeSchemaBreaking.Error()
	}
	parts := make([]string, 0, len(e.Result.BreakingChanges))
	for _, change := range e.Result.BreakingChanges {
		field := strings.TrimSpace(change.Field)
		if field == "" {
			parts = append(parts, change.Type)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s", change.Type, field))
	}
	return fmt.Sprintf("%s: %s", ErrContentTypeSchemaBreaking.Error(), strings.Join(parts, ", "))
}

func (e *schemaCompatibilityError) Unwrap() error {
	return ErrContentTypeSchemaBreaking
}

func resolveContentTypeVersion(payload map[string]any, slug string, storedVersion string) (schema.Version, error) {
	if strings.TrimSpace(storedVersion) != "" {
		version, err := schema.ParseVersion(storedVersion)
		if err != nil {
			return schema.Version{}, err
		}
		if trimmed := strings.TrimSpace(slug); trimmed != "" {
			version.Slug = trimmed
		}
		return version, nil
	}
	_, version, err := schema.EnsureSchemaVersion(payload, slug)
	if err != nil {
		return schema.Version{}, err
	}
	return version, nil
}
