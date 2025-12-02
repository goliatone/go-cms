package menus_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/goliatone/go-cms/pkg/testsupport"
	urlkit "github.com/goliatone/go-urlkit"
	"github.com/google/uuid"
)

type serviceFixture struct {
	Locales      []localeFixture                             `json:"locales"`
	Translations map[string][]menus.MenuItemTranslationInput `json:"translations"`
}

type localeFixture struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Display string `json:"display"`
}

func loadServiceFixture(t *testing.T) serviceFixture {
	t.Helper()
	path := filepath.Join("testdata", "service_fixture.json")
	raw, err := testsupport.LoadFixture(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	var fx serviceFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return fx
}

func (fx serviceFixture) locales() []content.Locale {
	locales := make([]content.Locale, len(fx.Locales))
	for i, loc := range fx.Locales {
		locales[i] = content.Locale{
			ID:        uuid.MustParse(loc.ID),
			Code:      loc.Code,
			Display:   loc.Display,
			IsActive:  true,
			IsDefault: i == 0,
		}
	}
	return locales
}

func (fx serviceFixture) translations(key string) []menus.MenuItemTranslationInput {
	items := fx.Translations[key]
	out := make([]menus.MenuItemTranslationInput, len(items))
	copy(out, items)
	return out
}

func TestService_CreateMenu_DuplicateCode(t *testing.T) {
	ctx := context.Background()
	svc := newService(t)

	_, err := svc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu initial: %v", err)
	}

	_, err = svc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if !errors.Is(err, menus.ErrMenuCodeExists) {
		t.Fatalf("expected ErrMenuCodeExists, got %v", err)
	}
}

func TestService_AddMenuItem_ShiftsSiblings(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	service, ids := newServiceWithIDs(t, fixture,
		uuid.MustParse("00000000-0000-0000-0000-000000000101"), // menu
		uuid.MustParse("00000000-0000-0000-0000-000000000201"), // item A
		uuid.MustParse("00000000-0000-0000-0000-000000000202"), // tr A en
		uuid.MustParse("00000000-0000-0000-0000-000000000203"), // tr A es
		uuid.MustParse("00000000-0000-0000-0000-000000000204"), // item B
		uuid.MustParse("00000000-0000-0000-0000-000000000205"), // tr B en
	)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	first, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": " page ",
			"slug": "home",
		},
		Translations: fixture.translations("home"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem first: %v", err)
	}
	if first.Position != 0 {
		t.Fatalf("expected position 0, got %d", first.Position)
	}
	if got := first.Target["type"]; got != "page" {
		t.Fatalf("expected target type normalized, got %v", got)
	}
	if len(first.Translations) != 2 {
		t.Fatalf("expected 2 translations, got %d", len(first.Translations))
	}

	second, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "about",
		},
		Translations: fixture.translations("about"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem second: %v", err)
	}
	if second.Position != 0 {
		t.Fatalf("expected new item at position 0, got %d", second.Position)
	}

	updatedFirst, err := service.GetMenu(ctx, menu.ID)
	if err != nil {
		t.Fatalf("GetMenu: %v", err)
	}
	if len(updatedFirst.Items) != 2 {
		t.Fatalf("expected 2 root items, got %d", len(updatedFirst.Items))
	}
	if updatedFirst.Items[0].ID != second.ID || updatedFirst.Items[1].ID != first.ID {
		t.Fatalf("expected reordered items, got %#v", []uuid.UUID{updatedFirst.Items[0].ID, updatedFirst.Items[1].ID})
	}

	// ensure deterministic IDs used
	if first.ID != ids[1] || second.ID != ids[4] {
		t.Fatalf("unexpected item IDs: first=%s second=%s", first.ID, second.ID)
	}
}

func TestService_AddMenuItemWithoutTranslationsWhenOptional(t *testing.T) {
	ctx := context.Background()
	locale := content.Locale{ID: uuid.New(), Code: "en", Display: "English"}
	service := newServiceWithLocales(t, []content.Locale{locale}, uuid.New, nil, menus.WithRequireTranslations(false))

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	item, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "home",
		},
	})
	if err != nil {
		t.Fatalf("AddMenuItem without translations: %v", err)
	}
	if len(item.Translations) != 0 {
		t.Fatalf("expected zero translations, got %d", len(item.Translations))
	}
}

