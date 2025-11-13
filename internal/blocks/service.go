package blocks

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type Service interface {
	RegisterDefinition(ctx context.Context, input RegisterDefinitionInput) (*Definition, error)
	GetDefinition(ctx context.Context, id uuid.UUID) (*Definition, error)
	ListDefinitions(ctx context.Context) ([]*Definition, error)
	UpdateDefinition(ctx context.Context, input UpdateDefinitionInput) (*Definition, error)
	DeleteDefinition(ctx context.Context, req DeleteDefinitionRequest) error
	SyncRegistry(ctx context.Context) error

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
	Description      *string
	Icon             *string
	Schema           map[string]any
	Defaults         map[string]any
	EditorStyleURL   *string
	FrontendStyleURL *string
}

type UpdateDefinitionInput struct {
	ID               uuid.UUID
	Name             *string
	Description      *string
	Icon             *string
	Schema           map[string]any
	Defaults         map[string]any
	EditorStyleURL   *string
	FrontendStyleURL *string
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
	ErrDefinitionSchemaRequired        = errors.New("blocks: definition schema required")
	ErrDefinitionExists                = errors.New("blocks: definition already exists")
	ErrDefinitionIDRequired            = errors.New("blocks: definition id required")
	ErrDefinitionInUse                 = errors.New("blocks: definition has active instances")
	ErrDefinitionSoftDeleteUnsupported = errors.New("blocks: soft delete not supported for definitions")

	ErrInstanceDefinitionRequired    = errors.New("blocks: definition id required")
	ErrInstanceRegionRequired        = errors.New("blocks: region required")
	ErrInstancePositionInvalid       = errors.New("blocks: position cannot be negative")
	ErrInstanceUpdaterRequired       = errors.New("blocks: updated_by is required")
	ErrInstanceSoftDeleteUnsupported = errors.New("blocks: soft delete not supported for instances")

	ErrTranslationContentRequired       = errors.New("blocks: translation content required")
	ErrTranslationExists                = errors.New("blocks: translation already exists for locale")
	ErrTranslationLocaleRequired        = errors.New("blocks: translation locale required")
	ErrTranslationNotFound              = errors.New("blocks: translation not found")
	ErrTranslationMinimum               = errors.New("blocks: at least one translation is required")
	ErrInstanceIDRequired               = errors.New("blocks: instance id required")
	ErrVersioningDisabled               = errors.New("blocks: versioning feature disabled")
	ErrInstanceVersionRequired          = errors.New("blocks: version identifier required")
	ErrInstanceVersionConflict          = errors.New("blocks: base version mismatch")
	ErrInstanceVersionAlreadyPublished  = errors.New("blocks: version already published")
	ErrInstanceVersionRetentionExceeded = errors.New("blocks: version retention limit reached")
	ErrMediaReferenceRequired           = errors.New("blocks: media reference requires id or path")
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

type service struct {
	definitions           DefinitionRepository
	instances             InstanceRepository
	translations          TranslationRepository
	versions              InstanceVersionRepository
	now                   func() time.Time
	id                    IDGenerator
	registry              *Registry
	media                 media.Service
	versioningEnabled     bool
	versionRetentionLimit int
	shortcodes            interfaces.ShortcodeService
	requireTranslations   bool
}

func NewService(defRepo DefinitionRepository, instRepo InstanceRepository, trRepo TranslationRepository, opts ...ServiceOption) Service {
	s := &service{
		definitions:  defRepo,
		instances:    instRepo,
		translations: trRepo,
		now:          time.Now,
		id:           uuid.New,
		media:        media.NewNoOpService(),
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.registry != nil {
		s.applyRegistry(context.Background())
	}

	return s
}

func (s *service) RegisterDefinition(ctx context.Context, input RegisterDefinitionInput) (*Definition, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrDefinitionNameRequired
	}
	if len(input.Schema) == 0 {
		return nil, ErrDefinitionSchemaRequired
	}

	if _, err := s.definitions.GetByName(ctx, name); err == nil {
		return nil, ErrDefinitionExists
	}

	definition := &Definition{
		ID:               s.id(),
		Name:             name,
		Description:      input.Description,
		Icon:             input.Icon,
		Schema:           maps.Clone(input.Schema),
		Defaults:         maps.Clone(input.Defaults),
		EditorStyleURL:   input.EditorStyleURL,
		FrontendStyleURL: input.FrontendStyleURL,
		CreatedAt:        s.now(),
	}

	return s.definitions.Create(ctx, definition)
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

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, ErrDefinitionNameRequired
		}
		if !strings.EqualFold(name, definition.Name) {
			if _, err := s.definitions.GetByName(ctx, name); err == nil {
				return nil, ErrDefinitionExists
			} else {
				var nf *NotFoundError
				if !errors.As(err, &nf) {
					return nil, err
				}
			}
		}
		definition.Name = name
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
	}
	if input.Defaults != nil {
		definition.Defaults = maps.Clone(input.Defaults)
	}
	if input.EditorStyleURL != nil {
		definition.EditorStyleURL = cloneStringValue(input.EditorStyleURL)
	}
	if input.FrontendStyleURL != nil {
		definition.FrontendStyleURL = cloneStringValue(input.FrontendStyleURL)
	}
	definition.UpdatedAt = s.now()

	return s.definitions.Update(ctx, definition)
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

	return s.instances.Create(ctx, instance)
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
	return withTranslations[0], nil
}

