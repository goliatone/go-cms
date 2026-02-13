package content_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/testsupport"
	repocache "github.com/goliatone/go-repository-cache/cache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestContentService_WithBunStorageAndCache(t *testing.T) {
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

	registerContentModels(t, bunDB)
	seedContentEntities(t, bunDB)

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

	svc := content.NewService(contentRepo, contentTypeRepo, localeRepo)

	created, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: mustUUID("00000000-0000-0000-0000-000000000210"),
		Slug:          "company-overview",
		Status:        "published",
		CreatedBy:     mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		UpdatedBy:     mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Company Overview",
				Content: map[string]any{"body": "Welcome"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	if _, err := svc.Get(ctx, created.ID); err != nil {
		t.Fatalf("first get: %v", err)
	}

	if _, err := svc.Get(ctx, created.ID); err != nil {
		t.Fatalf("cached get: %v", err)
	}
}

func TestContentService_AllowsOptionalTranslations(t *testing.T) {
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

	registerContentModels(t, bunDB)
	seedContentEntities(t, bunDB)

	contentRepo := content.NewBunContentRepository(bunDB)
	contentTypeRepo := content.NewBunContentTypeRepository(bunDB)
	localeRepo := content.NewBunLocaleRepository(bunDB)

	authorID := mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	svc := content.NewService(
		contentRepo,
		contentTypeRepo,
		localeRepo,
		content.WithRequireTranslations(false),
	)

	created, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:            mustUUID("00000000-0000-0000-0000-000000000210"),
		Slug:                     "optional-summary",
		Status:                   "draft",
		CreatedBy:                authorID,
		UpdatedBy:                authorID,
		AllowMissingTranslations: true,
	})
	if err != nil {
		t.Fatalf("create content without translations: %v", err)
	}
	if created == nil {
		t.Fatal("expected content record")
	}
	if len(created.Translations) != 0 {
		t.Fatalf("expected zero translations, got %d", len(created.Translations))
	}

	fetched, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get content without translations: %v", err)
	}
	if len(fetched.Translations) != 0 {
		t.Fatalf("expected fetched content to have zero translations, got %d", len(fetched.Translations))
	}

	if err := contentRepo.ReplaceTranslations(ctx, created.ID, nil); err != nil {
		t.Fatalf("replace translations with empty set: %v", err)
	}

	afterReplace, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get content after empty replace: %v", err)
	}
	if len(afterReplace.Translations) != 0 {
		t.Fatalf("expected zero translations after replace, got %d", len(afterReplace.Translations))
	}
}

func TestContentService_CreateTranslationConcurrencyWithBunStorage(t *testing.T) {
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

	registerContentModels(t, bunDB)
	seedContentEntities(t, bunDB)

	contentRepo := content.NewBunContentRepository(bunDB)
	contentTypeRepo := content.NewBunContentTypeRepository(bunDB)
	localeRepo := content.NewBunLocaleRepository(bunDB)

	svc := content.NewService(contentRepo, contentTypeRepo, localeRepo)
	creator, ok := svc.(content.TranslationCreator)
	if !ok {
		t.Fatalf("expected translation creator capability, got %T", svc)
	}

	source, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: mustUUID("00000000-0000-0000-0000-000000000210"),
		Slug:          "translation-race",
		Status:        "published",
		CreatedBy:     mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		UpdatedBy:     mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Hello",
				Content: map[string]any{"body": "Welcome"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create source content: %v", err)
	}

	if _, err := bunDB.NewInsert().Model(&content.Locale{
		ID:        mustUUID("00000000-0000-0000-0000-000000000202"),
		Code:      "fr",
		Display:   "French",
		IsActive:  true,
		IsDefault: false,
	}).Exec(ctx); err != nil {
		t.Fatalf("insert fr locale: %v", err)
	}

	const workers = 2
	results := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, createErr := creator.CreateTranslation(ctx, content.CreateContentTranslationRequest{
				SourceID:     source.ID,
				TargetLocale: "fr",
			})
			results <- createErr
		}()
	}
	wg.Wait()
	close(results)

	success := 0
	conflicts := 0
	for createErr := range results {
		if createErr == nil {
			success++
			continue
		}
		if errors.Is(createErr, content.ErrTranslationAlreadyExists) {
			conflicts++
			continue
		}
		t.Fatalf("unexpected create translation error: %v", createErr)
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("expected one success and one conflict, got success=%d conflicts=%d", success, conflicts)
	}
}

func TestBunContentTypeRepository_ListAndSearchOrdersBySlugAndCreatedAt(t *testing.T) {
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

	registerContentModels(t, bunDB)

	envID := mustUUID("00000000-0000-0000-0000-000000000301")
	base := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	schema := map[string]any{"fields": []map[string]any{{"name": "body", "type": "richtext"}}}

	records := []*content.ContentType{
		{
			ID:            mustUUID("00000000-0000-0000-0000-000000000311"),
			Name:          "Alpha Old",
			Slug:          "alpha",
			Schema:        schema,
			EnvironmentID: envID,
			CreatedAt:     base,
			UpdatedAt:     base,
		},
		{
			ID:            mustUUID("00000000-0000-0000-0000-000000000312"),
			Name:          "Alpha New",
			Slug:          "alpha",
			Schema:        schema,
			EnvironmentID: envID,
			CreatedAt:     base.Add(2 * time.Hour),
			UpdatedAt:     base.Add(2 * time.Hour),
		},
		{
			ID:            mustUUID("00000000-0000-0000-0000-000000000313"),
			Name:          "Bravo",
			Slug:          "bravo",
			Schema:        schema,
			EnvironmentID: envID,
			CreatedAt:     base.Add(-3 * time.Hour),
			UpdatedAt:     base.Add(-3 * time.Hour),
		},
	}

	for _, record := range records {
		if _, err := bunDB.NewInsert().Model(record).Exec(ctx); err != nil {
			t.Fatalf("insert content type %s: %v", record.ID, err)
		}
	}

	repo := content.NewBunContentTypeRepository(bunDB)
	expected := []uuid.UUID{records[0].ID, records[1].ID, records[2].ID}

	assertOrder := func(label string, got []*content.ContentType) {
		t.Helper()
		if len(got) != len(expected) {
			t.Fatalf("%s: expected %d records, got %d", label, len(expected), len(got))
		}
		for i, id := range expected {
			if got[i].ID != id {
				t.Fatalf("%s: expected index %d to be %s, got %s", label, i, id, got[i].ID)
			}
		}
	}

	listed, err := repo.List(ctx, envID.String())
	if err != nil {
		t.Fatalf("list content types: %v", err)
	}
	assertOrder("list", listed)

	searched, err := repo.Search(ctx, "a", envID.String())
	if err != nil {
		t.Fatalf("search content types: %v", err)
	}
	assertOrder("search", searched)
}

func registerContentModels(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()
	models := []any{
		(*content.Locale)(nil),
		(*content.ContentType)(nil),
		(*content.Content)(nil),
		(*content.ContentTranslation)(nil),
	}

	for _, model := range models {
		if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			t.Fatalf("create table %T: %v", model, err)
		}
	}
	if _, err := db.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_content_translations_content_locale_unique ON content_translations(content_id, locale_id)"); err != nil {
		t.Fatalf("create index idx_content_translations_content_locale_unique: %v", err)
	}
	if _, err := db.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_content_translations_group_locale_unique ON content_translations(translation_group_id, locale_id) WHERE translation_group_id IS NOT NULL"); err != nil {
		t.Fatalf("create index idx_content_translations_group_locale_unique: %v", err)
	}
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