func TestService_AddMenuItemAllowsMissingTranslationsOverride(t *testing.T) {
	ctx := context.Background()
	service := newService(t)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	item, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:                   menu.ID,
		Position:                 0,
		Target:                   map[string]any{"type": "page", "slug": "home"},
		AllowMissingTranslations: true,
	})
	if err != nil {
		t.Fatalf("AddMenuItem with allow missing: %v", err)
	}
	if len(item.Translations) != 0 {
		t.Fatalf("expected zero translations, got %d", len(item.Translations))
	}

	if _, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 1,
		Target: map[string]any{
			"type": "page",
			"slug": "about",
		},
	}); !errors.Is(err, menus.ErrMenuItemTranslations) {
		t.Fatalf("expected ErrMenuItemTranslations without override, got %v", err)
	}
}

func TestService_AddMenuItem_UnknownLocale(t *testing.T) {
	ctx := context.Background()
	service := newService(t)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "home",
		},
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "fr", Label: "Accueil"},
		},
	})
	if !errors.Is(err, menus.ErrUnknownLocale) {
		t.Fatalf("expected ErrUnknownLocale, got %v", err)
	}
}

func TestService_AddMenuItemTranslation_Duplicate(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	service := newService(t)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	item, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "home",
		},
		Translations: fixture.translations("home"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem: %v", err)
	}

	_, err = service.AddMenuItemTranslation(ctx, menus.AddMenuItemTranslationInput{
		ItemID: item.ID,
		Locale: "en",
		Label:  "Homepage",
	})
	if !errors.Is(err, menus.ErrTranslationExists) {
		t.Fatalf("expected ErrTranslationExists, got %v", err)
	}
}

func TestService_AddMenuItemTranslationEmitsActivity(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)

	hook := &activity.CaptureHook{}
	emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{Enabled: true, Channel: "cms"})

	ids := []uuid.UUID{
		uuid.MustParse("00000000-0000-0000-0000-000000000301"), // menu
		uuid.MustParse("00000000-0000-0000-0000-000000000302"), // menu item
		uuid.MustParse("00000000-0000-0000-0000-000000000303"), // initial translation
		uuid.MustParse("00000000-0000-0000-0000-000000000304"), // added translation
	}
	var idx int
	idGen := func() uuid.UUID {
		if idx >= len(ids) {
			return uuid.New()
		}
		value := ids[idx]
		idx++
		return value
	}

	service := newServiceWithLocales(t, fixture.locales(), idGen, nil, menus.WithActivityEmitter(emitter))

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	item, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "about",
		},
		Translations: fixture.translations("about"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem: %v", err)
	}

	tr, err := service.AddMenuItemTranslation(ctx, menus.AddMenuItemTranslationInput{
		ItemID: item.ID,
		Locale: "es",
		Label:  "Acerca de",
	})
	if err != nil {
		t.Fatalf("AddMenuItemTranslation: %v", err)
	}

	var translationEvents []activity.Event
	for _, event := range hook.Events {
		if event.ObjectType == "menu_item_translation" {
			translationEvents = append(translationEvents, event)
		}
	}
	if len(translationEvents) != 1 {
		t.Fatalf("expected 1 menu_item_translation event, got %d", len(translationEvents))
	}

	event := translationEvents[0]
	if event.Verb != "create" {
		t.Fatalf("expected verb create got %s", event.Verb)
	}
	if event.ObjectID != tr.ID.String() {
		t.Fatalf("expected object id %s got %s", tr.ID, event.ObjectID)
	}
	if event.Metadata["menu_id"] != menu.ID.String() {
		t.Fatalf("expected menu_id metadata %s got %v", menu.ID, event.Metadata["menu_id"])
	}
	if event.Metadata["menu_code"] != menu.Code {
		t.Fatalf("expected menu_code metadata %s got %v", menu.Code, event.Metadata["menu_code"])
	}
	if event.Metadata["locale"] != "es" {
		t.Fatalf("expected locale metadata es got %v", event.Metadata["locale"])
	}
}

