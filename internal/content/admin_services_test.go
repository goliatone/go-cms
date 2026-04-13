package content

import (
	"context"
	"errors"
	"testing"

	cmsschema "github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

type adminContentEmbeddedResolverStub struct{}

func (adminContentEmbeddedResolverStub) SyncEmbeddedBlocks(context.Context, uuid.UUID, []ContentTranslationInput, uuid.UUID) error {
	return nil
}

func (adminContentEmbeddedResolverStub) MergeLegacyBlocks(context.Context, *Content) error {
	return nil
}

func (adminContentEmbeddedResolverStub) MigrateEmbeddedBlocks(context.Context, string, []map[string]any) ([]map[string]any, error) {
	return nil, nil
}

func (adminContentEmbeddedResolverStub) ValidateEmbeddedBlocks(context.Context, string, []map[string]any, EmbeddedBlockValidationMode) error {
	return nil
}

func (adminContentEmbeddedResolverStub) ValidateBlockAvailability(context.Context, string, cmsschema.BlockAvailability, []map[string]any) error {
	return nil
}

func TestAdminContentReadServiceGetReturnsTranslationMissing(t *testing.T) {
	t.Parallel()

	localeRepo := NewMemoryLocaleRepository()
	en := &Locale{ID: uuid.New(), Code: "en", Display: "English", IsActive: true, IsDefault: true}
	es := &Locale{ID: uuid.New(), Code: "es", Display: "Spanish", IsActive: true}
	localeRepo.Put(en)
	localeRepo.Put(es)

	typeRepo := NewMemoryContentTypeRepository()
	contentRepo := NewMemoryContentRepository()
	service := NewService(contentRepo, typeRepo, localeRepo, WithEmbeddedBlocksResolver(adminContentEmbeddedResolverStub{}))
	adminRead := NewAdminContentReadService(service, NewContentTypeService(typeRepo), localeRepo, nil)

	contentType, err := typeRepo.Create(context.Background(), &ContentType{
		ID:     uuid.New(),
		Name:   "Article",
		Slug:   "article",
		Schema: map[string]any{"type": "object"},
		Capabilities: map[string]any{
			"navigation": map[string]any{
				"enabled":            true,
				"eligible_locations": []any{"site.main"},
				"default_locations":  []any{"site.main"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	record, err := service.Create(context.Background(), CreateContentRequest{
		ContentTypeID:            contentType.ID,
		Slug:                     "hello-world",
		Status:                   "draft",
		CreatedBy:                uuid.New(),
		UpdatedBy:                uuid.New(),
		AllowMissingTranslations: true,
		Translations: []ContentTranslationInput{{
			Locale:  "en",
			Title:   "Hello",
			Content: map[string]any{"body": "world"},
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	_, err = adminRead.Get(context.Background(), record.ID.String(), interfaces.AdminContentGetOptions{
		Locale:                   "es",
		AllowMissingTranslations: false,
	})
	if !errors.Is(err, interfaces.ErrTranslationMissing) {
		t.Fatalf("expected ErrTranslationMissing, got %v", err)
	}
}

func TestAdminContentWriteServiceCreatePersistsEmbeddedBlocksAndMetadata(t *testing.T) {
	t.Parallel()

	localeRepo := NewMemoryLocaleRepository()
	en := &Locale{ID: uuid.New(), Code: "en", Display: "English", IsActive: true, IsDefault: true}
	localeRepo.Put(en)

	typeRepo := NewMemoryContentTypeRepository()
	contentRepo := NewMemoryContentRepository()
	service := NewService(contentRepo, typeRepo, localeRepo, WithEmbeddedBlocksResolver(adminContentEmbeddedResolverStub{}))
	adminWrite := NewAdminContentWriteService(service, NewContentTypeService(typeRepo), localeRepo)

	contentType, err := typeRepo.Create(context.Background(), &ContentType{
		ID:     uuid.New(),
		Name:   "Article",
		Slug:   "article",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	record, err := adminWrite.Create(context.Background(), interfaces.AdminContentCreateRequest{
		ContentTypeID:          contentType.ID,
		Title:                  "Hello",
		Slug:                   "hello-world",
		Locale:                 "en",
		Status:                 "draft",
		CreatedBy:              uuid.New(),
		UpdatedBy:              uuid.New(),
		Navigation:             map[string]string{"site.main": "show"},
		EffectiveMenuLocations: []string{"site.main"},
		EmbeddedBlocks: []map[string]any{
			{"_type": "hero", "headline": "Welcome"},
		},
		Data: map[string]any{"body": "world"},
	})
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}
	if record == nil || len(record.EmbeddedBlocks) != 1 {
		t.Fatalf("expected embedded blocks in projected record, got %+v", record)
	}
	stored, err := service.Get(context.Background(), record.ID, WithTranslations())
	if err != nil {
		t.Fatalf("get stored content: %v", err)
	}
	embedded, ok := ExtractEmbeddedBlocks(stored.Translations[0].Content)
	if !ok || len(embedded) != 1 {
		t.Fatalf("expected embedded blocks in stored translation payload, got %+v", stored.Translations[0].Content)
	}
}

func TestAdminContentWriteServiceCreateTranslationForwardsPathRouteKeyAndMetadata(t *testing.T) {
	t.Parallel()

	localeRepo := NewMemoryLocaleRepository()
	en := &Locale{ID: uuid.New(), Code: "en", Display: "English", IsActive: true, IsDefault: true}
	fr := &Locale{ID: uuid.New(), Code: "fr", Display: "French", IsActive: true}
	localeRepo.Put(en)
	localeRepo.Put(fr)

	typeRepo := NewMemoryContentTypeRepository()
	contentRepo := NewMemoryContentRepository()
	service := NewService(contentRepo, typeRepo, localeRepo, WithEmbeddedBlocksResolver(adminContentEmbeddedResolverStub{}))
	adminWrite := NewAdminContentWriteService(service, NewContentTypeService(typeRepo), localeRepo)

	contentType, err := typeRepo.Create(context.Background(), &ContentType{
		ID:     uuid.New(),
		Name:   "Page",
		Slug:   "page",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	source, err := service.Create(context.Background(), CreateContentRequest{
		ContentTypeID: contentType.ID,
		Slug:          "home",
		Status:        "draft",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []ContentTranslationInput{{
			Locale: "en",
			Title:  "Home",
			Content: map[string]any{
				"path":      "/home",
				"route_key": "pages/home",
				"body":      "Welcome",
			},
		}},
	})
	if err != nil {
		t.Fatalf("create source content: %v", err)
	}

	record, err := adminWrite.CreateTranslation(context.Background(), interfaces.AdminContentCreateTranslationRequest{
		SourceID:     source.ID,
		SourceLocale: "en",
		TargetLocale: "fr",
		Status:       "draft",
		Path:         "/fr/accueil",
		RouteKey:     "pages/home",
		Metadata: map[string]any{
			"translation_create_locale": map[string]any{"idempotency_key": "home-fr"},
		},
	})
	if err != nil {
		t.Fatalf("admin create translation: %v", err)
	}
	if record == nil {
		t.Fatalf("expected created translation record")
	}
	if got := record.Locale; got != "fr" {
		t.Fatalf("expected locale fr, got %q", got)
	}
	if got := record.Data["path"]; got != "/fr/accueil" {
		t.Fatalf("expected localized path /fr/accueil, got %v", got)
	}
	if got := record.Data["route_key"]; got != "pages/home" {
		t.Fatalf("expected route_key pages/home, got %v", got)
	}
	replay, _ := record.Metadata["translation_create_locale"].(map[string]any)
	if got := replay["idempotency_key"]; got != "home-fr" {
		t.Fatalf("expected translation metadata to be projected in admin record, got %+v", record.Metadata)
	}
}

func TestAdminContentDBReadServiceListAppliesSQLPaginationAndSort(t *testing.T) {
	ctx := context.Background()
	bunDB, service, typeRepo, localeRepo := newAdminContentDBTestFixture(t)
	adminRead := NewAdminContentDBReadService(bunDB, service, NewContentTypeService(typeRepo), localeRepo, nil)

	contentType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "Article",
		Slug:   "article",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	createAdminContentDBRecord(t, ctx, service, contentType.ID, "alpha", "draft", []ContentTranslationInput{
		{Locale: "en", Title: "Alpha", Content: map[string]any{"body": "alpha"}},
		{Locale: "es", Title: "Alfa", Content: map[string]any{"body": "alfa"}},
	})
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "bravo", "published", []ContentTranslationInput{
		{Locale: "en", Title: "Bravo", Content: map[string]any{"body": "bravo"}},
	})
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "charlie", "draft", []ContentTranslationInput{
		{Locale: "fr", Title: "Charlie", Content: map[string]any{"body": "charlie"}},
	})

	records, total, err := adminRead.List(ctx, interfaces.AdminContentListOptions{
		Locale:         "en",
		FallbackLocale: "fr",
		SortBy:         "title",
		Page:           1,
		PerPage:        2,
	})
	if err != nil {
		t.Fatalf("list admin content: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(records) != 2 {
		t.Fatalf("expected page size 2, got %d", len(records))
	}
	if records[0].Title != "Alpha" || records[1].Title != "Bravo" {
		t.Fatalf("expected sorted page [Alpha Bravo], got [%s %s]", records[0].Title, records[1].Title)
	}
}

func TestAdminContentDBReadServiceListFiltersByResolvedLocaleAndSearch(t *testing.T) {
	ctx := context.Background()
	bunDB, service, typeRepo, localeRepo := newAdminContentDBTestFixture(t)
	adminRead := NewAdminContentDBReadService(bunDB, service, NewContentTypeService(typeRepo), localeRepo, nil)

	contentType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "Article",
		Slug:   "article",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	createAdminContentDBRecord(t, ctx, service, contentType.ID, "alpha", "draft", []ContentTranslationInput{
		{Locale: "en", Title: "Alpha", Content: map[string]any{"body": "alpha"}},
	})
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "bonjour", "draft", []ContentTranslationInput{
		{Locale: "fr", Title: "Bonjour", Content: map[string]any{"body": "bonjour"}},
	})

	localeRecords, total, err := adminRead.List(ctx, interfaces.AdminContentListOptions{
		Locale:                   "en",
		FallbackLocale:           "fr",
		AllowMissingTranslations: true,
		Filters:                  map[string]any{"locale": "fr"},
	})
	if err != nil {
		t.Fatalf("list with locale filter: %v", err)
	}
	if total != 1 || len(localeRecords) != 1 {
		t.Fatalf("expected one locale-filtered record, got total=%d len=%d", total, len(localeRecords))
	}
	if localeRecords[0].Slug != "bonjour" || localeRecords[0].ResolvedLocale != "fr" {
		t.Fatalf("expected fallback-fr record, got %+v", localeRecords[0])
	}

	searchRecords, total, err := adminRead.List(ctx, interfaces.AdminContentListOptions{
		Locale: "en",
		Search: "alp",
	})
	if err != nil {
		t.Fatalf("list with search: %v", err)
	}
	if total != 1 || len(searchRecords) != 1 {
		t.Fatalf("expected one search record, got total=%d len=%d", total, len(searchRecords))
	}
	if searchRecords[0].Slug != "alpha" {
		t.Fatalf("expected alpha search result, got %+v", searchRecords[0])
	}
}

func newAdminContentDBTestFixture(t *testing.T) (*bun.DB, Service, ContentTypeRepository, LocaleRepository) {
	t.Helper()

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = bunDB.Close()
	})

	registerAdminContentDBModels(t, bunDB)

	localeRepo := NewBunLocaleRepository(bunDB)
	typeRepo := NewBunContentTypeRepository(bunDB)
	contentRepo := NewBunContentRepository(bunDB)
	service := NewService(contentRepo, typeRepo, localeRepo, WithEmbeddedBlocksResolver(adminContentEmbeddedResolverStub{}))

	for _, locale := range []*Locale{
		{ID: uuid.New(), Code: "en", Display: "English", IsActive: true, IsDefault: true},
		{ID: uuid.New(), Code: "es", Display: "Spanish", IsActive: true},
		{ID: uuid.New(), Code: "fr", Display: "French", IsActive: true},
	} {
		if _, err := bunDB.NewInsert().Model(locale).Exec(context.Background()); err != nil {
			t.Fatalf("insert locale %s: %v", locale.Code, err)
		}
	}

	return bunDB, service, typeRepo, localeRepo
}

func registerAdminContentDBModels(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()
	models := []any{
		(*Locale)(nil),
		(*ContentType)(nil),
		(*Content)(nil),
		(*ContentTranslation)(nil),
	}
	for _, model := range models {
		if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			t.Fatalf("create table %T: %v", model, err)
		}
	}
	if _, err := db.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_content_translations_content_locale_unique ON content_translations(content_id, locale_id)"); err != nil {
		t.Fatalf("create translation locale unique index: %v", err)
	}
}

func createAdminContentDBRecord(t *testing.T, ctx context.Context, svc Service, contentTypeID uuid.UUID, slug, status string, translations []ContentTranslationInput) *Content {
	t.Helper()
	record, err := svc.Create(ctx, CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          slug,
		Status:        status,
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  translations,
	})
	if err != nil {
		t.Fatalf("create content %s: %v", slug, err)
	}
	return record
}
