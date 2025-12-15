package cms_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/google/uuid"
)

func TestModule_Menus_UpsertMenuItemByPath_DefaultPositionAppends(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	module, err := cms.New(cms.DefaultConfig())
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	actor := uuid.New()
	if _, err := module.Menus().UpsertMenu(ctx, "primary", nil, actor); err != nil {
		t.Fatalf("upsert menu: %v", err)
	}

	for _, spec := range []struct {
		path string
		url  string
	}{
		{path: "primary.a", url: "/a"},
		{path: "primary.b", url: "/b"},
		{path: "primary.c", url: "/c"},
	} {
		if _, err := module.Menus().UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
			Path: spec.path,
			Type: "item",
			Target: map[string]any{
				"type": "url",
				"url":  spec.url,
			},
			Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: spec.path}},
			Actor:        actor,
		}); err != nil {
			t.Fatalf("upsert menu item %s: %v", spec.path, err)
		}
	}

	nodes, err := module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 root nodes, got %d", len(nodes))
	}
	if nodes[0].URL != "/a" || nodes[1].URL != "/b" || nodes[2].URL != "/c" {
		t.Fatalf("expected insertion order /a,/b,/c got %q,%q,%q", nodes[0].URL, nodes[1].URL, nodes[2].URL)
	}
}

func TestModule_Menus_UpdateMenuItemByPath_PositionClampsToAppend(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	module, err := cms.New(cms.DefaultConfig())
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	actor := uuid.New()
	pos0, pos1, pos2 := 0, 1, 2
	if err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
		Menus:    module.Menus(),
		MenuCode: "primary",
		Locale:   "en",
		Actor:    actor,
		Items: []cms.SeedMenuItem{
			{Path: "primary.a", Position: &pos0, Type: "item", Target: map[string]any{"type": "url", "url": "/a"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "a"}}},
			{Path: "primary.b", Position: &pos1, Type: "item", Target: map[string]any{"type": "url", "url": "/b"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "b"}}},
			{Path: "primary.c", Position: &pos2, Type: "item", Target: map[string]any{"type": "url", "url": "/c"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "c"}}},
		},
	}); err != nil {
		t.Fatalf("seed menu: %v", err)
	}

	icon := "updated"
	if _, err := module.Menus().UpdateMenuItemByPath(ctx, "primary", "primary.b", cms.UpdateMenuItemByPathInput{
		Icon:  &icon,
		Actor: actor,
	}); err != nil {
		t.Fatalf("update menu item without position: %v", err)
	}

	nodes, err := module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation after update: %v", err)
	}
	if len(nodes) != 3 || nodes[0].URL != "/a" || nodes[1].URL != "/b" || nodes[2].URL != "/c" {
		t.Fatalf("expected update without position to preserve /a,/b,/c")
	}

	tooLarge := 99
	if _, err := module.Menus().UpdateMenuItemByPath(ctx, "primary", "primary.b", cms.UpdateMenuItemByPathInput{
		Position: &tooLarge,
		Actor:    actor,
	}); err != nil {
		t.Fatalf("update menu item: %v", err)
	}

	nodes, err = module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 root nodes, got %d", len(nodes))
	}
	if nodes[0].URL != "/a" || nodes[1].URL != "/c" || nodes[2].URL != "/b" {
		t.Fatalf("expected /a,/c,/b got %q,%q,%q", nodes[0].URL, nodes[1].URL, nodes[2].URL)
	}

	if _, err := module.Menus().ReconcileMenuByCode(ctx, "primary", actor); err != nil {
		t.Fatalf("reconcile menu: %v", err)
	}
	nodes, err = module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation after reconcile: %v", err)
	}
	if nodes[0].URL != "/a" || nodes[1].URL != "/c" || nodes[2].URL != "/b" {
		t.Fatalf("expected reconcile to preserve order /a,/c,/b got %q,%q,%q", nodes[0].URL, nodes[1].URL, nodes[2].URL)
	}
}

