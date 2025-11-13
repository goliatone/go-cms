package widgets

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// Service exposes widget management capabilities.
type Service interface {
	RegisterDefinition(ctx context.Context, input RegisterDefinitionInput) (*Definition, error)
	GetDefinition(ctx context.Context, id uuid.UUID) (*Definition, error)
	ListDefinitions(ctx context.Context) ([]*Definition, error)
	DeleteDefinition(ctx context.Context, req DeleteDefinitionRequest) error
	SyncRegistry(ctx context.Context) error

	CreateInstance(ctx context.Context, input CreateInstanceInput) (*Instance, error)
	UpdateInstance(ctx context.Context, input UpdateInstanceInput) (*Instance, error)
	GetInstance(ctx context.Context, id uuid.UUID) (*Instance, error)
	ListInstancesByDefinition(ctx context.Context, definitionID uuid.UUID) ([]*Instance, error)
	ListInstancesByArea(ctx context.Context, areaCode string) ([]*Instance, error)
	ListAllInstances(ctx context.Context) ([]*Instance, error)
	DeleteInstance(ctx context.Context, req DeleteInstanceRequest) error

	AddTranslation(ctx context.Context, input AddTranslationInput) (*Translation, error)
	UpdateTranslation(ctx context.Context, input UpdateTranslationInput) (*Translation, error)
	GetTranslation(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error)
	DeleteTranslation(ctx context.Context, req DeleteTranslationRequest) error

	RegisterAreaDefinition(ctx context.Context, input RegisterAreaDefinitionInput) (*AreaDefinition, error)
	ListAreaDefinitions(ctx context.Context) ([]*AreaDefinition, error)
	AssignWidgetToArea(ctx context.Context, input AssignWidgetToAreaInput) ([]*AreaPlacement, error)
	RemoveWidgetFromArea(ctx context.Context, input RemoveWidgetFromAreaInput) error
	ReorderAreaWidgets(ctx context.Context, input ReorderAreaWidgetsInput) ([]*AreaPlacement, error)
	ResolveArea(ctx context.Context, input ResolveAreaInput) ([]*ResolvedWidget, error)
	EvaluateVisibility(ctx context.Context, instance *Instance, input VisibilityContext) (bool, error)
}

// RegisterDefinitionInput captures the information required to register a widget definition.
type RegisterDefinitionInput struct {
	Name        string
	Description *string
	Schema      map[string]any
	Defaults    map[string]any
	Category    *string
	Icon        *string
}

type DeleteDefinitionRequest struct {
	ID         uuid.UUID
	HardDelete bool
}

// CreateInstanceInput defines the payload required to create a widget instance.
type CreateInstanceInput struct {
	DefinitionID    uuid.UUID
	BlockInstanceID *uuid.UUID
	AreaCode        *string
	Placement       map[string]any
	Configuration   map[string]any
	VisibilityRules map[string]any
	PublishOn       *time.Time
	UnpublishOn     *time.Time
	Position        int
	CreatedBy       uuid.UUID
	UpdatedBy       uuid.UUID
}

// UpdateInstanceInput defines mutable fields for a widget instance.
type UpdateInstanceInput struct {
	InstanceID      uuid.UUID
	Configuration   map[string]any
	VisibilityRules map[string]any
	Placement       map[string]any
	PublishOn       *time.Time
	UnpublishOn     *time.Time
	Position        *int
	UpdatedBy       uuid.UUID
	AreaCode        *string
}

type DeleteInstanceRequest struct {
	InstanceID uuid.UUID
	HardDelete bool
}

// AddTranslationInput describes the payload to add localized widget content.
type AddTranslationInput struct {
	InstanceID uuid.UUID
	LocaleID   uuid.UUID
	Content    map[string]any
}

// UpdateTranslationInput updates the localized widget content.
type UpdateTranslationInput struct {
	InstanceID uuid.UUID
	LocaleID   uuid.UUID
	Content    map[string]any
}

type DeleteTranslationRequest struct {
	InstanceID uuid.UUID
	LocaleID   uuid.UUID
}

// RegisterAreaDefinitionInput captures metadata for a widget area.
type RegisterAreaDefinitionInput struct {
	Code        string
	Name        string
	Description *string
	Scope       AreaScope
	ThemeID     *uuid.UUID
	TemplateID  *uuid.UUID
}

// AssignWidgetToAreaInput describes how to bind a widget instance to an area.
type AssignWidgetToAreaInput struct {
	AreaCode   string
	LocaleID   *uuid.UUID
	InstanceID uuid.UUID
	Position   *int
	Metadata   map[string]any
}

// RemoveWidgetFromAreaInput removes a widget instance from an area/locale combination.
type RemoveWidgetFromAreaInput struct {
	AreaCode   string
	LocaleID   *uuid.UUID
	InstanceID uuid.UUID
}

