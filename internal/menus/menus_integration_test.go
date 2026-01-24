package menus_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/testsupport"
	repocache "github.com/goliatone/go-repository-cache/cache"
	urlkit "github.com/goliatone/go-urlkit"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestMenuService_WithBunStorageAndCache(t *testing.T) {
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

	registerMenuModels(t, bunDB)
	seedMenuIntegrationEntities(t, bunDB)

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
	pageRepo := pages.NewBunPageRepositoryWithCache(bunDB, cacheService, keySerializer)
	menuRepo := menus.NewBunMenuRepositoryWithCache(bunDB, cacheService, keySerializer)
	menuItemRepo := menus.NewBunMenuItemRepositoryWithCache(bunDB, cacheService, keySerializer)
	menuTranslationRepo := menus.NewBunMenuItemTranslationRepositoryWithCache(bunDB, cacheService, keySerializer)

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)
	pageSvc := pages.NewService(pageRepo, contentRepo, localeRepo)
	routeManager := urlkit.NewRouteManager(&urlkit.Config{
		Groups: []urlkit.GroupConfig{
			{
				Name:    "frontend",
				BaseURL: "https://example.com",
				Paths: map[string]string{
					"page": "/pages/:slug",
				},
				Groups: []urlkit.GroupConfig{
					{
						Name: "es",
						Path: "/es",
						Paths: map[string]string{
							"page": "/paginas/:slug",
						},
					},
				},
			},
		},
	})

	urlResolver := menus.NewURLKitResolver(menus.URLKitResolverOptions{
		Manager:      routeManager,
		DefaultGroup: "frontend",
		LocaleGroups: map[string]string{
			"es": "frontend.es",
		},
		DefaultRoute: "page",
		SlugParam:    "slug",
	})

	menuSvc := menus.NewService(menuRepo, menuItemRepo, menuTranslationRepo, localeRepo,
		menus.WithPageRepository(pageRepo),
		menus.WithURLResolver(urlResolver),
	)

	authorID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: mustUUID("00000000-0000-0000-0000-000000000210"),
		Slug:          "company-overview",
		Status:        "published",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{
				Locale: "en",
				Title:  "Company",
				Content: map[string]any{
					"body": "Company overview body",
				},
			},
			{
				Locale: "es",
				Title:  "Empresa",
				Content: map[string]any{
					"body": "Resumen de la empresa",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageRecord, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: mustUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		Slug:       "company-overview",
		Status:     "published",
		CreatedBy:  authorID,
		UpdatedBy:  authorID,
		Translations: []pages.PageTranslationInput{
			{Locale: "en", Title: "Company", Path: "/company"},
			{Locale: "es", Title: "Empresa", Path: "/es/empresa"},
		},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	menuRecord, err := menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}

	if _, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menuRecord.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": pageRecord.Slug,
		},
		CreatedBy: authorID,
		UpdatedBy: authorID,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Company"},
			{Locale: "es", Label: "Empresa"},
		},
	}); err != nil {
		t.Fatalf("add menu item: %v", err)
	}

	fixture := loadNavigationFixture(t, "testdata/navigation_integration.json")

	navigationEN, err := menuSvc.ResolveNavigation(ctx, fixture.MenuCode, "en")
	if err != nil {
		t.Fatalf("resolve navigation en: %v", err)
	}
	assertNavigation(t, fixture.Navigation, navigationEN)

	navigationES, err := menuSvc.ResolveNavigation(ctx, fixture.MenuCode, "es")
	if err != nil {
		t.Fatalf("resolve navigation es: %v", err)
	}
	assertNavigation(t, fixture.NavigationES, navigationES)

	// second call should hit cache without error
	if _, err := menuSvc.ResolveNavigation(ctx, fixture.MenuCode, "en"); err != nil {
		t.Fatalf("resolve navigation cached: %v", err)
	}
}

