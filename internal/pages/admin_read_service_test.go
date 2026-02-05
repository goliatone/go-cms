package pages_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type adminReadExpectations struct {
	title           string
	path            string
	summary         string
	metaTitle       string
	metaDescription string
	tags            []string
	schemaVersion   string
	requestedLocale string
	resolvedLocale  string
	contentBody     string
}

type adminReadFixture struct {
	ctx                context.Context
	svc                interfaces.AdminPageReadService
	pageID             uuid.UUID
	templateID         uuid.UUID
	translationGroupID uuid.UUID
	listOpts           interfaces.AdminPageListOptions
	getOpts            interfaces.AdminPageGetOptions
	expected           adminReadExpectations
}

type adminReadBlocksFixture struct {
	ctx    context.Context
	svc    interfaces.AdminPageReadService
	pageID uuid.UUID
}

func TestAdminPageReadServiceListGetParity(t *testing.T) {
	fixture := newAdminReadFixture(t)

	list, total, err := fixture.svc.List(fixture.ctx, fixture.listOpts)
	if err != nil {
		t.Fatalf("list admin records: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(list) != 1 {
		t.Fatalf("expected list length 1, got %d", len(list))
	}

	get, err := fixture.svc.Get(fixture.ctx, fixture.pageID.String(), fixture.getOpts)
	if err != nil {
		t.Fatalf("get admin record: %v", err)
	}
	if get == nil {
		t.Fatalf("expected record, got nil")
	}

	if !reflect.DeepEqual(list[0], *get) {
		t.Fatalf("expected list/get parity, list=%#v get=%#v", list[0], *get)
	}
}

func TestAdminPageReadServiceHydration(t *testing.T) {
	fixture := newAdminReadFixture(t)

	record, err := fixture.svc.Get(fixture.ctx, fixture.pageID.String(), fixture.getOpts)
	if err != nil {
		t.Fatalf("get admin record: %v", err)
	}
	if record == nil {
		t.Fatalf("expected record, got nil")
	}

	if record.Title != fixture.expected.title {
		t.Fatalf("expected title %q, got %q", fixture.expected.title, record.Title)
	}
	if record.Path != fixture.expected.path {
		t.Fatalf("expected path %q, got %q", fixture.expected.path, record.Path)
	}
	if record.RequestedLocale != fixture.expected.requestedLocale {
		t.Fatalf("expected requested locale %q, got %q", fixture.expected.requestedLocale, record.RequestedLocale)
	}
	if record.ResolvedLocale != fixture.expected.resolvedLocale {
		t.Fatalf("expected resolved locale %q, got %q", fixture.expected.resolvedLocale, record.ResolvedLocale)
	}
	if record.Summary == nil || *record.Summary != fixture.expected.summary {
		t.Fatalf("expected summary %q, got %#v", fixture.expected.summary, record.Summary)
	}
	if record.MetaTitle != fixture.expected.metaTitle {
		t.Fatalf("expected meta title %q, got %q", fixture.expected.metaTitle, record.MetaTitle)
	}
	if record.MetaDescription != fixture.expected.metaDescription {
		t.Fatalf("expected meta description %q, got %q", fixture.expected.metaDescription, record.MetaDescription)
	}
	if !reflect.DeepEqual(record.Tags, fixture.expected.tags) {
		t.Fatalf("expected tags %#v, got %#v", fixture.expected.tags, record.Tags)
	}
	if record.SchemaVersion != fixture.expected.schemaVersion {
		t.Fatalf("expected schema version %q, got %q", fixture.expected.schemaVersion, record.SchemaVersion)
	}
	if record.TranslationGroupID == nil || *record.TranslationGroupID != fixture.translationGroupID {
		t.Fatalf("expected translation group %q, got %#v", fixture.translationGroupID, record.TranslationGroupID)
	}

	payload, ok := record.Content.(map[string]any)
	if !ok {
		t.Fatalf("expected content payload map, got %T", record.Content)
	}
	if got, ok := payload["content"].(string); !ok || got != fixture.expected.contentBody {
		t.Fatalf("expected content body %q, got %#v", fixture.expected.contentBody, payload["content"])
	}

	if record.Data == nil {
		t.Fatalf("expected data payload")
	}
	expectDataString(t, record.Data, "title", fixture.expected.title)
	expectDataString(t, record.Data, "path", fixture.expected.path)
	expectDataString(t, record.Data, "summary", fixture.expected.summary)
	expectDataString(t, record.Data, "meta_title", fixture.expected.metaTitle)
	expectDataString(t, record.Data, "meta_description", fixture.expected.metaDescription)
	expectDataString(t, record.Data, "requested_locale", fixture.expected.requestedLocale)
	expectDataString(t, record.Data, "resolved_locale", fixture.expected.resolvedLocale)
	expectDataTags(t, record.Data, fixture.expected.tags)
}

func TestAdminPageReadServiceFallbackLocale(t *testing.T) {
	ctx := context.Background()
	localeRepo := content.NewMemoryLocaleRepository()
	localeEN := uuid.New()
	localeES := uuid.New()
	localeRepo.Put(&content.Locale{
		ID:       localeEN,
		Code:     "en",
		Display:  "English",
		IsActive: true,
	})
	localeRepo.Put(&content.Locale{
		ID:       localeES,
		Code:     "es",
		Display:  "Spanish",
		IsActive: true,
	})

	contentTypeRepo := content.NewMemoryContentTypeRepository()
	contentTypeID := uuid.New()
	seedContentType(t, contentTypeRepo, &content.ContentType{
		ID:            contentTypeID,
		Name:          "Page",
		SchemaVersion: "schema-v1",
	})

	contentRepo := content.NewMemoryContentRepository()
	contentID := uuid.New()
	_, err := contentRepo.Create(ctx, &content.Content{
		ID:            contentID,
		ContentTypeID: contentTypeID,
		Slug:          "content",
		Status:        "draft",
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  localeES,
				Title:     "Contenido",
				Content: map[string]any{
					"content": "Hola",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("seed content: %v", err)
	}

	pageRepo := pages.NewMemoryPageRepository()
	pageID := uuid.New()
	_, err = pageRepo.Create(ctx, &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: uuid.New(),
		Slug:       "home",
		Status:     "draft",
		Translations: []*pages.PageTranslation{
			{
				ID:       uuid.New(),
				PageID:   pageID,
				LocaleID: localeES,
				Locale:   "es",
				Title:    "Titulo ES",
				Path:     "/es",
			},
		},
	})
	if err != nil {
		t.Fatalf("seed page: %v", err)
	}

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)
	pageSvc := pages.NewService(pageRepo, contentRepo, localeRepo)
	adminSvc := pages.NewAdminPageReadService(pageSvc, contentSvc, localeRepo)

	record, err := adminSvc.Get(ctx, pageID.String(), interfaces.AdminPageGetOptions{
		Locale:         "en",
		FallbackLocale: "es",
		IncludeData:    true,
	})
	if err != nil {
		t.Fatalf("get admin record: %v", err)
	}
	if record.RequestedLocale != "en" {
		t.Fatalf("expected requested locale %q, got %q", "en", record.RequestedLocale)
	}
	if record.ResolvedLocale != "es" {
		t.Fatalf("expected resolved locale %q, got %q", "es", record.ResolvedLocale)
	}
	if record.Title != "Titulo ES" {
		t.Fatalf("expected title %q, got %q", "Titulo ES", record.Title)
	}
	if record.Path != "/es" {
		t.Fatalf("expected path %q, got %q", "/es", record.Path)
	}
	expectDataString(t, record.Data, "requested_locale", "en")
	expectDataString(t, record.Data, "resolved_locale", "es")
}

func TestAdminPageReadServiceMissingTranslationAllowed(t *testing.T) {
	ctx := context.Background()
	localeRepo := content.NewMemoryLocaleRepository()
	localeFR := uuid.New()
	localeRepo.Put(&content.Locale{
		ID:       localeFR,
		Code:     "fr",
		Display:  "French",
		IsActive: true,
	})

	contentTypeRepo := content.NewMemoryContentTypeRepository()
	contentTypeID := uuid.New()
	seedContentType(t, contentTypeRepo, &content.ContentType{
		ID:            contentTypeID,
		Name:          "Page",
		SchemaVersion: "schema-v1",
	})

	contentRepo := content.NewMemoryContentRepository()
	contentID := uuid.New()
	_, err := contentRepo.Create(ctx, &content.Content{
		ID:            contentID,
		ContentTypeID: contentTypeID,
		Slug:          "content",
		Status:        "draft",
	})
	if err != nil {
		t.Fatalf("seed content: %v", err)
	}

	pageRepo := pages.NewMemoryPageRepository()
	pageID := uuid.New()
	_, err = pageRepo.Create(ctx, &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: uuid.New(),
		Slug:       "home",
		Status:     "draft",
	})
	if err != nil {
		t.Fatalf("seed page: %v", err)
	}

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)
	pageSvc := pages.NewService(pageRepo, contentRepo, localeRepo)
	adminSvc := pages.NewAdminPageReadService(pageSvc, contentSvc, localeRepo)

	record, err := adminSvc.Get(ctx, pageID.String(), interfaces.AdminPageGetOptions{
		Locale:                   "fr",
		AllowMissingTranslations: true,
	})
	if err != nil {
		t.Fatalf("get admin record: %v", err)
	}
	if record.RequestedLocale != "fr" {
		t.Fatalf("expected requested locale %q, got %q", "fr", record.RequestedLocale)
	}
	if record.ResolvedLocale != "" {
		t.Fatalf("expected empty resolved locale, got %q", record.ResolvedLocale)
	}
	if record.Title != "" || record.Path != "" {
		t.Fatalf("expected empty localized fields, got title=%q path=%q", record.Title, record.Path)
	}
	if record.Summary != nil {
		t.Fatalf("expected nil summary, got %#v", record.Summary)
	}
}