func TestService_AddMenuItem_PageValidation(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	enLocale := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	pageID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	repo := newStubPageRepository(&pages.Page{
		ID:   pageID,
		Slug: "home",
		Translations: []*pages.PageTranslation{
			{LocaleID: enLocale, Path: "/home"},
		},
	})

	locales := fixture.locales()
	service := newServiceWithLocales(t, []content.Locale{locales[0]}, uuid.New, repo)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	homeTranslations := fixture.translations("home")
	item, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": " home ",
		},
		Translations: homeTranslations[:1],
	})
	if err != nil {
		t.Fatalf("AddMenuItem: %v", err)
	}

	if got := item.Target["slug"].(string); got != "home" {
		t.Fatalf("expected slug 'home', got %q", got)
	}
	if got := item.Target["page_id"].(string); got != pageID.String() {
		t.Fatalf("expected page_id %s, got %s", pageID, got)
	}

	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 1,
		Target: map[string]any{
			"type": "page",
			"slug": "missing",
		},
		Translations: fixture.translations("about"),
	})
	if !errors.Is(err, menus.ErrMenuItemPageNotFound) {
		t.Fatalf("expected ErrMenuItemPageNotFound, got %v", err)
	}
}

func TestService_ResolveNavigation_PageIntegration(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	enLocale := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	esLocale := uuid.MustParse("00000000-0000-0000-0000-000000000202")
	pageID := uuid.MustParse("20000000-0000-0000-0000-000000000001")
	repo := newStubPageRepository(&pages.Page{
		ID:   pageID,
		Slug: "company",
		Translations: []*pages.PageTranslation{
			{LocaleID: enLocale, Path: "/company"},
			{LocaleID: esLocale, Path: "/es/empresa"},
		},
	})

	locs := fixture.locales()
	service := newServiceWithLocales(t, locs, uuid.New, repo)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "company",
		},
		Translations: fixture.translations("navigation_company"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem: %v", err)
	}

	navEN, err := service.ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("ResolveNavigation en: %v", err)
	}
	if len(navEN) != 1 {
		t.Fatalf("expected 1 nav item, got %d", len(navEN))
	}
	if navEN[0].Label != "Company" {
		t.Fatalf("expected label 'Company', got %q", navEN[0].Label)
	}
	if navEN[0].URL != "/company" {
		t.Fatalf("expected URL '/company', got %q", navEN[0].URL)
	}

	navES, err := service.ResolveNavigation(ctx, "primary", "es")
	if err != nil {
		t.Fatalf("ResolveNavigation es: %v", err)
	}
	if len(navES) != 1 {
		t.Fatalf("expected 1 nav item, got %d", len(navES))
	}
	if navES[0].Label != "Empresa" {
		t.Fatalf("expected label 'Empresa', got %q", navES[0].Label)
	}
	if navES[0].URL != "/es/empresa" {
		t.Fatalf("expected URL '/es/empresa', got %q", navES[0].URL)
	}
}

func TestService_ResolveNavigation_URLKitResolver(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	enLocale := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	esLocale := uuid.MustParse("00000000-0000-0000-0000-000000000202")
	pageID := uuid.MustParse("30000000-0000-0000-0000-000000000001")
	repo := newStubPageRepository(&pages.Page{
		ID:   pageID,
		Slug: "company",
		Translations: []*pages.PageTranslation{
			{LocaleID: enLocale, Path: "/company"},
			{LocaleID: esLocale, Path: "/es/empresa"},
		},
	})

	manager := urlkit.NewRouteManager(&urlkit.Config{
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

	resolver := menus.NewURLKitResolver(menus.URLKitResolverOptions{
		Manager:      manager,
		DefaultGroup: "frontend",
		LocaleGroups: map[string]string{
			"es": "frontend.es",
		},
		DefaultRoute: "page",
		SlugParam:    "slug",
	})

	service := newServiceWithLocales(t, fixture.locales(), uuid.New, repo, menus.WithURLResolver(resolver))

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "company",
		},
		Translations: fixture.translations("navigation_company"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem: %v", err)
	}

	navEN, err := service.ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("ResolveNavigation en: %v", err)
	}
	if len(navEN) != 1 {
		t.Fatalf("expected 1 nav item, got %d", len(navEN))
	}
	if navEN[0].URL != "https://example.com/pages/company" {
		t.Fatalf("expected urlkit url, got %q", navEN[0].URL)
	}

	navES, err := service.ResolveNavigation(ctx, "primary", "es")
	if err != nil {
		t.Fatalf("ResolveNavigation es: %v", err)
	}
	if len(navES) != 1 {
		t.Fatalf("expected 1 nav item, got %d", len(navES))
	}
	if navES[0].URL != "https://example.com/es/paginas/company" {
		t.Fatalf("expected localized urlkit url, got %q", navES[0].URL)
	}
}