func TestMenuService_AllowsOptionalTranslations(t *testing.T) {
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

	registerMenuModels(t, bunDB)
	seedMenuIntegrationEntities(t, bunDB)

	contentRepo := content.NewBunContentRepository(bunDB)
	contentTypeRepo := content.NewBunContentTypeRepository(bunDB)
	localeRepo := content.NewBunLocaleRepository(bunDB)
	pageRepo := pages.NewBunPageRepository(bunDB)
	menuRepo := menus.NewBunMenuRepository(bunDB)
	menuItemRepo := menus.NewBunMenuItemRepository(bunDB)
	menuTranslationRepo := menus.NewBunMenuItemTranslationRepository(bunDB)

	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)
	pageSvc := pages.NewService(
		pageRepo,
		contentRepo,
		localeRepo,
		pages.WithRequireTranslations(false),
	)

	menuSvc := menus.NewService(
		menuRepo,
		menuItemRepo,
		menuTranslationRepo,
		localeRepo,
		menus.WithRequireTranslations(false),
	)

	authorID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: mustUUID("00000000-0000-0000-0000-000000000210"),
		Slug:          "navigation-optional",
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Navigation Optional",
				Content: map[string]any{"body": "Placeholder"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageRecord, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:                contentRecord.ID,
		TemplateID:               mustUUID("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		Slug:                     "navigation-optional",
		Status:                   "draft",
		CreatedBy:                authorID,
		UpdatedBy:                authorID,
		AllowMissingTranslations: true,
	})
	if err != nil {
		t.Fatalf("create page without translations: %v", err)
	}

	menuRecord, err := menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}

	if _, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:                   menuRecord.ID,
		Position:                 0,
		Target:                   map[string]any{"type": "page", "slug": pageRecord.Slug},
		CreatedBy:                authorID,
		UpdatedBy:                authorID,
		AllowMissingTranslations: true,
	}); err != nil {
		t.Fatalf("add menu item without translations: %v", err)
	}

	navigation, err := menuSvc.ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation: %v", err)
	}
	if len(navigation) != 1 {
		t.Fatalf("expected 1 navigation node, got %d", len(navigation))
	}
	if navigation[0].Label == "" {
		t.Fatalf("expected navigation label fallback, got empty string")
	}
	if navigation[0].URL == "" {
		t.Fatalf("expected navigation url fallback, got empty string")
	}
}

func TestMenuService_BulkReorderMaintainsHierarchy(t *testing.T) {
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

	registerMenuModels(t, bunDB)
	seedMenuIntegrationEntities(t, bunDB)

	menuRepo := menus.NewBunMenuRepository(bunDB)
	itemRepo := menus.NewBunMenuItemRepository(bunDB)
	translationRepo := menus.NewBunMenuItemTranslationRepository(bunDB)
	localeRepo := content.NewBunLocaleRepository(bunDB)

	menuSvc := menus.NewService(
		menuRepo,
		itemRepo,
		translationRepo,
		localeRepo,
	)

	menu, err := menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		Location:  "site.primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}
	if menu.Location != "site.primary" {
		t.Fatalf("expected location persisted, got %q", menu.Location)
	}

	rootA, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:    menu.ID,
		Position:  0,
		Target:    map[string]any{"type": "external", "url": "/a"},
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Root A"},
			{Locale: "es", Label: "Raíz A"},
		},
	})
	if err != nil {
		t.Fatalf("add rootA: %v", err)
	}

	rootB, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:    menu.ID,
		Position:  1,
		Target:    map[string]any{"type": "external", "url": "/b"},
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Root B"},
			{Locale: "es", Label: "Raíz B"},
		},
	})
	if err != nil {
		t.Fatalf("add rootB: %v", err)
	}

	child, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:    menu.ID,
		ParentID:  &rootA.ID,
		Position:  0,
		Target:    map[string]any{"type": "external", "url": "/child"},
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Child"},
			{Locale: "es", Label: "Hijo"},
		},
	})
	if err != nil {
		t.Fatalf("add child: %v", err)
	}

	if _, err := menuSvc.BulkReorderMenuItems(ctx, menus.BulkReorderMenuItemsInput{
		MenuID:    menu.ID,
		UpdatedBy: uuid.Nil,
		Items: []menus.ItemOrder{
			{ItemID: rootB.ID, Position: 0},
			{ItemID: rootA.ID, Position: 1},
			{ItemID: child.ID, ParentID: &rootB.ID, Position: 0},
		},
	}); err != nil {
		t.Fatalf("bulk reorder: %v", err)
	}

	reloaded, err := menuSvc.GetMenu(ctx, menu.ID)
	if err != nil {
		t.Fatalf("get menu: %v", err)
	}
	if len(reloaded.Items) != 2 {
		t.Fatalf("expected 2 root items, got %d", len(reloaded.Items))
	}
	if reloaded.Items[0].ID != rootB.ID {
		t.Fatalf("expected rootB first, got %s", reloaded.Items[0].ID)
	}
	if len(reloaded.Items[0].Children) != 1 || reloaded.Items[0].Children[0].ID != child.ID {
		t.Fatalf("expected child under rootB, got %+v", reloaded.Items[0].Children)
	}
	if len(reloaded.Items[0].Children[0].Translations) != 2 {
		t.Fatalf("expected child translations preserved, got %d", len(reloaded.Items[0].Children[0].Translations))
	}
}

