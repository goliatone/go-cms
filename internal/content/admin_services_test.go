package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	replay, ok := record.Metadata["translation_create_locale"].(map[string]any)
	if !ok {
		t.Fatalf("expected translation metadata to be projected in admin record, got %+v", record.Metadata)
	}
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

func TestAdminContentDBReadServiceListAppliesContentTypeIDScopeBeforeCount(t *testing.T) {
	ctx := context.Background()
	bunDB, service, typeRepo, localeRepo := newAdminContentDBTestFixture(t)
	adminRead := NewAdminContentDBReadService(bunDB, service, NewContentTypeService(typeRepo), localeRepo, nil)

	archiveType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "Archive Event",
		Slug:   "archive-event",
		Schema: map[string]any{"type": "object"},
		Capabilities: map[string]any{
			"translations":        true,
			"panel_slug":          "archive_event",
			"search_content_type": "archive_event",
		},
	})
	if err != nil {
		t.Fatalf("create archive content type: %v", err)
	}
	otherType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "Archive Event Session",
		Slug:   "archive-event-session",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create other content type: %v", err)
	}

	for i := range 30 {
		status := "published"
		if i%2 == 0 {
			status = "draft"
		}
		createAdminContentDBRecord(t, ctx, service, archiveType.ID, fmt.Sprintf("archive-event-%02d", i), status, []ContentTranslationInput{{
			Locale: "en",
			Title:  fmt.Sprintf("Archive Event %02d", i),
			Content: map[string]any{
				"body": fmt.Sprintf("event %02d", i),
			},
		}})
	}
	for i := range 5 {
		createAdminContentDBRecord(t, ctx, service, otherType.ID, fmt.Sprintf("other-%02d", i), "draft", []ContentTranslationInput{{
			Locale:  "en",
			Title:   fmt.Sprintf("Other %02d", i),
			Content: map[string]any{"body": "other"},
		}})
	}

	records, total, err := adminRead.List(ctx, interfaces.AdminContentListOptions{
		ContentTypeID: archiveType.ID.String(),
		Locale:        "en",
		Page:          2,
		PerPage:       10,
		SortBy:        "slug",
	})
	if err != nil {
		t.Fatalf("list scoped archive content: %v", err)
	}
	if total != 30 {
		t.Fatalf("expected scoped total 30 before pagination, got %d", total)
	}
	if len(records) != 10 {
		t.Fatalf("expected page size 10, got %d", len(records))
	}
	if records[0].Slug != "archive-event-10" || records[9].Slug != "archive-event-19" {
		t.Fatalf("expected second slug page archive-event-10..19, got first=%q last=%q", records[0].Slug, records[9].Slug)
	}
	for _, record := range records {
		if record.ContentTypeSlug != "archive-event" {
			t.Fatalf("expected physical content type slug archive-event, got %+v", record)
		}
	}
}

