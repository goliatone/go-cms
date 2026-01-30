package blocks

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/identity"
	"github.com/goliatone/go-cms/internal/media"
	cmsschema "github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-cms/internal/translationconfig"
	"github.com/goliatone/go-cms/internal/validation"
	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-slug"
	"github.com/google/uuid"
)

type Service interface {
	RegisterDefinition(ctx context.Context, input RegisterDefinitionInput) (*Definition, error)
	GetDefinition(ctx context.Context, id uuid.UUID) (*Definition, error)
	ListDefinitions(ctx context.Context) ([]*Definition, error)
	UpdateDefinition(ctx context.Context, input UpdateDefinitionInput) (*Definition, error)
	DeleteDefinition(ctx context.Context, req DeleteDefinitionRequest) error
	SyncRegistry(ctx context.Context) error
	CreateDefinitionVersion(ctx context.Context, input CreateDefinitionVersionInput) (*DefinitionVersion, error)
	GetDefinitionVersion(ctx context.Context, definitionID uuid.UUID, version string) (*DefinitionVersion, error)
	ListDefinitionVersions(ctx context.Context, definitionID uuid.UUID) ([]*DefinitionVersion, error)

	CreateInstance(ctx context.Context, input CreateInstanceInput) (*Instance, error)
	ListPageInstances(ctx context.Context, pageID uuid.UUID) ([]*Instance, error)
	ListGlobalInstances(ctx context.Context) ([]*Instance, error)
	UpdateInstance(ctx context.Context, input UpdateInstanceInput) (*Instance, error)
	DeleteInstance(ctx context.Context, req DeleteInstanceRequest) error

	AddTranslation(ctx context.Context, input AddTranslationInput) (*Translation, error)
	UpdateTranslation(ctx context.Context, input UpdateTranslationInput) (*Translation, error)
	DeleteTranslation(ctx context.Context, req DeleteTranslationRequest) error

	GetTranslation(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error)
	CreateDraft(ctx context.Context, req CreateInstanceDraftRequest) (*InstanceVersion, error)
	PublishDraft(ctx context.Context, req PublishInstanceDraftRequest) (*InstanceVersion, error)
	ListVersions(ctx context.Context, instanceID uuid.UUID) ([]*InstanceVersion, error)
	RestoreVersion(ctx context.Context, req RestoreInstanceVersionRequest) (*InstanceVersion, error)
}

type RegisterDefinitionInput struct {
	Name             string
	Slug             string
	Description      *string
	Icon             *string
	Category         *string
	Status           string
	Schema           map[string]any
	UISchema         map[string]any
	Defaults         map[string]any
	EditorStyleURL   *string
	FrontendStyleURL *string
}

type UpdateDefinitionInput struct {
	ID               uuid.UUID
	Name             *string
	Slug             *string
	Description      *string
	Icon             *string
	Category         *string
	Status           *string
	Schema           map[string]any
	UISchema         map[string]any
	Defaults         map[string]any
	EditorStyleURL   *string
	FrontendStyleURL *string
}

// CreateDefinitionVersionInput captures schema version updates for a definition.
type CreateDefinitionVersionInput struct {
	DefinitionID uuid.UUID
	Schema       map[string]any
	Defaults     map[string]any
}

type DeleteDefinitionRequest struct {
	ID         uuid.UUID
	HardDelete bool
}

type CreateInstanceInput struct {
	DefinitionID  uuid.UUID
	PageID        *uuid.UUID
	Region        string
	Position      int
	Configuration map[string]any
	IsGlobal      bool
	CreatedBy     uuid.UUID
	UpdatedBy     uuid.UUID
}

type UpdateInstanceInput struct {
	InstanceID    uuid.UUID
	PageID        *uuid.UUID
	Region        *string
	Position      *int
	Configuration map[string]any
	IsGlobal      *bool
	UpdatedBy     uuid.UUID
}

type DeleteInstanceRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}

type AddTranslationInput struct {
	BlockInstanceID    uuid.UUID
	LocaleID           uuid.UUID
	Content            map[string]any
	AttributeOverrides map[string]any
	MediaBindings      media.BindingSet
}

type UpdateTranslationInput struct {
	BlockInstanceID    uuid.UUID
	LocaleID           uuid.UUID
	Content            map[string]any
	AttributeOverrides map[string]any
	MediaBindings      media.BindingSet
	UpdatedBy          uuid.UUID
}

type DeleteTranslationRequest struct {
	BlockInstanceID          uuid.UUID
	LocaleID                 uuid.UUID
	DeletedBy                uuid.UUID
	AllowMissingTranslations bool
}

// CreateInstanceDraftRequest captures the payload required to create a block instance draft snapshot.
type CreateInstanceDraftRequest struct {
	InstanceID  uuid.UUID
	Snapshot    BlockVersionSnapshot
	CreatedBy   uuid.UUID
	UpdatedBy   uuid.UUID
	BaseVersion *int
}

// PublishInstanceDraftRequest captures details required to publish a block instance draft.
type PublishInstanceDraftRequest struct {
	InstanceID  uuid.UUID
	Version     int
	PublishedBy uuid.UUID
	PublishedAt *time.Time
}

// RestoreInstanceVersionRequest captures the request to restore a previously recorded version.
type RestoreInstanceVersionRequest struct {
	InstanceID uuid.UUID
	Version    int
	RestoredBy uuid.UUID
}

