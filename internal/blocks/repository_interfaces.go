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
}

// InstanceRepository exposes persistence operations for block instances.
type InstanceRepository interface {
	Create(ctx context.Context, instance *Instance) (*Instance, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Instance, error)
	ListByPage(ctx context.Context, pageID uuid.UUID) ([]*Instance, error)
	ListGlobal(ctx context.Context) ([]*Instance, error)
}

// TranslationRepository exposes persistence operations for block translations.
type TranslationRepository interface {
	Create(ctx context.Context, translation *Translation) (*Translation, error)
	GetByInstanceAndLocale(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error)
	ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]*Translation, error)
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