func TestMenuRepository_RoundTripNewFields(t *testing.T) {
	ctx := context.Background()

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	registerMenuModels(t, bunDB)
	seedMenuIntegrationEntities(t, bunDB)

	localeRepo := content.NewBunLocaleRepository(bunDB)
	menuRepo := menus.NewBunMenuRepository(bunDB)
	menuItemRepo := menus.NewBunMenuItemRepository(bunDB)
	menuTranslationRepo := menus.NewBunMenuItemTranslationRepository(bunDB)

	menuSvc := menus.NewService(menuRepo, menuItemRepo, menuTranslationRepo, localeRepo)

	menu, err := menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}

	group, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Type:     menus.MenuItemTypeGroup,
		Metadata: map[string]any{"section": "main"},
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", GroupTitle: "Main Menu", GroupTitleKey: "menu.group.main"},
		},
	})
	if err != nil {
		t.Fatalf("add group: %v", err)
	}

	_, err = menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 1,
		Type:     menus.MenuItemTypeSeparator,
	})
	if err != nil {
		t.Fatalf("add separator: %v", err)
	}

	child, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		ParentID: &group.ID,
		Position: 0,
		Type:     menus.MenuItemTypeItem,
		Target: map[string]any{
			"type": "page",
			"slug": "home",
		},
		Icon:        "home",
		Badge:       map[string]any{"count": 5},
		Permissions: []string{"admin", "editor"},
		Classes:     []string{"nav-item", "bold"},
		Styles:      map[string]string{"color": "blue"},
		Metadata:    map[string]any{"tenant": "tenant-1"},
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Home", LabelKey: "menu.home"},
		},
	})
	if err != nil {
		t.Fatalf("add child: %v", err)
	}

	reloaded, err := menuSvc.GetMenu(ctx, menu.ID)
	if err != nil {
		t.Fatalf("get menu: %v", err)
	}
	if len(reloaded.Items) != 2 {
		t.Fatalf("expected 2 root items, got %d", len(reloaded.Items))
	}
	if reloaded.Items[0].Type != menus.MenuItemTypeGroup {
		t.Fatalf("expected first item group, got %s", reloaded.Items[0].Type)
	}
	if reloaded.Items[0].Metadata["section"] != "main" {
		t.Fatalf("expected group metadata preserved, got %+v", reloaded.Items[0].Metadata)
	}
	if len(reloaded.Items[0].Translations) != 1 {
		t.Fatalf("expected group translation, got %d", len(reloaded.Items[0].Translations))
	}
	if reloaded.Items[0].Translations[0].GroupTitle != "Main Menu" || reloaded.Items[0].Translations[0].GroupTitleKey != "menu.group.main" {
		t.Fatalf("group translation fields not preserved: %#v", reloaded.Items[0].Translations[0])
	}

	item := reloaded.Items[0].Children[0]
	if item.ID != child.ID {
		t.Fatalf("expected child id %s, got %s", child.ID, item.ID)
	}
	if item.Icon != "home" || item.Badge["count"] != float64(5) {
		t.Fatalf("expected icon/badge preserved, got icon=%q badge=%v", item.Icon, item.Badge)
	}
	if len(item.Permissions) != 2 || item.Permissions[0] != "admin" || item.Permissions[1] != "editor" {
		t.Fatalf("permissions not preserved: %#v", item.Permissions)
	}
	if len(item.Classes) != 2 || item.Classes[0] != "nav-item" || item.Classes[1] != "bold" {
		t.Fatalf("classes not preserved: %#v", item.Classes)
	}
	if val, ok := item.Styles["color"]; !ok || val != "blue" {
		t.Fatalf("styles not preserved: %#v", item.Styles)
	}
	if val, ok := item.Metadata["tenant"]; !ok || val != "tenant-1" {
		t.Fatalf("metadata not preserved: %#v", item.Metadata)
	}
	if len(item.Translations) != 1 {
		t.Fatalf("expected item translation, got %d", len(item.Translations))
	}
	if item.Translations[0].Label != "Home" || item.Translations[0].LabelKey != "menu.home" {
		t.Fatalf("translation fields not preserved: %#v", item.Translations[0])
	}
}

