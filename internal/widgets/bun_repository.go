package widgets

import (
	"context"
	"fmt"

	"github.com/goliatone/go-errors"
	repository "github.com/goliatone/go-repository-bun"
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

// NewBunDefinitionRepositoryWithCache creates a definition repository with caching.
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
		return nil, mapRepositoryError(err, "widget_definition", id.String())
	}
	return record, nil
}

func (r *BunDefinitionRepository) GetByName(ctx context.Context, name string) (*Definition, error) {
	record, err := r.repo.GetByIdentifier(ctx, name)
	if err != nil {
		return nil, mapRepositoryError(err, "widget_definition", name)
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

// NewBunInstanceRepository creates an instance repository without caching.
func NewBunInstanceRepository(db *bun.DB) *BunInstanceRepository {
	return NewBunInstanceRepositoryWithCache(db, nil, nil)
}

// NewBunInstanceRepositoryWithCache creates an instance repository with caching.
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
		return nil, mapRepositoryError(err, "widget_instance", id.String())
	}
	return record, nil
}

func (r *BunInstanceRepository) ListByDefinition(ctx context.Context, definitionID uuid.UUID) ([]*Instance, error) {
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.definition_id = ?", definitionID)
	}))
	return records, err
}

func (r *BunInstanceRepository) ListByArea(ctx context.Context, areaCode string) ([]*Instance, error) {
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.area_code = ?", areaCode)
	}))
	return records, err
}

func (r *BunInstanceRepository) ListAll(ctx context.Context) ([]*Instance, error) {
	records, _, err := r.repo.List(ctx)
	return records, err
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
			return q.Where("?TableAlias.widget_instance_id = ?", instanceID)
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
		return nil, &NotFoundError{Resource: "widget_translation", Key: translationKey(instanceID, localeID)}
	}
	return records[0], nil
}

func (r *BunTranslationRepository) ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]*Translation, error) {
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.widget_instance_id = ?", instanceID)
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