func TestAdminContentDBReadServiceListFamiliesCountsBeforeVariantHydration(t *testing.T) {
	ctx := context.Background()
	bunDB, service, typeRepo, localeRepo := newAdminContentDBTestFixture(t)
	adminRead := NewAdminContentDBReadService(bunDB, service, NewContentTypeService(typeRepo), localeRepo, nil)

	archiveType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "Archive Event",
		Slug:   "archive-event",
		Schema: map[string]any{"type": "object"},
		Capabilities: map[string]any{
			"translations":        true,
			"panel_slug":          "archive_event",
			"search_content_type": "archive_event",
		},
	})
	if err != nil {
		t.Fatalf("create archive content type: %v", err)
	}

	familyIDs := make([]uuid.UUID, 30)
	for i := range 30 {
		familyID := uuid.NewMD5(uuid.NameSpaceURL, fmt.Appendf(nil, "archive-event-family-%02d", i))
		familyIDs[i] = familyID
		status := "published"
		if i%2 == 0 {
			status = "draft"
		}
		createAdminContentDBRecord(t, ctx, service, archiveType.ID, fmt.Sprintf("archive-event-%02d", i), status, []ContentTranslationInput{
			{
				Locale:   "en",
				FamilyID: &familyID,
				Title:    fmt.Sprintf("Archive Event %02d", i),
				Content:  map[string]any{"body": fmt.Sprintf("event %02d en", i)},
			},
			{
				Locale:   "es",
				FamilyID: &familyID,
				Title:    fmt.Sprintf("Evento Archivo %02d", i),
				Content:  map[string]any{"body": fmt.Sprintf("event %02d es", i)},
			},
		})
	}

	familyRead, ok := adminRead.(interfaces.AdminContentFamilyReadService)
	if !ok {
		t.Fatalf("expected DB admin read service to expose optimized family reads")
	}
	result, err := familyRead.ListFamilies(ctx, interfaces.AdminContentFamilyListOptions{
		ContentTypeID:   archiveType.ID.String(),
		Locale:          "en",
		Page:            2,
		PerPage:         10,
		SortBy:          "title",
		IncludeData:     true,
		IncludeMetadata: true,
	})
	if err != nil {
		t.Fatalf("list scoped archive families: %v", err)
	}
	if result.FamilyTotal != 30 {
		t.Fatalf("expected 30 family total before pagination, got %d", result.FamilyTotal)
	}
	if result.Page != 2 || result.PerPage != 10 {
		t.Fatalf("expected page metadata 2/10, got page=%d per_page=%d", result.Page, result.PerPage)
	}
	if len(result.Families) != 10 {
		t.Fatalf("expected 10 paged families, got %d", len(result.Families))
	}
	if got := result.Families[0].FamilyID; got != familyIDs[10].String() {
		t.Fatalf("expected second page to start at family %s, got %s", familyIDs[10], got)
	}
	if len(result.Families[0].Variants) != 2 {
		t.Fatalf("expected selected family variants to be hydrated, got %+v", result.Families[0].Variants)
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

func TestAdminContentDBReadServiceListFiltersByTranslationDataField(t *testing.T) {
	ctx := context.Background()
	bunDB, service, typeRepo, localeRepo := newAdminContentDBTestFixture(t)
	adminRead := NewAdminContentDBReadService(bunDB, service, NewContentTypeService(typeRepo), localeRepo, nil)

	contentType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "News",
		Slug:   "news",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	matchingFamilyID := uuid.New()
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "archive-release", "published", []ContentTranslationInput{
		{Locale: "en", FamilyID: &matchingFamilyID, Title: "Archive Release", Content: map[string]any{"path": "/en/news/archive-release"}},
		{Locale: "bo", FamilyID: &matchingFamilyID, Title: "Archive Release BO", Content: map[string]any{"path": "/bo/news/archive-release"}},
	})
	otherFamilyID := uuid.New()
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "other-news", "published", []ContentTranslationInput{
		{Locale: "en", FamilyID: &otherFamilyID, Title: "Other News", Content: map[string]any{"path": "/en/news/other-news"}},
		{Locale: "bo", FamilyID: &otherFamilyID, Title: "Other News BO", Content: map[string]any{"path": "/bo/news/other-news"}},
	})

	records, total, err := adminRead.List(ctx, interfaces.AdminContentListOptions{
		ContentTypeID:            contentType.ID.String(),
		Locale:                   "en",
		AllowMissingTranslations: true,
		IncludeData:              true,
		Filters:                  map[string]any{"path__ilike": "/bo/news/archive-release"},
	})
	if err != nil {
		t.Fatalf("list with non-visible path filter: %v", err)
	}
	if total != 0 || len(records) != 0 {
		t.Fatalf("expected flat path filter to match visible row data only, got total=%d len=%d records=%+v", total, len(records), records)
	}

	records, total, err = adminRead.List(ctx, interfaces.AdminContentListOptions{
		ContentTypeID:            contentType.ID.String(),
		Locale:                   "en",
		AllowMissingTranslations: true,
		IncludeData:              true,
		Filters:                  map[string]any{"path__ilike": "/en/news/archive-release"},
	})
	if err != nil {
		t.Fatalf("list with path filter: %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("expected one path-filtered record, got total=%d len=%d", total, len(records))
	}
	if records[0].Slug != "archive-release" {
		t.Fatalf("expected archive-release record, got %+v", records[0])
	}
}

