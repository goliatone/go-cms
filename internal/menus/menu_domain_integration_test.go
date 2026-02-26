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

func TestMenuService_LocalizedBindingsFallbackIntegration(t *testing.T) {
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
	menuItemRepo := menus.NewBunMenuItemRepository(bunDB)
	menuTranslationRepo := menus.NewBunMenuItemTranslationRepository(bunDB)
	localeRepo := content.NewBunLocaleRepository(bunDB)
	bindingRepo := menus.NewBunMenuLocationBindingRepository(bunDB)
	profileRepo := menus.NewBunMenuViewProfileRepository(bunDB)

	service := menus.NewService(
		menuRepo,
		menuItemRepo,
		menuTranslationRepo,
		localeRepo,
		menus.WithMenuLocationBindingRepository(bindingRepo),
		menus.WithMenuViewProfileRepository(profileRepo),
	)

	actor := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	defaultMenu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "site_primary",
		Status:    menus.MenuStatusPublished,
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("create default menu: %v", err)
	}
	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       defaultMenu.ID,
		Position:     0,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": "/default"},
		Translations: []menus.MenuItemTranslationInput{{Locale: "en", Label: "Default"}},
		CreatedBy:    actor,
		UpdatedBy:    actor,
	})
	if err != nil {
		t.Fatalf("add default menu item: %v", err)
	}

	spanishMenu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "site_primary_es",
		Status:    menus.MenuStatusDraft,
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("create spanish menu: %v", err)
	}
	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       spanishMenu.ID,
		Position:     0,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": "/es"},
		Translations: []menus.MenuItemTranslationInput{{Locale: "es", Label: "Espanol"}},
		CreatedBy:    actor,
		UpdatedBy:    actor,
	})
	if err != nil {
		t.Fatalf("add spanish menu item: %v", err)
	}

	_, err = service.UpsertMenuViewProfile(ctx, menus.UpsertMenuViewProfileInput{
		Code:        "top_one",
		Name:        "Top One",
		Mode:        menus.MenuViewModeTopLevelLimit,
		MaxTopLevel: ptrInt(1),
		Status:      menus.MenuStatusPublished,
		Actor:       actor,
	})
	if err != nil {
		t.Fatalf("upsert view profile: %v", err)
	}

	_, err = service.UpsertMenuLocationBinding(ctx, menus.UpsertMenuLocationBindingInput{
		Location:        "site.main",
		MenuCode:        "site_primary",
		ViewProfileCode: strPtr("top_one"),
		Priority:        5,
		Status:          menus.MenuStatusPublished,
		Actor:           actor,
	})
	if err != nil {
		t.Fatalf("upsert default binding: %v", err)
	}

	_, err = service.UpsertMenuLocationBinding(ctx, menus.UpsertMenuLocationBindingInput{
		Location: "site.main",
		MenuCode: "site_primary_es",
		Locale:   strPtr("es"),
		Priority: 10,
		Status:   menus.MenuStatusDraft,
		Actor:    actor,
	})
	if err != nil {
		t.Fatalf("upsert spanish binding: %v", err)
	}

	fallback, err := service.MenuByLocation(ctx, "site.main", "fr", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve fallback menu: %v", err)
	}
	if fallback.Binding == nil || fallback.Binding.MenuCode != "site_primary" {
		t.Fatalf("expected default binding, got %#v", fallback.Binding)
	}
	if len(fallback.Items) != 1 || fallback.Items[0].Label != "Default" {
		t.Fatalf("unexpected fallback items: %#v", fallback.Items)
	}

	preview, err := service.MenuByLocation(ctx, "site.main", "es", menus.MenuQueryOptions{IncludeDrafts: true, PreviewToken: "preview"})
	if err != nil {
		t.Fatalf("resolve preview menu: %v", err)
	}
	if preview.Binding == nil || preview.Binding.MenuCode != "site_primary_es" {
		t.Fatalf("expected spanish binding in preview, got %#v", preview.Binding)
	}
	if len(preview.Items) != 1 || preview.Items[0].Label != "Espanol" {
		t.Fatalf("unexpected preview items: %#v", preview.Items)
	}
}

func strPtr(v string) *string {
	return &v
}
