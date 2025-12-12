package content

import (
	"context"
	"fmt"
	"time"

	goerrors "github.com/goliatone/go-errors"
	"github.com/goliatone/go-repository-bun"
	"github.com/goliatone/go-repository-cache/cache"
	repositorycache "github.com/goliatone/go-repository-cache/repositorycache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type BunContentRepository struct {
	db           *bun.DB
	repo         repository.Repository[*Content]
	translations repository.Repository[*ContentTranslation]
	versions     repository.Repository[*ContentVersion]
}

func NewBunContentRepository(db *bun.DB) *BunContentRepository {
	return NewBunContentRepositoryWithCache(db, nil, nil)
}

func NewBunContentRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, keySerializer cache.KeySerializer) *BunContentRepository {
	base := NewContentRepository(db)
	translationBase := NewContentTranslationRepository(db)
	versionBase := NewContentVersionRepository(db)
	return &BunContentRepository{
		db:           db,
		repo:         wrapWithCache(base, cacheService, keySerializer),
		translations: wrapWithCache(translationBase, cacheService, keySerializer),
		versions:     wrapWithCache(versionBase, cacheService, keySerializer),
	}
}

func (r *BunContentRepository) Create(ctx context.Context, record *Content) (*Content, error) {
	if r.db == nil {
		return nil, fmt.Errorf("content repository: database not configured")
	}

	var created *Content
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		created, err = r.repo.CreateTx(ctx, tx, record)
		if err != nil {
			return err
		}

		if len(record.Translations) == 0 {
			return nil
		}

		now := time.Now().UTC()
		toInsert := make([]*ContentTranslation, 0, len(record.Translations))
		for _, tr := range record.Translations {
			if tr == nil {
				continue
			}
			cloned := *tr
			if cloned.ID == uuid.Nil {
				cloned.ID = uuid.New()
			}
			cloned.ContentID = created.ID
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
			return fmt.Errorf("insert translations: %w", err)
		}

		created.Translations = append([]*ContentTranslation{}, toInsert...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *BunContentRepository) GetByID(ctx context.Context, id uuid.UUID) (*Content, error) {
	result, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "content", id.String())
	}
	return result, nil
}

func (r *BunContentRepository) GetBySlug(ctx context.Context, slug string) (*Content, error) {
	result, err := r.repo.GetByIdentifier(ctx, slug)
	if err != nil {
		return nil, mapRepositoryError(err, "content", slug)
	}
	return result, nil
}

func (r *BunContentRepository) List(ctx context.Context) ([]*Content, error) {
	records, _, err := r.repo.List(ctx)
	return records, err
}

func (r *BunContentRepository) Update(ctx context.Context, record *Content) (*Content, error) {
	updated, err := r.repo.Update(ctx, record,
		repository.UpdateByID(record.ID.String()),
		repository.UpdateColumns(
			"current_version",
			"published_version",
			"status",
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

func (r *BunContentRepository) ReplaceTranslations(ctx context.Context, contentID uuid.UUID, translations []*ContentTranslation) error {
	if r.db == nil {
		return fmt.Errorf("content repository: database not configured")
	}

	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().
			Model((*ContentTranslation)(nil)).
			Where("?TableAlias.content_id = ?", contentID).
			Exec(ctx); err != nil {
			return fmt.Errorf("delete translations: %w", err)
		}

		if len(translations) == 0 {
			return nil
		}

		toInsert := make([]*ContentTranslation, 0, len(translations))
		now := time.Now().UTC()
		for _, tr := range translations {
			if tr == nil {
				continue
			}
			cloned := *tr
			if cloned.ID == uuid.Nil {
				cloned.ID = uuid.New()
			}
			cloned.ContentID = contentID
			if cloned.CreatedAt.IsZero() {
				cloned.CreatedAt = now
			}
			cloned.UpdatedAt = now
			toInsert = append(toInsert, &cloned)
		}

		if len(toInsert) == 0 {
			return nil
		}

		if _, err := tx.NewInsert().Model(&toInsert).Exec(ctx); err != nil {
			return fmt.Errorf("insert translations: %w", err)
		}
		return nil
	})
}

func (r *BunContentRepository) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
	if !hardDelete {
		return fmt.Errorf("content repository: soft delete not supported")
	}
	if r.db == nil {
		return fmt.Errorf("content repository: database not configured")
	}

	return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().
			Model((*ContentTranslation)(nil)).
			Where("?TableAlias.content_id = ?", id).
			Exec(ctx); err != nil {
			return fmt.Errorf("delete translations: %w", err)
		}

		if _, err := tx.NewDelete().
			Model((*ContentVersion)(nil)).
			Where("?TableAlias.content_id = ?", id).
			Exec(ctx); err != nil {
			return fmt.Errorf("delete versions: %w", err)
		}

		result, err := tx.NewDelete().
			Model((*Content)(nil)).
			Where("?TableAlias.id = ?", id).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("delete content: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("delete content rows affected: %w", err)
		}
		if affected == 0 {
			return &NotFoundError{Resource: "content", Key: id.String()}
		}
		return nil
	})
}