// ReorderAreaWidgetsInput updates ordering for widget placements within an area.
type ReorderAreaWidgetsInput struct {
	AreaCode string
	LocaleID *uuid.UUID
	Items    []AreaWidgetOrder
}

// AreaWidgetOrder describes the desired order for a placement.
type AreaWidgetOrder struct {
	PlacementID uuid.UUID
	Position    int
}

// ResolveAreaInput controls how area widgets are resolved for rendering.
type ResolveAreaInput struct {
	AreaCode          string
	LocaleID          *uuid.UUID
	FallbackLocaleIDs []uuid.UUID
	Audience          []string
	Segments          []string
	Now               time.Time
}

// ResolvedWidget pairs a widget instance with its placement metadata.
type ResolvedWidget struct {
	Instance  *Instance      `json:"instance"`
	Placement *AreaPlacement `json:"placement"`
}

// VisibilityContext provides ambient information for visibility evaluation.
type VisibilityContext struct {
	Now         time.Time
	LocaleID    *uuid.UUID
	Audience    []string
	Segments    []string
	CustomRules map[string]any
}

var (
	ErrDefinitionNameRequired          = errors.New("widgets: definition name required")
	ErrDefinitionSchemaRequired        = errors.New("widgets: definition schema required")
	ErrDefinitionExists                = errors.New("widgets: definition already exists")
	ErrDefinitionDefaultsInvalid       = errors.New("widgets: defaults contain unknown fields")
	ErrDefinitionInUse                 = errors.New("widgets: definition has active instances")
	ErrDefinitionSoftDeleteUnsupported = errors.New("widgets: soft delete not supported for definitions")

	ErrInstanceDefinitionRequired    = errors.New("widgets: definition id required")
	ErrInstanceCreatorRequired       = errors.New("widgets: created_by is required")
	ErrInstanceUpdaterRequired       = errors.New("widgets: updated_by is required")
	ErrInstanceIDRequired            = errors.New("widgets: instance id required")
	ErrInstancePositionInvalid       = errors.New("widgets: position cannot be negative")
	ErrInstanceConfigurationInvalid  = errors.New("widgets: configuration contains unknown fields")
	ErrInstanceScheduleInvalid       = errors.New("widgets: publish_on must be before unpublish_on")
	ErrVisibilityRulesInvalid        = errors.New("widgets: visibility_rules contains unsupported keys")
	ErrVisibilityScheduleInvalid     = errors.New("widgets: visibility schedule timestamps must be RFC3339")
	ErrInstanceSoftDeleteUnsupported = errors.New("widgets: soft delete not supported for instances")

	ErrTranslationContentRequired = errors.New("widgets: translation content required")
	ErrTranslationLocaleRequired  = errors.New("widgets: translation locale required")
	ErrTranslationExists          = errors.New("widgets: translation already exists for locale")
	ErrTranslationNotFound        = errors.New("widgets: translation not found")

	ErrAreaCodeRequired           = errors.New("widgets: area code required")
	ErrAreaCodeInvalid            = errors.New("widgets: area code must contain letters, numbers, dot, or underscore")
	ErrAreaNameRequired           = errors.New("widgets: area name required")
	ErrAreaDefinitionExists       = errors.New("widgets: area code already exists")
	ErrAreaDefinitionNotFound     = errors.New("widgets: area definition not found")
	ErrAreaFeatureDisabled        = errors.New("widgets: area repositories not configured")
	ErrAreaInstanceRequired       = errors.New("widgets: instance id required")
	ErrAreaPlacementExists        = errors.New("widgets: widget already assigned to area for locale")
	ErrAreaPlacementPosition      = errors.New("widgets: placement position must be zero or positive")
	ErrAreaPlacementNotFound      = errors.New("widgets: placement not found")
	ErrAreaWidgetOrderMismatch    = errors.New("widgets: reorder input must include every placement")
	ErrVisibilityLocaleRestricted = errors.New("widgets: locale not permitted for widget")
)

// IDGenerator produces unique identifiers.
type IDGenerator func() uuid.UUID

// ServiceOption configures widget service behaviour.
type ServiceOption func(*service)

// WithClock overrides the time source used by the service.
func WithClock(clock func() time.Time) ServiceOption {
	return func(s *service) {
		if clock != nil {
			s.now = clock
		}
	}
}

// WithIDGenerator overrides the ID generator.
func WithIDGenerator(generator IDGenerator) ServiceOption {
	return func(s *service) {
		if generator != nil {
			s.id = generator
		}
	}
}

// WithRegistry injects a widget registry that provides built-in and host-defined widgets.
func WithRegistry(reg *Registry) ServiceOption {
	return func(s *service) {
		if reg != nil {
			s.registry = reg
		}
	}
}

