package cms_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/google/uuid"
)

func TestModule_Menus_SeedMenuAndResolveNavigation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	module, err := cms.New(cms.DefaultConfig())
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	pos0 := 0
	pos1 := 1
	pos2 := 2

	actor := uuid.New()
	if err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
		Menus:    module.Menus(),
		MenuCode: "primary",
		Locale:   "en",
		Actor:    actor,
		Items: []cms.SeedMenuItem{
			{
				Path:     "primary.home",
				Position: &pos0,
				Type:     "item",
				Target:   map[string]any{"type": "url", "url": "/"},
				Translations: []cms.MenuItemTranslationInput{
					{Locale: "en", Label: "Home"},
				},
			},
			{
				Path:     "primary.content",
				Position: &pos1,
				Type:     "group",
				Translations: []cms.MenuItemTranslationInput{
					{Locale: "en", GroupTitleKey: "menu.group.content"},
				},
			},
			{
				Path:     "primary.content.pages",
				Position: &pos0,
				Type:     "item",
				Target:   map[string]any{"type": "url", "url": "/pages"},
				Translations: []cms.MenuItemTranslationInput{
					{Locale: "en", LabelKey: "menu.pages"},
				},
			},
			{
				Path:     "primary.separator",
				Position: &pos2,
				Type:     "separator",
			},
		},
	}); err != nil {
		t.Fatalf("seed menu: %v", err)
	}

	nodes, err := module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation: %v", err)
	}

	var (
		foundHome  bool
		foundGroup bool
		foundPages bool
	)
	for _, node := range nodes {
		if node.Type == "item" && node.URL == "/" && node.Label == "Home" {
			foundHome = true
		}
		if node.Type == "group" {
			foundGroup = true
			for _, child := range node.Children {
				if child.Type == "item" && child.URL == "/pages" {
					foundPages = true
				}
			}
		}
	}

	if !foundHome {
		t.Fatalf("expected home navigation item")
	}
	if !foundGroup {
		t.Fatalf("expected group navigation node")
	}
	if !foundPages {
		t.Fatalf("expected pages navigation item under group")
	}
}

func TestModule_Menus_LocationBasedResolutionViaPublicContract(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	module, err := cms.New(cms.DefaultConfig())
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	actor := uuid.New()
	if _, err := module.Menus().UpsertMenuWithLocation(ctx, "primary", "site.primary", nil, actor); err != nil {
		t.Fatalf("upsert menu with location: %v", err)
	}
	if _, err := module.Menus().UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
		Path:   "primary.home",
		Type:   "item",
		Target: map[string]any{"type": "url", "url": "/"},
		Translations: []cms.MenuItemTranslationInput{
			{Locale: "en", Label: "Home"},
		},
		Actor: actor,
	}); err != nil {
		t.Fatalf("upsert menu item: %v", err)
	}

	menu, err := module.Menus().GetMenuByLocation(ctx, "site.primary")
	if err != nil {
		t.Fatalf("get menu by location: %v", err)
	}
	if menu == nil || menu.Code != "primary" {
		t.Fatalf("expected primary menu, got %#v", menu)
	}
	if menu.Location != "site.primary" {
		t.Fatalf("expected site.primary location, got %q", menu.Location)
	}

	nodes, err := module.Menus().ResolveNavigationByLocation(ctx, "site.primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation by location: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Label != "Home" || nodes[0].URL != "/" {
		t.Fatalf("unexpected navigation node %#v", nodes[0])
	}
}