var (
	ErrDefinitionNameRequired          = errors.New("blocks: definition name required")
	ErrDefinitionSlugRequired          = errors.New("blocks: definition slug required")
	ErrDefinitionSlugInvalid           = errors.New("blocks: definition slug invalid")
	ErrDefinitionSlugExists            = errors.New("blocks: definition slug already exists")
	ErrDefinitionSchemaRequired        = errors.New("blocks: definition schema required")
	ErrDefinitionSchemaInvalid         = errors.New("blocks: definition schema invalid")
	ErrDefinitionSchemaVersionInvalid  = errors.New("blocks: definition schema version invalid")
	ErrDefinitionExists                = errors.New("blocks: definition already exists")
	ErrDefinitionIDRequired            = errors.New("blocks: definition id required")
	ErrDefinitionInUse                 = errors.New("blocks: definition has active instances")
	ErrDefinitionSoftDeleteUnsupported = errors.New("blocks: soft delete not supported for definitions")
	ErrDefinitionVersionRequired       = errors.New("blocks: definition version required")
	ErrDefinitionVersionExists         = errors.New("blocks: definition version already exists")
	ErrDefinitionVersioningDisabled    = errors.New("blocks: definition versioning disabled")

	ErrInstanceDefinitionRequired    = errors.New("blocks: definition id required")
	ErrInstanceRegionRequired        = errors.New("blocks: region required")
	ErrInstancePositionInvalid       = errors.New("blocks: position cannot be negative")
	ErrInstanceUpdaterRequired       = errors.New("blocks: updated_by is required")
	ErrInstanceSoftDeleteUnsupported = errors.New("blocks: soft delete not supported for instances")

	ErrTranslationContentRequired       = errors.New("blocks: translation content required")
	ErrTranslationExists                = errors.New("blocks: translation already exists for locale")
	ErrTranslationLocaleRequired        = errors.New("blocks: translation locale required")
	ErrTranslationSchemaInvalid         = errors.New("blocks: translation content invalid")
	ErrTranslationNotFound              = errors.New("blocks: translation not found")
	ErrTranslationMinimum               = errors.New("blocks: at least one translation is required")
	ErrTranslationsDisabled             = errors.New("blocks: translations feature disabled")
	ErrInstanceIDRequired               = errors.New("blocks: instance id required")
	ErrVersioningDisabled               = errors.New("blocks: versioning feature disabled")
	ErrInstanceVersionRequired          = errors.New("blocks: version identifier required")
	ErrInstanceVersionConflict          = errors.New("blocks: base version mismatch")
	ErrInstanceVersionAlreadyPublished  = errors.New("blocks: version already published")
	ErrInstanceVersionRetentionExceeded = errors.New("blocks: version retention limit reached")
	ErrMediaReferenceRequired           = errors.New("blocks: media reference requires id or path")
	ErrBlockSchemaMigrationRequired     = errors.New("blocks: schema migration required")
	ErrBlockSchemaValidationFailed      = errors.New("blocks: schema validation failed")
)

type IDGenerator func() uuid.UUID

type ServiceOption func(*service)

func WithClock(clock func() time.Time) ServiceOption {
	return func(s *service) {
		if clock != nil {
			s.now = clock
		}
	}
}

func WithIDGenerator(generator IDGenerator) ServiceOption {
	return func(s *service) {
		if generator != nil {
			s.id = generator
			s.idCustom = true
		}
	}
}

func WithRegistry(reg *Registry) ServiceOption {
	return func(s *service) {
		if reg != nil {
			s.registry = reg
		}
	}
}

// WithDefinitionVersionRepository wires the repository used for block definition version persistence.
func WithDefinitionVersionRepository(repo DefinitionVersionRepository) ServiceOption {
	return func(s *service) {
		if repo != nil {
			s.definitionVersions = repo
		}
	}
}

// WithSchemaMigrator wires the migration runner used for block schema upgrades.
func WithSchemaMigrator(migrator *Migrator) ServiceOption {
	return func(s *service) {
		if migrator != nil {
			s.schemaMigrator = migrator
		}
	}
}

// WithMediaService wires the media resolution helper used to enrich translations.
func WithMediaService(mediaSvc media.Service) ServiceOption {
	return func(s *service) {
		if mediaSvc != nil {
			s.media = mediaSvc
		}
	}
}

// WithInstanceVersionRepository wires the repository used for instance version persistence.
func WithInstanceVersionRepository(repo InstanceVersionRepository) ServiceOption {
	return func(s *service) {
		s.versions = repo
	}
}

// WithShortcodeService wires the shortcode renderer used to process translation content.
func WithShortcodeService(svc interfaces.ShortcodeService) ServiceOption {
	return func(s *service) {
		s.shortcodes = svc
	}
}

// WithActivityEmitter wires the activity emitter used for activity records.
func WithActivityEmitter(emitter *activity.Emitter) ServiceOption {
	return func(s *service) {
		if emitter != nil {
			s.activity = emitter
		}
	}
}

// WithVersioningEnabled toggles versioning workflows for block instances.
func WithVersioningEnabled(enabled bool) ServiceOption {
	return func(s *service) {
		s.versioningEnabled = enabled
	}
}

// WithVersionRetentionLimit constrains how many versions are retained per instance.
func WithVersionRetentionLimit(limit int) ServiceOption {
	return func(s *service) {
		if limit < 0 {
			limit = 0
		}
		s.versionRetentionLimit = limit
	}
}

// WithRequireTranslations toggles whether at least one translation must remain attached to an instance.
func WithRequireTranslations(required bool) ServiceOption {
	return func(s *service) {
		s.requireTranslations = required
	}
}

// WithTranslationsEnabled toggles translation handling entirely.
func WithTranslationsEnabled(enabled bool) ServiceOption {
	return func(s *service) {
		s.translationsEnabled = enabled
	}
}

// WithTranslationState wires a shared, runtime-configurable translation state.
func WithTranslationState(state *translationconfig.State) ServiceOption {
	return func(s *service) {
		s.translationState = state
	}
}

// WithDefinitionSlugNormalizer overrides the slug normalizer used by block definitions.
func WithDefinitionSlugNormalizer(normalizer slug.Normalizer) ServiceOption {
	return func(s *service) {
		if normalizer != nil {
			s.slugger = normalizer
		}
	}
}

type service struct {
	definitions           DefinitionRepository
	definitionVersions    DefinitionVersionRepository
	instances             InstanceRepository
	translations          TranslationRepository
	versions              InstanceVersionRepository
	now                   func() time.Time
	id                    IDGenerator
	idCustom              bool
	registry              *Registry
	schemaMigrator        *Migrator
	media                 media.Service
	versioningEnabled     bool
	versionRetentionLimit int
	shortcodes            interfaces.ShortcodeService
	requireTranslations   bool
	translationsEnabled   bool
	translationState      *translationconfig.State
	slugger               slug.Normalizer
	activity              *activity.Emitter
}