// WithShortcodeService wires the shortcode renderer used for widget translations.
func WithShortcodeService(svc interfaces.ShortcodeService) ServiceOption {
	return func(s *service) {
		if svc != nil {
			s.shortcodes = svc
		}
	}
}

// WithAreaDefinitionRepository wires the area definition repository.
func WithAreaDefinitionRepository(repo AreaDefinitionRepository) ServiceOption {
	return func(s *service) {
		if repo != nil {
			s.areas = repo
		}
	}
}

// WithAreaPlacementRepository wires the area placement repository.
func WithAreaPlacementRepository(repo AreaPlacementRepository) ServiceOption {
	return func(s *service) {
		if repo != nil {
			s.placements = repo
		}
	}
}

type service struct {
	definitions  DefinitionRepository
	instances    InstanceRepository
	translations TranslationRepository
	areas        AreaDefinitionRepository
	placements   AreaPlacementRepository
	now          func() time.Time
	id           IDGenerator
	registry     *Registry
	shortcodes   interfaces.ShortcodeService
}

// NewService constructs a widget service instance.
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

	s.applyRegistry(context.Background())

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

	if err := validateDefaultsAgainstSchema(input.Schema, input.Defaults); err != nil {
		return nil, err
	}

	if existing, err := s.definitions.GetByName(ctx, name); err == nil && existing != nil {
		return nil, ErrDefinitionExists
	} else if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
	}

	now := s.now()
	definition := &Definition{
		ID:          s.id(),
		Name:        name,
		Description: cloneString(input.Description),
		Schema:      deepCloneMap(input.Schema),
		Defaults:    deepCloneMap(input.Defaults),
		Category:    cloneString(input.Category),
		Icon:        cloneString(input.Icon),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return s.definitions.Create(ctx, definition)
}

func (s *service) GetDefinition(ctx context.Context, id uuid.UUID) (*Definition, error) {
	return s.definitions.GetByID(ctx, id)
}

func (s *service) ListDefinitions(ctx context.Context) ([]*Definition, error) {
	return s.definitions.List(ctx)
}

