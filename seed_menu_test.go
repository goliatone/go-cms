package cms_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/identity"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/google/uuid"
)

func TestSeedMenu_OrderIndependentAndIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	module, err := cms.New(cms.DefaultConfig())
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	actor := uuid.New()
	opts := cms.SeedMenuOptions{
		Menus:    module.Menus(),
		MenuCode: "admin",
		Locale:   "en",
		Actor:    actor,
		Items: []cms.SeedMenuItem{
			{
				Path:   "admin.content.pages",
				Type:   menus.MenuItemTypeItem,
				Target: map[string]any{"type": "url", "url": "/admin/pages"},
				Translations: []cms.MenuItemTranslationInput{
					{Locale: "", Label: "Pages"},
				},
			},
			{
				Path: "admin.content",
				Type: menus.MenuItemTypeGroup,
				Translations: []cms.MenuItemTranslationInput{
					{Locale: "", GroupTitle: "Content"},
				},
			},
		},
	}

	if err := cms.SeedMenu(ctx, opts); err != nil {
		t.Fatalf("seed menu: %v", err)
	}
	if err := cms.SeedMenu(ctx, opts); err != nil {
		t.Fatalf("seed menu second time: %v", err)
	}

	internalSvc := module.Container().MenuService()
	parent, err := internalSvc.GetMenuItemByExternalCode(ctx, "admin", "admin.content")
	if err != nil {
		t.Fatalf("get parent: %v", err)
	}
	child, err := internalSvc.GetMenuItemByExternalCode(ctx, "admin", "admin.content.pages")
	if err != nil {
		t.Fatalf("get child: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("expected child to be linked to parent %s, got %v", parent.ID, child.ParentID)
	}
}

func TestSeedMenu_DeterministicIDsAcrossInstances(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	makeModule := func(t *testing.T) (*cms.Module, menus.Service) {
		t.Helper()
		module, err := cms.New(cms.DefaultConfig())
		if err != nil {
			t.Fatalf("new module: %v", err)
		}
		return module, module.Container().MenuService()
	}

	seed := func(t *testing.T, module *cms.Module) {
		t.Helper()
		actor := uuid.New()
		if err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
			Menus:    module.Menus(),
			MenuCode: "admin",
			Locale:   "en",
			Actor:    actor,
			Items: []cms.SeedMenuItem{
				{
					Path: "admin.content",
					Type: menus.MenuItemTypeGroup,
					Translations: []cms.MenuItemTranslationInput{
						{Locale: "en", GroupTitle: "Content"},
					},
				},
				{
					Path:   "admin.content.pages",
					Type:   menus.MenuItemTypeItem,
					Target: map[string]any{"type": "url", "url": "/admin/pages"},
					Translations: []cms.MenuItemTranslationInput{
						{Locale: "en", Label: "Pages"},
					},
				},
			},
		}); err != nil {
			t.Fatalf("seed menu: %v", err)
		}
	}

	module1, svc1 := makeModule(t)
	seed(t, module1)
	menu1, err := svc1.GetMenuByCode(ctx, "admin")
	if err != nil {
		t.Fatalf("get menu 1: %v", err)
	}
	if menu1.ID != identity.MenuUUID("admin") {
		t.Fatalf("expected deterministic menu id %s got %s", identity.MenuUUID("admin"), menu1.ID)
	}
	item1, err := svc1.GetMenuItemByExternalCode(ctx, "admin", "admin.content.pages")
	if err != nil {
		t.Fatalf("get item 1: %v", err)
	}

	module2, svc2 := makeModule(t)
	seed(t, module2)
	menu2, err := svc2.GetMenuByCode(ctx, "admin")
	if err != nil {
		t.Fatalf("get menu 2: %v", err)
	}
	item2, err := svc2.GetMenuItemByExternalCode(ctx, "admin", "admin.content.pages")
	if err != nil {
		t.Fatalf("get item 2: %v", err)
	}

	if menu1.ID != menu2.ID {
		t.Fatalf("expected deterministic menu ids to match, got %s and %s", menu1.ID, menu2.ID)
	}
	if item1.ID != item2.ID {
		t.Fatalf("expected deterministic item ids to match, got %s and %s", item1.ID, item2.ID)
	}
}

func TestSeedMenu_AutoCreateParents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	module, err := cms.New(cms.DefaultConfig())
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	actor := uuid.New()
	if err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
		Menus:             module.Menus(),
		MenuCode:          "admin",
		Locale:            "en",
		Actor:             actor,
		AutoCreateParents: true,
		Items: []cms.SeedMenuItem{
			{
				Path:   "admin.content.pages",
				Type:   menus.MenuItemTypeItem,
				Target: map[string]any{"type": "url", "url": "/admin/pages"},
				Translations: []cms.MenuItemTranslationInput{
					{Locale: "en", Label: "Pages"},
				},
			},
		},
	}); err != nil {
		t.Fatalf("seed menu: %v", err)
	}

	internalSvc := module.Container().MenuService()
	parent, err := internalSvc.GetMenuItemByExternalCode(ctx, "admin", "admin.content")
	if err != nil {
		t.Fatalf("get parent: %v", err)
	}
	if parent.Type != menus.MenuItemTypeGroup {
		t.Fatalf("expected scaffolded parent type %q got %q", menus.MenuItemTypeGroup, parent.Type)
	}

	child, err := internalSvc.GetMenuItemByExternalCode(ctx, "admin", "admin.content.pages")
	if err != nil {
		t.Fatalf("get child: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("expected child to be linked to parent %s, got %v", parent.ID, child.ParentID)
	}
}