func TestModule_Menus_OrderingHelpersAndEnsurePrune(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	module, err := cms.New(cms.DefaultConfig())
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	actor := uuid.New()
	pos0, pos1, pos2 := 0, 1, 2
	if err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
		Menus:    module.Menus(),
		MenuCode: "primary",
		Locale:   "en",
		Actor:    actor,
		Ensure:   true,
		Items: []cms.SeedMenuItem{
			{Path: "primary.a", Position: &pos1, Type: "item", Target: map[string]any{"type": "url", "url": "/a"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "a"}}},
			{Path: "primary.b", Position: &pos0, Type: "item", Target: map[string]any{"type": "url", "url": "/b"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "b"}}},
			{Path: "primary.group", Position: &pos2, Type: "group", Translations: []cms.MenuItemTranslationInput{{Locale: "en", GroupTitle: "Group"}}},
			{Path: "primary.group.x", Position: &pos0, Type: "item", Target: map[string]any{"type": "url", "url": "/x"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "x"}}},
			{Path: "primary.group.y", Position: &pos1, Type: "item", Target: map[string]any{"type": "url", "url": "/y"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "y"}}},
		},
	}); err != nil {
		t.Fatalf("seed menu: %v", err)
	}

	nodes, err := module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 root nodes, got %d", len(nodes))
	}
	if nodes[0].URL != "/b" || nodes[1].URL != "/a" || nodes[2].Type != "group" {
		t.Fatalf("expected ensured order /b,/a,group got %q,%q,%q", nodes[0].URL, nodes[1].URL, nodes[2].Type)
	}

	if err := module.Menus().MoveMenuItemToTop(ctx, "primary", "primary.group.y", actor); err != nil {
		t.Fatalf("move to top: %v", err)
	}
	nodes, err = module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation: %v", err)
	}
	if len(nodes) != 3 || len(nodes[2].Children) != 2 {
		t.Fatalf("expected group with 2 children")
	}
	if nodes[2].Children[0].URL != "/y" || nodes[2].Children[1].URL != "/x" {
		t.Fatalf("expected /y,/x under group got %q,%q", nodes[2].Children[0].URL, nodes[2].Children[1].URL)
	}

	if err := module.Menus().MoveMenuItemBefore(ctx, "primary", "primary.a", "primary.group.y", actor); err != nil {
		t.Fatalf("move before: %v", err)
	}
	nodes, err = module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation: %v", err)
	}
	if len(nodes) != 2 || len(nodes[1].Children) != 3 {
		t.Fatalf("expected root with /b and group with 3 children")
	}
	if nodes[1].Children[0].URL != "/a" || nodes[1].Children[1].URL != "/y" || nodes[1].Children[2].URL != "/x" {
		t.Fatalf("expected /a,/y,/x under group got %q,%q,%q", nodes[1].Children[0].URL, nodes[1].Children[1].URL, nodes[1].Children[2].URL)
	}

	if err := module.Menus().SetMenuSiblingOrder(ctx, "primary", "primary.group", []string{"primary.group.x", "primary.group.y"}, actor); err != nil {
		t.Fatalf("set sibling order: %v", err)
	}
	nodes, err = module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation: %v", err)
	}
	if nodes[1].Children[0].URL != "/x" || nodes[1].Children[1].URL != "/y" {
		t.Fatalf("expected /x,/y first under group got %q,%q", nodes[1].Children[0].URL, nodes[1].Children[1].URL)
	}

	if _, err := module.Menus().UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
		Path:         "primary.extra",
		Type:         "item",
		Target:       map[string]any{"type": "url", "url": "/extra"},
		Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "extra"}},
		Actor:        actor,
	}); err != nil {
		t.Fatalf("add extra item: %v", err)
	}

	if err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
		Menus:            module.Menus(),
		MenuCode:         "primary",
		Locale:           "en",
		Actor:            actor,
		Ensure:           true,
		PruneUnspecified: true,
		Items: []cms.SeedMenuItem{
			{Path: "primary.b", Position: &pos0, Type: "item", Target: map[string]any{"type": "url", "url": "/b"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "b"}}},
			{Path: "primary.a", Position: &pos1, Type: "item", Target: map[string]any{"type": "url", "url": "/a"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "a"}}},
			{Path: "primary.group", Position: &pos2, Type: "group", Translations: []cms.MenuItemTranslationInput{{Locale: "en", GroupTitle: "Group"}}},
			{Path: "primary.group.x", Position: &pos0, Type: "item", Target: map[string]any{"type": "url", "url": "/x"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "x"}}},
			{Path: "primary.group.y", Position: &pos1, Type: "item", Target: map[string]any{"type": "url", "url": "/y"}, Translations: []cms.MenuItemTranslationInput{{Locale: "en", Label: "y"}}},
		},
	}); err != nil {
		t.Fatalf("seed menu ensure+prune: %v", err)
	}

	items, err := module.Menus().ListMenuItemsByCode(ctx, "primary")
	if err != nil {
		t.Fatalf("list menu items: %v", err)
	}
	for _, item := range items {
		if item != nil && item.Path == "primary.extra" {
			t.Fatalf("expected primary.extra to be pruned")
		}
	}
}