func (s *service) DeleteDefinition(ctx context.Context, req DeleteDefinitionRequest) error {
	if req.ID == uuid.Nil {
		return &NotFoundError{Resource: "widget_definition", Key: ""}
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
	if input.CreatedBy == uuid.Nil {
		return nil, ErrInstanceCreatorRequired
	}
	if input.UpdatedBy == uuid.Nil {
		return nil, ErrInstanceUpdaterRequired
	}
	if input.Position < 0 {
		return nil, ErrInstancePositionInvalid
	}

	if err := validateSchedule(input.PublishOn, input.UnpublishOn); err != nil {
		return nil, err
	}

	if err := validateVisibilityRules(input.VisibilityRules); err != nil {
		return nil, err
	}

	definition, err := s.definitions.GetByID(ctx, input.DefinitionID)
	if err != nil {
		return nil, err
	}

	if err := validateConfiguration(definition.Schema, input.Configuration); err != nil {
		return nil, err
	}

	config := mergeConfiguration(definition.Defaults, input.Configuration)

	if s.registry != nil {
		if factory := s.registry.InstanceFactory(definition.Name); factory != nil {
			generated, genErr := factory(ctx, definition, input)
			if genErr != nil {
				return nil, genErr
			}
			if err := validateConfiguration(definition.Schema, generated); err != nil {
				return nil, err
			}
			config = mergeConfiguration(config, generated)
		}
	}

	instance := &Instance{
		ID:              s.id(),
		DefinitionID:    definition.ID,
		Configuration:   config,
		Placement:       deepCloneMap(input.Placement),
		VisibilityRules: deepCloneMap(input.VisibilityRules),
		PublishOn:       cloneTime(input.PublishOn),
		UnpublishOn:     cloneTime(input.UnpublishOn),
		Position:        input.Position,
		CreatedBy:       input.CreatedBy,
		UpdatedBy:       input.UpdatedBy,
		CreatedAt:       s.now(),
		UpdatedAt:       s.now(),
	}

	if input.BlockInstanceID != nil {
		clone := *input.BlockInstanceID
		instance.BlockInstanceID = &clone
	}
	if input.AreaCode != nil {
		area := strings.TrimSpace(*input.AreaCode)
		if area != "" {
			instance.AreaCode = &area
		}
	}

	return s.instances.Create(ctx, instance)
}

func (s *service) UpdateInstance(ctx context.Context, input UpdateInstanceInput) (*Instance, error) {
	if input.InstanceID == uuid.Nil {
		return nil, &NotFoundError{Resource: "widget_instance", Key: ""}
	}
	if input.UpdatedBy == uuid.Nil {
		return nil, ErrInstanceUpdaterRequired
	}

	instance, err := s.instances.GetByID(ctx, input.InstanceID)
	if err != nil {
		return nil, err
	}

	definition, err := s.definitions.GetByID(ctx, instance.DefinitionID)
	if err != nil {
		return nil, err
	}

	if input.Position != nil && *input.Position < 0 {
		return nil, ErrInstancePositionInvalid
	}

	if err := validateSchedule(input.PublishOn, input.UnpublishOn); err != nil {
		return nil, err
	}

	if err := validateVisibilityRules(input.VisibilityRules); err != nil {
		return nil, err
	}

	if input.Configuration != nil {
		if err := validateConfiguration(definition.Schema, input.Configuration); err != nil {
			return nil, err
		}
		instance.Configuration = mergeConfiguration(definition.Defaults, input.Configuration)
	}

	if input.VisibilityRules != nil {
		instance.VisibilityRules = deepCloneMap(input.VisibilityRules)
	}
	if input.Placement != nil {
		instance.Placement = deepCloneMap(input.Placement)
	}

	if input.PublishOn != nil {
		instance.PublishOn = cloneTime(input.PublishOn)
	}
	if input.UnpublishOn != nil {
		instance.UnpublishOn = cloneTime(input.UnpublishOn)
	}
	if input.Position != nil {
		instance.Position = *input.Position
	}
	if input.AreaCode != nil {
		area := strings.TrimSpace(*input.AreaCode)
		if area == "" {
			instance.AreaCode = nil
		} else {
			instance.AreaCode = &area
		}
	}

	instance.UpdatedBy = input.UpdatedBy
	instance.UpdatedAt = s.now()

	updated, err := s.instances.Update(ctx, instance)
	if err != nil {
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

func (s *service) GetInstance(ctx context.Context, id uuid.UUID) (*Instance, error) {
	instance, err := s.instances.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	enriched, err := s.attachTranslations(ctx, []*Instance{instance})
	if err != nil {
		return nil, err
	}
	if len(enriched) == 0 {
		return instance, nil
	}
	return enriched[0], nil
}

func (s *service) ListInstancesByDefinition(ctx context.Context, definitionID uuid.UUID) ([]*Instance, error) {
	instances, err := s.instances.ListByDefinition(ctx, definitionID)
	if err != nil {
		return nil, err
	}
	return s.attachTranslations(ctx, instances)
}

func (s *service) ListInstancesByArea(ctx context.Context, areaCode string) ([]*Instance, error) {
	instances, err := s.instances.ListByArea(ctx, areaCode)
	if err != nil {
		return nil, err
	}
	return s.attachTranslations(ctx, instances)
}

func (s *service) ListAllInstances(ctx context.Context) ([]*Instance, error) {
	instances, err := s.instances.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return s.attachTranslations(ctx, instances)
}

func (s *service) DeleteInstance(ctx context.Context, req DeleteInstanceRequest) error {
	if req.InstanceID == uuid.Nil {
		return ErrInstanceIDRequired
	}
	if !req.HardDelete {
		return ErrInstanceSoftDeleteUnsupported
	}
	if _, err := s.instances.GetByID(ctx, req.InstanceID); err != nil {
		return err
	}
	if s.placements != nil {
		if err := s.placements.DeleteByInstance(ctx, req.InstanceID); err != nil {
			return err
		}
	}
	translations, err := s.translations.ListByInstance(ctx, req.InstanceID)
	if err != nil {
		return err
	}
	for _, tr := range translations {
		if err := s.translations.Delete(ctx, tr.ID); err != nil {
			return err
		}
	}
	return s.instances.Delete(ctx, req.InstanceID)
}

func (s *service) AddTranslation(ctx context.Context, input AddTranslationInput) (*Translation, error) {
	if input.InstanceID == uuid.Nil {
		return nil, &NotFoundError{Resource: "widget_instance", Key: ""}
	}
	if input.LocaleID == uuid.Nil {
		return nil, ErrTranslationLocaleRequired
	}
	if input.Content == nil {
		return nil, ErrTranslationContentRequired
	}

	if _, err := s.instances.GetByID(ctx, input.InstanceID); err != nil {
		return nil, err
	}

	if existing, err := s.translations.GetByInstanceAndLocale(ctx, input.InstanceID, input.LocaleID); err == nil && existing != nil {
		return nil, ErrTranslationExists
	} else if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
	}

	translation := &Translation{
		ID:               s.id(),
		WidgetInstanceID: input.InstanceID,
		LocaleID:         input.LocaleID,
		Content:          deepCloneMap(input.Content),
		CreatedAt:        s.now(),
		UpdatedAt:        s.now(),
	}

	created, err := s.translations.Create(ctx, translation)
	if err != nil {
		return nil, err
	}
	return s.cloneAndRenderTranslation(ctx, created)
}

func (s *service) UpdateTranslation(ctx context.Context, input UpdateTranslationInput) (*Translation, error) {
	if input.InstanceID == uuid.Nil {
		return nil, &NotFoundError{Resource: "widget_instance", Key: ""}
	}
	if input.LocaleID == uuid.Nil {
		return nil, ErrTranslationLocaleRequired
	}
	if input.Content == nil {
		return nil, ErrTranslationContentRequired
	}

	translation, err := s.translations.GetByInstanceAndLocale(ctx, input.InstanceID, input.LocaleID)
	if err != nil {
		return nil, err
	}

	translation.Content = deepCloneMap(input.Content)
	translation.UpdatedAt = s.now()

	updated, err := s.translations.Update(ctx, translation)
	if err != nil {
		return nil, err
	}
	return s.cloneAndRenderTranslation(ctx, updated)
}

func (s *service) GetTranslation(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error) {
	record, err := s.translations.GetByInstanceAndLocale(ctx, instanceID, localeID)
	if err != nil {
		return nil, err
	}
	return s.cloneAndRenderTranslation(ctx, record)
}

func (s *service) DeleteTranslation(ctx context.Context, req DeleteTranslationRequest) error {
	if req.InstanceID == uuid.Nil {
		return ErrInstanceIDRequired
	}
	if req.LocaleID == uuid.Nil {
		return ErrTranslationLocaleRequired
	}

	translation, err := s.translations.GetByInstanceAndLocale(ctx, req.InstanceID, req.LocaleID)
	if err != nil {
		return err
	}
	if err := s.translations.Delete(ctx, translation.ID); err != nil {
		return err
	}
	return nil
}

func (s *service) RegisterAreaDefinition(ctx context.Context, input RegisterAreaDefinitionInput) (*AreaDefinition, error) {
	if err := s.ensureAreaSupport(); err != nil {
		return nil, err
	}

	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, ErrAreaCodeRequired
	}
	if !isValidAreaCode(code) {
		return nil, ErrAreaCodeInvalid
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrAreaNameRequired
	}
	scope := input.Scope
	if scope == "" {
		scope = AreaScopeGlobal
	}

	if existing, err := s.areas.GetByCode(ctx, code); err == nil && existing != nil {
		return nil, ErrAreaDefinitionExists
	} else if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
	}

	def := &AreaDefinition{
		ID:          s.id(),
		Code:        code,
		Name:        name,
		Description: cloneString(input.Description),
		Scope:       scope,
		ThemeID:     cloneUUIDPtr(input.ThemeID),
		TemplateID:  cloneUUIDPtr(input.TemplateID),
		CreatedAt:   s.now(),
		UpdatedAt:   s.now(),
	}

	return s.areas.Create(ctx, def)
}

