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
	GetByCode(ctx context.Context, code string, env ...string) (*Menu, error)
	GetByLocation(ctx context.Context, location string, env ...string) (*Menu, error)
	List(ctx context.Context, env ...string) ([]*Menu, error)
	Update(ctx context.Context, menu *Menu) (*Menu, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// MenuItemRepository exposes persistence operations for menu items.
type MenuItemRepository interface {
	Create(ctx context.Context, item *MenuItem) (*MenuItem, error)
	GetByID(ctx context.Context, id uuid.UUID) (*MenuItem, error)
	GetByMenuAndCanonicalKey(ctx context.Context, menuID uuid.UUID, key string) (*MenuItem, error)
	GetByMenuAndExternalCode(ctx context.Context, menuID uuid.UUID, code string) (*MenuItem, error)
	ListByMenu(ctx context.Context, menuID uuid.UUID) ([]*MenuItem, error)
	ListChildren(ctx context.Context, parentID uuid.UUID) ([]*MenuItem, error)
	Update(ctx context.Context, item *MenuItem) (*MenuItem, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// BulkUpdateHierarchy persists parent/position updates for multiple items atomically.
	BulkUpdateHierarchy(ctx context.Context, items []*MenuItem) error
	// BulkUpdateParentLinks persists parent_id/parent_ref/position updates for multiple items atomically.
	BulkUpdateParentLinks(ctx context.Context, items []*MenuItem) error
}

// MenuItemTranslationRepository exposes persistence operations for menu item translations.
type MenuItemTranslationRepository interface {
	Create(ctx context.Context, translation *MenuItemTranslation) (*MenuItemTranslation, error)
	GetByMenuItemAndLocale(ctx context.Context, menuItemID uuid.UUID, localeID uuid.UUID) (*MenuItemTranslation, error)
	ListByMenuItem(ctx context.Context, menuItemID uuid.UUID) ([]*MenuItemTranslation, error)
	Update(ctx context.Context, translation *MenuItemTranslation) (*MenuItemTranslation, error)
	Delete(ctx context.Context, id uuid.UUID) error
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
