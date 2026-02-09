package blocks

import (
	"context"
	"time"

	"github.com/goliatone/go-cms/media"
	"github.com/google/uuid"
)

// Service exposes block definition and instance management capabilities.
type Service interface {
	RegisterDefinition(ctx context.Context, input RegisterDefinitionInput) (*Definition, error)
	GetDefinition(ctx context.Context, id uuid.UUID) (*Definition, error)
	ListDefinitions(ctx context.Context, env ...string) ([]*Definition, error)
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

// RegisterDefinitionInput captures definition attributes for creation.
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
	EnvironmentKey   string
}

// UpdateDefinitionInput captures mutable definition fields.
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
	EnvironmentKey   *string
}

// CreateDefinitionVersionInput captures schema version updates for a definition.
type CreateDefinitionVersionInput struct {
	DefinitionID uuid.UUID
	Schema       map[string]any
	Defaults     map[string]any
}

// DeleteDefinitionRequest captures block definition deletion inputs.
type DeleteDefinitionRequest struct {
	ID         uuid.UUID
	HardDelete bool
}

// CreateInstanceInput defines the payload required to create a block instance.
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

// UpdateInstanceInput defines mutable block instance fields.
type UpdateInstanceInput struct {
	InstanceID    uuid.UUID
	PageID        *uuid.UUID
	Region        *string
	Position      *int
	Configuration map[string]any
	IsGlobal      *bool
	UpdatedBy     uuid.UUID
}

// DeleteInstanceRequest captures block instance deletion inputs.
type DeleteInstanceRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}

// AddTranslationInput captures localized content additions.
type AddTranslationInput struct {
	BlockInstanceID    uuid.UUID
	LocaleID           uuid.UUID
	Content            map[string]any
	AttributeOverrides map[string]any
	MediaBindings      media.BindingSet
}

// UpdateTranslationInput captures localized content updates.
type UpdateTranslationInput struct {
	BlockInstanceID    uuid.UUID
	LocaleID           uuid.UUID
	Content            map[string]any
	AttributeOverrides map[string]any
	MediaBindings      media.BindingSet
	UpdatedBy          uuid.UUID
}

// DeleteTranslationRequest removes a localized block translation.
type DeleteTranslationRequest struct {
	BlockInstanceID          uuid.UUID
	LocaleID                 uuid.UUID
	DeletedBy                uuid.UUID
	AllowMissingTranslations bool
}

// CreateInstanceDraftRequest captures draft snapshot data for an instance.
type CreateInstanceDraftRequest struct {
	InstanceID  uuid.UUID
	Snapshot    BlockVersionSnapshot
	CreatedBy   uuid.UUID
	UpdatedBy   uuid.UUID
	BaseVersion *int
}

// PublishInstanceDraftRequest captures publish inputs for a draft snapshot.
type PublishInstanceDraftRequest struct {
	InstanceID  uuid.UUID
	Version     int
	PublishedBy uuid.UUID
	PublishedAt *time.Time
}

// RestoreInstanceVersionRequest captures restore inputs for an instance version.
type RestoreInstanceVersionRequest struct {
	InstanceID uuid.UUID
	Version    int
	RestoredBy uuid.UUID
}