func (s *service) ListAreaDefinitions(ctx context.Context) ([]*AreaDefinition, error) {
	if err := s.ensureAreaSupport(); err != nil {
		return nil, err
	}
	return s.areas.List(ctx)
}

func (s *service) AssignWidgetToArea(ctx context.Context, input AssignWidgetToAreaInput) ([]*AreaPlacement, error) {
	if err := s.ensureAreaSupport(); err != nil {
		return nil, err
	}
	code := strings.TrimSpace(input.AreaCode)
	if code == "" {
		return nil, ErrAreaCodeRequired
	}
	if !isValidAreaCode(code) {
		return nil, ErrAreaCodeInvalid
	}
	if input.InstanceID == uuid.Nil {
		return nil, ErrAreaInstanceRequired
	}

	if _, err := s.areas.GetByCode(ctx, code); err != nil {
		var nf *NotFoundError
		if errors.As(err, &nf) {
			return nil, ErrAreaDefinitionNotFound
		}
		return nil, err
	}

	instance, err := s.instances.GetByID(ctx, input.InstanceID)
	if err != nil {
		return nil, err
	}

	existing, err := s.placements.ListByAreaAndLocale(ctx, code, input.LocaleID)
	if err != nil {
		return nil, err
	}
	for _, placement := range existing {
		if placement.InstanceID == input.InstanceID {
			return nil, ErrAreaPlacementExists
		}
	}

	position := len(existing)
	if input.Position != nil {
		if *input.Position < 0 {
			return nil, ErrAreaPlacementPosition
		}
		if *input.Position <= len(existing) {
			position = *input.Position
		}
	}

	updated := make([]*AreaPlacement, 0, len(existing)+1)
	for idx, placement := range existing {
		if idx == position {
			updated = append(updated, nil)
		}
		updated = append(updated, cloneAreaPlacement(placement))
	}
	if position >= len(updated) {
		updated = append(updated, nil)
	}

	newPlacement := &AreaPlacement{
		ID:         s.id(),
		AreaCode:   code,
		LocaleID:   cloneUUIDPtr(input.LocaleID),
		InstanceID: instance.ID,
		Position:   position,
		Metadata:   deepCloneMap(input.Metadata),
		CreatedAt:  s.now(),
		UpdatedAt:  s.now(),
	}
	updated[position] = newPlacement

	for idx := range updated {
		updated[idx].Position = idx
	}

	if err := s.placements.Replace(ctx, code, input.LocaleID, updated); err != nil {
		return nil, err
	}
	return s.placements.ListByAreaAndLocale(ctx, code, input.LocaleID)
}

