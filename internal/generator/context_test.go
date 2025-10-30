package generator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/google/uuid"
)

func TestLoadContextBuildsLocalizedPages(t *testing.T) {
	ctx := context.Background()

	localeEN := uuid.New()
	localeES := uuid.New()

	pageID1 := uuid.New()
	pageID2 := uuid.New()
	contentID := uuid.New()
	templateID := uuid.New()
	themeID := uuid.New()

	now := time.Date(2024, 1, 25, 12, 0, 0, 0, time.UTC)

	contentRecord := &content.Content{
		ID:             contentID,
		Slug:           "company-overview",
		Status:         "published",
		UpdatedAt:      now.Add(-2 * time.Hour),
		CurrentVersion: 3,
		PublishedVersion: func() *int {
			v := 2
			return &v
		}(),
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  localeEN,
				Title:     "Company Overview",
				Content: map[string]any{
					"body": "English body",
				},
				UpdatedAt: now.Add(-time.Hour),
			},
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  localeES,
				Title:     "Resumen de la empresa",
				Content: map[string]any{
					"body": "Contenido en español",
				},
				UpdatedAt: now.Add(-30 * time.Minute),
			},
		},
	}

	pageBase := &pages.Page{
		ContentID:   contentID,
		TemplateID:  templateID,
		Slug:        "company",
		Status:      "published",
		PublishedAt: ptrTime(now.Add(-90 * time.Minute)),
		UpdatedAt:   now.Add(-45 * time.Minute),
		PublishedVersion: func() *int {
			v := 2
			return &v
		}(),
		IsVisible: true,
	}

	page1 := clonePage(pageBase)
	page1.ID = pageID1
	page1.Translations = []*pages.PageTranslation{
		{
			ID:        uuid.New(),
			PageID:    pageID1,
			LocaleID:  localeEN,
			Title:     "Company",
			Path:      "/company",
			UpdatedAt: now.Add(-40 * time.Minute),
		},
		{
			ID:        uuid.New(),
			PageID:    pageID1,
			LocaleID:  localeES,
			Title:     "Empresa",
			Path:      "/es/empresa",
			UpdatedAt: now.Add(-35 * time.Minute),
		},
	}

	page2 := clonePage(pageBase)
	page2.ID = pageID2
	page2.Translations = []*pages.PageTranslation{
		{
			ID:        uuid.New(),
			PageID:    pageID2,
			LocaleID:  localeEN,
			Title:     "Vision",
			Path:      "/vision",
			UpdatedAt: now.Add(-50 * time.Minute),
		},
		{
			ID:        uuid.New(),
			PageID:    pageID2,
			LocaleID:  localeES,
			Title:     "Visión",
			Path:      "/es/vision",
			UpdatedAt: now.Add(-25 * time.Minute),
		},
	}

	contentSvc := &stubContentService{
		records: map[uuid.UUID]*content.Content{
			contentID: contentRecord,
		},
	}
	pagesSvc := &stubPagesService{
		records: map[uuid.UUID]*pages.Page{
			pageID1: page1,
			pageID2: page2,
		},
		listing: []*pages.Page{page1, page2},
	}
	menuSvc := newStubMenuService()
	themeSvc := &stubThemesService{
		template: &themes.Template{
			ID:        templateID,
			ThemeID:   themeID,
			Name:      "profile",
			UpdatedAt: now.Add(-4 * time.Hour),
		},
		theme: &themes.Theme{
			ID:        themeID,
			Name:      "aurora",
			Version:   "1.0.0",
			UpdatedAt: now.Add(-24 * time.Hour),
		},
	}
	locales := &stubLocaleLookup{
		records: map[string]*content.Locale{
			"en": {ID: localeEN, Code: "en"},
			"es": {ID: localeES, Code: "es"},
		},
	}

	cfg := Config{
		OutputDir:     "dist",
		DefaultLocale: "en",
		Locales:       []string{"es"},
		Menus: map[string]string{
			"main": "main-nav",
		},
	}

	svc := NewService(cfg, Dependencies{
		Pages:   pagesSvc,
		Content: contentSvc,
		Menus:   menuSvc,
		Themes:  themeSvc,
		Locales: locales,
		Logger:  logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	buildCtx, err := svc.loadContext(ctx, BuildOptions{})
	if err != nil {
		t.Fatalf("load context: %v", err)
	}

	if buildCtx == nil {
		t.Fatal("expected build context")
	}
	if diff := len(buildCtx.Locales); diff != 2 {
		t.Fatalf("expected 2 locales, got %d", diff)
	}
	if buildCtx.Locales[0].Code != "en" || !buildCtx.Locales[0].IsDefault {
		t.Fatalf("expected default locale first, got %#v", buildCtx.Locales[0])
	}
	if buildCtx.GeneratedAt != now {
		t.Fatalf("expected GeneratedAt %v, got %v", now, buildCtx.GeneratedAt)
	}

	if len(buildCtx.Pages) != 4 {
		t.Fatalf("expected 4 localized pages, got %d", len(buildCtx.Pages))
	}

	for _, entry := range buildCtx.Pages {
		if entry.Locale.Code != "en" && entry.Locale.Code != "es" {
			t.Fatalf("unexpected locale %q", entry.Locale.Code)
		}
		if entry.Menus["main"] == nil {
			t.Fatalf("expected menu data for %s", entry.Locale.Code)
		}
		if entry.Metadata.Hash == "" {
			t.Fatalf("expected metadata hash for %s", entry.Locale.Code)
		}
		if entry.Metadata.LastModified.IsZero() {
			t.Fatalf("expected last modified timestamp for %s", entry.Locale.Code)
		}
	}

	if got := menuSvc.calls["en"]; got != 1 {
		t.Fatalf("expected menu resolver to be called once for en, got %d", got)
	}
	if got := menuSvc.calls["es"]; got != 1 {
		t.Fatalf("expected menu resolver to be called once for es, got %d", got)
	}

	if themeSvc.templateCalls != 1 {
		t.Fatalf("expected template lookup once, got %d", themeSvc.templateCalls)
	}
	if themeSvc.themeCalls != 1 {
		t.Fatalf("expected theme lookup once, got %d", themeSvc.themeCalls)
	}
}

func TestLoadContextAppliesLocaleFilter(t *testing.T) {
	ctx := context.Background()

	localeEN := uuid.New()
	localeES := uuid.New()
	contentID := uuid.New()
	pageID := uuid.New()
	templateID := uuid.New()
	themeID := uuid.New()
	now := time.Date(2024, 1, 25, 15, 0, 0, 0, time.UTC)

	contentRecord := &content.Content{
		ID:        contentID,
		Slug:      "about",
		Status:    "published",
		UpdatedAt: now,
		Translations: []*content.ContentTranslation{
			{ID: uuid.New(), ContentID: contentID, LocaleID: localeEN, Title: "About", Content: map[string]any{"body": "en"}, UpdatedAt: now},
			{ID: uuid.New(), ContentID: contentID, LocaleID: localeES, Title: "Acerca", Content: map[string]any{"body": "es"}, UpdatedAt: now},
		},
	}

	page := &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: templateID,
		Slug:       "about",
		Status:     "published",
		IsVisible:  true,
		UpdatedAt:  now,
		Translations: []*pages.PageTranslation{
			{ID: uuid.New(), PageID: pageID, LocaleID: localeEN, Title: "About", Path: "/about", UpdatedAt: now},
			{ID: uuid.New(), PageID: pageID, LocaleID: localeES, Title: "Acerca", Path: "/es/acerca", UpdatedAt: now},
		},
	}

	contentSvc := &stubContentService{
		records: map[uuid.UUID]*content.Content{contentID: contentRecord},
	}
	pagesSvc := &stubPagesService{
		records: map[uuid.UUID]*pages.Page{pageID: page},
		listing: []*pages.Page{page},
	}
	menuSvc := newStubMenuService()
	themeSvc := &stubThemesService{
		template: &themes.Template{ID: templateID, ThemeID: themeID, Name: "landing"},
		theme:    &themes.Theme{ID: themeID, Name: "aurora", Version: "1.0"},
	}
	locales := &stubLocaleLookup{
		records: map[string]*content.Locale{
			"en": {ID: localeEN, Code: "en"},
			"es": {ID: localeES, Code: "es"},
		},
	}

	cfg := Config{
		OutputDir:     "dist",
		DefaultLocale: "en",
		Locales:       []string{"en", "es"},
		Menus: map[string]string{
			"main": "main-nav",
		},
	}

	svc := NewService(cfg, Dependencies{
		Pages:   pagesSvc,
		Content: contentSvc,
		Menus:   menuSvc,
		Themes:  themeSvc,
		Locales: locales,
		Logger:  logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	buildCtx, err := svc.loadContext(ctx, BuildOptions{
		Locales: []string{"es"},
		PageIDs: []uuid.UUID{pageID},
	})
	if err != nil {
		t.Fatalf("load context: %v", err)
	}

	if len(buildCtx.Pages) != 1 {
		t.Fatalf("expected 1 localized page, got %d", len(buildCtx.Pages))
	}
	if buildCtx.Pages[0].Locale.Code != "es" {
		t.Fatalf("expected locale es, got %s", buildCtx.Pages[0].Locale.Code)
	}
}

func TestLoadContextPropagatesMenuErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC)

	localeEN := uuid.New()
	contentID := uuid.New()
	pageID := uuid.New()
	templateID := uuid.New()
	menuErr := errors.New("menu resolution failed")

	contentSvc := &stubContentService{
		records: map[uuid.UUID]*content.Content{
			contentID: {
				ID:        contentID,
				Slug:      "home",
				Status:    "published",
				UpdatedAt: now,
				Translations: []*content.ContentTranslation{
					{ID: uuid.New(), ContentID: contentID, LocaleID: localeEN, Title: "Home", Content: map[string]any{"body": "home"}, UpdatedAt: now},
				},
			},
		},
	}
	pageSvc := &stubPagesService{
		records: map[uuid.UUID]*pages.Page{
			pageID: {
				ID:         pageID,
				ContentID:  contentID,
				TemplateID: templateID,
				Slug:       "home",
				Status:     "published",
				IsVisible:  true,
				UpdatedAt:  now,
				Translations: []*pages.PageTranslation{
					{ID: uuid.New(), PageID: pageID, LocaleID: localeEN, Title: "Home", Path: "/", UpdatedAt: now},
				},
			},
		},
		listing: []*pages.Page{{
			ID:         pageID,
			ContentID:  contentID,
			TemplateID: templateID,
			Slug:       "home",
			Status:     "published",
			IsVisible:  true,
			UpdatedAt:  now,
			Translations: []*pages.PageTranslation{
				{ID: uuid.New(), PageID: pageID, LocaleID: localeEN, Title: "Home", Path: "/", UpdatedAt: now},
			},
		}},
	}
	menuSvc := &errorMenuService{stubMenusService: newStubMenuService(), err: menuErr}
	themeSvc := &stubThemesService{
		template: &themes.Template{ID: templateID, ThemeID: uuid.New(), Name: "default"},
	}
	locales := &stubLocaleLookup{
		records: map[string]*content.Locale{
			"en": {ID: localeEN, Code: "en"},
		},
	}

	cfg := Config{
		OutputDir:     "dist",
		DefaultLocale: "en",
		Menus: map[string]string{
			"main": "main",
		},
	}

	svc := NewService(cfg, Dependencies{
		Pages:   pageSvc,
		Content: contentSvc,
		Menus:   menuSvc,
		Themes:  themeSvc,
		Locales: locales,
		Logger:  logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	if _, err := svc.loadContext(ctx, BuildOptions{}); err == nil || !errors.Is(err, menuErr) {
		t.Fatalf("expected menu error, got %v", err)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func clonePage(src *pages.Page) *pages.Page {
	if src == nil {
		return nil
	}
	clone := *src
	return &clone
}

type stubContentService struct {
	records map[uuid.UUID]*content.Content
}

func (s *stubContentService) Create(context.Context, content.CreateContentRequest) (*content.Content, error) {
	return nil, errUnsupported
}

func (s *stubContentService) Get(_ context.Context, id uuid.UUID) (*content.Content, error) {
	rec, ok := s.records[id]
	if !ok {
		return nil, errUnsupported
	}
	return rec, nil
}

func (s *stubContentService) List(context.Context) ([]*content.Content, error) {
	return nil, errUnsupported
}

func (s *stubContentService) Update(context.Context, content.UpdateContentRequest) (*content.Content, error) {
	return nil, errUnsupported
}

func (s *stubContentService) Delete(context.Context, content.DeleteContentRequest) error {
	return errUnsupported
}

func (s *stubContentService) Schedule(context.Context, content.ScheduleContentRequest) (*content.Content, error) {
	return nil, errUnsupported
}

func (s *stubContentService) CreateDraft(context.Context, content.CreateContentDraftRequest) (*content.ContentVersion, error) {
	return nil, errUnsupported
}

func (s *stubContentService) PublishDraft(context.Context, content.PublishContentDraftRequest) (*content.ContentVersion, error) {
	return nil, errUnsupported
}

func (s *stubContentService) ListVersions(context.Context, uuid.UUID) ([]*content.ContentVersion, error) {
	return nil, errUnsupported
}

func (s *stubContentService) RestoreVersion(context.Context, content.RestoreContentVersionRequest) (*content.ContentVersion, error) {
	return nil, errUnsupported
}

type stubPagesService struct {
	records map[uuid.UUID]*pages.Page
	listing []*pages.Page
}

func (s *stubPagesService) Create(context.Context, pages.CreatePageRequest) (*pages.Page, error) {
	return nil, errUnsupported
}

func (s *stubPagesService) Get(_ context.Context, id uuid.UUID) (*pages.Page, error) {
	rec, ok := s.records[id]
	if !ok {
		return nil, errUnsupported
	}
	return rec, nil
}

func (s *stubPagesService) List(context.Context) ([]*pages.Page, error) {
	return append([]*pages.Page{}, s.listing...), nil
}

func (s *stubPagesService) Update(context.Context, pages.UpdatePageRequest) (*pages.Page, error) {
	return nil, errUnsupported
}

func (s *stubPagesService) Delete(context.Context, pages.DeletePageRequest) error {
	return errUnsupported
}

func (s *stubPagesService) Schedule(context.Context, pages.SchedulePageRequest) (*pages.Page, error) {
	return nil, errUnsupported
}

func (s *stubPagesService) CreateDraft(context.Context, pages.CreatePageDraftRequest) (*pages.PageVersion, error) {
	return nil, errUnsupported
}

func (s *stubPagesService) PublishDraft(context.Context, pages.PublishPagePublishRequest) (*pages.PageVersion, error) {
	return nil, errUnsupported
}

func (s *stubPagesService) ListVersions(context.Context, uuid.UUID) ([]*pages.PageVersion, error) {
	return nil, errUnsupported
}

func (s *stubPagesService) RestoreVersion(context.Context, pages.RestorePageVersionRequest) (*pages.PageVersion, error) {
	return nil, errUnsupported
}

type stubMenusService struct {
	calls map[string]int
}

func newStubMenuService() *stubMenusService {
	return &stubMenusService{
		calls: map[string]int{},
	}
}

func (s *stubMenusService) CreateMenu(context.Context, menus.CreateMenuInput) (*menus.Menu, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) GetMenu(context.Context, uuid.UUID) (*menus.Menu, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) GetMenuByCode(context.Context, string) (*menus.Menu, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) AddMenuItem(context.Context, menus.AddMenuItemInput) (*menus.MenuItem, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) UpdateMenuItem(context.Context, menus.UpdateMenuItemInput) (*menus.MenuItem, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) ReorderMenuItems(context.Context, menus.ReorderMenuItemsInput) ([]*menus.MenuItem, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) AddMenuItemTranslation(context.Context, menus.AddMenuItemTranslationInput) (*menus.MenuItemTranslation, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) ResolveNavigation(_ context.Context, menuCode string, locale string) ([]menus.NavigationNode, error) {
	s.calls[locale]++
	id := uuid.NewSHA1(uuid.NameSpaceURL, []byte(menuCode+"-"+locale))
	return []menus.NavigationNode{
		{
			ID:    id,
			Label: menuCode + "-" + locale,
			URL:   "/" + locale,
		},
	}, nil
}

func (s *stubMenusService) InvalidateCache(context.Context) error {
	return nil
}

type errorMenuService struct {
	*stubMenusService
	err error
}

func (s *errorMenuService) ResolveNavigation(context.Context, string, string) ([]menus.NavigationNode, error) {
	return nil, s.err
}

type stubThemesService struct {
	template      *themes.Template
	theme         *themes.Theme
	templateCalls int
	themeCalls    int
}

func (s *stubThemesService) RegisterTheme(context.Context, themes.RegisterThemeInput) (*themes.Theme, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) GetTheme(context.Context, uuid.UUID) (*themes.Theme, error) {
	s.themeCalls++
	return s.theme, nil
}

func (s *stubThemesService) GetThemeByName(context.Context, string) (*themes.Theme, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) ListThemes(context.Context) ([]*themes.Theme, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) ListActiveThemes(context.Context) ([]*themes.Theme, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) ActivateTheme(context.Context, uuid.UUID) (*themes.Theme, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) DeactivateTheme(context.Context, uuid.UUID) (*themes.Theme, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) RegisterTemplate(context.Context, themes.RegisterTemplateInput) (*themes.Template, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) UpdateTemplate(context.Context, themes.UpdateTemplateInput) (*themes.Template, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) DeleteTemplate(context.Context, uuid.UUID) error {
	return errUnsupported
}

func (s *stubThemesService) GetTemplate(context.Context, uuid.UUID) (*themes.Template, error) {
	s.templateCalls++
	return s.template, nil
}

func (s *stubThemesService) ListTemplates(context.Context, uuid.UUID) ([]*themes.Template, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) TemplateRegions(context.Context, uuid.UUID) ([]themes.RegionInfo, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) ThemeRegionIndex(context.Context, uuid.UUID) (map[string][]themes.RegionInfo, error) {
	return nil, errUnsupported
}

func (s *stubThemesService) ListActiveSummaries(context.Context) ([]themes.ThemeSummary, error) {
	return nil, errUnsupported
}

type stubLocaleLookup struct {
	records map[string]*content.Locale
}

func (s *stubLocaleLookup) GetByCode(_ context.Context, code string) (*content.Locale, error) {
	rec, ok := s.records[strings.ToLower(code)]
	if !ok {
		return nil, errUnsupported
	}
	return rec, nil
}

var errUnsupported = errors.New("stub: unsupported operation")