type stubPageRepository struct {
	byID   map[uuid.UUID]*pages.Page
	bySlug map[string]*pages.Page
}

func newStubPageRepository(pgs ...*pages.Page) *stubPageRepository {
	repo := &stubPageRepository{
		byID:   make(map[uuid.UUID]*pages.Page, len(pgs)),
		bySlug: make(map[string]*pages.Page, len(pgs)),
	}
	for _, pg := range pgs {
		repo.byID[pg.ID] = pg
		repo.bySlug[strings.TrimSpace(pg.Slug)] = pg
	}
	return repo
}

func (s *stubPageRepository) GetByID(ctx context.Context, id uuid.UUID) (*pages.Page, error) {
	if page, ok := s.byID[id]; ok {
		return page, nil
	}
	return nil, &pages.PageNotFoundError{Key: id.String()}
}

func (s *stubPageRepository) GetBySlug(ctx context.Context, slug string) (*pages.Page, error) {
	if page, ok := s.bySlug[strings.TrimSpace(slug)]; ok {
		return page, nil
	}
	return nil, &pages.PageNotFoundError{Key: strings.TrimSpace(slug)}
}

func TestService_BulkReorderMenuItems_ValidatesCycles(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	service := newService(t)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	parent, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "parent",
		},
		Translations: fixture.translations("parent"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem parent: %v", err)
	}

	child, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		ParentID: &parent.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "child",
		},
		Translations: fixture.translations("child"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem child: %v", err)
	}

	_, err = service.BulkReorderMenuItems(ctx, menus.ReorderMenuItemsInput{
		MenuID: menu.ID,
		Items: []menus.ItemOrder{
			{ItemID: parent.ID, ParentID: &child.ID, Position: 0},
			{ItemID: child.ID, ParentID: &parent.ID, Position: 0},
		},
	})
	if assertErr := assertError(err, menus.ErrMenuItemCycle); assertErr != nil {
		t.Fatal(assertErr)
	}
}

func TestService_BulkReorderMenuItems_AppliesNewOrder(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	service := newService(t)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	first, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "first",
		},
		Translations: fixture.translations("first"),
	})
	if err != nil {
		t.Fatalf("add first: %v", err)
	}

	second, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 1,
		Target: map[string]any{
			"type": "page",
			"slug": "second",
		},
		Translations: fixture.translations("second"),
	})
	if err != nil {
		t.Fatalf("add second: %v", err)
	}

	result, err := service.BulkReorderMenuItems(ctx, menus.ReorderMenuItemsInput{
		MenuID: menu.ID,
		Items: []menus.ItemOrder{
			{ItemID: first.ID, Position: 1},
			{ItemID: second.ID, Position: 0},
		},
	})
	if err != nil {
		t.Fatalf("BulkReorderMenuItems: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	if result[0].ID != second.ID || result[0].Position != 0 {
		t.Fatalf("expected second to be first with position 0, got %+v", result[0])
	}
	if result[1].ID != first.ID || result[1].Position != 1 {
		t.Fatalf("expected first to be second with position 1, got %+v", result[1])
	}
}