func TestAdminPageReadServiceMissingTranslationError(t *testing.T) {
	ctx := context.Background()
	localeRepo := content.NewMemoryLocaleRepository()
	localeFR := uuid.New()
	localeRepo.Put(&content.Locale{
		ID:       localeFR,
		Code:     "fr",
		Display:  "French",
		IsActive: true,
	})

	contentTypeRepo := content.NewMemoryContentTypeRepository()
	contentTypeID := uuid.New()
	seedContentType(t, contentTypeRepo, &content.ContentType{
		ID:            contentTypeID,
		Name:          "Page",
		SchemaVersion: "schema-v1",
	})

	contentRepo := content.NewMemoryContentRepository()
	contentID := uuid.New()
	_, err := contentRepo.Create(ctx, &content.Content{
		ID:            contentID,
		ContentTypeID: contentTypeID,
		Slug:          "content",
		Status:        "draft",
	})
	if err != nil {
		t.Fatalf("seed content: %v", err)
	}

	pageRepo := pages.NewMemoryPageRepository()
	pageID := uuid.New()
	_, err = pageRepo.Create(ctx, &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: uuid.New(),
		Slug:       "home",
		Status:     "draft",
	})
	if err != nil {
		t.Fatalf("seed page: %v", err)
	}

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)
	pageSvc := pages.NewService(pageRepo, contentRepo, localeRepo)
	adminSvc := pages.NewAdminPageReadService(pageSvc, contentSvc, localeRepo)

	_, err = adminSvc.Get(ctx, pageID.String(), interfaces.AdminPageGetOptions{
		Locale: "fr",
	})
	if !errors.Is(err, interfaces.ErrTranslationMissing) {
		t.Fatalf("expected ErrTranslationMissing, got %v", err)
	}
	if !errors.Is(err, pages.ErrPageTranslationNotFound) {
		t.Fatalf("expected ErrPageTranslationNotFound, got %v", err)
	}
}