func NewService(defRepo DefinitionRepository, instRepo InstanceRepository, trRepo TranslationRepository, opts ...ServiceOption) Service {
	s := &service{
		definitions:         defRepo,
		instances:           instRepo,
		translations:        trRepo,
		now:                 time.Now,
		id:                  uuid.New,
		media:               media.NewNoOpService(),
		translationsEnabled: true,
		slugger:             slug.Default(),
		activity:            activity.NewEmitter(nil, activity.Config{}),
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.schemaMigrator == nil && s.registry != nil {
		s.schemaMigrator = s.registry.Migrator()
	}

	if s.registry != nil {
		s.applyRegistry(context.Background())
	}

	return s
}

func (s *service) translationsRequired() bool {
	enabled := s.translationsEnabled
	required := s.requireTranslations
	if s.translationState != nil {
		enabled = s.translationState.Enabled()
		required = s.translationState.RequireTranslations()
	}
	return enabled && required
}

func (s *service) translationsEnabledFlag() bool {
	if s.translationState != nil {
		return s.translationState.Enabled()
	}
	return s.translationsEnabled
}

func (s *service) emitActivity(ctx context.Context, actor uuid.UUID, verb, objectType string, objectID uuid.UUID, meta map[string]any) {
	if s.activity == nil || !s.activity.Enabled() || objectID == uuid.Nil {
		return
	}
	event := activity.Event{
		Verb:       verb,
		ActorID:    actor.String(),
		ObjectType: objectType,
		ObjectID:   objectID.String(),
		Metadata:   meta,
	}
	_ = s.activity.Emit(ctx, event)
}

func (s *service) RegisterDefinition(ctx context.Context, input RegisterDefinitionInput) (*Definition, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrDefinitionNameRequired
	}
	if len(input.Schema) == 0 {
		return nil, ErrDefinitionSchemaRequired
	}
	slugValue, err := s.normalizeDefinitionSlug(name, input.Slug)
	if err != nil {
		return nil, err
	}
	if err := s.ensureDefinitionSlugAvailable(ctx, slugValue, uuid.Nil); err != nil {
		return nil, err
	}
	normalizedSchema, version, err := cmsschema.EnsureSchemaVersion(input.Schema, slugValue)
	if err != nil {
		return nil, ErrDefinitionSchemaVersionInvalid
	}
	if err := validation.ValidateSchema(normalizedSchema); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDefinitionSchemaInvalid, err)
	}

	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "draft"
	}

	definition := &Definition{
		ID:               identity.BlockDefinitionUUID(slugValue),
		Name:             name,
		Slug:             slugValue,
		Description:      input.Description,
		Icon:             input.Icon,
		Category:         cloneStringValue(input.Category),
		Status:           status,
		Schema:           maps.Clone(normalizedSchema),
		UISchema:         maps.Clone(input.UISchema),
		SchemaVersion:    version.String(),
		Defaults:         maps.Clone(input.Defaults),
		EditorStyleURL:   input.EditorStyleURL,
		FrontendStyleURL: input.FrontendStyleURL,
		CreatedAt:        s.now(),
	}
	if s.idCustom {
		definition.ID = s.id()
	}

	created, err := s.definitions.Create(ctx, definition)
	if err != nil {
		return nil, err
	}
	if s.definitionVersions != nil {
		if _, err := s.upsertDefinitionVersion(ctx, created, normalizedSchema, input.Defaults, version, true); err != nil {
			return nil, err
		}
	}
	return created, nil
}

func (s *service) GetDefinition(ctx context.Context, id uuid.UUID) (*Definition, error) {
	return s.definitions.GetByID(ctx, id)
}

func (s *service) ListDefinitions(ctx context.Context) ([]*Definition, error) {
	return s.definitions.List(ctx)
}

func (s *service) UpdateDefinition(ctx context.Context, input UpdateDefinitionInput) (*Definition, error) {
	if input.ID == uuid.Nil {
		return nil, ErrDefinitionIDRequired
	}
	definition, err := s.definitions.GetByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	schemaUpdated := false
	defaultsUpdated := false

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, ErrDefinitionNameRequired
		}
		definition.Name = name
	}

	if input.Slug != nil {
		slugValue, err := s.normalizeDefinitionSlug(definition.Name, *input.Slug)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(slugValue, definition.Slug) {
			if err := s.ensureDefinitionSlugAvailable(ctx, slugValue, definition.ID); err != nil {
				return nil, err
			}
		}
		definition.Slug = slugValue
	} else if strings.TrimSpace(definition.Slug) == "" {
		slugValue, err := s.normalizeDefinitionSlug(definition.Name, definition.Slug)
		if err != nil {
			return nil, err
		}
		if err := s.ensureDefinitionSlugAvailable(ctx, slugValue, definition.ID); err != nil {
			return nil, err
		}
		definition.Slug = slugValue
	}
	if input.Description != nil {
		definition.Description = cloneStringValue(input.Description)
	}
	if input.Icon != nil {
		definition.Icon = cloneStringValue(input.Icon)
	}
	if input.Schema != nil {
		if len(input.Schema) == 0 {
			return nil, ErrDefinitionSchemaRequired
		}
		definition.Schema = maps.Clone(input.Schema)
		schemaUpdated = true
	}
	if input.Defaults != nil {
		definition.Defaults = maps.Clone(input.Defaults)
		defaultsUpdated = true
	}
	if input.EditorStyleURL != nil {
		definition.EditorStyleURL = cloneStringValue(input.EditorStyleURL)
	}
	if input.FrontendStyleURL != nil {
		definition.FrontendStyleURL = cloneStringValue(input.FrontendStyleURL)
	}
	if definition.Schema == nil {
		return nil, ErrDefinitionSchemaRequired
	}
	normalizedSchema, version, err := cmsschema.EnsureSchemaVersion(definition.Schema, definition.Name)
	if err != nil {
		return nil, ErrDefinitionSchemaVersionInvalid
	}
	if err := validation.ValidateSchema(normalizedSchema); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDefinitionSchemaInvalid, err)
	}
	definition.Schema = normalizedSchema
	definition.SchemaVersion = version.String()
	definition.UpdatedAt = s.now()

	updated, err := s.definitions.Update(ctx, definition)
	if err != nil {
		return nil, err
	}
	if s.definitionVersions != nil && (schemaUpdated || defaultsUpdated) {
		if _, err := s.upsertDefinitionVersion(ctx, updated, normalizedSchema, updated.Defaults, version, false); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

func (s *service) DeleteDefinition(ctx context.Context, req DeleteDefinitionRequest) error {
	if req.ID == uuid.Nil {
		return ErrDefinitionIDRequired
	}
	if !req.HardDelete {
		return ErrDefinitionSoftDeleteUnsupported
	}

	instances, err := s.instances.ListByDefinition(ctx, req.ID)
	if err != nil {
		return err
	}
	if len(instances) > 0 {
		return ErrDefinitionInUse
	}
	return s.definitions.Delete(ctx, req.ID)
}

func (s *service) CreateDefinitionVersion(ctx context.Context, input CreateDefinitionVersionInput) (*DefinitionVersion, error) {
	if input.DefinitionID == uuid.Nil {
		return nil, ErrDefinitionIDRequired
	}
	if input.Schema == nil {
		return nil, ErrDefinitionSchemaRequired
	}
	definition, err := s.definitions.GetByID(ctx, input.DefinitionID)
	if err != nil {
		return nil, err
	}
	normalizedSchema, version, err := cmsschema.EnsureSchemaVersion(input.Schema, definition.Name)
	if err != nil {
		return nil, ErrDefinitionSchemaVersionInvalid
	}
	if err := validation.ValidateSchema(normalizedSchema); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrDefinitionSchemaInvalid, err)
	}
	if s.definitionVersions == nil {
		return nil, ErrDefinitionVersioningDisabled
	}
	if _, err := s.definitionVersions.GetByDefinitionAndVersion(ctx, definition.ID, version.String()); err == nil {
		return nil, ErrDefinitionVersionExists
	} else {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
	}
	created, err := s.upsertDefinitionVersion(ctx, definition, normalizedSchema, input.Defaults, version, true)
	if err != nil {
		return nil, err
	}
	if compareSchemaVersions(version.String(), definition.SchemaVersion) >= 0 {
		definition.Schema = normalizedSchema
		definition.Defaults = maps.Clone(input.Defaults)
		definition.SchemaVersion = version.String()
		definition.UpdatedAt = s.now()
		if _, err := s.definitions.Update(ctx, definition); err != nil {
			return nil, err
		}
	}
	return created, nil
}

