package blocks

import (
	"context"
	"fmt"

	"github.com/goliatone/go-errors"
	"github.com/goliatone/go-repository-bun"
	"github.com/goliatone/go-repository-cache/cache"
	"github.com/goliatone/go-repository-cache/repositorycache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// BunDefinitionRepository implements DefinitionRepository with optional caching.
type BunDefinitionRepository struct {
	repo repository.Repository[*Definition]
}

// NewBunDefinitionRepository creates a definition repository without caching.
func NewBunDefinitionRepository(db *bun.DB) *BunDefinitionRepository {
	return NewBunDefinitionRepositoryWithCache(db, nil, nil)
}

// NewBunDefinitionRepositoryWithCache creates a definition repository with caching services.
func NewBunDefinitionRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunDefinitionRepository {
	base := NewDefinitionRepository(db)
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
	}
	return &BunDefinitionRepository{repo: base}
}

func (r *BunDefinitionRepository) Create(ctx context.Context, definition *Definition) (*Definition, error) {
	record, err := r.repo.Create(ctx, definition)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunDefinitionRepository) GetByID(ctx context.Context, id uuid.UUID) (*Definition, error) {
	record, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "block_definition", id.String())
	}
	return record, nil
}

func (r *BunDefinitionRepository) GetByName(ctx context.Context, name string) (*Definition, error) {
	record, err := r.repo.GetByIdentifier(ctx, name)
	if err != nil {
		return nil, mapRepositoryError(err, "block_definition", name)
	}
	return record, nil
}

func (r *BunDefinitionRepository) List(ctx context.Context) ([]*Definition, error) {
	records, _, err := r.repo.List(ctx)
	return records, err
}

// BunInstanceRepository implements InstanceRepository with optional caching.
type BunInstanceRepository struct {
	repo repository.Repository[*Instance]
}

// NewBunInstanceRepository creates a block instance repository without caching.
func NewBunInstanceRepository(db *bun.DB) *BunInstanceRepository {
	return NewBunInstanceRepositoryWithCache(db, nil, nil)
}

// NewBunInstanceRepositoryWithCache creates a block instance repository with caching services.
func NewBunInstanceRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunInstanceRepository {
	base := NewInstanceRepository(db)
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
	}
	return &BunInstanceRepository{repo: base}
}

func (r *BunInstanceRepository) Create(ctx context.Context, instance *Instance) (*Instance, error) {
	record, err := r.repo.Create(ctx, instance)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunInstanceRepository) GetByID(ctx context.Context, id uuid.UUID) (*Instance, error) {
	record, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "block_instance", id.String())
	}
	return record, nil
}

func (r *BunInstanceRepository) ListByPage(ctx context.Context, pageID uuid.UUID) ([]*Instance, error) {
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.page_id = ?", pageID)
	}))
	return records, err
}

func (r *BunInstanceRepository) ListGlobal(ctx context.Context) ([]*Instance, error) {
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.is_global = ?", true)
	}))
	return records, err
}

func (r *BunInstanceRepository) Update(ctx context.Context, instance *Instance) (*Instance, error) {
	updated, err := r.repo.Update(ctx, instance,
		repository.UpdateByID(instance.ID.String()),
		repository.UpdateColumns(
			"current_version",
			"published_version",
			"published_at",
			"published_by",
			"updated_by",
			"updated_at",
		),
	)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// BunInstanceVersionRepository implements InstanceVersionRepository with optional caching.
type BunInstanceVersionRepository struct {
	repo repository.Repository[*InstanceVersion]
}

// NewBunInstanceVersionRepository creates a block instance version repository without caching.
func NewBunInstanceVersionRepository(db *bun.DB) *BunInstanceVersionRepository {
	return NewBunInstanceVersionRepositoryWithCache(db, nil, nil)
}

// NewBunInstanceVersionRepositoryWithCache creates a block instance version repository with caching services.
func NewBunInstanceVersionRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunInstanceVersionRepository {
	base := NewInstanceVersionRepository(db)
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
	}
	return &BunInstanceVersionRepository{repo: base}
}

func (r *BunInstanceVersionRepository) Create(ctx context.Context, version *InstanceVersion) (*InstanceVersion, error) {
	record, err := r.repo.Create(ctx, version)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunInstanceVersionRepository) ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]*InstanceVersion, error) {
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.block_instance_id = ?", instanceID)
	}), repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.OrderExpr("?TableAlias.version ASC")
	}))
	return records, err
}

func (r *BunInstanceVersionRepository) GetVersion(ctx context.Context, instanceID uuid.UUID, number int) (*InstanceVersion, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.block_instance_id = ?", instanceID)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.version = ?", number)
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "block_version", Key: versionKey(instanceID, number)}
	}
	return records[0], nil
}

func (r *BunInstanceVersionRepository) GetLatest(ctx context.Context, instanceID uuid.UUID) (*InstanceVersion, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.block_instance_id = ?", instanceID)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.OrderExpr("?TableAlias.version DESC")
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "block_version", Key: instanceID.String()}
	}
	return records[0], nil
}

func (r *BunInstanceVersionRepository) Update(ctx context.Context, version *InstanceVersion) (*InstanceVersion, error) {
	updated, err := r.repo.Update(ctx, version,
		repository.UpdateByID(version.ID.String()),
		repository.UpdateColumns(
			"status",
			"published_at",
			"published_by",
		),
	)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// BunTranslationRepository implements TranslationRepository with optional caching.
type BunTranslationRepository struct {
	repo repository.Repository[*Translation]
}

// NewBunTranslationRepository creates a translation repository without caching.
func NewBunTranslationRepository(db *bun.DB) *BunTranslationRepository {
	return NewBunTranslationRepositoryWithCache(db, nil, nil)
}

// NewBunTranslationRepositoryWithCache creates a translation repository with caching.
func NewBunTranslationRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunTranslationRepository {
	base := NewTranslationRepository(db)
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
	}
	return &BunTranslationRepository{repo: base}
}

func (r *BunTranslationRepository) Create(ctx context.Context, translation *Translation) (*Translation, error) {
	record, err := r.repo.Create(ctx, translation)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunTranslationRepository) GetByInstanceAndLocale(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.block_instance_id = ?", instanceID)
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
		return nil, &NotFoundError{Resource: "block_translation", Key: translationKey(instanceID, localeID)}
	}
	return records[0], nil
}

func (r *BunTranslationRepository) ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]*Translation, error) {
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.block_instance_id = ?", instanceID)
	}))
	return records, err
}

func mapRepositoryError(err error, resource, key string) error {
	if err == nil {
		return nil
	}

	if errors.IsCategory(err, repository.CategoryDatabaseNotFound) {
		return &NotFoundError{Resource: resource, Key: key}
	}

	return fmt.Errorf("%s repository error: %w", resource, err)
}

func versionKey(instanceID uuid.UUID, version int) string {
	return fmt.Sprintf("%s:%d", instanceID.String(), version)
}
