package themes

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

// BunThemeRepository implements ThemeRepository with optional caching.
type BunThemeRepository struct {
	repo repository.Repository[*Theme]
}

// NewBunThemeRepository creates a theme repository without caching.
func NewBunThemeRepository(db *bun.DB) *BunThemeRepository {
	return NewBunThemeRepositoryWithCache(db, nil, nil)
}

// NewBunThemeRepositoryWithCache creates a theme repository with caching support.
func NewBunThemeRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunThemeRepository {
	base := NewThemeRepository(db)
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
	}
	return &BunThemeRepository{repo: base}
}

func (r *BunThemeRepository) Create(ctx context.Context, theme *Theme) (*Theme, error) {
	record, err := r.repo.Create(ctx, theme)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunThemeRepository) Update(ctx context.Context, theme *Theme) (*Theme, error) {
	record, err := r.repo.Update(ctx, theme)
	if err != nil {
		return nil, mapRepositoryError(err, "theme", theme.ID.String())
	}
	return record, nil
}

func (r *BunThemeRepository) GetByID(ctx context.Context, id uuid.UUID) (*Theme, error) {
	record, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "theme", id.String())
	}
	return record, nil
}

func (r *BunThemeRepository) GetByName(ctx context.Context, name string) (*Theme, error) {
	record, err := r.repo.GetByIdentifier(ctx, name)
	if err != nil {
		return nil, mapRepositoryError(err, "theme", name)
	}
	return record, nil
}

func (r *BunThemeRepository) List(ctx context.Context) ([]*Theme, error) {
	records, _, err := r.repo.List(ctx)
	return records, err
}

func (r *BunThemeRepository) ListActive(ctx context.Context) ([]*Theme, error) {
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.is_active = TRUE")
	}))
	return records, err
}

// BunTemplateRepository implements TemplateRepository with optional caching.
type BunTemplateRepository struct {
	repo repository.Repository[*Template]
}

// NewBunTemplateRepository creates a template repository without caching.
func NewBunTemplateRepository(db *bun.DB) *BunTemplateRepository {
	return NewBunTemplateRepositoryWithCache(db, nil, nil)
}

// NewBunTemplateRepositoryWithCache creates a template repository with caching.
func NewBunTemplateRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, serializer cache.KeySerializer) *BunTemplateRepository {
	base := NewTemplateRepository(db)
	if cacheService != nil && serializer != nil {
		base = repositorycache.New(base, cacheService, serializer)
	}
	return &BunTemplateRepository{repo: base}
}

func (r *BunTemplateRepository) Create(ctx context.Context, template *Template) (*Template, error) {
	record, err := r.repo.Create(ctx, template)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *BunTemplateRepository) Update(ctx context.Context, template *Template) (*Template, error) {
	record, err := r.repo.Update(ctx, template)
	if err != nil {
		return nil, mapRepositoryError(err, "template", template.ID.String())
	}
	return record, nil
}

func (r *BunTemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*Template, error) {
	record, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "template", id.String())
	}
	return record, nil
}

func (r *BunTemplateRepository) GetBySlug(ctx context.Context, themeID uuid.UUID, slug string) (*Template, error) {
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.theme_id = ?", themeID)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.slug = ?", slug)
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "template", Key: slug}
	}
	return records[0], nil
}

func (r *BunTemplateRepository) ListByTheme(ctx context.Context, themeID uuid.UUID) ([]*Template, error) {
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return q.Where("?TableAlias.theme_id = ?", themeID)
	}))
	return records, err
}

func (r *BunTemplateRepository) ListAll(ctx context.Context) ([]*Template, error) {
	records, _, err := r.repo.List(ctx)
	return records, err
}

func (r *BunTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.repo.Delete(ctx, &Template{ID: id})
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