func (s *service) GetDefinitionVersion(ctx context.Context, definitionID uuid.UUID, version string) (*DefinitionVersion, error) {
	if definitionID == uuid.Nil {
		return nil, ErrDefinitionIDRequired
	}
	if strings.TrimSpace(version) == "" {
		return nil, ErrDefinitionVersionRequired
	}
	if s.definitionVersions == nil {
		return nil, ErrDefinitionVersioningDisabled
	}
	return s.definitionVersions.GetByDefinitionAndVersion(ctx, definitionID, strings.TrimSpace(version))
}

func (s *service) ListDefinitionVersions(ctx context.Context, definitionID uuid.UUID) ([]*DefinitionVersion, error) {
	if definitionID == uuid.Nil {
		return nil, ErrDefinitionIDRequired
	}
	if s.definitionVersions == nil {
		return nil, ErrDefinitionVersioningDisabled
	}
	return s.definitionVersions.ListByDefinition(ctx, definitionID)
}

func (s *service) CreateInstance(ctx context.Context, input CreateInstanceInput) (*Instance, error) {
	if input.DefinitionID == uuid.Nil {
		return nil, ErrInstanceDefinitionRequired
	}

	if strings.TrimSpace(input.Region) == "" {
		return nil, ErrInstanceRegionRequired
	}

	if input.Position < 0 {
		return nil, ErrInstancePositionInvalid
	}

	if _, err := s.definitions.GetByID(ctx, input.DefinitionID); err != nil {
		return nil, err
	}

	instance := &Instance{
		ID:            s.id(),
		DefinitionID:  input.DefinitionID,
		Region:        strings.TrimSpace(input.Region),
		Position:      input.Position,
		Configuration: maps.Clone(input.Configuration),
		IsGlobal:      input.IsGlobal,
		CreatedBy:     input.CreatedBy,
		UpdatedBy:     input.UpdatedBy,
		CreatedAt:     s.now(),
		UpdatedAt:     s.now(),
	}

	if input.PageID != nil {
		clone := *input.PageID
		instance.PageID = &clone
	}

	created, err := s.instances.Create(ctx, instance)
	if err != nil {
		return nil, err
	}

	s.emitActivity(ctx, pickActor(input.CreatedBy, input.UpdatedBy), "create", "block_instance", created.ID, map[string]any{
		"region":    created.Region,
		"position":  created.Position,
		"page_id":   created.PageID,
		"is_global": created.IsGlobal,
	})

	return created, nil
}

func (s *service) ListPageInstances(ctx context.Context, pageID uuid.UUID) ([]*Instance, error) {
	instances, err := s.instances.ListByPage(ctx, pageID)
	if err != nil {
		return nil, err
	}
	return s.attachTranslations(ctx, instances)
}

func (s *service) ListGlobalInstances(ctx context.Context) ([]*Instance, error) {
	instances, err := s.instances.ListGlobal(ctx)
	if err != nil {
		return nil, err
	}
	return s.attachTranslations(ctx, instances)
}

func (s *service) UpdateInstance(ctx context.Context, input UpdateInstanceInput) (*Instance, error) {
	if input.InstanceID == uuid.Nil {
		return nil, ErrInstanceIDRequired
	}
	if input.UpdatedBy == uuid.Nil {
		return nil, ErrInstanceUpdaterRequired
	}

	instance, err := s.instances.GetByID(ctx, input.InstanceID)
	if err != nil {
		return nil, err
	}
	originalRegion := instance.Region
	originalPosition := instance.Position

	if input.PageID != nil {
		if *input.PageID == uuid.Nil {
			instance.PageID = nil
		} else {
			pageID := *input.PageID
			instance.PageID = &pageID
		}
	}
	if input.Region != nil {
		region := strings.TrimSpace(*input.Region)
		if region == "" {
			return nil, ErrInstanceRegionRequired
		}
		instance.Region = region
	}
	if input.Position != nil {
		if *input.Position < 0 {
			return nil, ErrInstancePositionInvalid
		}
		instance.Position = *input.Position
	}
	if input.Configuration != nil {
		instance.Configuration = maps.Clone(input.Configuration)
	}
	if input.IsGlobal != nil {
		instance.IsGlobal = *input.IsGlobal
		if *input.IsGlobal {
			instance.PageID = nil
		}
	}

	now := s.now()
	instance.UpdatedBy = input.UpdatedBy
	instance.UpdatedAt = now

	preparedVersion, err := s.prepareInstanceVersion(ctx, instance, input.UpdatedBy, now)
	if err != nil {
		return nil, err
	}

	updated, err := s.instances.Update(ctx, instance)
	if err != nil {
		return nil, err
	}

	if err := s.persistVersion(ctx, preparedVersion); err != nil {
		return nil, err
	}

	withTranslations, err := s.attachTranslations(ctx, []*Instance{updated})
	if err != nil {
		return nil, err
	}
	if len(withTranslations) == 0 {
		return updated, nil
	}
	enriched := withTranslations[0]

	verb := "update"
	if originalRegion != updated.Region || originalPosition != updated.Position {
		verb = "reorder"
	}
	s.emitActivity(ctx, input.UpdatedBy, verb, "block_instance", updated.ID, map[string]any{
		"region":    updated.Region,
		"position":  updated.Position,
		"page_id":   updated.PageID,
		"is_global": updated.IsGlobal,
	})

	return enriched, nil
}

func (s *service) DeleteInstance(ctx context.Context, req DeleteInstanceRequest) error {
	if req.ID == uuid.Nil {
		return ErrInstanceIDRequired
	}
	if !req.HardDelete {
		return ErrInstanceSoftDeleteUnsupported
	}

	record, err := s.instances.GetByID(ctx, req.ID)
	if err != nil {
		return err
	}

	translations, err := s.translations.ListByInstance(ctx, req.ID)
	if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return err
		}
	}
	for _, tr := range translations {
		if err := s.translations.Delete(ctx, tr.ID); err != nil {
			return err
		}
	}
	if err := s.instances.Delete(ctx, req.ID); err != nil {
		return err
	}
	s.emitActivity(ctx, pickActor(req.DeletedBy, record.UpdatedBy, record.CreatedBy), "delete", "block_instance", record.ID, map[string]any{
		"region":    record.Region,
		"position":  record.Position,
		"page_id":   record.PageID,
		"is_global": record.IsGlobal,
	})
	return nil
}