func (s *service) DeleteInstance(ctx context.Context, req DeleteInstanceRequest) error {
	if req.ID == uuid.Nil {
		return ErrInstanceIDRequired
	}
	if !req.HardDelete {
		return ErrInstanceSoftDeleteUnsupported
	}

	if _, err := s.instances.GetByID(ctx, req.ID); err != nil {
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
	return s.instances.Delete(ctx, req.ID)
}

func (s *service) AddTranslation(ctx context.Context, input AddTranslationInput) (*Translation, error) {
	if input.Content == nil {
		return nil, ErrTranslationContentRequired
	}
	if input.LocaleID == uuid.Nil {
		return nil, ErrTranslationLocaleRequired
	}
	if err := validateMediaBindings(input.MediaBindings); err != nil {
		return nil, err
	}

	if _, err := s.instances.GetByID(ctx, input.BlockInstanceID); err != nil {
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
		Content:           maps.Clone(input.Content),
		AttributeOverride: maps.Clone(input.AttributeOverrides),
		MediaBindings:     media.CloneBindingSet(input.MediaBindings),
		CreatedAt:         s.now(),
		UpdatedAt:         s.now(),
	}

	created, err := s.translations.Create(ctx, translation)
	if err != nil {
		return nil, err
	}
	instance, err := s.instances.GetByID(ctx, input.BlockInstanceID)
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
	translation.Content = maps.Clone(input.Content)
	translation.UpdatedAt = s.now()

	updated, err := s.translations.Update(ctx, translation)
	if err != nil {
		return nil, err
	}

	instance, err := s.instances.GetByID(ctx, input.BlockInstanceID)
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

	return s.hydrateTranslation(ctx, updated)
}

func (s *service) DeleteTranslation(ctx context.Context, req DeleteTranslationRequest) error {
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
	if s.requireTranslations && !req.AllowMissingTranslations && len(translations) <= 1 {
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
	return s.persistVersion(ctx, preparedVersion)
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

func (s *service) SyncRegistry(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.applyRegistry(ctx)
	return nil
}

func (s *service) applyRegistry(ctx context.Context) {
	for _, def := range s.registry.List() {
		if _, err := s.definitions.GetByName(ctx, def.Name); err == nil {
			continue
		}
		_, _ = s.RegisterDefinition(ctx, def)
	}
}

type Registry struct {
	entries map[string]RegisterDefinitionInput
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]RegisterDefinitionInput)}
}

func (r *Registry) Register(input RegisterDefinitionInput) {
	if r.entries == nil {
		r.entries = make(map[string]RegisterDefinitionInput)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return
	}
	r.entries[name] = input
}

func (r *Registry) List() []RegisterDefinitionInput {
	out := make([]RegisterDefinitionInput, 0, len(r.entries))
	for _, entry := range r.entries {
		out = append(out, entry)
	}
	return out
}
