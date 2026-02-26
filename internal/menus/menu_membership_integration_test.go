package menus_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

type phase3IntegrationFixture struct {
	menuSvc    menus.Service
	contentSvc content.Service
	contentTy  *content.ContentType
	actor      uuid.UUID
}

func TestMenuService_ContentContributionsDefaultLocationsIntegration(t *testing.T) {
	fixture := newPhase3IntegrationFixture(t)
	fixture.createMenuWithBinding(t, "main_nav", "site.main", "Main", "/main")
	fixture.createMenuWithBinding(t, "footer_nav", "site.footer", "Footer", "/footer")
	fixture.createContent(t, "about", "About", "/about", nil)

	mainResolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve site.main: %v", err)
	}
	assertLabels(t, mainResolved.Items, []string{"Main", "About"})

	footerResolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.footer", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve site.footer: %v", err)
	}
	assertLabels(t, footerResolved.Items, []string{"Footer"})
}

func TestMenuService_ContentContributionsOverridesIntegration(t *testing.T) {
	fixture := newPhase3IntegrationFixture(t)
	fixture.createMenuWithBinding(t, "main_nav", "site.main", "Main", "/main")
	fixture.createMenuWithBinding(t, "footer_nav", "site.footer", "Footer", "/footer")
	fixture.createContent(t, "home", "Home", "/home", nil)
	fixture.createContent(t, "legal", "Legal", "/legal", map[string]any{
		"_navigation": map[string]any{
			"site.main":   "hide",
			"site.footer": "show",
		},
	})

	mainResolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.main", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve site.main: %v", err)
	}
	assertLabels(t, mainResolved.Items, []string{"Main", "Home"})

	footerResolved, err := fixture.menuSvc.MenuByLocation(context.Background(), "site.footer", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve site.footer: %v", err)
	}
	assertLabels(t, footerResolved.Items, []string{"Footer", "Legal"})
	legal := findNodeByLabel(footerResolved.Items, "Legal")
	if legal == nil || legal.ContributionOrigin != content.NavigationOriginOverride {
		t.Fatalf("expected override contribution for Legal, got %#v", legal)
	}
}

func newPhase3IntegrationFixture(t *testing.T) *phase3IntegrationFixture {
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
	registerMenuModels(t, bunDB)
	seedMenuIntegrationEntities(t, bunDB)

	menuRepo := menus.NewBunMenuRepository(bunDB)
	menuItemRepo := menus.NewBunMenuItemRepository(bunDB)
	menuTranslationRepo := menus.NewBunMenuItemTranslationRepository(bunDB)
	bindingRepo := menus.NewBunMenuLocationBindingRepository(bunDB)
	profileRepo := menus.NewBunMenuViewProfileRepository(bunDB)

	contentRepo := content.NewBunContentRepository(bunDB)
	localeRepo := content.NewBunLocaleRepository(bunDB)
	contentTypeRepo := content.NewMemoryContentTypeRepository()

	contentType := &content.ContentType{
		ID:   uuid.MustParse("00000000-0000-0000-0000-000000000210"),
		Name: "page",
		Slug: "page",
		Schema: map[string]any{
			"fields": []map[string]any{{"name": "body", "type": "richtext"}},
		},
	}
	contentType.Capabilities = map[string]any{
		"navigation": navigationCapabilities(true, content.NavigationMergeAppend),
	}
	if err := contentTypeRepo.Put(contentType); err != nil {
		t.Fatalf("seed content type capabilities: %v", err)
	}

	menuSvc := menus.NewService(
		menuRepo,
		menuItemRepo,
		menuTranslationRepo,
		localeRepo,
		menus.WithMenuLocationBindingRepository(bindingRepo),
		menus.WithMenuViewProfileRepository(profileRepo),
		menus.WithContentRepository(contentRepo),
		menus.WithContentTypeRepository(contentTypeRepo),
	)
	contentSvc := content.NewService(contentRepo, contentTypeRepo, localeRepo)

	return &phase3IntegrationFixture{
		menuSvc:    menuSvc,
		contentSvc: contentSvc,
		contentTy:  contentType,
		actor:      uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
	}
}

func (f *phase3IntegrationFixture) createMenuWithBinding(t *testing.T, code, location, label, url string) {
	t.Helper()
	ctx := context.Background()
	menu, err := f.menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      code,
		Location:  location,
		Status:    menus.MenuStatusPublished,
		CreatedBy: f.actor,
		UpdatedBy: f.actor,
	})
	if err != nil {
		t.Fatalf("create menu %s: %v", code, err)
	}
	_, err = f.menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		Position:     0,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": url},
		Translations: []menus.MenuItemTranslationInput{{Locale: "en", Label: label}},
		CreatedBy:    f.actor,
		UpdatedBy:    f.actor,
	})
	if err != nil {
		t.Fatalf("add menu item: %v", err)
	}
	_, err = f.menuSvc.UpsertMenuLocationBinding(ctx, menus.UpsertMenuLocationBindingInput{
		Location: location,
		MenuCode: code,
		Priority: 10,
		Status:   menus.MenuStatusPublished,
		Actor:    f.actor,
	})
	if err != nil {
		t.Fatalf("upsert binding: %v", err)
	}
}

func (f *phase3IntegrationFixture) createContent(t *testing.T, slug, title, path string, metadata map[string]any) {
	t.Helper()
	entryMetadata := map[string]any{"path": path}
	for key, value := range metadata {
		entryMetadata[key] = value
	}
	_, err := f.contentSvc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: f.contentTy.ID,
		Slug:          slug,
		Status:        menus.MenuStatusPublished,
		CreatedBy:     f.actor,
		UpdatedBy:     f.actor,
		Metadata:      entryMetadata,
		Translations: []content.ContentTranslationInput{
			{
				Locale: "en",
				Title:  title,
				Content: map[string]any{
					"body": title,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content %s: %v", slug, err)
	}
}