func (s *service) AddTranslation(ctx context.Context, input AddTranslationInput) (*Translation, error) {
	if !s.translationsEnabledFlag() {
		return nil, ErrTranslationsDisabled
	}
	if input.Content == nil {
		return nil, ErrTranslationContentRequired
	}
	if input.LocaleID == uuid.Nil {
		return nil, ErrTranslationLocaleRequired
	}
	if err := validateMediaBindings(input.MediaBindings); err != nil {
		return nil, err
	}

	instance, err := s.instances.GetByID(ctx, input.BlockInstanceID)
	if err != nil {
		return nil, err
	}
	definition, err := s.definitions.GetByID(ctx, instance.DefinitionID)
	if err != nil {
		return nil, err
	}
	normalizedContent, err := s.prepareTranslationPayload(definition, input.Content)
	if err != nil {
		return nil, err
	}

	if existing, err := s.translations.GetByInstanceAndLocale(ctx, input.BlockInstanceID, input.LocaleID); err == nil && existing != nil {
		return nil, ErrTranslationExists
	} else if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
	}

	translation := &Translation{
		ID:                s.id(),
		BlockInstanceID:   input.BlockInstanceID,
		LocaleID:          input.LocaleID,
		Content:           maps.Clone(normalizedContent),
		AttributeOverride: maps.Clone(input.AttributeOverrides),
		MediaBindings:     media.CloneBindingSet(input.MediaBindings),
		CreatedAt:         s.now(),
		UpdatedAt:         s.now(),
	}

	created, err := s.translations.Create(ctx, translation)
	if err != nil {
		return nil, err
	}
	instance.UpdatedAt = translation.UpdatedAt

	preparedVersion, err := s.prepareInstanceVersion(ctx, instance, uuid.Nil, translation.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if _, err := s.instances.Update(ctx, instance); err != nil {
		return nil, err
	}
	if err := s.persistVersion(ctx, preparedVersion); err != nil {
		return nil, err
	}

	return s.hydrateTranslation(ctx, created)
}

func (s *service) GetTranslation(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error) {
	record, err := s.translations.GetByInstanceAndLocale(ctx, instanceID, localeID)
	if err != nil {
		return nil, err
	}
	return s.hydrateTranslation(ctx, record)
}

func (s *service) UpdateTranslation(ctx context.Context, input UpdateTranslationInput) (*Translation, error) {
	if !s.translationsEnabledFlag() {
		return nil, ErrTranslationsDisabled
	}
	if input.BlockInstanceID == uuid.Nil {
		return nil, ErrInstanceIDRequired
	}
	if input.LocaleID == uuid.Nil {
		return nil, ErrTranslationLocaleRequired
	}
	if input.Content == nil {
		return nil, ErrTranslationContentRequired
	}
	if input.UpdatedBy == uuid.Nil {
		return nil, ErrInstanceUpdaterRequired
	}
	if input.MediaBindings != nil {
		if err := validateMediaBindings(input.MediaBindings); err != nil {
			return nil, err
		}
	}

	instance, err := s.instances.GetByID(ctx, input.BlockInstanceID)
	if err != nil {
		return nil, err
	}
	definition, err := s.definitions.GetByID(ctx, instance.DefinitionID)
	if err != nil {
		return nil, err
	}
	normalizedContent, err := s.prepareTranslationPayload(definition, input.Content)
	if err != nil {
		return nil, err
	}

	translation, err := s.translations.GetByInstanceAndLocale(ctx, input.BlockInstanceID, input.LocaleID)
	if err != nil {
		return nil, err
	}

	if input.AttributeOverrides != nil {
		translation.AttributeOverride = maps.Clone(input.AttributeOverrides)
	}
	if input.MediaBindings != nil {
		translation.MediaBindings = media.CloneBindingSet(input.MediaBindings)
	}
	translation.Content = maps.Clone(normalizedContent)
	translation.UpdatedAt = s.now()

	updated, err := s.translations.Update(ctx, translation)
	if err != nil {
		return nil, err
	}

	instance.UpdatedBy = input.UpdatedBy
	instance.UpdatedAt = translation.UpdatedAt

	preparedVersion, err := s.prepareInstanceVersion(ctx, instance, input.UpdatedBy, translation.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if _, err := s.instances.Update(ctx, instance); err != nil {
		return nil, err
	}
	if err := s.persistVersion(ctx, preparedVersion); err != nil {
		return nil, err
	}

	record, err := s.hydrateTranslation(ctx, updated)
	if err != nil {
		return nil, err
	}

	s.emitActivity(ctx, input.UpdatedBy, "update", "block_translation", record.ID, map[string]any{
		"instance_id": input.BlockInstanceID.String(),
		"locale_id":   input.LocaleID.String(),
	})

	return record, nil
}

func (s *service) DeleteTranslation(ctx context.Context, req DeleteTranslationRequest) error {
	if !s.translationsEnabledFlag() {
		return ErrTranslationsDisabled
	}
	if req.BlockInstanceID == uuid.Nil {
		return ErrInstanceIDRequired
	}
	if req.LocaleID == uuid.Nil {
		return ErrTranslationLocaleRequired
	}

	translations, err := s.translations.ListByInstance(ctx, req.BlockInstanceID)
	if err != nil {
		return err
	}

	var target *Translation
	for _, tr := range translations {
		if tr.LocaleID == req.LocaleID {
			target = tr
			break
		}
	}
	if target == nil {
		return ErrTranslationNotFound
	}
	if s.translationsRequired() && !req.AllowMissingTranslations && len(translations) <= 1 {
		return ErrTranslationMinimum
	}

	if err := s.translations.Delete(ctx, target.ID); err != nil {
		return err
	}

	instance, err := s.instances.GetByID(ctx, req.BlockInstanceID)
	if err != nil {
		return err
	}
	if req.DeletedBy != uuid.Nil {
		instance.UpdatedBy = req.DeletedBy
	}
	now := s.now()
	instance.UpdatedAt = now

	preparedVersion, err := s.prepareInstanceVersion(ctx, instance, req.DeletedBy, now)
	if err != nil {
		return err
	}
	if _, err := s.instances.Update(ctx, instance); err != nil {
		return err
	}
	if err := s.persistVersion(ctx, preparedVersion); err != nil {
		return err
	}
	s.emitActivity(ctx, pickActor(req.DeletedBy, instance.UpdatedBy, instance.CreatedBy), "delete", "block_translation", target.ID, map[string]any{
		"instance_id": req.BlockInstanceID.String(),
		"locale_id":   req.LocaleID.String(),
	})
	return nil
}

func (s *service) CreateDraft(ctx context.Context, req CreateInstanceDraftRequest) (*InstanceVersion, error) {
	if !s.versioningEnabled || s.versions == nil {
		return nil, ErrVersioningDisabled
	}
	if req.InstanceID == uuid.Nil {
		return nil, ErrInstanceIDRequired
	}

	instance, err := s.instances.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}

	versions, err := s.versions.ListByInstance(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}

	if s.versionRetentionLimit > 0 && len(versions) >= s.versionRetentionLimit {
		return nil, ErrInstanceVersionRetentionExceeded
	}

	next := nextInstanceVersionNumber(versions)
	if req.BaseVersion != nil && *req.BaseVersion != next-1 {
		return nil, ErrInstanceVersionConflict
	}

	now := s.now()
	version := &InstanceVersion{
		ID:              s.id(),
		BlockInstanceID: req.InstanceID,
		Version:         next,
		Status:          domain.StatusDraft,
		Snapshot:        cloneBlockVersionSnapshot(req.Snapshot),
		CreatedBy:       req.CreatedBy,
		CreatedAt:       now,
	}

	created, err := s.versions.Create(ctx, version)
	if err != nil {
		return nil, err
	}

	instance.CurrentVersion = created.Version
	instance.UpdatedAt = now
	switch {
	case req.UpdatedBy != uuid.Nil:
		instance.UpdatedBy = req.UpdatedBy
	case req.CreatedBy != uuid.Nil:
		instance.UpdatedBy = req.CreatedBy
	}

	if _, err := s.instances.Update(ctx, instance); err != nil {
		return nil, err
	}

	return cloneInstanceVersion(created), nil
}

