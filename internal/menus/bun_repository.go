package menus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
	goerrors "github.com/goliatone/go-errors"
	repository "github.com/goliatone/go-repository-bun"
	cache "github.com/goliatone/go-repository-cache/cache"
	repositorycache "github.com/goliatone/go-repository-cache/repositorycache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// BunMenuRepository implements MenuRepository with optional caching.
type BunMenuRepository struct {
	db           *bun.DB
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
		db:           db,
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

func (r *BunMenuRepository) GetByCode(ctx context.Context, code string, env ...string) (*Menu, error) {
	normalizedEnv := normalizeEnvironmentKey(env...)
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.code = ?", code)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return applyEnvironmentFilter(q, normalizedEnv)
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, mapRepositoryError(err, "menu", code)
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "menu", Key: code}
	}
	return records[0], nil
}

func (r *BunMenuRepository) GetByLocation(ctx context.Context, location string, env ...string) (*Menu, error) {
	if r == nil || r.db == nil {
		return nil, &NotFoundError{Resource: "menu", Key: location}
	}
	record := new(Menu)
	q := r.db.NewSelect().Model(record).Where("location = ?", location).Limit(1)
	q = applyEnvironmentFilter(q, normalizeEnvironmentKey(env...))
	err := q.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &NotFoundError{Resource: "menu", Key: location}
		}
		return nil, mapRepositoryError(err, "menu", location)
	}
	return record, nil
}

func (r *BunMenuRepository) List(ctx context.Context, env ...string) ([]*Menu, error) {
	normalizedEnv := normalizeEnvironmentKey(env...)
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return applyEnvironmentFilter(q, normalizedEnv)
	}))
	return records, err
}

func (r *BunMenuRepository) Update(ctx context.Context, menu *Menu) (*Menu, error) {
	record, err := r.repo.Update(ctx, menu)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.repo.Delete(ctx, &Menu{ID: id})
}

func (r *BunMenuRepository) InvalidateCache(ctx context.Context) error {
	if r.cacheService == nil || r.cachePrefix == "" {
		return nil
	}
	return r.cacheService.DeleteByPrefix(ctx, r.cachePrefix)
}

// BunMenuItemRepository implements MenuItemRepository with optional caching.
type BunMenuItemRepository struct {
	db           *bun.DB
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
	return &BunMenuItemRepository{db: db, repo: base, cacheService: svc, cachePrefix: prefix}
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

func (r *BunMenuItemRepository) GetByMenuAndCanonicalKey(ctx context.Context, menuID uuid.UUID, key string) (*MenuItem, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.menu_id = ?", menuID).
				Where("?TableAlias.canonical_key = ?", key)
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "menu_item", Key: fmt.Sprintf("%s:%s", menuID, key)}
	}
	return records[0], nil
}

func (r *BunMenuItemRepository) GetByMenuAndExternalCode(ctx context.Context, menuID uuid.UUID, code string) (*MenuItem, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.menu_id = ?", menuID).
				Where("?TableAlias.external_code = ?", code)
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "menu_item", Key: fmt.Sprintf("%s:%s", menuID, code)}
	}
	return records[0], nil
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

func (r *BunMenuItemRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.repo.Delete(ctx, &MenuItem{ID: id})
}

func (r *BunMenuItemRepository) BulkUpdateHierarchy(ctx context.Context, items []*MenuItem) error {
	if len(items) == 0 {
		return nil
	}
	_, err := r.repo.UpdateMany(ctx, items,
		repository.UpdateColumns("parent_id", "position", "updated_at", "updated_by"),
	)
	return err
}

func (r *BunMenuItemRepository) BulkUpdateParentLinks(ctx context.Context, items []*MenuItem) error {
	if len(items) == 0 {
		return nil
	}
	_, err := r.repo.UpdateMany(ctx, items,
		repository.UpdateColumns("parent_id", "parent_ref", "position", "updated_at", "updated_by"),
	)
	return err
}