func TestAdminPageReadServiceEmbeddedBlocksPreferred(t *testing.T) {
	embedded := []map[string]any{{
		content.EmbeddedBlockTypeKey: "hero",
		"title":                      "Hello",
	}}
	fixture := newAdminReadBlocksFixture(t, map[string]any{
		content.EmbeddedBlocksKey: embedded,
	})

	record, err := fixture.svc.Get(fixture.ctx, fixture.pageID.String(), interfaces.AdminPageGetOptions{
		Locale:        "en",
		IncludeBlocks: true,
	})
	if err != nil {
		t.Fatalf("get admin record: %v", err)
	}
	blocksPayload, ok := record.Blocks.([]map[string]any)
	if !ok {
		t.Fatalf("expected embedded blocks payload, got %T", record.Blocks)
	}
	if !reflect.DeepEqual(blocksPayload, embedded) {
		t.Fatalf("expected embedded blocks %#v, got %#v", embedded, blocksPayload)
	}
}

func TestAdminPageReadServiceLegacyBlocksFallback(t *testing.T) {
	blockID := uuid.New()
	fixture := newAdminReadBlocksFixture(t, map[string]any{
		"content": "body",
	}, []*blocks.Instance{{
		ID: blockID,
	}})

	record, err := fixture.svc.Get(fixture.ctx, fixture.pageID.String(), interfaces.AdminPageGetOptions{
		Locale:        "en",
		IncludeBlocks: true,
	})
	if err != nil {
		t.Fatalf("get admin record: %v", err)
	}
	blocksPayload, ok := record.Blocks.([]string)
	if !ok {
		t.Fatalf("expected legacy block ids, got %T", record.Blocks)
	}
	if !reflect.DeepEqual(blocksPayload, []string{blockID.String()}) {
		t.Fatalf("expected legacy block ids %#v, got %#v", []string{blockID.String()}, blocksPayload)
	}
}

