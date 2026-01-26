package pages_test

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/testsupport"
	repocache "github.com/goliatone/go-repository-cache/cache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestPageService_WithBunStorageAndCache(t *testing.T) {
	ctx := context.Background()

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)

	registerPageModels(t, bunDB)
	seedPageEntities(t, bunDB)

	cacheCfg := repocache.DefaultConfig()
	cacheCfg.TTL = time.Minute
	cacheService, err := repocache.NewCacheService(cacheCfg)
	if err != nil {
		t.Fatalf("new cache service: %v", err)
	}
	keySerializer := repocache.NewDefaultKeySerializer()

	contentRepo := content.NewBunContentRepositoryWithCache(bunDB, cacheService, keySerializer)
	contentTypeRepo := content.NewBunContentTypeRepositoryWithCache(bunDB, cacheService, keySerializer)
	localeRepo := content.NewBunLocaleRepositoryWithCache(bunDB, cacheService, keySerializer)

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)

	createdContent, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: mustUUID("00000000-0000-0000-0000-000000000210"),
		Slug:          "about",
		Status:        "published",
		CreatedBy:     mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		UpdatedBy:     mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		Translations: []content.ContentTranslationInput{{
			Locale:  "en",
			Title:   "About",
			Content: map[string]any{"body": "Welcome"},
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageRepo := pages.NewBunPageRepositoryWithCache(bunDB, cacheService, keySerializer)

	pageSvc := pages.NewService(pageRepo, contentRepo, localeRepo)

	page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  createdContent.ID,
		TemplateID: mustUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		Slug:       "about",
		Status:     "published",
		CreatedBy:  mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		UpdatedBy:  mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "About",
			Path:   "/about",
		}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	if _, err := pageSvc.Get(ctx, page.ID); err != nil {
		t.Fatalf("first get page: %v", err)
	}
	if _, err := pageSvc.Get(ctx, page.ID); err != nil {
		t.Fatalf("cached get page: %v", err)
	}
}

func TestPageService_AllowsOptionalTranslations(t *testing.T) {
	ctx := context.Background()

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)

	registerPageModels(t, bunDB)
	seedPageEntities(t, bunDB)

	contentRepo := content.NewBunContentRepository(bunDB)
	contentTypeRepo := content.NewBunContentTypeRepository(bunDB)
	localeRepo := content.NewBunLocaleRepository(bunDB)
	pageRepo := pages.NewBunPageRepository(bunDB)

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)
	pageSvc := pages.NewService(
		pageRepo,
		contentRepo,
		localeRepo,
		pages.WithRequireTranslations(false),
	)

	authorID := mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: mustUUID("00000000-0000-0000-0000-000000000210"),
		Slug:          "translationless-page",
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Translationless Page",
				Content: map[string]any{"body": "Placeholder"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content for page: %v", err)
	}

	pageRecord, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:                contentRecord.ID,
		TemplateID:               mustUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		Slug:                     "translationless-page",
		Status:                   "draft",
		CreatedBy:                authorID,
		UpdatedBy:                authorID,
		AllowMissingTranslations: true,
	})
	if err != nil {
		t.Fatalf("create page without translations: %v", err)
	}
	if pageRecord == nil {
		t.Fatal("expected page record")
	}
	if len(pageRecord.Translations) != 0 {
		t.Fatalf("expected zero translations, got %d", len(pageRecord.Translations))
	}

	fetched, err := pageSvc.Get(ctx, pageRecord.ID)
	if err != nil {
		t.Fatalf("get page without translations: %v", err)
	}
	if len(fetched.Translations) != 0 {
		t.Fatalf("expected fetched page to have zero translations, got %d", len(fetched.Translations))
	}

	if err := pageRepo.ReplaceTranslations(ctx, pageRecord.ID, nil); err != nil {
		t.Fatalf("replace page translations with empty set: %v", err)
	}

	afterReplace, err := pageSvc.Get(ctx, pageRecord.ID)
	if err != nil {
		t.Fatalf("get page after empty replace: %v", err)
	}
	if len(afterReplace.Translations) != 0 {
		t.Fatalf("expected zero translations after replace, got %d", len(afterReplace.Translations))
	}
}

func registerPageModels(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()

	models := []any{
		(*content.Locale)(nil),
		(*content.ContentType)(nil),
		(*content.Content)(nil),
		(*content.ContentTranslation)(nil),
		(*pages.Page)(nil),
		(*pages.PageTranslation)(nil),
		(*pages.PageVersion)(nil),
	}

	for _, model := range models {
		if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			t.Fatalf("create table %T: %v", model, err)
		}
	}
}

func seedPageEntities(t *testing.T, db *bun.DB) {
	seedContentEntities(t, db)
}

func seedContentEntities(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()

	locale := &content.Locale{
		ID:        mustUUID("00000000-0000-0000-0000-000000000201"),
		Code:      "en",
		Display:   "English",
		IsActive:  true,
		IsDefault: true,
	}
	if _, err := db.NewInsert().Model(locale).Exec(ctx); err != nil {
		t.Fatalf("insert locale: %v", err)
	}

	schema := map[string]any{"fields": []map[string]any{{"name": "body", "type": "richtext"}}}
	ct := &content.ContentType{
		ID:     mustUUID("00000000-0000-0000-0000-000000000210"),
		Name:   "page",
		Slug:   "page",
		Schema: schema,
	}
	if _, err := db.NewInsert().Model(ct).Exec(ctx); err != nil {
		t.Fatalf("insert content type: %v", err)
	}
}

func mustUUID(v string) uuid.UUID {
	id, err := uuid.Parse(v)
	if err != nil {
		panic(err)
	}
	return id
}
