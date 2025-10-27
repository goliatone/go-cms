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
	"github.com/google/uuid"
)

type Service interface {
	RegisterDefinition(ctx context.Context, input RegisterDefinitionInput) (*Definition, error)
	GetDefinition(ctx context.Context, id uuid.UUID) (*Definition, error)
	ListDefinitions(ctx context.Context) ([]*Definition, error)

	CreateInstance(ctx context.Context, input CreateInstanceInput) (*Instance, error)
	ListPageInstances(ctx context.Context, pageID uuid.UUID) ([]*Instance, error)
	ListGlobalInstances(ctx context.Context) ([]*Instance, error)

	AddTranslation(ctx context.Context, input AddTranslationInput) (*Translation, error)
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

type AddTranslationInput struct {
	BlockInstanceID    uuid.UUID
	LocaleID           uuid.UUID
	Content            map[string]any
	AttributeOverrides map[string]any
	MediaBindings      media.BindingSet
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
	ErrDefinitionNameRequired   = errors.New("blocks: definition name required")
	ErrDefinitionSchemaRequired = errors.New("blocks: definition schema required")
	ErrDefinitionExists         = errors.New("blocks: definition already exists")

	ErrInstanceDefinitionRequired = errors.New("blocks: definition id required")
	ErrInstanceRegionRequired     = errors.New("blocks: region required")
	ErrInstancePositionInvalid    = errors.New("blocks: position cannot be negative")

	ErrTranslationContentRequired       = errors.New("blocks: translation content required")
	ErrTranslationExists                = errors.New("blocks: translation already exists for locale")
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

func (s *service) AddTranslation(ctx context.Context, input AddTranslationInput) (*Translation, error) {
	if input.Content == nil {
		return nil, ErrTranslationContentRequired
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
	return s.hydrateTranslation(ctx, created)
}

func (s *service) GetTranslation(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error) {
	record, err := s.translations.GetByInstanceAndLocale(ctx, instanceID, localeID)
	if err != nil {
		return nil, err
	}
	return s.hydrateTranslation(ctx, record)
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
	}
	if translation.AttributeOverride != nil {
		clone.AttributeOverride = maps.Clone(translation.AttributeOverride)
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
