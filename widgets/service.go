package widgets

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service exposes widget management capabilities to external consumers.
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

// DeleteDefinitionRequest controls hard-delete behavior for widget definitions.
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

// DeleteInstanceRequest controls hard-delete behavior for widget instances.
type DeleteInstanceRequest struct {
	InstanceID uuid.UUID
	DeletedBy  uuid.UUID
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

// DeleteTranslationRequest removes a localized widget translation.
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

// VisibilityContext provides ambient information for visibility evaluation.
type VisibilityContext struct {
	Now         time.Time
	LocaleID    *uuid.UUID
	Audience    []string
	Segments    []string
	CustomRules map[string]any
}