func (s *service) RemoveWidgetFromArea(ctx context.Context, input RemoveWidgetFromAreaInput) error {
	if err := s.ensureAreaSupport(); err != nil {
		return err
	}
	code := strings.TrimSpace(input.AreaCode)
	if code == "" {
		return ErrAreaCodeRequired
	}
	if input.InstanceID == uuid.Nil {
		return ErrAreaInstanceRequired
	}

	existing, err := s.placements.ListByAreaAndLocale(ctx, code, input.LocaleID)
	if err != nil {
		return err
	}
	found := false
	for _, placement := range existing {
		if placement.InstanceID == input.InstanceID {
			found = true
			break
		}
	}
	if !found {
		return ErrAreaPlacementNotFound
	}

	return s.placements.DeleteByAreaLocaleInstance(ctx, code, input.LocaleID, input.InstanceID)
}

func (s *service) ReorderAreaWidgets(ctx context.Context, input ReorderAreaWidgetsInput) ([]*AreaPlacement, error) {
	if err := s.ensureAreaSupport(); err != nil {
		return nil, err
	}
	code := strings.TrimSpace(input.AreaCode)
	if code == "" {
		return nil, ErrAreaCodeRequired
	}

	placements, err := s.placements.ListByAreaAndLocale(ctx, code, input.LocaleID)
	if err != nil {
		return nil, err
	}
	if len(placements) == 0 {
		return nil, nil
	}
	if len(input.Items) != len(placements) {
		return nil, ErrAreaWidgetOrderMismatch
	}

	index := make(map[uuid.UUID]*AreaPlacement, len(placements))
	for _, placement := range placements {
		index[placement.ID] = placement
	}

	slices.SortFunc(input.Items, func(a, b AreaWidgetOrder) int {
		return a.Position - b.Position
	})

	ordered := make([]*AreaPlacement, len(input.Items))
	for idx, entry := range input.Items {
		if entry.PlacementID == uuid.Nil {
			return nil, ErrAreaPlacementNotFound
		}
		if entry.Position < 0 {
			return nil, ErrAreaPlacementPosition
		}
		placement, ok := index[entry.PlacementID]
		if !ok {
			return nil, ErrAreaPlacementNotFound
		}
		clone := cloneAreaPlacement(placement)
		clone.Position = idx
		ordered[idx] = clone
	}

	if err := s.placements.Replace(ctx, code, input.LocaleID, ordered); err != nil {
		return nil, err
	}
	return s.placements.ListByAreaAndLocale(ctx, code, input.LocaleID)
}

func (s *service) ResolveArea(ctx context.Context, input ResolveAreaInput) ([]*ResolvedWidget, error) {
	if err := s.ensureAreaSupport(); err != nil {
		return nil, err
	}
	code := strings.TrimSpace(input.AreaCode)
	if code == "" {
		return nil, ErrAreaCodeRequired
	}

	localeChain := buildLocaleChain(input.LocaleID, input.FallbackLocaleIDs)
	var placements []*AreaPlacement
	for _, locale := range localeChain {
		records, err := s.placements.ListByAreaAndLocale(ctx, code, locale)
		if err != nil {
			return nil, err
		}
		if len(records) > 0 {
			placements = records
			break
		}
	}
	if len(placements) == 0 {
		return nil, nil
	}

	result := make([]*ResolvedWidget, 0, len(placements))
	visCtx := VisibilityContext{
		Now:      input.Now,
		LocaleID: input.LocaleID,
		Audience: input.Audience,
		Segments: input.Segments,
	}

	for _, placement := range placements {
		instance, err := s.instances.GetByID(ctx, placement.InstanceID)
		if err != nil {
			return nil, err
		}
		visible, err := s.EvaluateVisibility(ctx, instance, visCtx)
		if err != nil {
			if errors.Is(err, ErrVisibilityLocaleRestricted) {
				continue
			}
			return nil, err
		}
		if !visible {
			continue
		}
		enriched, err := s.attachTranslations(ctx, []*Instance{instance})
		if err != nil {
			return nil, err
		}
		resolved := &ResolvedWidget{
			Instance:  enriched[0],
			Placement: cloneAreaPlacement(placement),
		}
		result = append(result, resolved)
	}

	return result, nil
}

