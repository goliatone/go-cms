package widgets

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// DefinitionRepository exposes persistence operations for widget definitions.
type DefinitionRepository interface {
	Create(ctx context.Context, definition *Definition) (*Definition, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Definition, error)
	GetByName(ctx context.Context, name string) (*Definition, error)
	List(ctx context.Context) ([]*Definition, error)
	Update(ctx context.Context, definition *Definition) (*Definition, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// InstanceRepository exposes persistence operations for widget instances.
type InstanceRepository interface {
	Create(ctx context.Context, instance *Instance) (*Instance, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Instance, error)
	ListByDefinition(ctx context.Context, definitionID uuid.UUID) ([]*Instance, error)
	ListByArea(ctx context.Context, areaCode string) ([]*Instance, error)
	ListAll(ctx context.Context) ([]*Instance, error)
	Update(ctx context.Context, instance *Instance) (*Instance, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// TranslationRepository exposes persistence operations for widget translations.
type TranslationRepository interface {
	Create(ctx context.Context, translation *Translation) (*Translation, error)
	GetByInstanceAndLocale(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error)
	ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]*Translation, error)
	Update(ctx context.Context, translation *Translation) (*Translation, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// AreaDefinitionRepository manages widget area metadata.
type AreaDefinitionRepository interface {
	Create(ctx context.Context, definition *AreaDefinition) (*AreaDefinition, error)
	GetByCode(ctx context.Context, code string) (*AreaDefinition, error)
	List(ctx context.Context) ([]*AreaDefinition, error)
}

// AreaPlacementRepository manages widget placements within areas.
type AreaPlacementRepository interface {
	ListByAreaAndLocale(ctx context.Context, areaCode string, localeID *uuid.UUID) ([]*AreaPlacement, error)
	Replace(ctx context.Context, areaCode string, localeID *uuid.UUID, placements []*AreaPlacement) error
	DeleteByAreaLocaleInstance(ctx context.Context, areaCode string, localeID *uuid.UUID, instanceID uuid.UUID) error
	DeleteByInstance(ctx context.Context, instanceID uuid.UUID) error
}

// NotFoundError is returned when a widget resource cannot be located.
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
