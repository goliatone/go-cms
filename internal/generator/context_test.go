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
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

func TestLoadContextBuildsLocalizedPages(t *testing.T) {
	ctx := context.Background()

	localeEN := uuid.New()
	localeES := uuid.New()

	pageTypeID := uuid.New()
	pageID1 := uuid.New()
	pageID2 := uuid.New()
	templateID := uuid.New()
	themeID := uuid.New()

	now := time.Date(2024, 1, 25, 12, 0, 0, 0, time.UTC)

	page1Content := &content.Content{
		ID:             pageID1,
		ContentTypeID:  pageTypeID,
		Slug:           "company",
		Status:         "published",
		UpdatedAt:      now.Add(-2 * time.Hour),
		CurrentVersion: 3,
		Metadata: map[string]any{
			"template_id": templateID.String(),
		},
		PublishedVersion: func() *int {
			v := 2
			return &v
		}(),
		IsVisible: true,
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: pageID1,
				LocaleID:  localeEN,
				Title:     "Company",
				Content: map[string]any{
					"body": "English body",
					"path": "/company",
				},
				UpdatedAt: now.Add(-time.Hour),
			},
			{
				ID:        uuid.New(),
				ContentID: pageID1,
				LocaleID:  localeES,
				Title:     "Empresa",
				Content: map[string]any{
					"body": "Contenido en español",
					"path": "/es/empresa",
				},
				UpdatedAt: now.Add(-30 * time.Minute),
			},
		},
	}

	page2Content := &content.Content{
		ID:            pageID2,
		ContentTypeID: pageTypeID,
		Slug:          "vision",
		Status:        "published",
		PublishedAt:   ptrTime(now.Add(-90 * time.Minute)),
		UpdatedAt:     now.Add(-45 * time.Minute),
		Metadata: map[string]any{
			"template_id": templateID.String(),
		},
		PublishedVersion: func() *int {
			v := 2
			return &v
		}(),
		IsVisible: true,
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: pageID2,
				LocaleID:  localeEN,
				Title:     "Vision",
				Content: map[string]any{
					"body": "Vision body",
					"path": "/vision",
				},
				UpdatedAt: now.Add(-50 * time.Minute),
			},
			{
				ID:        uuid.New(),
				ContentID: pageID2,
				LocaleID:  localeES,
				Title:     "Visión",
				Content: map[string]any{
					"body": "Vision es",
					"path": "/es/vision",
				},
				UpdatedAt: now.Add(-25 * time.Minute),
			},
		},
	}

	contentSvc := &stubContentService{
		records: map[uuid.UUID]*content.Content{
			pageID1: page1Content,
			pageID2: page2Content,
		},
		listing: []*content.Content{page1Content, page2Content},
	}
	contentTypeSvc := newStubContentTypeService(pageTypeID, "page")
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
			ThemePath: "testdata/theme",
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
		Theming: ThemingConfig{
			DefaultTheme:   "aurora",
			DefaultVariant: "contrast",
		},
	}

	svc := NewService(cfg, Dependencies{
		Content:      contentSvc,
		ContentTypes: contentTypeSvc,
		Menus:        menuSvc,
		Themes:       themeSvc,
		Locales:      locales,
		Logger:       logging.NoOp(),
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
	pageTypeID := uuid.New()
	pageID := uuid.New()
	templateID := uuid.New()
	themeID := uuid.New()
	now := time.Date(2024, 1, 25, 15, 0, 0, 0, time.UTC)

	pageContent := &content.Content{
		ID:            pageID,
		ContentTypeID: pageTypeID,
		Slug:          "about",
		Status:        "published",
		UpdatedAt:     now,
		Metadata: map[string]any{
			"template_id": templateID.String(),
		},
		IsVisible: true,
		Translations: []*content.ContentTranslation{
			{ID: uuid.New(), ContentID: pageID, LocaleID: localeEN, Title: "About", Content: map[string]any{"body": "en", "path": "/about"}, UpdatedAt: now},
			{ID: uuid.New(), ContentID: pageID, LocaleID: localeES, Title: "Acerca", Content: map[string]any{"body": "es", "path": "/es/acerca"}, UpdatedAt: now},
		},
	}

	contentSvc := &stubContentService{
		records: map[uuid.UUID]*content.Content{pageID: pageContent},
		listing: []*content.Content{pageContent},
	}
	contentTypeSvc := newStubContentTypeService(pageTypeID, "page")
	menuSvc := newStubMenuService()
	themeSvc := &stubThemesService{
		template: &themes.Template{ID: templateID, ThemeID: themeID, Name: "landing"},
		theme:    &themes.Theme{ID: themeID, Name: "aurora", Version: "1.0", ThemePath: "testdata/theme"},
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
		Theming: ThemingConfig{
			DefaultTheme:   "aurora",
			DefaultVariant: "contrast",
		},
	}

	svc := NewService(cfg, Dependencies{
		Content:      contentSvc,
		ContentTypes: contentTypeSvc,
		Menus:        menuSvc,
		Themes:       themeSvc,
		Locales:      locales,
		Logger:       logging.NoOp(),
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
	pageTypeID := uuid.New()
	pageID := uuid.New()
	templateID := uuid.New()
	menuErr := errors.New("menu resolution failed")

	pageContent := &content.Content{
		ID:            pageID,
		ContentTypeID: pageTypeID,
		Slug:          "home",
		Status:        "published",
		UpdatedAt:     now,
		Metadata: map[string]any{
			"template_id": templateID.String(),
		},
		IsVisible: true,
		Translations: []*content.ContentTranslation{
			{ID: uuid.New(), ContentID: pageID, LocaleID: localeEN, Title: "Home", Content: map[string]any{"body": "home", "path": "/"}, UpdatedAt: now},
		},
	}

	contentSvc := &stubContentService{
		records: map[uuid.UUID]*content.Content{pageID: pageContent},
		listing: []*content.Content{pageContent},
	}
	contentTypeSvc := newStubContentTypeService(pageTypeID, "page")
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
		Content:      contentSvc,
		ContentTypes: contentTypeSvc,
		Menus:        menuSvc,
		Themes:       themeSvc,
		Locales:      locales,
		Logger:       logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	if _, err := svc.loadContext(ctx, BuildOptions{}); err == nil || !errors.Is(err, menuErr) {
		t.Fatalf("expected menu error, got %v", err)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

type stubContentService struct {
	records map[uuid.UUID]*content.Content
	listing []*content.Content
}

func (s *stubContentService) Create(context.Context, content.CreateContentRequest) (*content.Content, error) {
	return nil, errUnsupported
}

func (s *stubContentService) Get(_ context.Context, id uuid.UUID, _ ...content.ContentGetOption) (*content.Content, error) {
	rec, ok := s.records[id]
	if !ok {
		return nil, errUnsupported
	}
	return rec, nil
}

func (s *stubContentService) List(context.Context, ...string) ([]*content.Content, error) {
	if len(s.listing) == 0 {
		return []*content.Content{}, nil
	}
	return append([]*content.Content{}, s.listing...), nil
}

func (s *stubContentService) CheckTranslations(context.Context, uuid.UUID, []string, interfaces.TranslationCheckOptions) ([]string, error) {
	return nil, errUnsupported
}

func (s *stubContentService) AvailableLocales(context.Context, uuid.UUID, interfaces.TranslationCheckOptions) ([]string, error) {
	return nil, errUnsupported
}

func (s *stubContentService) Update(context.Context, content.UpdateContentRequest) (*content.Content, error) {
	return nil, errUnsupported
}

func (s *stubContentService) Delete(context.Context, content.DeleteContentRequest) error {
	return errUnsupported
}

func (s *stubContentService) UpdateTranslation(context.Context, content.UpdateContentTranslationRequest) (*content.ContentTranslation, error) {
	return nil, errUnsupported
}

func (s *stubContentService) DeleteTranslation(context.Context, content.DeleteContentTranslationRequest) error {
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

func (s *stubContentService) PreviewDraft(context.Context, content.PreviewContentDraftRequest) (*content.ContentPreview, error) {
	return nil, errUnsupported
}

func (s *stubContentService) ListVersions(context.Context, uuid.UUID) ([]*content.ContentVersion, error) {
	return nil, errUnsupported
}

func (s *stubContentService) RestoreVersion(context.Context, content.RestoreContentVersionRequest) (*content.ContentVersion, error) {
	return nil, errUnsupported
}

type stubContentTypeService struct {
	bySlug map[string]*content.ContentType
	byID   map[uuid.UUID]*content.ContentType
}

func newStubContentTypeService(id uuid.UUID, slug string) *stubContentTypeService {
	record := &content.ContentType{ID: id, Slug: slug}
	return &stubContentTypeService{
		bySlug: map[string]*content.ContentType{
			strings.ToLower(slug): record,
		},
		byID: map[uuid.UUID]*content.ContentType{
			id: record,
		},
	}
}

func (s *stubContentTypeService) Create(context.Context, content.CreateContentTypeRequest) (*content.ContentType, error) {
	return nil, errUnsupported
}

func (s *stubContentTypeService) Update(context.Context, content.UpdateContentTypeRequest) (*content.ContentType, error) {
	return nil, errUnsupported
}

func (s *stubContentTypeService) Delete(context.Context, content.DeleteContentTypeRequest) error {
	return errUnsupported
}

func (s *stubContentTypeService) Get(_ context.Context, id uuid.UUID) (*content.ContentType, error) {
	if record, ok := s.byID[id]; ok {
		return record, nil
	}
	return nil, &content.NotFoundError{Resource: "content_type", Key: id.String()}
}

func (s *stubContentTypeService) GetBySlug(_ context.Context, slug string, _ ...string) (*content.ContentType, error) {
	if record, ok := s.bySlug[strings.ToLower(strings.TrimSpace(slug))]; ok {
		return record, nil
	}
	return nil, &content.NotFoundError{Resource: "content_type", Key: slug}
}

func (s *stubContentTypeService) List(context.Context, ...string) ([]*content.ContentType, error) {
	out := make([]*content.ContentType, 0, len(s.byID))
	for _, record := range s.byID {
		out = append(out, record)
	}
	return out, nil
}

func (s *stubContentTypeService) Search(context.Context, string, ...string) ([]*content.ContentType, error) {
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

func (s *stubMenusService) GetOrCreateMenu(context.Context, menus.CreateMenuInput) (*menus.Menu, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) UpsertMenu(context.Context, menus.UpsertMenuInput) (*menus.Menu, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) GetMenu(context.Context, uuid.UUID) (*menus.Menu, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) GetMenuByCode(context.Context, string, ...string) (*menus.Menu, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) GetMenuByLocation(context.Context, string, ...string) (*menus.Menu, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) AddMenuItem(context.Context, menus.AddMenuItemInput) (*menus.MenuItem, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) UpsertMenuItem(context.Context, menus.UpsertMenuItemInput) (*menus.MenuItem, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) UpdateMenuItem(context.Context, menus.UpdateMenuItemInput) (*menus.MenuItem, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) AddMenuItemTranslation(context.Context, menus.AddMenuItemTranslationInput) (*menus.MenuItemTranslation, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) UpsertMenuItemTranslation(context.Context, menus.UpsertMenuItemTranslationInput) (*menus.MenuItemTranslation, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) GetMenuItemByExternalCode(context.Context, string, string, ...string) (*menus.MenuItem, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) ResolveNavigation(_ context.Context, menuCode string, locale string, _ ...string) ([]menus.NavigationNode, error) {
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

func (s *stubMenusService) ResolveNavigationByLocation(ctx context.Context, location string, locale string, env ...string) ([]menus.NavigationNode, error) {
	return s.ResolveNavigation(ctx, location, locale, env...)
}

func (s *stubMenusService) InvalidateCache(context.Context) error {
	return nil
}

func (s *stubMenusService) DeleteMenu(context.Context, menus.DeleteMenuRequest) error {
	return errUnsupported
}

func (s *stubMenusService) ResetMenuByCode(context.Context, string, uuid.UUID, bool, ...string) error {
	return errUnsupported
}

func (s *stubMenusService) DeleteMenuItem(context.Context, menus.DeleteMenuItemRequest) error {
	return errUnsupported
}

func (s *stubMenusService) BulkReorderMenuItems(context.Context, menus.BulkReorderMenuItemsInput) ([]*menus.MenuItem, error) {
	return nil, errUnsupported
}

func (s *stubMenusService) ReconcileMenu(context.Context, menus.ReconcileMenuRequest) (*menus.ReconcileResult, error) {
	return nil, errUnsupported
}

type errorMenuService struct {
	*stubMenusService
	err error
}

func (s *errorMenuService) ResolveNavigation(context.Context, string, string, ...string) ([]menus.NavigationNode, error) {
	return nil, s.err
}

func (s *errorMenuService) ResolveNavigationByLocation(context.Context, string, string, ...string) ([]menus.NavigationNode, error) {
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