func (r *BunMenuItemRepository) InvalidateCache(ctx context.Context) error {
	if r.cacheService == nil || r.cachePrefix == "" {
		return nil
	}
	return r.cacheService.DeleteByPrefix(ctx, r.cachePrefix)
}

func (r *BunMenuItemRepository) ResetMenuContents(ctx context.Context, menuID uuid.UUID) (itemsDeleted int, translationsDeleted int, err error) {
	if r.db == nil {
		return 0, 0, fmt.Errorf("menu item repository: database not configured")
	}

	var (
		itemsAffected        int64
		translationsAffected int64
	)

	err = r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var ids []uuid.UUID
		if err := tx.NewSelect().
			Model((*MenuItem)(nil)).
			Column("id").
			Where("?TableAlias.menu_id = ?", menuID).
			Scan(ctx, &ids); err != nil {
			return fmt.Errorf("list menu item ids: %w", err)
		}

		if len(ids) > 0 {
			res, err := tx.NewDelete().
				Model((*MenuItemTranslation)(nil)).
				Where("?TableAlias.menu_item_id IN (?)", bun.In(ids)).
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("delete menu item translations: %w", err)
			}
			affected, _ := res.RowsAffected()
			translationsAffected += affected
		}

		res, err := tx.NewDelete().
			Model((*MenuItem)(nil)).
			Where("?TableAlias.menu_id = ?", menuID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("delete menu items: %w", err)
		}
		affected, _ := res.RowsAffected()
		itemsAffected += affected

		return nil
	})

	return int(itemsAffected), int(translationsAffected), err
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

func (r *BunMenuItemTranslationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.repo.Delete(ctx, &MenuItemTranslation{ID: id})
}

func (r *BunMenuItemTranslationRepository) InvalidateCache(ctx context.Context) error {
	if r.cacheService == nil || r.cachePrefix == "" {
		return nil
	}
	return r.cacheService.DeleteByPrefix(ctx, r.cachePrefix)
}

// BunMenuLocationBindingRepository implements MenuLocationBindingRepository with optional caching.
type BunMenuLocationBindingRepository struct {
	repo         repository.Repository[*MenuLocationBinding]
	cacheService cache.CacheService
	cachePrefix  string
}

const menuLocationBindingNamespace = "menu_location_binding"

// NewBunMenuLocationBindingRepository creates a location binding repository without caching.
func NewBunMenuLocationBindingRepository(db *bun.DB) *BunMenuLocationBindingRepository {
	return NewBunMenuLocationBindingRepositoryWithCache(db, nil, nil)
}

// NewBunMenuLocationBindingRepositoryWithCache creates a location binding repository with caching.
func NewBunMenuLocationBindingRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunMenuLocationBindingRepository {
	base := NewMenuLocationBindingRepository(db)
	var svc cache.CacheService
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
		svc = cacheService
	}
	prefix := ""
	if svc != nil {
		prefix = cachePrefix(menuLocationBindingNamespace)
	}
	return &BunMenuLocationBindingRepository{repo: base, cacheService: svc, cachePrefix: prefix}
}

func (r *BunMenuLocationBindingRepository) Create(ctx context.Context, binding *MenuLocationBinding) (*MenuLocationBinding, error) {
	record, err := r.repo.Create(ctx, binding)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuLocationBindingRepository) GetByID(ctx context.Context, id uuid.UUID) (*MenuLocationBinding, error) {
	record, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "menu_location_binding", id.String())
	}
	return record, nil
}

func (r *BunMenuLocationBindingRepository) ListByLocation(ctx context.Context, location string, env ...string) ([]*MenuLocationBinding, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.location = ?", location)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return applyEnvironmentFilter(q, normalizeEnvironmentKey(env...))
		}),
	)
	return records, err
}

func (r *BunMenuLocationBindingRepository) List(ctx context.Context, env ...string) ([]*MenuLocationBinding, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return applyEnvironmentFilter(q, normalizeEnvironmentKey(env...))
		}),
	)
	return records, err
}