func TestAdminContentDBReadServiceListFiltersByCSVInValue(t *testing.T) {
	ctx := context.Background()
	bunDB, service, typeRepo, localeRepo := newAdminContentDBTestFixture(t)
	adminRead := NewAdminContentDBReadService(bunDB, service, NewContentTypeService(typeRepo), localeRepo, nil)

	contentType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "News",
		Slug:   "news",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	createAdminContentDBRecord(t, ctx, service, contentType.ID, "draft-news", "draft", []ContentTranslationInput{
		{Locale: "en", Title: "Draft News", Content: map[string]any{"path": "/en/news/draft"}},
	})
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "published-news", "published", []ContentTranslationInput{
		{Locale: "en", Title: "Published News", Content: map[string]any{"path": "/en/news/published"}},
	})
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "archived-news", "archived", []ContentTranslationInput{
		{Locale: "en", Title: "Archived News", Content: map[string]any{"path": "/en/news/archived"}},
	})

	records, total, err := adminRead.List(ctx, interfaces.AdminContentListOptions{
		ContentTypeID: contentType.ID.String(),
		Locale:        "en",
		Filters:       map[string]any{"status__in": "draft,published"},
	})
	if err != nil {
		t.Fatalf("list with csv in filter: %v", err)
	}
	if total != 2 || len(records) != 2 {
		t.Fatalf("expected two csv-in records, got total=%d len=%d records=%+v", total, len(records), records)
	}
	for _, record := range records {
		if record.Status == "archived" {
			t.Fatalf("did not expect archived record in csv-in results: %+v", records)
		}
	}

	records, total, err = adminRead.List(ctx, interfaces.AdminContentListOptions{
		ContentTypeID: contentType.ID.String(),
		Locale:        "en",
		Filters:       map[string]any{"slug__ilike": "published"},
	})
	if err != nil {
		t.Fatalf("list with native ilike filter: %v", err)
	}
	if total != 1 || len(records) != 1 || records[0].Slug != "published-news" {
		t.Fatalf("expected one native ilike record, got total=%d records=%+v", total, records)
	}
}

func TestAdminContentDBReadServiceListFiltersEscapesLikeWildcards(t *testing.T) {
	ctx := context.Background()
	bunDB, service, typeRepo, localeRepo := newAdminContentDBTestFixture(t)
	adminRead := NewAdminContentDBReadService(bunDB, service, NewContentTypeService(typeRepo), localeRepo, nil)

	contentType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "News",
		Slug:   "news",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	createAdminContentDBRecord(t, ctx, service, contentType.ID, "literal-underscore", "published", []ContentTranslationInput{
		{Locale: "en", Title: "Literal Underscore", Content: map[string]any{"path": "/en/news/under_score"}},
	})
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "wildcard-candidate", "published", []ContentTranslationInput{
		{Locale: "en", Title: "Wildcard Candidate", Content: map[string]any{"path": "/en/news/underXscore"}},
	})

	records, total, err := adminRead.List(ctx, interfaces.AdminContentListOptions{
		ContentTypeID: contentType.ID.String(),
		Locale:        "en",
		IncludeData:   true,
		Filters:       map[string]any{"path__ilike": "under_score"},
	})
	if err != nil {
		t.Fatalf("list with literal wildcard filter: %v", err)
	}
	if total != 1 || len(records) != 1 {
		t.Fatalf("expected one literal underscore match, got total=%d len=%d records=%+v", total, len(records), records)
	}
	if records[0].Slug != "literal-underscore" {
		t.Fatalf("expected literal underscore record, got %+v", records[0])
	}
}

func TestAdminContentDBReadServiceListFamiliesFiltersByTranslationDataField(t *testing.T) {
	ctx := context.Background()
	bunDB, service, typeRepo, localeRepo := newAdminContentDBTestFixture(t)
	adminRead := NewAdminContentDBReadService(bunDB, service, NewContentTypeService(typeRepo), localeRepo, nil)

	contentType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "News",
		Slug:   "news",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	matchingFamilyID := uuid.New()
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "archive-release", "published", []ContentTranslationInput{
		{Locale: "en", FamilyID: &matchingFamilyID, Title: "Archive Release", Content: map[string]any{"path": "/en/news/archive-release"}},
		{Locale: "bo", FamilyID: &matchingFamilyID, Title: "Archive Release BO", Content: map[string]any{"path": "/bo/news/archive-release"}},
	})
	otherFamilyID := uuid.New()
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "other-news", "published", []ContentTranslationInput{
		{Locale: "en", FamilyID: &otherFamilyID, Title: "Other News", Content: map[string]any{"path": "/en/news/other-news"}},
		{Locale: "bo", FamilyID: &otherFamilyID, Title: "Other News BO", Content: map[string]any{"path": "/bo/news/other-news"}},
	})

	familyRead, ok := adminRead.(interfaces.AdminContentFamilyReadService)
	if !ok {
		t.Fatalf("expected DB admin read service to expose optimized family reads")
	}
	result, err := familyRead.ListFamilies(ctx, interfaces.AdminContentFamilyListOptions{
		ContentTypeID:            contentType.ID.String(),
		Locale:                   "en",
		AllowMissingTranslations: true,
		IncludeData:              true,
		Filters:                  map[string]any{"path__ilike": "/bo/news/archive-release"},
	})
	if err != nil {
		t.Fatalf("list families with path filter: %v", err)
	}
	if result.FamilyTotal != 1 || len(result.Families) != 1 {
		t.Fatalf("expected one path-filtered family, got total=%d len=%d", result.FamilyTotal, len(result.Families))
	}
	if result.Families[0].FamilyID != matchingFamilyID.String() {
		t.Fatalf("expected matching family %s, got %+v", matchingFamilyID, result.Families[0])
	}
	if len(result.Families[0].Variants) != 2 {
		t.Fatalf("expected family variants to remain hydrated, got %+v", result.Families[0].Variants)
	}
}

