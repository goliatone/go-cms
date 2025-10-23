package pages

import (
	"context"
	"fmt"

	goerrors "github.com/goliatone/go-errors"
	repository "github.com/goliatone/go-repository-bun"
	"github.com/goliatone/go-repository-cache/cache"
	repositorycache "github.com/goliatone/go-repository-cache/repositorycache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type BunPageRepository struct {
	repo repository.Repository[*Page]
}

func NewBunPageRepository(db *bun.DB) *BunPageRepository {
	return NewBunPageRepositoryWithCache(db, nil, nil)
}

// NewBunPageRepositoryWithCache constructs a PageRepository backed by bun with optional caching.
func NewBunPageRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, keySerializer cache.KeySerializer) *BunPageRepository {
	base := NewPageRepository(db)
	wrapped := wrapWithCache(base, cacheService, keySerializer)
	return &BunPageRepository{repo: wrapped}
}

func (r *BunPageRepository) Create(ctx context.Context, record *Page) (*Page, error) {
	created, err := r.repo.Create(ctx, record)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *BunPageRepository) GetByID(ctx context.Context, id uuid.UUID) (*Page, error) {
	result, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "page", id.String())
	}
	return result, nil
}

func (r *BunPageRepository) GetBySlug(ctx context.Context, slug string) (*Page, error) {
	result, err := r.repo.GetByIdentifier(ctx, slug)
	if err != nil {
		return nil, mapRepositoryError(err, "page", slug)
	}
	return result, nil
}

func (r *BunPageRepository) List(ctx context.Context) ([]*Page, error) {
	records, _, err := r.repo.List(ctx)
	return records, err
}

func mapRepositoryError(err error, resource, key string) error {
	if err == nil {
		return nil
	}

	if goerrors.IsCategory(err, repository.CategoryDatabaseNotFound) {
		return &PageNotFoundError{
			Key: key,
		}
	}

	return fmt.Errorf("%s repository error: %w", resource, err)
}

func wrapWithCache[T any](base repository.Repository[T], cacheService cache.CacheService, keySerializer cache.KeySerializer) repository.Repository[T] {
	if cacheService == nil || keySerializer == nil {
		return base
	}
	return repositorycache.New(base, cacheService, keySerializer)
}