func TestService_DeleteMenu_RemovesMenuAndItems(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)

	menuRepo := menus.NewMemoryMenuRepository()
	itemRepo := menus.NewMemoryMenuItemRepository()
	trRepo := menus.NewMemoryMenuItemTranslationRepository()
	localeRepo := content.NewMemoryLocaleRepository()
	for _, loc := range fixture.locales() {
		locale := loc
		localeRepo.Put(&locale)
	}

	service := menus.NewService(
		menuRepo,
		itemRepo,
		trRepo,
		localeRepo,
		menus.WithIDGenerator(uuid.New),
		menus.WithClock(func() time.Time { return time.Unix(0, 0) }),
	)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	parent, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		Position:     0,
		Target:       map[string]any{"type": "page", "slug": "parent"},
		Translations: fixture.translations("parent"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem parent: %v", err)
	}

	child, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ParentID:     &parent.ID,
		Position:     0,
		Target:       map[string]any{"type": "page", "slug": "child"},
		Translations: fixture.translations("child"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem child: %v", err)
	}

	if err := service.DeleteMenu(ctx, menus.DeleteMenuRequest{
		MenuID:    menu.ID,
		DeletedBy: uuid.Nil,
	}); err != nil {
		t.Fatalf("DeleteMenu: %v", err)
	}

	if _, err := service.GetMenu(ctx, menu.ID); !errors.Is(err, menus.ErrMenuNotFound) {
		t.Fatalf("expected ErrMenuNotFound, got %v", err)
	}

	if menusLeft, err := menuRepo.List(ctx); err != nil {
		t.Fatalf("menu repo list: %v", err)
	} else if len(menusLeft) != 0 {
		t.Fatalf("expected zero menus, got %d", len(menusLeft))
	}

	if items, err := itemRepo.ListByMenu(ctx, menu.ID); err != nil {
		t.Fatalf("item repo list: %v", err)
	} else if len(items) != 0 {
		t.Fatalf("expected zero items, got %d", len(items))
	}

	if translations, err := trRepo.ListByMenuItem(ctx, parent.ID); err != nil {
		t.Fatalf("parent translations: %v", err)
	} else if len(translations) != 0 {
		t.Fatalf("expected parent translations removed, got %d", len(translations))
	}

	if translations, err := trRepo.ListByMenuItem(ctx, child.ID); err != nil {
		t.Fatalf("child translations: %v", err)
	} else if len(translations) != 0 {
		t.Fatalf("expected child translations removed, got %d", len(translations))
	}
}

func TestService_DeleteMenu_GuardrailsRequireForce(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)

	menuRepo := menus.NewMemoryMenuRepository()
	itemRepo := menus.NewMemoryMenuItemRepository()
	trRepo := menus.NewMemoryMenuItemTranslationRepository()
	localeRepo := content.NewMemoryLocaleRepository()
	for _, loc := range fixture.locales() {
		locale := loc
		localeRepo.Put(&locale)
	}

	resolver := &stubMenuUsageResolver{
		bindings: []menus.MenuUsageBinding{
			{ThemeName: "aurora", LocationCode: "primary"},
		},
	}

	service := menus.NewService(
		menuRepo,
		itemRepo,
		trRepo,
		localeRepo,
		menus.WithMenuUsageResolver(resolver),
	)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	err = service.DeleteMenu(ctx, menus.DeleteMenuRequest{
		MenuID:    menu.ID,
		DeletedBy: uuid.Nil,
	})
	if !errors.Is(err, menus.ErrMenuInUse) {
		t.Fatalf("expected ErrMenuInUse, got %v", err)
	}
	var usageErr *menus.MenuInUseError
	if !errors.As(err, &usageErr) || len(usageErr.Bindings) != 1 {
		t.Fatalf("expected MenuInUseError with bindings, got %v", err)
	}

	if err := service.DeleteMenu(ctx, menus.DeleteMenuRequest{
		MenuID:    menu.ID,
		DeletedBy: uuid.Nil,
		Force:     true,
	}); err != nil {
		t.Fatalf("DeleteMenu force: %v", err)
	}
}

func TestService_DeleteMenuItem_RequiresCascade(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	service := newService(t)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	parent, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		Position:     0,
		Target:       map[string]any{"type": "page", "slug": "parent"},
		Translations: fixture.translations("parent"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem parent: %v", err)
	}

	if _, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ParentID:     &parent.ID,
		Position:     0,
		Target:       map[string]any{"type": "page", "slug": "child"},
		Translations: fixture.translations("child"),
	}); err != nil {
		t.Fatalf("AddMenuItem child: %v", err)
	}

	err = service.DeleteMenuItem(ctx, menus.DeleteMenuItemRequest{
		ItemID:          parent.ID,
		DeletedBy:       uuid.Nil,
		CascadeChildren: false,
	})
	if !errors.Is(err, menus.ErrMenuItemHasChildren) {
		t.Fatalf("expected ErrMenuItemHasChildren, got %v", err)
	}

	menuState, err := service.GetMenu(ctx, menu.ID)
	if err != nil {
		t.Fatalf("GetMenu: %v", err)
	}
	if len(menuState.Items) != 1 || len(menuState.Items[0].Children) != 1 {
		t.Fatalf("expected parent and child to remain, got %+v", menuState.Items)
	}
}

