package environments

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// EnvironmentRepository exposes persistence operations for environments.
type EnvironmentRepository interface {
	Create(ctx context.Context, env *Environment) (*Environment, error)
	Update(ctx context.Context, env *Environment) (*Environment, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Environment, error)
	GetByKey(ctx context.Context, key string) (*Environment, error)
	List(ctx context.Context) ([]*Environment, error)
	ListActive(ctx context.Context) ([]*Environment, error)
	GetDefault(ctx context.Context) (*Environment, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// NotFoundError is returned when an environment cannot be located.
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