func (s *service) PublishDraft(ctx context.Context, req PublishInstanceDraftRequest) (*InstanceVersion, error) {
	if !s.versioningEnabled || s.versions == nil {
		return nil, ErrVersioningDisabled
	}
	if req.InstanceID == uuid.Nil {
		return nil, ErrInstanceIDRequired
	}
	if req.Version <= 0 {
		return nil, ErrInstanceVersionRequired
	}

	instance, err := s.instances.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}

	version, err := s.versions.GetVersion(ctx, req.InstanceID, req.Version)
	if err != nil {
		return nil, err
	}
	if version.Status == domain.StatusPublished {
		return nil, ErrInstanceVersionAlreadyPublished
	}

	if definition, defErr := s.definitions.GetByID(ctx, instance.DefinitionID); defErr == nil {
		migrated, migratedAny, err := s.migrateSnapshot(definition, version.Snapshot)
		if err != nil {
			return nil, err
		}
		if migratedAny {
			if err := s.validateSnapshot(definition, migrated); err != nil {
				return nil, err
			}
			version.Snapshot = migrated
		}
	} else if defErr != nil {
		return nil, defErr
	}

	publishedAt := s.now()
	if req.PublishedAt != nil && !req.PublishedAt.IsZero() {
		publishedAt = *req.PublishedAt
	}

	version.Status = domain.StatusPublished
	version.PublishedAt = &publishedAt
	if req.PublishedBy != uuid.Nil {
		version.PublishedBy = &req.PublishedBy
	}

	updatedVersion, err := s.versions.Update(ctx, version)
	if err != nil {
		return nil, err
	}

	if instance.PublishedVersion != nil && *instance.PublishedVersion != updatedVersion.Version {
		previous, prevErr := s.versions.GetVersion(ctx, req.InstanceID, *instance.PublishedVersion)
		if prevErr == nil && previous.Status == domain.StatusPublished {
			previous.Status = domain.StatusArchived
			if _, archiveErr := s.versions.Update(ctx, previous); archiveErr != nil {
				return nil, archiveErr
			}
		}
	}

	instance.PublishedVersion = &updatedVersion.Version
	instance.PublishedAt = &publishedAt
	if req.PublishedBy != uuid.Nil {
		instance.PublishedBy = &req.PublishedBy
	}
	if updatedVersion.Version > instance.CurrentVersion {
		instance.CurrentVersion = updatedVersion.Version
	}
	instance.UpdatedAt = s.now()
	if req.PublishedBy != uuid.Nil {
		instance.UpdatedBy = req.PublishedBy
	}

	if _, err := s.instances.Update(ctx, instance); err != nil {
		return nil, err
	}

	s.emitActivity(ctx, req.PublishedBy, "publish", "block_instance", instance.ID, map[string]any{
		"version":   updatedVersion.Version,
		"status":    updatedVersion.Status,
		"page_id":   instance.PageID,
		"region":    instance.Region,
		"is_global": instance.IsGlobal,
	})

	return cloneInstanceVersion(updatedVersion), nil
}

func (s *service) ListVersions(ctx context.Context, instanceID uuid.UUID) ([]*InstanceVersion, error) {
	if !s.versioningEnabled || s.versions == nil {
		return nil, ErrVersioningDisabled
	}
	if instanceID == uuid.Nil {
		return nil, ErrInstanceIDRequired
	}

	versions, err := s.versions.ListByInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return cloneInstanceVersions(versions), nil
}

func (s *service) RestoreVersion(ctx context.Context, req RestoreInstanceVersionRequest) (*InstanceVersion, error) {
	if !s.versioningEnabled || s.versions == nil {
		return nil, ErrVersioningDisabled
	}
	if req.InstanceID == uuid.Nil {
		return nil, ErrInstanceIDRequired
	}
	if req.Version <= 0 {
		return nil, ErrInstanceVersionRequired
	}

	version, err := s.versions.GetVersion(ctx, req.InstanceID, req.Version)
	if err != nil {
		return nil, err
	}

	return s.CreateDraft(ctx, CreateInstanceDraftRequest{
		InstanceID: req.InstanceID,
		Snapshot:   cloneBlockVersionSnapshot(version.Snapshot),
		CreatedBy:  req.RestoredBy,
		UpdatedBy:  req.RestoredBy,
	})
}

func nextInstanceVersionNumber(records []*InstanceVersion) int {
	max := 0
	for _, version := range records {
		if version == nil {
			continue
		}
		if version.Version > max {
			max = version.Version
		}
	}
	return max + 1
}

func pickActor(ids ...uuid.UUID) uuid.UUID {
	for _, id := range ids {
		if id != uuid.Nil {
			return id
		}
	}
	return uuid.Nil
}