func TestService_DeleteMenuItem_CascadeRemovesChildrenAndReorders(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)

	menuRepo := menus.NewMemoryMenuRepository()
	itemRepo := menus.NewMemoryMenuItemRepository()
	trRepo := menus.NewMemoryMenuItemTranslationRepository()
	localeRepo := content.NewMemoryLocaleRepository()
	for _, loc := range fixture.locales() {
		locale := loc
		localeRepo.Put(&locale)
	}

	service := menus.NewService(
		menuRepo,
		itemRepo,
		trRepo,
		localeRepo,
	)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	parent, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		Position:     0,
		Target:       map[string]any{"type": "page", "slug": "parent"},
		Translations: fixture.translations("parent"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem parent: %v", err)
	}

	child, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ParentID:     &parent.ID,
		Position:     0,
		Target:       map[string]any{"type": "page", "slug": "child"},
		Translations: fixture.translations("child"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem child: %v", err)
	}

	sibling, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		Position:     1,
		Target:       map[string]any{"type": "page", "slug": "sibling"},
		Translations: fixture.translations("sibling"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem sibling: %v", err)
	}

	if err := service.DeleteMenuItem(ctx, menus.DeleteMenuItemRequest{
		ItemID:          parent.ID,
		DeletedBy:       uuid.Nil,
		CascadeChildren: true,
	}); err != nil {
		t.Fatalf("DeleteMenuItem: %v", err)
	}

	if _, err := itemRepo.GetByID(ctx, parent.ID); err == nil {
		t.Fatalf("expected parent to be removed")
	}
	if _, err := itemRepo.GetByID(ctx, child.ID); err == nil {
		t.Fatalf("expected child to be removed")
	}

	items, err := itemRepo.ListByMenu(ctx, menu.ID)
	if err != nil {
		t.Fatalf("ListByMenu: %v", err)
	}
	if len(items) != 1 || items[0].ID != sibling.ID {
		t.Fatalf("expected only sibling to remain, got %+v", items)
	}
	if items[0].Position != 0 {
		t.Fatalf("expected sibling position reset to 0, got %d", items[0].Position)
	}

	if translations, err := trRepo.ListByMenuItem(ctx, parent.ID); err != nil {
		t.Fatalf("parent translations: %v", err)
	} else if len(translations) != 0 {
		t.Fatalf("expected parent translations removed, got %d", len(translations))
	}
	if translations, err := trRepo.ListByMenuItem(ctx, child.ID); err != nil {
		t.Fatalf("child translations: %v", err)
	} else if len(translations) != 0 {
		t.Fatalf("expected child translations removed, got %d", len(translations))
	}
}

func TestService_BulkReorderMenuItems_MovesBetweenParents(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	service := newService(t)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	first, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		Position:     0,
		Target:       map[string]any{"type": "page", "slug": "first"},
		Translations: fixture.translations("first"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem first: %v", err)
	}

	second, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		Position:     1,
		Target:       map[string]any{"type": "page", "slug": "second"},
		Translations: fixture.translations("second"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem second: %v", err)
	}

	child, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ParentID:     &first.ID,
		Position:     0,
		Target:       map[string]any{"type": "page", "slug": "child"},
		Translations: fixture.translations("child"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem child: %v", err)
	}

	if _, err := service.BulkReorderMenuItems(ctx, menus.BulkReorderMenuItemsInput{
		MenuID:    menu.ID,
		UpdatedBy: uuid.Nil,
		Items: []menus.ItemOrder{
			{ItemID: second.ID, Position: 0},
			{ItemID: first.ID, Position: 1},
			{ItemID: child.ID, ParentID: &second.ID, Position: 0},
		},
	}); err != nil {
		t.Fatalf("BulkReorderMenuItems move: %v", err)
	}

	reloaded, err := service.GetMenu(ctx, menu.ID)
	if err != nil {
		t.Fatalf("GetMenu: %v", err)
	}
	if len(reloaded.Items) != 2 {
		t.Fatalf("expected 2 root items, got %d", len(reloaded.Items))
	}
	if reloaded.Items[0].ID != second.ID {
		t.Fatalf("expected second to be first root, got %s", reloaded.Items[0].ID)
	}
	if len(reloaded.Items[0].Children) != 1 || reloaded.Items[0].Children[0].ID != child.ID {
		t.Fatalf("expected child to move under second, got %+v", reloaded.Items[0].Children)
	}
	if len(reloaded.Items[0].Children[0].Translations) == 0 {
		t.Fatalf("expected child translations to remain")
	}
}