func TestAdminPageReadServiceBlocksExcludedWithoutIncludeFlag(t *testing.T) {
	fixture := newAdminReadBlocksFixture(t, map[string]any{
		content.EmbeddedBlocksKey: []map[string]any{{
			content.EmbeddedBlockTypeKey: "hero",
			"title":                      "Hello",
		}},
	}, []*blocks.Instance{{
		ID: uuid.New(),
	}})

	record, err := fixture.svc.Get(fixture.ctx, fixture.pageID.String(), interfaces.AdminPageGetOptions{
		Locale: "en",
	})
	if err != nil {
		t.Fatalf("get admin record: %v", err)
	}
	if record.Blocks != nil {
		t.Fatalf("expected blocks to be nil when IncludeBlocks is false, got %#v", record.Blocks)
	}
}

func newAdminReadFixture(t *testing.T) adminReadFixture {
	t.Helper()

	ctx := context.Background()
	localeRepo := content.NewMemoryLocaleRepository()
	localeID := uuid.New()
	localeRepo.Put(&content.Locale{
		ID:       localeID,
		Code:     "en",
		Display:  "English",
		IsActive: true,
	})

	contentTypeRepo := content.NewMemoryContentTypeRepository()
	contentTypeID := uuid.New()
	seedContentType(t, contentTypeRepo, &content.ContentType{
		ID:            contentTypeID,
		Name:          "Page",
		SchemaVersion: "schema-v1",
	})

	contentRepo := content.NewMemoryContentRepository()
	contentID := uuid.New()
	translationGroupID := uuid.New()
	contentSummary := "Content summary"
	contentTranslation := &content.ContentTranslation{
		ID:                 uuid.New(),
		ContentID:          contentID,
		LocaleID:           localeID,
		TranslationGroupID: &translationGroupID,
		Title:              "Content Title",
		Summary:            &contentSummary,
		Locale: &content.Locale{
			ID:   localeID,
			Code: "en",
		},
		Content: map[string]any{
			"content":          "Body text",
			"tags":             []string{"alpha", "beta"},
			"meta_title":       "Meta Title From Content",
			"meta_description": "Meta Description From Content",
			"_schema":          "schema-from-content",
		},
	}
	_, err := contentRepo.Create(ctx, &content.Content{
		ID:            contentID,
		ContentTypeID: contentTypeID,
		Slug:          "content",
		Status:        "draft",
		Translations:  []*content.ContentTranslation{contentTranslation},
	})
	if err != nil {
		t.Fatalf("seed content: %v", err)
	}

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)

	pageRepo := pages.NewMemoryPageRepository()
	pageID := uuid.New()
	templateID := uuid.New()
	pageSummary := "Page summary"
	createdAt := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := createdAt.Add(2 * time.Hour)
	_, err = pageRepo.Create(ctx, &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: templateID,
		Slug:       "home",
		Status:     "draft",
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		Translations: []*pages.PageTranslation{
			{
				ID:       uuid.New(),
				PageID:   pageID,
				LocaleID: localeID,
				Locale:   "en",
				Title:    "Page Title",
				Path:     "/home",
				Summary:  &pageSummary,
			},
		},
	})
	if err != nil {
		t.Fatalf("seed page: %v", err)
	}

	pageSvc := pages.NewService(pageRepo, contentRepo, localeRepo)
	adminSvc := pages.NewAdminPageReadService(pageSvc, contentSvc, localeRepo)

	expected := adminReadExpectations{
		title:           "Page Title",
		path:            "/home",
		summary:         "Page summary",
		metaTitle:       "Meta Title From Content",
		metaDescription: "Meta Description From Content",
		tags:            []string{"alpha", "beta"},
		schemaVersion:   "schema-v1",
		requestedLocale: "en",
		resolvedLocale:  "en",
		contentBody:     "Body text",
	}

	return adminReadFixture{
		ctx:                ctx,
		svc:                adminSvc,
		pageID:             pageID,
		templateID:         templateID,
		translationGroupID: translationGroupID,
		listOpts: interfaces.AdminPageListOptions{
			Locale:         "en",
			IncludeContent: true,
			IncludeData:    true,
		},
		getOpts: interfaces.AdminPageGetOptions{
			Locale:         "en",
			IncludeContent: true,
			IncludeData:    true,
		},
		expected: expected,
	}
}