func (s *service) attachTranslations(ctx context.Context, instances []*Instance) ([]*Instance, error) {
	enriched := make([]*Instance, 0, len(instances))
	for _, inst := range instances {
		clone := *inst
		records, err := s.translations.ListByInstance(ctx, inst.ID)
		if err != nil {
			var nf *NotFoundError
			if !errors.As(err, &nf) {
				return nil, err
			}
		}
		if len(records) > 0 {
			hydrated := make([]*Translation, 0, len(records))
			for _, record := range records {
				translation, err := s.hydrateTranslation(ctx, record)
				if err != nil {
					return nil, err
				}
				hydrated = append(hydrated, translation)
			}
			clone.Translations = hydrated
		}
		enriched = append(enriched, &clone)
	}
	return enriched, nil
}

func (s *service) prepareInstanceVersion(ctx context.Context, instance *Instance, actor uuid.UUID, timestamp time.Time) (*InstanceVersion, error) {
	if !s.versioningEnabled || s.versions == nil {
		return nil, nil
	}
	records, err := s.versions.ListByInstance(ctx, instance.ID)
	if err != nil {
		return nil, err
	}
	if s.versionRetentionLimit > 0 && len(records) >= s.versionRetentionLimit {
		return nil, ErrInstanceVersionRetentionExceeded
	}

	snapshot, err := s.buildInstanceSnapshot(ctx, instance)
	if err != nil {
		return nil, err
	}

	next := nextInstanceVersionNumber(records)
	instance.CurrentVersion = next

	version := &InstanceVersion{
		ID:              s.id(),
		BlockInstanceID: instance.ID,
		Version:         next,
		Status:          domain.StatusDraft,
		Snapshot:        snapshot,
		CreatedAt:       timestamp,
		CreatedBy:       actor,
	}
	if version.CreatedBy == uuid.Nil {
		version.CreatedBy = instance.UpdatedBy
	}
	return version, nil
}

func (s *service) persistVersion(ctx context.Context, version *InstanceVersion) error {
	if version == nil || s.versions == nil {
		return nil
	}
	_, err := s.versions.Create(ctx, version)
	return err
}

func (s *service) buildInstanceSnapshot(ctx context.Context, instance *Instance) (BlockVersionSnapshot, error) {
	snapshot := BlockVersionSnapshot{
		Configuration: maps.Clone(instance.Configuration),
	}
	translations, err := s.translations.ListByInstance(ctx, instance.ID)
	if err != nil {
		return BlockVersionSnapshot{}, err
	}
	if len(translations) > 0 {
		snaps := make([]BlockVersionTranslationSnapshot, 0, len(translations))
		for _, tr := range translations {
			snaps = append(snaps, BlockVersionTranslationSnapshot{
				Locale:             tr.LocaleID.String(),
				Content:            maps.Clone(tr.Content),
				AttributeOverrides: maps.Clone(tr.AttributeOverride),
			})
		}
		snapshot.Translations = snaps
	}
	return snapshot, nil
}

func (s *service) prepareTranslationPayload(definition *Definition, content map[string]any) (map[string]any, error) {
	if definition == nil || definition.Schema == nil {
		return nil, ErrDefinitionSchemaRequired
	}
	version, err := resolveDefinitionSchemaVersion(definition.Schema, definition.Name)
	if err != nil {
		return nil, ErrDefinitionSchemaVersionInvalid
	}
	clean := stripSchemaVersion(content)
	if err := validation.ValidatePayload(definition.Schema, clean); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrTranslationSchemaInvalid, err)
	}
	return applySchemaVersion(clean, version), nil
}

func (s *service) migrateSnapshot(definition *Definition, snapshot BlockVersionSnapshot) (BlockVersionSnapshot, bool, error) {
	if definition == nil {
		return snapshot, false, nil
	}
	if len(snapshot.Translations) == 0 {
		return snapshot, false, nil
	}
	target, err := resolveDefinitionSchemaVersion(definition.Schema, definition.Name)
	if err != nil {
		return snapshot, false, ErrDefinitionSchemaVersionInvalid
	}
	updated := cloneBlockVersionSnapshot(snapshot)
	migratedAny := false
	for idx, tr := range updated.Translations {
		migrated, didMigrate, err := s.migratePayload(definition.Name, target, tr.Content)
		if err != nil {
			return snapshot, false, err
		}
		if didMigrate {
			migratedAny = true
		}
		updated.Translations[idx].Content = migrated
	}
	return updated, migratedAny, nil
}

func (s *service) migratePayload(slug string, target cmsschema.Version, payload map[string]any) (map[string]any, bool, error) {
	current, ok := cmsschema.RootSchemaVersion(payload)
	if !ok || current.String() == target.String() {
		return applySchemaVersion(stripSchemaVersion(payload), target), false, nil
	}
	if s.schemaMigrator == nil {
		return nil, false, ErrBlockSchemaMigrationRequired
	}
	if current.Slug != "" && current.Slug != target.Slug {
		return nil, false, ErrDefinitionSchemaVersionInvalid
	}
	trimmed := stripSchemaVersion(payload)
	migrated, err := s.schemaMigrator.Migrate(strings.TrimSpace(slug), current.String(), target.String(), trimmed)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %v", ErrBlockSchemaMigrationRequired, err)
	}
	return applySchemaVersion(migrated, target), true, nil
}

func (s *service) validateSnapshot(definition *Definition, snapshot BlockVersionSnapshot) error {
	if definition == nil || definition.Schema == nil {
		return nil
	}
	for _, tr := range snapshot.Translations {
		if tr.Content == nil {
			continue
		}
		clean := stripSchemaVersion(tr.Content)
		if err := validation.ValidatePayload(definition.Schema, clean); err != nil {
			return fmt.Errorf("%w: %s", ErrBlockSchemaValidationFailed, err)
		}
	}
	return nil
}

func (s *service) upsertDefinitionVersion(ctx context.Context, definition *Definition, schema map[string]any, defaults map[string]any, version cmsschema.Version, createOnly bool) (*DefinitionVersion, error) {
	if s.definitionVersions == nil {
		return nil, ErrDefinitionVersioningDisabled
	}
	if definition == nil || definition.ID == uuid.Nil {
		return nil, ErrDefinitionIDRequired
	}
	if strings.TrimSpace(version.String()) == "" {
		return nil, ErrDefinitionVersionRequired
	}

	existing, err := s.definitionVersions.GetByDefinitionAndVersion(ctx, definition.ID, version.String())
	if err == nil {
		if createOnly {
			return nil, ErrDefinitionVersionExists
		}
		existing.SchemaVersion = version.String()
		existing.Schema = maps.Clone(schema)
		existing.Defaults = maps.Clone(defaults)
		existing.UpdatedAt = s.now()
		return s.definitionVersions.Update(ctx, existing)
	}
	var nf *NotFoundError
	if !errors.As(err, &nf) && err != nil {
		return nil, err
	}

	record := &DefinitionVersion{
		ID:            s.id(),
		DefinitionID:  definition.ID,
		SchemaVersion: version.String(),
		Schema:        maps.Clone(schema),
		Defaults:      maps.Clone(defaults),
		CreatedAt:     s.now(),
		UpdatedAt:     s.now(),
	}
	return s.definitionVersions.Create(ctx, record)
}

