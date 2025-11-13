package blocks

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// DefinitionRepository exposes persistence operations for block definitions.
type DefinitionRepository interface {
	Create(ctx context.Context, definition *Definition) (*Definition, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Definition, error)
	GetByName(ctx context.Context, name string) (*Definition, error)
	List(ctx context.Context) ([]*Definition, error)
	Update(ctx context.Context, definition *Definition) (*Definition, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// InstanceRepository exposes persistence operations for block instances.
type InstanceRepository interface {
	Create(ctx context.Context, instance *Instance) (*Instance, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Instance, error)
	ListByPage(ctx context.Context, pageID uuid.UUID) ([]*Instance, error)
	ListGlobal(ctx context.Context) ([]*Instance, error)
	ListByDefinition(ctx context.Context, definitionID uuid.UUID) ([]*Instance, error)
	Update(ctx context.Context, instance *Instance) (*Instance, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// InstanceVersionRepository exposes persistence operations for block instance versions.
type InstanceVersionRepository interface {
	Create(ctx context.Context, version *InstanceVersion) (*InstanceVersion, error)
	ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]*InstanceVersion, error)
	GetVersion(ctx context.Context, instanceID uuid.UUID, number int) (*InstanceVersion, error)
	GetLatest(ctx context.Context, instanceID uuid.UUID) (*InstanceVersion, error)
	Update(ctx context.Context, version *InstanceVersion) (*InstanceVersion, error)
}

// TranslationRepository exposes persistence operations for block translations.
type TranslationRepository interface {
	Create(ctx context.Context, translation *Translation) (*Translation, error)
	GetByInstanceAndLocale(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error)
	ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]*Translation, error)
	Update(ctx context.Context, translation *Translation) (*Translation, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// NotFoundError is returned when a block resource cannot be located.
type NotFoundError struct {
	Resource string
	Key      string
}

func (e *NotFoundError) Error() string {
	if e.Key == "" {
		return fmt.Sprintf("%s not found", e.Resource)
	}
	return fmt.Sprintf("%s %q not found", e.Resource, e.Key)
}