func newAdminReadBlocksFixture(t *testing.T, payload map[string]any, legacy ...[]*blocks.Instance) adminReadBlocksFixture {
	t.Helper()

	ctx := context.Background()
	localeRepo := content.NewMemoryLocaleRepository()
	localeID := uuid.New()
	localeRepo.Put(&content.Locale{
		ID:       localeID,
		Code:     "en",
		Display:  "English",
		IsActive: true,
	})

	contentTypeRepo := content.NewMemoryContentTypeRepository()
	contentTypeID := uuid.New()
	seedContentType(t, contentTypeRepo, &content.ContentType{
		ID:            contentTypeID,
		Name:          "Page",
		SchemaVersion: "schema-v1",
	})

	contentRepo := content.NewMemoryContentRepository()
	contentID := uuid.New()
	_, err := contentRepo.Create(ctx, &content.Content{
		ID:            contentID,
		ContentTypeID: contentTypeID,
		Slug:          "content",
		Status:        "draft",
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  localeID,
				Title:     "Content Title",
				Content:   payload,
			},
		},
	})
	if err != nil {
		t.Fatalf("seed content: %v", err)
	}

	pageRepo := pages.NewMemoryPageRepository()
	pageID := uuid.New()
	page := &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: uuid.New(),
		Slug:       "home",
		Status:     "draft",
		Translations: []*pages.PageTranslation{
			{
				ID:       uuid.New(),
				PageID:   pageID,
				LocaleID: localeID,
				Locale:   "en",
				Title:    "Page Title",
				Path:     "/home",
			},
		},
	}
	if len(legacy) > 0 {
		page.Blocks = legacy[0]
	}
	_, err = pageRepo.Create(ctx, page)
	if err != nil {
		t.Fatalf("seed page: %v", err)
	}

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)
	pageSvc := pages.NewService(pageRepo, contentRepo, localeRepo)
	adminSvc := pages.NewAdminPageReadService(pageSvc, contentSvc, localeRepo)

	return adminReadBlocksFixture{
		ctx:    ctx,
		svc:    adminSvc,
		pageID: pageID,
	}
}

func expectDataString(t *testing.T, data map[string]any, key, want string) {
	t.Helper()
	value, ok := data[key]
	if !ok {
		t.Fatalf("expected data key %q", key)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("expected data key %q to be string, got %T", key, value)
	}
	if text != want {
		t.Fatalf("expected data key %q to be %q, got %q", key, want, text)
	}
}

func expectDataTags(t *testing.T, data map[string]any, want []string) {
	t.Helper()
	value, ok := data["tags"]
	if !ok {
		t.Fatalf("expected data key %q", "tags")
	}
	switch tags := value.(type) {
	case []string:
		if !reflect.DeepEqual(tags, want) {
			t.Fatalf("expected data tags %#v, got %#v", want, tags)
		}
	case []any:
		out := make([]string, 0, len(tags))
		for _, entry := range tags {
			text, ok := entry.(string)
			if ok {
				out = append(out, text)
			}
		}
		if !reflect.DeepEqual(out, want) {
			t.Fatalf("expected data tags %#v, got %#v", want, out)
		}
	default:
		t.Fatalf("expected data tags to be []string or []any, got %T", value)
	}
}