func registerMenuModels(t *testing.T, db *bun.DB) {
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
		(*menus.Menu)(nil),
		(*menus.MenuItem)(nil),
		(*menus.MenuItemTranslation)(nil),
	}

	for _, model := range models {
		if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			t.Fatalf("create table %T: %v", model, err)
		}
	}
}

func seedMenuIntegrationEntities(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()

	locales := []content.Locale{
		{
			ID:        mustUUID("00000000-0000-0000-0000-000000000201"),
			Code:      "en",
			Display:   "English",
			IsActive:  true,
			IsDefault: true,
		},
		{
			ID:       mustUUID("00000000-0000-0000-0000-000000000202"),
			Code:     "es",
			Display:  "Spanish",
			IsActive: true,
		},
	}
	for _, locale := range locales {
		locale := locale
		if _, err := db.NewInsert().Model(&locale).Exec(ctx); err != nil {
			t.Fatalf("insert locale: %v", err)
		}
	}

	ct := &content.ContentType{
		ID:   mustUUID("00000000-0000-0000-0000-000000000210"),
		Name: "page",
		Schema: map[string]any{
			"fields": []map[string]any{{"name": "body", "type": "richtext"}},
		},
	}
	if _, err := db.NewInsert().Model(ct).Exec(ctx); err != nil {
		t.Fatalf("insert content type: %v", err)
	}
}

type navigationFixture struct {
	MenuCode     string            `json:"menu_code"`
	Navigation   []navigationEntry `json:"navigation"`
	NavigationES []navigationEntry `json:"navigation_es"`
}

type navigationEntry struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

func loadNavigationFixture(t *testing.T, path string) navigationFixture {
	t.Helper()
	raw, err := testsupport.LoadFixture(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	var fx navigationFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return fx
}

func assertNavigation(t *testing.T, expected []navigationEntry, got []menus.NavigationNode) {
	t.Helper()
	if len(expected) != len(got) {
		t.Fatalf("navigation length mismatch: want %d, got %d", len(expected), len(got))
	}
	for i, entry := range expected {
		if got[i].Label != entry.Label {
			t.Fatalf("navigation[%d] label mismatch: want %q, got %q", i, entry.Label, got[i].Label)
		}
		if got[i].URL != entry.URL {
			t.Fatalf("navigation[%d] url mismatch: want %q, got %q", i, entry.URL, got[i].URL)
		}
	}
}

func mustUUID(value string) uuid.UUID {
	id, err := uuid.Parse(value)
	if err != nil {
		panic(err)
	}
	return id
}