func TestService_UpdateMenuItem_TargetValidation(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	service := newService(t)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("CreateMenu: %v", err)
	}

	item, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		Position:     0,
		Target:       map[string]any{"type": "page", "slug": "home"},
		Translations: fixture.translations("home"),
	})
	if err != nil {
		t.Fatalf("AddMenuItem: %v", err)
	}

	_, err = service.UpdateMenuItem(ctx, menus.UpdateMenuItemInput{
		ItemID:    item.ID,
		Target:    map[string]any{"type": ""},
		UpdatedBy: uuid.Nil,
	})
	if !errors.Is(err, menus.ErrMenuItemTargetMissing) {
		t.Fatalf("expected ErrMenuItemTargetMissing, got %v", err)
	}

	updated, err := service.UpdateMenuItem(ctx, menus.UpdateMenuItemInput{
		ItemID: item.ID,
		Target: map[string]any{
			"type": "external",
			"url":  "https://example.com",
		},
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("UpdateMenuItem: %v", err)
	}
	if updated.Target["type"] != "external" {
		t.Fatalf("expected target type external, got %v", updated.Target["type"])
	}
}

type stubMenuUsageResolver struct {
	bindings []menus.MenuUsageBinding
	err      error
}

func (s *stubMenuUsageResolver) ResolveMenuUsage(context.Context, uuid.UUID) ([]menus.MenuUsageBinding, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.bindings, nil
}

func newService(t *testing.T) menus.Service {
	t.Helper()
	fixture := loadServiceFixture(t)
	return newServiceWithLocales(t, fixture.locales(), uuid.New, nil)
}

func newServiceWithIDs(t *testing.T, fixture serviceFixture, ids ...uuid.UUID) (menus.Service, []uuid.UUID) {
	t.Helper()
	idx := 0
	idGen := func() uuid.UUID {
		if idx >= len(ids) {
			t.Fatalf("id generator exhausted")
		}
		id := ids[idx]
		idx++
		return id
	}
	svc := newServiceWithLocales(t, fixture.locales(), idGen, nil)
	return svc, ids
}

func newServiceWithLocales(t *testing.T, locales []content.Locale, idGen menus.IDGenerator, pageRepo menus.PageRepository, extra ...menus.ServiceOption) menus.Service {
	t.Helper()

	menuRepo := menus.NewMemoryMenuRepository()
	itemRepo := menus.NewMemoryMenuItemRepository()
	trRepo := menus.NewMemoryMenuItemTranslationRepository()
	localeRepo := content.NewMemoryLocaleRepository()
	for i := range locales {
		locale := locales[i]
		localeRepo.Put(&locale)
	}

	now := func() time.Time { return time.Unix(0, 0) }
	opts := []menus.ServiceOption{
		menus.WithClock(now),
		menus.WithIDGenerator(idGen),
	}
	if pageRepo != nil {
		opts = append(opts, menus.WithPageRepository(pageRepo))
	}
	opts = append(opts, extra...)
	return menus.NewService(menuRepo, itemRepo, trRepo, localeRepo, opts...)
}

func assertError(err error, target error) error {
	if err == nil {
		return errors.New("expected error, got nil")
	}
	if !errors.Is(err, target) {
		return errors.Join(errors.New("unexpected error"), err)
	}
	return nil
}
