package content

import (
	"context"
	"fmt"
	"strings"
	"time"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
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

func (r *BunContentRepository) GetBySlug(ctx context.Context, slug string, contentTypeID uuid.UUID, env ...string) (*Content, error) {
	normalizedEnv := normalizeEnvironmentKey(env...)
	records, _, err := r.repo.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.slug = ?", slug).
				Where("?TableAlias.content_type_id = ?", contentTypeID)
		}),
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return applyEnvironmentFilter(q, normalizedEnv)
		}),
		repository.SelectPaginate(1, 0),
	)
	if err != nil {
		return nil, mapRepositoryError(err, "content", slug)
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "content", Key: slug}
	}
	return records[0], nil
}

func (r *BunContentRepository) List(ctx context.Context, env ...ContentListOption) ([]*Content, error) {
	opts := parseContentListOptions(env...)
	normalizedEnv := normalizeEnvironmentKey(opts.envKey)
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return applyEnvironmentFilter(q, normalizedEnv)
	}))
	if err != nil {
		return nil, err
	}
	if !opts.includeTranslations || len(records) == 0 {
		return records, nil
	}

	ids := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		ids = append(ids, record.ID)
	}
	if len(ids) == 0 {
		return records, nil
	}

	translations, _, err := r.translations.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.content_id IN (?)", bun.In(ids)).
				Relation("Locale")
		}),
	)
	if err != nil {
		return nil, err
	}

	byContent := make(map[uuid.UUID][]*ContentTranslation, len(ids))
	for _, tr := range translations {
		if tr == nil {
			continue
		}
		byContent[tr.ContentID] = append(byContent[tr.ContentID], tr)
	}
	for _, record := range records {
		if record == nil {
			continue
		}
		record.Translations = byContent[record.ID]
	}
	return records, nil
}

func (r *BunContentRepository) Update(ctx context.Context, record *Content) (*Content, error) {
	updated, err := r.repo.Update(ctx, record,
		repository.UpdateByID(record.ID.String()),
		repository.UpdateColumns(
			"current_version",
			"published_version",
			"status",
			"metadata",
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

// ListTranslations returns translations for a content record.
func (r *BunContentRepository) ListTranslations(ctx context.Context, contentID uuid.UUID) ([]*ContentTranslation, error) {
	if r.db == nil {
		return nil, fmt.Errorf("content repository: database not configured")
	}
	records, _, err := r.translations.List(ctx,
		repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("?TableAlias.content_id = ?", contentID).
				Relation("Locale")
		}),
	)
	if err != nil {
		return nil, err
	}
	return records, nil
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

func (r *BunContentTypeRepository) Create(ctx context.Context, record *ContentType) (*ContentType, error) {
	created, err := r.repo.Create(ctx, record)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (r *BunContentTypeRepository) GetByID(ctx context.Context, id uuid.UUID) (*ContentType, error) {
	result, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "content_type", id.String())
	}
	if result == nil || result.DeletedAt != nil {
		return nil, &NotFoundError{Resource: "content_type", Key: id.String()}
	}
	return result, nil
}

func (r *BunContentTypeRepository) GetBySlug(ctx context.Context, slug string, env ...string) (*ContentType, error) {
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
		return nil, mapRepositoryError(err, "content_type", slug)
	}
	if len(records) == 0 {
		return nil, &NotFoundError{Resource: "content_type", Key: slug}
	}
	result := records[0]
	if result == nil || result.DeletedAt != nil {
		return nil, &NotFoundError{Resource: "content_type", Key: slug}
	}
	return result, nil
}

func (r *BunContentTypeRepository) List(ctx context.Context, env ...string) ([]*ContentType, error) {
	normalizedEnv := normalizeEnvironmentKey(env...)
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		return applyEnvironmentFilter(q, normalizedEnv).
			OrderExpr("?TableAlias.slug ASC").
			OrderExpr("?TableAlias.created_at ASC")
	}))
	if err != nil {
		return nil, err
	}
	return filterActiveContentTypes(records), nil
}

func (r *BunContentTypeRepository) Search(ctx context.Context, query string, env ...string) ([]*ContentType, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return r.List(ctx, env...)
	}
	like := "%" + strings.ToLower(query) + "%"
	normalizedEnv := normalizeEnvironmentKey(env...)
	records, _, err := r.repo.List(ctx, repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
		filtered := applyEnvironmentFilter(
			q.Where("LOWER(?TableAlias.name) LIKE ?", like).
				WhereOr("LOWER(?TableAlias.slug) LIKE ?", like),
			normalizedEnv,
		)
		return filtered.
			OrderExpr("?TableAlias.slug ASC").
			OrderExpr("?TableAlias.created_at ASC")
	}))
	if err != nil {
		return nil, err
	}
	return filterActiveContentTypes(records), nil
}

func (r *BunContentTypeRepository) Update(ctx context.Context, record *ContentType) (*ContentType, error) {
	updated, err := r.repo.Update(ctx, record,
		repository.UpdateByID(record.ID.String()),
		repository.UpdateColumns(
			"name",
			"slug",
			"description",
			"schema",
			"ui_schema",
			"capabilities",
			"icon",
			"schema_version",
			"status",
			"deleted_at",
			"updated_at",
		),
	)
	if err != nil {
		return nil, mapRepositoryError(err, "content_type", record.ID.String())
	}
	return updated, nil
}

func (r *BunContentTypeRepository) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
	if hardDelete {
		return r.repo.Delete(ctx, &ContentType{ID: id})
	}
	now := time.Now().UTC()
	_, err := r.repo.Update(ctx, &ContentType{ID: id, DeletedAt: &now, UpdatedAt: now, Status: ContentTypeStatusDeprecated},
		repository.UpdateByID(id.String()),
		repository.UpdateColumns("deleted_at", "updated_at", "status"),
	)
	if err != nil {
		return mapRepositoryError(err, "content_type", id.String())
	}
	return nil
}

func filterActiveContentTypes(records []*ContentType) []*ContentType {
	if len(records) == 0 {
		return records
	}
	filtered := make([]*ContentType, 0, len(records))
	for _, record := range records {
		if record == nil || record.DeletedAt != nil {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
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

func (r *BunLocaleRepository) GetByID(ctx context.Context, id uuid.UUID) (*Locale, error) {
	result, err := r.repo.GetByID(ctx, id.String())
	if err != nil {
		return nil, mapRepositoryError(err, "locale", id.String())
	}
	return result, nil
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