func (r *BunContentRepository) CreateVersion(ctx context.Context, version *ContentVersion) (*ContentVersion, error) {
	created, err := r.versions.Create(ctx, version)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *BunContentRepository) ListVersions(ctx context.Context, contentID uuid.UUID) ([]*ContentVersion, error) {
	records, _, err := r.versions.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.content_id = ?", contentID)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.OrderExpr("?TableAlias.version ASC")
		}),
	)
	return records, err
}

func (r *BunContentRepository) GetVersion(ctx context.Context, contentID uuid.UUID, number int) (*ContentVersion, error) {
	records, _, err := r.versions.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.content_id = ?", contentID)
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
		return nil, &NotFoundError{
			Resource: "content_version",
			Key:      contentVersionKey(contentID, number),
		}
	}
	return records[0], nil
}

func (r *BunContentRepository) GetLatestVersion(ctx context.Context, contentID uuid.UUID) (*ContentVersion, error) {
	records, _, err := r.versions.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.content_id = ?", contentID)
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
		return nil, &NotFoundError{
			Resource: "content_version",
			Key:      contentID.String(),
		}
	}
	return records[0], nil
}

func (r *BunContentRepository) UpdateVersion(ctx context.Context, version *ContentVersion) (*ContentVersion, error) {
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

type BunContentTypeRepository struct {
	repo repository.Repository[*ContentType]
}

func NewBunContentTypeRepository(db *bun.DB) *BunContentTypeRepository {
	return NewBunContentTypeRepositoryWithCache(db, nil, nil)
}

func NewBunContentTypeRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, keySerializer cache.KeySerializer) *BunContentTypeRepository {
	base := NewContentTypeRepository(db)
	wrapped := wrapWithCache(base, cacheService, keySerializer)
	return &BunContentTypeRepository{repo: wrapped}
}

func (r *BunContentTypeRepository) GetByID(ctx context.Context, id uuid.UUID) (*ContentType, error) {
	result, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "content_type", id.String())
	}
	return result, nil
}

type BunLocaleRepository struct {
	repo repository.Repository[*Locale]
}

func NewBunLocaleRepository(db *bun.DB) *BunLocaleRepository {
	return NewBunLocaleRepositoryWithCache(db, nil, nil)
}

// NewBunLocaleRepositoryWithCache constructs a LocaleRepository with optional caching.
func NewBunLocaleRepositoryWithCache(db *bun.DB, cacheService cache.CacheService, keySerializer cache.KeySerializer) *BunLocaleRepository {
	base := NewLocaleRepository(db)
	wrapped := wrapWithCache(base, cacheService, keySerializer)
	return &BunLocaleRepository{repo: wrapped}
}

func (r *BunLocaleRepository) GetByCode(ctx context.Context, code string) (*Locale, error) {
	result, err := r.repo.GetByIdentifier(ctx, code)
	if err != nil {
		return nil, mapRepositoryError(err, "locale", code)
	}
	return result, nil
}

func mapRepositoryError(err error, resource, key string) error {
	if err == nil {
		return nil
	}
	if goerrors.IsCategory(err, repository.CategoryDatabaseNotFound) {
		return &NotFoundError{
			Resource: resource,
			Key:      key,
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

func contentVersionKey(contentID uuid.UUID, version int) string {
	return fmt.Sprintf("%s:%d", contentID.String(), version)
}
