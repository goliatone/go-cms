package environments

import (
	"context"
	"fmt"

	"github.com/goliatone/go-errors"
	repository "github.com/goliatone/go-repository-bun"
	"github.com/goliatone/go-repository-cache/cache"
	repositorycache "github.com/goliatone/go-repository-cache/repositorycache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// BunEnvironmentRepository implements EnvironmentRepository with optional caching.
type BunEnvironmentRepository struct {
	repo repository.Repository[*Environment]
}

// NewBunEnvironmentRepository creates an environment repository without caching.
func NewBunEnvironmentRepository(db *bun.DB) *BunEnvironmentRepository {
	return NewBunEnvironmentRepositoryWithCache(db, nil, nil)
}

// NewBunEnvironmentRepositoryWithCache creates an environment repository with caching support.
func NewBunEnvironmentRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunEnvironmentRepository {
	base := NewEnvironmentRepository(db)
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
	}
	return &BunEnvironmentRepository{repo: base}
}

func (r *BunEnvironmentRepository) Create(ctx context.Context, env *Environment) (*Environment, error) {
	record, err := r.repo.Create(ctx, env)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunEnvironmentRepository) Update(ctx context.Context, env *Environment) (*Environment, error) {
	updated, err := r.repo.Update(ctx, env,
		repository.UpdateByID(env.ID.String()),
		repository.UpdateColumns(
			"key",
			"name",
			"description",
			"is_active",
			"is_default",
			"updated_at",
			"deleted_at",
		),
	)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *BunEnvironmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*Environment, error) {
	record, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "environment", id.String())
	}
	return record, nil
}

func (r *BunEnvironmentRepository) GetByKey(ctx context.Context, key string) (*Environment, error) {
	record, err := r.repo.GetByIdentifier(ctx, key)
	if err != nil {
		return nil, mapRepositoryError(err, "environment", key)
	}
	return record, nil
}

func (r *BunEnvironmentRepository) List(ctx context.Context) ([]*Environment, error) {
	records, _, err := r.repo.List(ctx)
	return records, err
}

func (r *BunEnvironmentRepository) ListActive(ctx context.Context) ([]*Environment, error) {
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.is_active = TRUE")
	}))
	return records, err
}

func (r *BunEnvironmentRepository) GetDefault(ctx context.Context) (*Environment, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.is_default = TRUE")
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "environment", Key: "default"}
	}
	return records[0], nil
}

func (r *BunEnvironmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.repo.Delete(ctx, &Environment{ID: id})
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