func (r *BunMenuLocationBindingRepository) Update(ctx context.Context, binding *MenuLocationBinding) (*MenuLocationBinding, error) {
	record, err := r.repo.Update(ctx, binding)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuLocationBindingRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.repo.Delete(ctx, &MenuLocationBinding{ID: id})
}

func (r *BunMenuLocationBindingRepository) InvalidateCache(ctx context.Context) error {
	if r.cacheService == nil || r.cachePrefix == "" {
		return nil
	}
	return r.cacheService.DeleteByPrefix(ctx, r.cachePrefix)
}

// BunMenuViewProfileRepository implements MenuViewProfileRepository with optional caching.
type BunMenuViewProfileRepository struct {
	repo         repository.Repository[*MenuViewProfile]
	cacheService cache.CacheService
	cachePrefix  string
}

const menuViewProfileNamespace = "menu_view_profile"

// NewBunMenuViewProfileRepository creates a menu view profile repository without caching.
func NewBunMenuViewProfileRepository(db *bun.DB) *BunMenuViewProfileRepository {
	return NewBunMenuViewProfileRepositoryWithCache(db, nil, nil)
}

// NewBunMenuViewProfileRepositoryWithCache creates a menu view profile repository with caching.
func NewBunMenuViewProfileRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunMenuViewProfileRepository {
	base := NewMenuViewProfileRepository(db)
	var svc cache.CacheService
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
		svc = cacheService
	}
	prefix := ""
	if svc != nil {
		prefix = cachePrefix(menuViewProfileNamespace)
	}
	return &BunMenuViewProfileRepository{repo: base, cacheService: svc, cachePrefix: prefix}
}

func (r *BunMenuViewProfileRepository) Create(ctx context.Context, profile *MenuViewProfile) (*MenuViewProfile, error) {
	record, err := r.repo.Create(ctx, profile)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuViewProfileRepository) GetByID(ctx context.Context, id uuid.UUID) (*MenuViewProfile, error) {
	record, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "menu_view_profile", id.String())
	}
	return record, nil
}

func (r *BunMenuViewProfileRepository) GetByCode(ctx context.Context, code string, env ...string) (*MenuViewProfile, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.code = ?", code)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return applyEnvironmentFilter(q, normalizeEnvironmentKey(env...))
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "menu_view_profile", Key: code}
	}
	return records[0], nil
}

func (r *BunMenuViewProfileRepository) List(ctx context.Context, env ...string) ([]*MenuViewProfile, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return applyEnvironmentFilter(q, normalizeEnvironmentKey(env...))
		}),
	)
	return records, err
}

func (r *BunMenuViewProfileRepository) Update(ctx context.Context, profile *MenuViewProfile) (*MenuViewProfile, error) {
	record, err := r.repo.Update(ctx, profile)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunMenuViewProfileRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.repo.Delete(ctx, &MenuViewProfile{ID: id})
}

func (r *BunMenuViewProfileRepository) InvalidateCache(ctx context.Context) error {
	if r.cacheService == nil || r.cachePrefix == "" {
		return nil
	}
	return r.cacheService.DeleteByPrefix(ctx, r.cachePrefix)
}

func normalizeEnvironmentKey(env ...string) string {
	if len(env) == 0 {
		return ""
	}
	return cmsenv.NormalizeKey(env[0])
}

func applyEnvironmentFilter(q *bun.SelectQuery, envKey string) *bun.SelectQuery {
	if q == nil {
		return q
	}
	if strings.TrimSpace(envKey) == "" {
		return q.Where("?TableAlias.environment_id = (SELECT id FROM environments WHERE is_default = TRUE LIMIT 1)")
	}
	if envID, err := uuid.Parse(envKey); err == nil {
		return q.Where("?TableAlias.environment_id = ?", envID)
	}
	return q.Where("?TableAlias.environment_id = (SELECT id FROM environments WHERE key = ? LIMIT 1)", envKey)
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
