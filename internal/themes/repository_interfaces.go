package themes

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ThemeRepository exposes persistence operations for themes.
type ThemeRepository interface {
	Create(ctx context.Context, theme *Theme) (*Theme, error)
	Update(ctx context.Context, theme *Theme) (*Theme, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Theme, error)
	GetByName(ctx context.Context, name string) (*Theme, error)
	List(ctx context.Context) ([]*Theme, error)
	ListActive(ctx context.Context) ([]*Theme, error)
}

// TemplateRepository exposes persistence operations for templates.
type TemplateRepository interface {
	Create(ctx context.Context, template *Template) (*Template, error)
	Update(ctx context.Context, template *Template) (*Template, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Template, error)
	GetBySlug(ctx context.Context, themeID uuid.UUID, slug string) (*Template, error)
	ListByTheme(ctx context.Context, themeID uuid.UUID) ([]*Template, error)
	ListAll(ctx context.Context) ([]*Template, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// NotFoundError is returned when a theme resource cannot be located.
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
