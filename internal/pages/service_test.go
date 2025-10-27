package pages_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
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

func TestPageServiceVersionLifecycle(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	contentTypeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()
	pageStore := pages.NewMemoryPageRepository()

	contentTypeID := uuid.New()
	contentTypeStore.Put(&content.ContentType{ID: contentTypeID, Name: "page"})

	localeID := uuid.New()
	localeStore.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
	createdContent, err := contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "page-versioned",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations:  []content.ContentTranslationInput{{Locale: "en", Title: "Versioned"}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	fixedNow := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	pageSvc := pages.NewService(
		pageStore,
		contentStore,
		localeStore,
		pages.WithPageClock(func() time.Time { return fixedNow }),
		pages.WithPageVersioningEnabled(true),
		pages.WithPageVersionRetentionLimit(5),
	)

	ctx := context.Background()
	pageAuthor := uuid.New()
	page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  createdContent.ID,
		TemplateID: uuid.New(),
		Slug:       "landing",
		CreatedBy:  pageAuthor,
		UpdatedBy:  pageAuthor,
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Landing",
			Path:   "/landing",
		}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	blockID := uuid.New()
	instanceID := uuid.New()
	draftSnapshot := pages.PageVersionSnapshot{
		Regions: map[string][]pages.PageBlockPlacement{
			"hero": {{
				Region:     "hero",
				Position:   0,
				BlockID:    blockID,
				InstanceID: instanceID,
				Snapshot:   map[string]any{"headline": "Draft"},
			}},
		},
		Metadata: map[string]any{"notes": "initial"},
	}

	firstDraft, err := pageSvc.CreateDraft(ctx, pages.CreatePageDraftRequest{
		PageID:    page.ID,
		Snapshot:  draftSnapshot,
		CreatedBy: pageAuthor,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if firstDraft.Version != 1 {
		t.Fatalf("expected version 1 got %d", firstDraft.Version)
	}
	if firstDraft.Status != domain.StatusDraft {
		t.Fatalf("expected draft status got %s", firstDraft.Status)
	}

	publishUser := uuid.New()
	publishedAt := time.Date(2024, 3, 16, 10, 0, 0, 0, time.UTC)
	published, err := pageSvc.PublishDraft(ctx, pages.PublishPagePublishRequest{
		PageID:      page.ID,
		Version:     firstDraft.Version,
		PublishedBy: publishUser,
		PublishedAt: &publishedAt,
	})
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}
	if published.Status != domain.StatusPublished {
		t.Fatalf("expected published status got %s", published.Status)
	}

	secondSnapshot := pages.PageVersionSnapshot{
		Regions: map[string][]pages.PageBlockPlacement{
			"hero": {{
				Region:     "hero",
				Position:   0,
				BlockID:    blockID,
				InstanceID: instanceID,
				Snapshot:   map[string]any{"headline": "Updated"},
			}},
		},
	}
	baseVersion := published.Version
	secondDraft, err := pageSvc.CreateDraft(ctx, pages.CreatePageDraftRequest{
		PageID:      page.ID,
		Snapshot:    secondSnapshot,
		CreatedBy:   pageAuthor,
		UpdatedBy:   pageAuthor,
		BaseVersion: &baseVersion,
	})
	if err != nil {
		t.Fatalf("create second draft: %v", err)
	}
	if secondDraft.Version != 2 {
		t.Fatalf("expected second draft version 2 got %d", secondDraft.Version)
	}

	secondPublisher := uuid.New()
	secondPublished, err := pageSvc.PublishDraft(ctx, pages.PublishPagePublishRequest{
		PageID:      page.ID,
		Version:     secondDraft.Version,
		PublishedBy: secondPublisher,
	})
	if err != nil {
		t.Fatalf("publish second draft: %v", err)
	}
	if secondPublished.Status != domain.StatusPublished {
		t.Fatalf("expected second version published got %s", secondPublished.Status)
	}

	versions, err := pageSvc.ListVersions(ctx, page.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions got %d", len(versions))
	}
	if versions[0].Status != domain.StatusArchived {
		t.Fatalf("expected first version archived got %s", versions[0].Status)
	}
	if versions[1].Status != domain.StatusPublished {
		t.Fatalf("expected second version published got %s", versions[1].Status)
	}

	restorer := uuid.New()
	restored, err := pageSvc.RestoreVersion(ctx, pages.RestorePageVersionRequest{
		PageID:     page.ID,
		Version:    1,
		RestoredBy: restorer,
	})
	if err != nil {
		t.Fatalf("restore version: %v", err)
	}
	if restored.Version != 3 {
		t.Fatalf("expected restored version 3 got %d", restored.Version)
	}
	if restored.Status != domain.StatusDraft {
		t.Fatalf("expected restored draft status got %s", restored.Status)
	}

	allVersions, err := pageSvc.ListVersions(ctx, page.ID)
	if err != nil {
		t.Fatalf("list versions after restore: %v", err)
	}
	if len(allVersions) != 3 {
		t.Fatalf("expected 3 versions got %d", len(allVersions))
	}
	if allVersions[2].Status != domain.StatusDraft {
		t.Fatalf("expected newest version draft got %s", allVersions[2].Status)
	}

	updatedPage, err := pageSvc.Get(ctx, page.ID)
	if err != nil {
		t.Fatalf("get page: %v", err)
	}
	if updatedPage.PublishedVersion == nil || *updatedPage.PublishedVersion != 2 {
		t.Fatalf("expected published version pointer 2 got %v", updatedPage.PublishedVersion)
	}
	if updatedPage.CurrentVersion != 3 {
		t.Fatalf("expected current version 3 got %d", updatedPage.CurrentVersion)
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