func (s *service) EvaluateVisibility(_ context.Context, instance *Instance, input VisibilityContext) (bool, error) {
	if instance == nil {
		return false, nil
	}
	now := input.Now
	if now.IsZero() {
		now = s.now()
	}
	if instance.PublishOn != nil && instance.PublishOn.After(now) {
		return false, nil
	}
	if instance.UnpublishOn != nil && instance.UnpublishOn.Before(now) {
		return false, nil
	}

	rules := instance.VisibilityRules
	if len(rules) == 0 {
		return true, nil
	}

	if scheduleRaw, ok := rules["schedule"].(map[string]any); ok {
		allowed, err := evaluateSchedule(now, scheduleRaw)
		if err != nil {
			return false, err
		}
		if !allowed {
			return false, nil
		}
	}

	if audienceRaw, ok := rules["audience"]; ok {
		if !matchStringRule(audienceRaw, input.Audience) {
			return false, nil
		}
	}

	if segmentsRaw, ok := rules["segments"]; ok {
		if !matchStringRule(segmentsRaw, input.Segments) {
			return false, nil
		}
	}

	if localesRaw, ok := rules["locales"]; ok {
		allowed, err := matchLocaleRule(localesRaw, input.LocaleID)
		if err != nil {
			return false, err
		}
		if !allowed {
			return false, ErrVisibilityLocaleRestricted
		}
	}

	return true, nil
}

func (s *service) attachTranslations(ctx context.Context, instances []*Instance) ([]*Instance, error) {
	if len(instances) == 0 {
		return nil, nil
	}

	enriched := make([]*Instance, 0, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		clone := *inst
		translations, err := s.translations.ListByInstance(ctx, inst.ID)
		if err != nil {
			var nf *NotFoundError
			if !errors.As(err, &nf) {
				return nil, err
			}
		}
		if translations != nil {
			hydrated := make([]*Translation, 0, len(translations))
			for _, tr := range translations {
				cloned, err := s.cloneAndRenderTranslation(ctx, tr)
				if err != nil {
					return nil, err
				}
				if cloned != nil {
					hydrated = append(hydrated, cloned)
				}
			}
			clone.Translations = hydrated
		}
		enriched = append(enriched, &clone)
	}
	return enriched, nil
}

func (s *service) cloneAndRenderTranslation(ctx context.Context, tr *Translation) (*Translation, error) {
	if tr == nil {
		return nil, nil
	}
	cloned := *tr
	if tr.Content != nil {
		cloned.Content = deepCloneMap(tr.Content)
		if s.shortcodes != nil {
			if err := s.renderShortcodesInMap(ctx, cloned.Content, ""); err != nil {
				return nil, err
			}
		}
	}
	return &cloned, nil
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
	for _, entry := range s.registry.List() {
		if entry.Name == "" {
			continue
		}
		if _, err := s.definitions.GetByName(ctx, entry.Name); err == nil {
			continue
		}
		_, _ = s.RegisterDefinition(ctx, entry)
	}
}

func validateConfiguration(schema map[string]any, configuration map[string]any) error {
	if configuration == nil {
		return nil
	}
	allowed := allowedFields(schema)
	if len(allowed) == 0 {
		return nil
	}
	for key := range configuration {
		if !allowed[key] {
			return ErrInstanceConfigurationInvalid
		}
	}
	return nil
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

func validateDefaultsAgainstSchema(schema map[string]any, defaults map[string]any) error {
	if defaults == nil {
		return nil
	}
	allowed := allowedFields(schema)
	if len(allowed) == 0 {
		return nil
	}
	for key := range defaults {
		if !allowed[key] {
			return ErrDefinitionDefaultsInvalid
		}
	}
	return nil
}

func validateSchedule(publishOn, unpublishOn *time.Time) error {
	if publishOn == nil || unpublishOn == nil {
		return nil
	}
	if publishOn.After(*unpublishOn) {
		return ErrInstanceScheduleInvalid
	}
	return nil
}

func validateVisibilityRules(rules map[string]any) error {
	if rules == nil {
		return nil
	}

	allowedKeys := map[string]struct{}{
		"audience":   {},
		"schedule":   {},
		"segments":   {},
		"conditions": {},
	}

	for key := range rules {
		if _, ok := allowedKeys[key]; !ok {
			return ErrVisibilityRulesInvalid
		}
	}

	schedule, hasSchedule := rules["schedule"]
	if !hasSchedule {
		return nil
	}

	scheduleMap, ok := schedule.(map[string]any)
	if !ok {
		return ErrVisibilityScheduleInvalid
	}

	for _, field := range []string{"starts_at", "ends_at"} {
		raw, exists := scheduleMap[field]
		if !exists || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case string:
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return ErrVisibilityScheduleInvalid
			}
		case time.Time:
			// already a timestamp, nothing to do
		default:
			return ErrVisibilityScheduleInvalid
		}
	}

	return nil
}