func validateMediaBindings(bindings media.BindingSet) error {
	for slot, entries := range bindings {
		for _, binding := range entries {
			reference := binding.Reference
			if strings.TrimSpace(reference.ID) == "" && strings.TrimSpace(reference.Path) == "" {
				return fmt.Errorf("%w: %s", ErrMediaReferenceRequired, slot)
			}
		}
	}
	return nil
}

func (s *service) hydrateTranslation(ctx context.Context, translation *Translation) (*Translation, error) {
	if translation == nil {
		return nil, nil
	}
	clone := *translation
	if translation.Content != nil {
		clone.Content = maps.Clone(translation.Content)
		if s.shortcodes != nil {
			if err := s.renderShortcodesInMap(ctx, clone.Content, ""); err != nil {
				return nil, err
			}
		}
	}
	if translation.AttributeOverride != nil {
		clone.AttributeOverride = maps.Clone(translation.AttributeOverride)
		if s.shortcodes != nil {
			if err := s.renderShortcodesInMap(ctx, clone.AttributeOverride, ""); err != nil {
				return nil, err
			}
		}
	}
	clone.MediaBindings = media.CloneBindingSet(translation.MediaBindings)
	if len(clone.MediaBindings) == 0 {
		clone.ResolvedMedia = nil
		return &clone, nil
	}
	resolved, err := s.media.ResolveBindings(ctx, clone.MediaBindings, media.ResolveOptions{})
	if err != nil {
		return nil, err
	}
	clone.ResolvedMedia = resolved
	return &clone, nil
}

func (s *service) renderShortcodesInMap(ctx context.Context, values map[string]any, locale string) error {
	if values == nil || s.shortcodes == nil {
		return nil
	}
	for key, value := range values {
		rendered, err := s.renderShortcodeValue(ctx, value, locale)
		if err != nil {
			return err
		}
		values[key] = rendered
	}
	return nil
}

func (s *service) renderShortcodeValue(ctx context.Context, value any, locale string) (any, error) {
	if s.shortcodes == nil {
		return value, nil
	}

	switch v := value.(type) {
	case string:
		if !containsShortcodeSyntax(v) {
			return v, nil
		}
		output, err := s.shortcodes.Process(ctx, v, interfaces.ShortcodeProcessOptions{Locale: locale})
		if err != nil {
			return nil, err
		}
		return output, nil
	case []any:
		if len(v) == 0 {
			return v, nil
		}
		out := make([]any, len(v))
		for i, item := range v {
			rendered, err := s.renderShortcodeValue(ctx, item, locale)
			if err != nil {
				return nil, err
			}
			out[i] = rendered
		}
		return out, nil
	case []string:
		out := make([]string, len(v))
		for i, item := range v {
			rendered, err := s.renderShortcodeValue(ctx, item, locale)
			if err != nil {
				return nil, err
			}
			if str, ok := rendered.(string); ok {
				out[i] = str
			} else {
				out[i] = item
			}
		}
		return out, nil
	case map[string]any:
		if err := s.renderShortcodesInMap(ctx, v, locale); err != nil {
			return nil, err
		}
		return v, nil
	default:
		return value, nil
	}
}

func containsShortcodeSyntax(input string) bool {
	return strings.Contains(input, "{{<") || strings.Contains(input, "{{%") || strings.Contains(input, "[")
}

func cloneStringValue(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := strings.Clone(*value)
	return &cloned
}

func definitionSlug(definition *Definition) string {
	if definition == nil {
		return ""
	}
	if trimmed := strings.TrimSpace(definition.Slug); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(definition.Name)
}

func (s *service) normalizeDefinitionSlug(name, rawSlug string) (string, error) {
	candidate := strings.TrimSpace(rawSlug)
	if candidate == "" {
		candidate = strings.TrimSpace(name)
	}
	if candidate == "" {
		return "", ErrDefinitionSlugRequired
	}
	if s.slugger == nil {
		s.slugger = slug.Default()
	}
	normalized, err := s.slugger.Normalize(candidate)
	if err != nil || normalized == "" {
		return "", ErrDefinitionSlugInvalid
	}
	return normalized, nil
}

func (s *service) ensureDefinitionSlugAvailable(ctx context.Context, slug string, currentID uuid.UUID) error {
	if slug == "" {
		return ErrDefinitionSlugRequired
	}
	existing, err := s.definitions.GetBySlug(ctx, slug)
	if err != nil {
		var nf *NotFoundError
		if errors.As(err, &nf) {
			return nil
		}
		return err
	}
	if existing == nil {
		return nil
	}
	if currentID != uuid.Nil && existing.ID == currentID {
		return nil
	}
	return ErrDefinitionSlugExists
}

func (s *service) SyncRegistry(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.applyRegistry(ctx)
	return nil
}

func (s *service) applyRegistry(ctx context.Context) {
	if s.registry == nil {
		return
	}
	for _, def := range s.registry.List() {
		existing, err := s.definitions.GetByName(ctx, def.Name)
		if err != nil {
			_, _ = s.RegisterDefinition(ctx, def)
			continue
		}
		targetVersion, err := resolveDefinitionSchemaVersion(def.Schema, def.Name)
		if err != nil {
			continue
		}
		currentVersion, err := resolveDefinitionSchemaVersion(existing.Schema, existing.Name)
		if err != nil {
			currentVersion = cmsschema.DefaultVersion(existing.Name)
		}
		if compareSchemaVersions(targetVersion.String(), currentVersion.String()) > 0 {
			_, _ = s.UpdateDefinition(ctx, UpdateDefinitionInput{
				ID:               existing.ID,
				Schema:           def.Schema,
				Defaults:         def.Defaults,
				Description:      def.Description,
				Icon:             def.Icon,
				EditorStyleURL:   def.EditorStyleURL,
				FrontendStyleURL: def.FrontendStyleURL,
			})
		}
	}
	if s.definitionVersions == nil {
		return
	}
	for _, def := range s.registry.ListAllVersions() {
		definition, err := s.definitions.GetByName(ctx, def.Name)
		if err != nil {
			continue
		}
		normalizedSchema, version, err := cmsschema.EnsureSchemaVersion(def.Schema, def.Name)
		if err != nil {
			continue
		}
		_, _ = s.upsertDefinitionVersion(ctx, definition, normalizedSchema, def.Defaults, version, false)
	}
}
