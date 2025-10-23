package content_test

import (
	"context"
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