func allowedFields(schema map[string]any) map[string]bool {
	if len(schema) == 0 {
		return map[string]bool{}
	}
	fields, ok := schema["fields"]
	if !ok {
		return map[string]bool{}
	}

	result := make(map[string]bool)
	switch typed := fields.(type) {
	case []any:
		for _, entry := range typed {
			if fieldMap, ok := entry.(map[string]any); ok {
				if name, ok := fieldMap["name"].(string); ok {
					name = strings.TrimSpace(name)
					if name != "" {
						result[name] = true
					}
				}
			}
		}
	case []map[string]any:
		for _, fieldMap := range typed {
			if name, ok := fieldMap["name"].(string); ok {
				name = strings.TrimSpace(name)
				if name != "" {
					result[name] = true
				}
			}
		}
	}
	return result
}

func mergeConfiguration(base map[string]any, overlay map[string]any) map[string]any {
	merged := deepCloneMap(base)
	if merged == nil {
		merged = make(map[string]any)
	}
	if overlay != nil {
		for key, value := range overlay {
			merged[key] = deepCloneValue(value)
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func cloneString(src *string) *string {
	if src == nil {
		return nil
	}
	cloned := strings.Clone(*src)
	return &cloned
}

func cloneTime(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}
	cloned := *src
	return &cloned
}

func (s *service) ensureAreaSupport() error {
	if s.areas == nil || s.placements == nil {
		return ErrAreaFeatureDisabled
	}
	return nil
}

func buildLocaleChain(primary *uuid.UUID, fallbacks []uuid.UUID) []*uuid.UUID {
	chain := make([]*uuid.UUID, 0, len(fallbacks)+2)
	if primary != nil {
		chain = append(chain, primary)
	}
	for idx := range fallbacks {
		locale := fallbacks[idx]
		value := locale
		chain = append(chain, &value)
	}
	chain = append(chain, nil)
	return chain
}

func evaluateSchedule(now time.Time, schedule map[string]any) (bool, error) {
	if schedule == nil {
		return true, nil
	}
	if raw, ok := schedule["starts_at"]; ok && raw != nil {
		start, err := parseTime(raw)
		if err != nil {
			return false, ErrVisibilityScheduleInvalid
		}
		if start.After(now) {
			return false, nil
		}
	}
	if raw, ok := schedule["ends_at"]; ok && raw != nil {
		end, err := parseTime(raw)
		if err != nil {
			return false, ErrVisibilityScheduleInvalid
		}
		if end.Before(now) {
			return false, nil
		}
	}
	return true, nil
}

func parseTime(value any) (time.Time, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed, nil
	case string:
		return time.Parse(time.RFC3339, typed)
	default:
		return time.Time{}, fmt.Errorf("unsupported time value %T", value)
	}
}

func matchStringRule(rule any, actual []string) bool {
	if rule == nil {
		return true
	}
	required := make(map[string]struct{})
	switch typed := rule.(type) {
	case []any:
		for _, entry := range typed {
			if str, ok := entry.(string); ok {
				required[strings.ToLower(strings.TrimSpace(str))] = struct{}{}
			}
		}
	case []string:
		for _, entry := range typed {
			required[strings.ToLower(strings.TrimSpace(entry))] = struct{}{}
		}
	case string:
		required[strings.ToLower(strings.TrimSpace(typed))] = struct{}{}
	default:
		return false
	}

	if len(required) == 0 {
		return true
	}
	for _, candidate := range actual {
		if _, ok := required[strings.ToLower(strings.TrimSpace(candidate))]; ok {
			return true
		}
	}
	return false
}

func matchLocaleRule(rule any, localeID *uuid.UUID) (bool, error) {
	if rule == nil {
		return true, nil
	}
	allowed := make(map[uuid.UUID]struct{})
	switch typed := rule.(type) {
	case []any:
		for _, entry := range typed {
			id, err := parseUUID(entry)
			if err != nil {
				return false, err
			}
			allowed[id] = struct{}{}
		}
	case []string:
		for _, entry := range typed {
			id, err := uuid.Parse(strings.TrimSpace(entry))
			if err != nil {
				return false, err
			}
			allowed[id] = struct{}{}
		}
	case string:
		id, err := uuid.Parse(strings.TrimSpace(typed))
		if err != nil {
			return false, err
		}
		allowed[id] = struct{}{}
	default:
		return false, fmt.Errorf("unsupported locale rule type %T", rule)
	}

	if localeID == nil {
		return false, nil
	}
	if _, ok := allowed[*localeID]; ok {
		return true, nil
	}
	return false, nil
}

func parseUUID(value any) (uuid.UUID, error) {
	switch typed := value.(type) {
	case uuid.UUID:
		return typed, nil
	case string:
		return uuid.Parse(strings.TrimSpace(typed))
	default:
		return uuid.Nil, fmt.Errorf("unsupported uuid value %T", value)
	}
}

func isValidAreaCode(code string) bool {
	if code == "" {
		return false
	}
	for _, r := range code {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}
