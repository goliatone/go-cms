package menus

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// MenuRepository exposes persistence operations for menu records.
type MenuRepository interface {
	Create(ctx context.Context, menu *Menu) (*Menu, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Menu, error)
	GetByCode(ctx context.Context, code string) (*Menu, error)
	List(ctx context.Context) ([]*Menu, error)
	Update(ctx context.Context, menu *Menu) (*Menu, error)
}

// MenuItemRepository exposes persistence operations for menu items.
type MenuItemRepository interface {
	Create(ctx context.Context, item *MenuItem) (*MenuItem, error)
	GetByID(ctx context.Context, id uuid.UUID) (*MenuItem, error)
	ListByMenu(ctx context.Context, menuID uuid.UUID) ([]*MenuItem, error)
	ListChildren(ctx context.Context, parentID uuid.UUID) ([]*MenuItem, error)
	Update(ctx context.Context, item *MenuItem) (*MenuItem, error)
}

// MenuItemTranslationRepository exposes persistence operations for menu item translations.
type MenuItemTranslationRepository interface {
	Create(ctx context.Context, translation *MenuItemTranslation) (*MenuItemTranslation, error)
	GetByMenuItemAndLocale(ctx context.Context, menuItemID uuid.UUID, localeID uuid.UUID) (*MenuItemTranslation, error)
	ListByMenuItem(ctx context.Context, menuItemID uuid.UUID) ([]*MenuItemTranslation, error)
	Update(ctx context.Context, translation *MenuItemTranslation) (*MenuItemTranslation, error)
}

// NotFoundError is returned when a menu resource cannot be located.
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