func TestAdminContentDBReadServiceListDeclinesUnsupportedFilterOperator(t *testing.T) {
	ctx := context.Background()
	bunDB, service, typeRepo, localeRepo := newAdminContentDBTestFixture(t)
	adminRead := NewAdminContentDBReadService(bunDB, service, NewContentTypeService(typeRepo), localeRepo, nil)

	contentType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "News",
		Slug:   "news",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	createAdminContentDBRecord(t, ctx, service, contentType.ID, "archive-release", "published", []ContentTranslationInput{
		{Locale: "en", Title: "Archive Release", Content: map[string]any{"path": "/en/news/archive-release"}},
	})

	_, _, err = adminRead.List(ctx, interfaces.AdminContentListOptions{
		ContentTypeID: contentType.ID.String(),
		Locale:        "en",
		Filters:       map[string]any{"path__contains": "archive"},
	})
	if !errors.Is(err, interfaces.ErrAdminContentFamilyReadUnsupported) {
		t.Fatalf("expected unsupported read error, got %v", err)
	}
}

func TestAdminContentDBReadServiceListFamiliesDeclinesUnsupportedFilterOperator(t *testing.T) {
	ctx := context.Background()
	bunDB, service, typeRepo, localeRepo := newAdminContentDBTestFixture(t)
	adminRead := NewAdminContentDBReadService(bunDB, service, NewContentTypeService(typeRepo), localeRepo, nil)

	contentType, err := typeRepo.Create(ctx, &ContentType{
		ID:     uuid.New(),
		Name:   "News",
		Slug:   "news",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}

	familyID := uuid.New()
	createAdminContentDBRecord(t, ctx, service, contentType.ID, "archive-release", "published", []ContentTranslationInput{
		{Locale: "en", FamilyID: &familyID, Title: "Archive Release", Content: map[string]any{"path": "/en/news/archive-release"}},
	})

	familyRead, ok := adminRead.(interfaces.AdminContentFamilyReadService)
	if !ok {
		t.Fatalf("expected DB admin read service to expose optimized family reads")
	}
	_, err = familyRead.ListFamilies(ctx, interfaces.AdminContentFamilyListOptions{
		ContentTypeID: contentType.ID.String(),
		Locale:        "en",
		Filters:       map[string]any{"path__contains": "archive"},
	})
	if !errors.Is(err, interfaces.ErrAdminContentFamilyReadUnsupported) {
		t.Fatalf("expected unsupported family read error, got %v", err)
	}
}

func newAdminContentDBTestFixture(t *testing.T) (*bun.DB, Service, ContentTypeRepository, LocaleRepository) {
	t.Helper()

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)
	cleanupAdminContentDBTestFixture(t, sqlDB, bunDB)

	registerAdminContentDBModels(t, bunDB)

	localeRepo := NewBunLocaleRepository(bunDB)
	typeRepo := NewBunContentTypeRepository(bunDB)
	contentRepo := NewBunContentRepository(bunDB)
	service := NewService(contentRepo, typeRepo, localeRepo, WithEmbeddedBlocksResolver(adminContentEmbeddedResolverStub{}))

	for _, locale := range []*Locale{
		{ID: uuid.New(), Code: "en", Display: "English", IsActive: true, IsDefault: true},
		{ID: uuid.New(), Code: "es", Display: "Spanish", IsActive: true},
		{ID: uuid.New(), Code: "fr", Display: "French", IsActive: true},
		{ID: uuid.New(), Code: "bo", Display: "Tibetan", IsActive: true},
	} {
		if _, err := bunDB.NewInsert().Model(locale).Exec(context.Background()); err != nil {
			t.Fatalf("insert locale %s: %v", locale.Code, err)
		}
	}

	return bunDB, service, typeRepo, localeRepo
}

func cleanupAdminContentDBTestFixture(t *testing.T, sqlDB *sql.DB, bunDB *bun.DB) {
	t.Helper()
	t.Cleanup(func() {
		if closeErr := bunDB.Close(); closeErr != nil {
			t.Errorf("close bun db: %v", closeErr)
		}
		if closeErr := sqlDB.Close(); closeErr != nil {
			t.Errorf("close sqlite db: %v", closeErr)
		}
	})
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
