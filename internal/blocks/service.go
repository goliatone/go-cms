package blocks

import (
	"context"
	"errors"
	"maps"
	"strings"
	"time"

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
}

var (
	ErrDefinitionNameRequired   = errors.New("blocks: definition name required")
	ErrDefinitionSchemaRequired = errors.New("blocks: definition schema required")
	ErrDefinitionExists         = errors.New("blocks: definition already exists")

	ErrInstanceDefinitionRequired = errors.New("blocks: definition id required")
	ErrInstanceRegionRequired     = errors.New("blocks: region required")
	ErrInstancePositionInvalid    = errors.New("blocks: position cannot be negative")

	ErrTranslationContentRequired = errors.New("blocks: translation content required")
	ErrTranslationExists          = errors.New("blocks: translation already exists for locale")
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

type service struct {
	definitions  DefinitionRepository
	instances    InstanceRepository
	translations TranslationRepository
	now          func() time.Time
	id           IDGenerator
	registry     *Registry
}

func NewService(defRepo DefinitionRepository, instRepo InstanceRepository, trRepo TranslationRepository, opts ...ServiceOption) Service {
	s := &service{
		definitions:  defRepo,
		instances:    instRepo,
		translations: trRepo,
		now:          time.Now,
		id:           uuid.New,
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
		CreatedAt:         s.now(),
		UpdatedAt:         s.now(),
	}

	return s.translations.Create(ctx, translation)
}

func (s *service) GetTranslation(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error) {
	return s.translations.GetByInstanceAndLocale(ctx, instanceID, localeID)
}

func (s *service) attachTranslations(ctx context.Context, instances []*Instance) ([]*Instance, error) {
	enriched := make([]*Instance, 0, len(instances))
	for _, inst := range instances {
		clone := *inst
		translations, err := s.translations.ListByInstance(ctx, inst.ID)
		if err != nil {
			var nf *NotFoundError
			if !errors.As(err, &nf) {
				return nil, err
			}
		}
		clone.Translations = translations
		enriched = append(enriched, &clone)
	}
	return enriched, nil
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
