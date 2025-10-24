package pages_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/google/uuid"
)

func TestPageServiceCreateSuccess(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	createdContent, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "welcome",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Welcome",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := pages.NewService(pageStore, contentStore, localeStore, pages.WithPageClock(func() time.Time {
		return time.Unix(0, 0)
	}))

	req := pages.CreatePageRequest{
		ContentID:  createdContent.ID,
		TemplateID: uuid.New(),
		Slug:       "home",
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Home",
			Path:   "/",
		}},
	}

	result, err := pageSvc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	if result.Slug != req.Slug {
		t.Fatalf("expected slug %q got %q", req.Slug, result.Slug)
	}

	if len(result.Translations) != 1 {
		t.Fatalf("expected 1 translation got %d", len(result.Translations))
	}

	if result.Translations[0].Path != "/" {
		t.Fatalf("expected path '/' got %q", result.Translations[0].Path)
	}
}

func TestPageServiceCreateDuplicateSlug(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "baseline",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Baseline"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := pages.NewService(pageStore, contentStore, localeStore)

	if _, err := pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   uuid.New(),
		Slug:         "news",
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "en", Title: "News", Path: "/news"}},
	}); err != nil {
		t.Fatalf("first page create: %v", err)
	}

	_, err = pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   uuid.New(),
		Slug:         "news",
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "en", Title: "News Again", Path: "/news-2"}},
	})
	if !errors.Is(err, pages.ErrSlugExists) {
		t.Fatalf("expected ErrSlugExists got %v", err)
	}
}

func TestPageServiceCreateUnknownLocale(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	typeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: typeID, Name: "page"})
	localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "article",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Article"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageSvc := pages.NewService(pageStore, contentStore, localeStore)

	_, err = pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:    contentRecord.ID,
		TemplateID:   uuid.New(),
		Slug:         "article",
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
		Translations: []pages.PageTranslationInput{{Locale: "es", Title: "Articulo", Path: "/articulo"}},
	})
	if !errors.Is(err, pages.ErrUnknownLocale) {
		t.Fatalf("expected ErrUnknownLocale got %v", err)
	}
}

func TestPageServiceListIncludesBlocks(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})
	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	contentRecord, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "landing",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Landing"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	blockDefRepo := blocks.NewMemoryDefinitionRepository()
	blockInstRepo := blocks.NewMemoryInstanceRepository()
	blockTransRepo := blocks.NewMemoryTranslationRepository()

	blockSvc := blocks.NewService(blockDefRepo, blockInstRepo, blockTransRepo)

	definition, err := blockSvc.RegisterDefinition(context.Background(), blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	pageID := uuid.New()
	pageSvc := pages.NewService(pageStore, contentStore, localeStore, pages.WithBlockService(blockSvc), pages.WithPageClock(func() time.Time {
		return time.Unix(0, 0)
	}), pages.WithIDGenerator(func() uuid.UUID { return pageID }))

	createdPage, err := pageSvc.Create(context.Background(), pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: uuid.New(),
		Slug:       "landing",
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Landing",
			Path:   "/landing",
		}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	instance, err := blockSvc.CreateInstance(context.Background(), blocks.CreateInstanceInput{
		DefinitionID: definition.ID,
		PageID:       &createdPage.ID,
		Region:       "hero",
		Position:     0,
		Configuration: map[string]any{
			"layout": "full",
		},
		CreatedBy: uuid.New(),
		UpdatedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("create block instance: %v", err)
	}

	if _, err := blockSvc.AddTranslation(context.Background(), blocks.AddTranslationInput{
		BlockInstanceID: instance.ID,
		LocaleID:        localeID,
		Content: map[string]any{
			"title": "Hero Title",
		},
	}); err != nil {
		t.Fatalf("add block translation: %v", err)
	}

	pagesList, err := pageSvc.List(context.Background())
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(pagesList) != 1 {
		t.Fatalf("expected one page got %d", len(pagesList))
	}
	if len(pagesList[0].Blocks) != 1 {
		t.Fatalf("expected blocks to be attached")
	}
	if pagesList[0].Blocks[0].Region != "hero" {
		t.Fatalf("unexpected block region %s", pagesList[0].Blocks[0].Region)
	}
}

func TestPageServiceListIncludesWidgets(t *testing.T) {
	ctx := context.Background()

	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	authorID := uuid.New()
	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "features",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Features",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	now := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
	widgetSvc := widgets.NewService(
		widgets.NewMemoryDefinitionRepository(),
		widgets.NewMemoryInstanceRepository(),
		widgets.NewMemoryTranslationRepository(),
		widgets.WithAreaDefinitionRepository(widgets.NewMemoryAreaDefinitionRepository()),
		widgets.WithAreaPlacementRepository(widgets.NewMemoryAreaPlacementRepository()),
		widgets.WithClock(func() time.Time { return now }),
	)

	definition, err := widgetSvc.RegisterDefinition(ctx, widgets.RegisterDefinitionInput{
		Name: "promo_banner",
		Schema: map[string]any{
			"fields": []any{
				map[string]any{"name": "headline"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register widget definition: %v", err)
	}

	instance, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
		DefinitionID:  definition.ID,
		Configuration: map[string]any{"headline": "Limited time offer"},
		Position:      0,
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
	})
	if err != nil {
		t.Fatalf("create widget instance: %v", err)
	}

	if _, err := widgetSvc.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
		Code:  "sidebar.primary",
		Name:  "Primary Sidebar",
		Scope: widgets.AreaScopeGlobal,
	}); err != nil {
		t.Fatalf("register area definition: %v", err)
	}

	if _, err := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   "sidebar.primary",
		InstanceID: instance.ID,
	}); err != nil {
		t.Fatalf("assign widget to area: %v", err)
	}

	pageID := uuid.New()
	pageSvc := pages.NewService(
		pageStore,
		contentStore,
		localeStore,
		pages.WithWidgetService(widgetSvc),
		pages.WithPageClock(func() time.Time { return now }),
		pages.WithIDGenerator(func() uuid.UUID { return pageID }),
	)

	createdPage, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: uuid.New(),
		Slug:       "features",
		CreatedBy:  authorID,
		UpdatedBy:  authorID,
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Features",
			Path:   "/features",
		}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	if createdPage.ID != pageID {
		t.Fatalf("expected deterministic page ID")
	}

	results, err := pageSvc.List(ctx)
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one page, got %d", len(results))
	}

	areaWidgets, ok := results[0].Widgets["sidebar.primary"]
	if !ok {
		t.Fatalf("expected widgets for sidebar.primary")
	}
	if len(areaWidgets) != 1 {
		t.Fatalf("expected one widget resolved, got %d", len(areaWidgets))
	}
	if areaWidgets[0] == nil || areaWidgets[0].Instance == nil {
		t.Fatalf("expected resolved widget instance")
	}
	if areaWidgets[0].Instance.ID != instance.ID {
		t.Fatalf("expected widget instance %s, got %s", instance.ID, areaWidgets[0].Instance.ID)
	}
}
