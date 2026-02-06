package pages

import (
	"context"
	"fmt"
	"strings"
	"time"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
	goerrors "github.com/goliatone/go-errors"
	repository "github.com/goliatone/go-repository-bun"
	"github.com/goliatone/go-repository-cache/cache"
	repositorycache "github.com/goliatone/go-repository-cache/repositorycache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type BunPageRepository struct {
	db           *bun.DB
	repo         repository.Repository[*Page]
	translations repository.Repository[*PageTranslation]
	versions     repository.Repository[*PageVersion]
}

func NewBunPageRepository(db *bun.DB) *BunPageRepository {
	return NewBunPageRepositoryWithCache(db, nil, nil)
}

// NewBunPageRepositoryWithCache constructs a PageRepository backed by bun with optional caching.
func NewBunPageRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, keySerializer cache.KeySerializer) *BunPageRepository {
	base := NewPageRepository(db)
	translationBase := NewPageTranslationRepository(db)
	versionBase := NewPageVersionRepository(db)
	return &BunPageRepository{
		db:           db,
		repo:         wrapWithCache(base, cacheService, keySerializer),
		translations: wrapWithCache(translationBase, cacheService, keySerializer),
		versions:     wrapWithCache(versionBase, cacheService, keySerializer),
	}
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

func (r *BunPageRepository) GetBySlug(ctx context.Context, slug string, env ...string) (*Page, error) {
	normalizedEnv := normalizeEnvironmentKey(env...)
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.slug = ?", slug)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return applyEnvironmentFilter(q, normalizedEnv)
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, mapRepositoryError(err, "page", slug)
	}
	if len(records) == 0 {
		return nil, &PageNotFoundError{Key: slug}
	}
	return records[0], nil
}

func (r *BunPageRepository) List(ctx context.Context, env ...string) ([]*Page, error) {
	normalizedEnv := normalizeEnvironmentKey(env...)
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return applyEnvironmentFilter(q, normalizedEnv)
	}))
	return records, err
}

func (r *BunPageRepository) Update(ctx context.Context, record *Page) (*Page, error) {
	updated, err := r.repo.Update(ctx, record,
		repository.UpdateByID(record.ID.String()),
		repository.UpdateColumns(
			"template_id",
			"parent_id",
			"current_version",
			"published_version",
			"status",
			"primary_locale",
			"publish_at",
			"unpublish_at",
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

func (r *BunPageRepository) ReplaceTranslations(ctx context.Context, pageID uuid.UUID, translations []*PageTranslation) error {
	if r.db == nil {
		return fmt.Errorf("page repository: database not configured")
	}

	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().
			Model((*PageTranslation)(nil)).
			Where("?TableAlias.page_id = ?", pageID).
			Exec(ctx); err != nil {
			return fmt.Errorf("delete page translations: %w", err)
		}

		if len(translations) == 0 {
			return nil
		}

		now := time.Now().UTC()
		toInsert := make([]*PageTranslation, 0, len(translations))
		for _, tr := range translations {
			if tr == nil {
				continue
			}
			cloned := *tr
			cloned.PageID = pageID
			if cloned.ID == uuid.Nil {
				cloned.ID = uuid.New()
			}
			if cloned.CreatedAt.IsZero() {
				cloned.CreatedAt = now
			}
			if cloned.UpdatedAt.IsZero() {
				cloned.UpdatedAt = now
			}
			toInsert = append(toInsert, &cloned)
		}

		if len(toInsert) == 0 {
			return nil
		}

		if _, err := tx.NewInsert().Model(&toInsert).Exec(ctx); err != nil {
			return fmt.Errorf("insert page translations: %w", err)
		}
		return nil
	})
}

// ListTranslations returns translations for a page record.
func (r *BunPageRepository) ListTranslations(ctx context.Context, pageID uuid.UUID) ([]*PageTranslation, error) {
	if r.db == nil {
		return nil, fmt.Errorf("page repository: database not configured")
	}
	if r.translations == nil {
		return nil, fmt.Errorf("page repository: translations repository not configured")
	}
	records, _, err := r.translations.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.page_id = ?", pageID)
		}),
	)
	if err != nil {
		return nil, err
	}
	return records, nil
}

func (r *BunPageRepository) CreateVersion(ctx context.Context, version *PageVersion) (*PageVersion, error) {
	created, err := r.versions.Create(ctx, version)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *BunPageRepository) ListVersions(ctx context.Context, pageID uuid.UUID) ([]*PageVersion, error) {
	records, _, err := r.versions.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.page_id = ?", pageID)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.OrderExpr("?TableAlias.version ASC")
		}),
	)
	return records, err
}

func (r *BunPageRepository) GetVersion(ctx context.Context, pageID uuid.UUID, number int) (*PageVersion, error) {
	records, _, err := r.versions.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.page_id = ?", pageID)
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
		return nil, &PageVersionNotFoundError{PageID: pageID, Version: number}
	}
	return records[0], nil
}

func (r *BunPageRepository) GetLatestVersion(ctx context.Context, pageID uuid.UUID) (*PageVersion, error) {
	records, _, err := r.versions.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.page_id = ?", pageID)
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
		return nil, &PageVersionNotFoundError{PageID: pageID}
	}
	return records[0], nil
}

func (r *BunPageRepository) UpdateVersion(ctx context.Context, version *PageVersion) (*PageVersion, error) {
	updated, err := r.versions.Update(ctx, version,
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

func (r *BunPageRepository) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
	if !hardDelete {
		return fmt.Errorf("page repository: soft delete not supported")
	}
	if r.db == nil {
		return fmt.Errorf("page repository: database not configured")
	}

	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().
			Model((*PageTranslation)(nil)).
			Where("?TableAlias.page_id = ?", id).
			Exec(ctx); err != nil {
			return fmt.Errorf("delete page translations: %w", err)
		}

		if _, err := tx.NewDelete().
			Model((*PageVersion)(nil)).
			Where("?TableAlias.page_id = ?", id).
			Exec(ctx); err != nil {
			return fmt.Errorf("delete page versions: %w", err)
		}

		result, err := tx.NewDelete().
			Model((*Page)(nil)).
			Where("?TableAlias.id = ?", id).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("delete page: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("page delete rows affected: %w", err)
		}
		if affected == 0 {
			return &PageNotFoundError{Key: id.String()}
		}
		return nil
	})
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
