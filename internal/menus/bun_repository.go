package menus

import (
	"context"
	"fmt"

	goerrors "github.com/goliatone/go-errors"
	repository "github.com/goliatone/go-repository-bun"
	cache "github.com/goliatone/go-repository-cache/cache"
	repositorycache "github.com/goliatone/go-repository-cache/repositorycache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// BunMenuRepository implements MenuRepository with optional caching.
type BunMenuRepository struct {
	repo         repository.Repository[*Menu]
	cacheService cache.CacheService
	cachePrefix  string
}

const menuNamespace = "menu"

// NewBunMenuRepository creates a menu repository without caching.
func NewBunMenuRepository(db *bun.DB) *BunMenuRepository {
	return NewBunMenuRepositoryWithCache(db, nil, nil)
}

// NewBunMenuRepositoryWithCache creates a menu repository with caching services.
func NewBunMenuRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunMenuRepository {
	base := NewMenuRepository(db)
	var svc cache.CacheService
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
		svc = cacheService
	}
	prefix := ""
	if svc != nil {
		prefix = cachePrefix(menuNamespace)
	}
	return &BunMenuRepository{
		repo:         base,
		cacheService: svc,
		cachePrefix:  prefix,
	}
}

func (r *BunMenuRepository) Create(ctx context.Context, menu *Menu) (*Menu, error) {
	record, err := r.repo.Create(ctx, menu)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuRepository) GetByID(ctx context.Context, id uuid.UUID) (*Menu, error) {
	record, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "menu", id.String())
	}
	return record, nil
}

func (r *BunMenuRepository) GetByCode(ctx context.Context, code string) (*Menu, error) {
	record, err := r.repo.GetByIdentifier(ctx, code)
	if err != nil {
		return nil, mapRepositoryError(err, "menu", code)
	}
	return record, nil
}

func (r *BunMenuRepository) List(ctx context.Context) ([]*Menu, error) {
	records, _, err := r.repo.List(ctx)
	return records, err
}

func (r *BunMenuRepository) Update(ctx context.Context, menu *Menu) (*Menu, error) {
	record, err := r.repo.Update(ctx, menu)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuRepository) InvalidateCache(ctx context.Context) error {
	if r.cacheService == nil || r.cachePrefix == "" {
		return nil
	}
	return r.cacheService.DeleteByPrefix(ctx, r.cachePrefix)
}

// BunMenuItemRepository implements MenuItemRepository with optional caching.
type BunMenuItemRepository struct {
	repo         repository.Repository[*MenuItem]
	cacheService cache.CacheService
	cachePrefix  string
}

const menuItemNamespace = "menu_item"

// NewBunMenuItemRepository creates a menu item repository without caching.
func NewBunMenuItemRepository(db *bun.DB) *BunMenuItemRepository {
	return NewBunMenuItemRepositoryWithCache(db, nil, nil)
}

// NewBunMenuItemRepositoryWithCache creates a menu item repository with caching services.
func NewBunMenuItemRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunMenuItemRepository {
	base := NewMenuItemRepository(db)
	var svc cache.CacheService
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
		svc = cacheService
	}
	prefix := ""
	if svc != nil {
		prefix = cachePrefix(menuItemNamespace)
	}
	return &BunMenuItemRepository{repo: base, cacheService: svc, cachePrefix: prefix}
}

func (r *BunMenuItemRepository) Create(ctx context.Context, item *MenuItem) (*MenuItem, error) {
	record, err := r.repo.Create(ctx, item)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuItemRepository) GetByID(ctx context.Context, id uuid.UUID) (*MenuItem, error) {
	record, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "menu_item", id.String())
	}
	return record, nil
}

func (r *BunMenuItemRepository) ListByMenu(ctx context.Context, menuID uuid.UUID) ([]*MenuItem, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.menu_id = ?", menuID).
				OrderExpr("?TableAlias.position ASC")
		}),
	)
	return records, err
}

func (r *BunMenuItemRepository) ListChildren(ctx context.Context, parentID uuid.UUID) ([]*MenuItem, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.parent_id = ?", parentID).
				OrderExpr("?TableAlias.position ASC")
		}),
	)
	return records, err
}

func (r *BunMenuItemRepository) Update(ctx context.Context, item *MenuItem) (*MenuItem, error) {
	record, err := r.repo.Update(ctx, item)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuItemRepository) InvalidateCache(ctx context.Context) error {
	if r.cacheService == nil || r.cachePrefix == "" {
		return nil
	}
	return r.cacheService.DeleteByPrefix(ctx, r.cachePrefix)
}

// BunMenuItemTranslationRepository implements MenuItemTranslationRepository with optional caching.
type BunMenuItemTranslationRepository struct {
	repo         repository.Repository[*MenuItemTranslation]
	cacheService cache.CacheService
	cachePrefix  string
}

const menuItemTranslationNamespace = "menu_item_translation"

// NewBunMenuItemTranslationRepository creates a translation repository without caching.
func NewBunMenuItemTranslationRepository(db *bun.DB) *BunMenuItemTranslationRepository {
	return NewBunMenuItemTranslationRepositoryWithCache(db, nil, nil)
}

// NewBunMenuItemTranslationRepositoryWithCache creates a translation repository with caching services.
func NewBunMenuItemTranslationRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunMenuItemTranslationRepository {
	base := NewMenuItemTranslationRepository(db)
	var svc cache.CacheService
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
		svc = cacheService
	}
	prefix := ""
	if svc != nil {
		prefix = cachePrefix(menuItemTranslationNamespace)
	}
	return &BunMenuItemTranslationRepository{repo: base, cacheService: svc, cachePrefix: prefix}
}

func (r *BunMenuItemTranslationRepository) Create(ctx context.Context, translation *MenuItemTranslation) (*MenuItemTranslation, error) {
	record, err := r.repo.Create(ctx, translation)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuItemTranslationRepository) GetByMenuItemAndLocale(ctx context.Context, menuItemID uuid.UUID, localeID uuid.UUID) (*MenuItemTranslation, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.menu_item_id = ?", menuItemID)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.locale_id = ?", localeID)
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "menu_item_translation", Key: translationKey(menuItemID, localeID)}
	}
	return records[0], nil
}

func (r *BunMenuItemTranslationRepository) ListByMenuItem(ctx context.Context, menuItemID uuid.UUID) ([]*MenuItemTranslation, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.menu_item_id = ?", menuItemID)
		}),
	)
	return records, err
}

func (r *BunMenuItemTranslationRepository) Update(ctx context.Context, translation *MenuItemTranslation) (*MenuItemTranslation, error) {
	record, err := r.repo.Update(ctx, translation)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuItemTranslationRepository) InvalidateCache(ctx context.Context) error {
	if r.cacheService == nil || r.cachePrefix == "" {
		return nil
	}
	return r.cacheService.DeleteByPrefix(ctx, r.cachePrefix)
}

func mapRepositoryError(err error, resource, key string) error {
	if err == nil {
		return nil
	}

	if goerrors.IsCategory(err, repository.CategoryDatabaseNotFound) {
		return &NotFoundError{Resource: resource, Key: key}
	}

	return fmt.Errorf("%s repository error: %w", resource, err)
}

func cachePrefix(namespace string) string {
	if namespace == "" {
		return ""
	}
	return namespace + cache.KeySeparator
}
