package cms_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/google/uuid"
)

func TestModule_Menus_AllowOutOfOrderUpserts_CollapsibleAndDeferredParents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	cfg := cms.DefaultConfig()
	cfg.Menus.AllowOutOfOrderUpserts = true

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	actor := uuid.New()

	if _, err := module.Menus().UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
		Path:        "primary.content",
		Type:        "group",
		Collapsible: true,
		Collapsed:   true,
		Translations: []cms.MenuItemTranslationInput{
			{Locale: "en", GroupTitle: "Content"},
		},
		Actor: actor,
	}); err != nil {
		t.Fatalf("upsert collapsible group without children: %v", err)
	}

	nodes, err := module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation (no children): %v", err)
	}

	var content *cms.NavigationNode
	for i := range nodes {
		if nodes[i].Type == "group" && nodes[i].Label == "Content" {
			content = &nodes[i]
			break
		}
	}
	if content == nil {
		t.Fatalf("expected content group node")
	}
	if content.Collapsible || content.Collapsed {
		t.Fatalf("expected group to not resolve as collapsible without children; got collapsible=%v collapsed=%v", content.Collapsible, content.Collapsed)
	}

	if _, err := module.Menus().UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
		Path:       "primary.content.pages",
		ParentPath: "primary.content",
		Type:       "item",
		Target:     map[string]any{"type": "url", "url": "/pages"},
		Translations: []cms.MenuItemTranslationInput{
			{Locale: "en", Label: "Pages"},
		},
		Actor: actor,
	}); err != nil {
		t.Fatalf("upsert child: %v", err)
	}

	nodes, err = module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation (with children): %v", err)
	}

	content = nil
	for i := range nodes {
		if nodes[i].Type == "group" && nodes[i].Label == "Content" {
			content = &nodes[i]
			break
		}
	}
	if content == nil {
		t.Fatalf("expected content group node after child insert")
	}
	if !content.Collapsible || !content.Collapsed {
		t.Fatalf("expected group to resolve as collapsible+collapsed once children exist; got collapsible=%v collapsed=%v", content.Collapsible, content.Collapsed)
	}
	if len(content.Children) != 1 || content.Children[0].URL != "/pages" {
		t.Fatalf("expected /pages under content group, got %v", content.Children)
	}

	if _, err := module.Menus().UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
		Path:       "primary.deferred.child",
		ParentPath: "primary.deferred",
		Type:       "item",
		Target:     map[string]any{"type": "url", "url": "/deferred/child"},
		Translations: []cms.MenuItemTranslationInput{
			{Locale: "en", Label: "Deferred Child"},
		},
		Actor: actor,
	}); err != nil {
		t.Fatalf("upsert child before parent: %v", err)
	}

	if _, err := module.Menus().UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
		Path: "primary.deferred",
		Type: "group",
		Translations: []cms.MenuItemTranslationInput{
			{Locale: "en", GroupTitle: "Deferred"},
		},
		Actor: actor,
	}); err != nil {
		t.Fatalf("upsert parent after child: %v", err)
	}

	nodes, err = module.Menus().ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation (after parent insert): %v", err)
	}

	var deferred *cms.NavigationNode
	for i := range nodes {
		if nodes[i].Type == "group" && nodes[i].Label == "Deferred" {
			deferred = &nodes[i]
			break
		}
	}
	if deferred == nil {
		t.Fatalf("expected deferred group node")
	}
	if len(deferred.Children) != 1 || deferred.Children[0].URL != "/deferred/child" {
		t.Fatalf("expected deferred child to be linked under parent, got %v", deferred.Children)
	}
	for _, node := range nodes {
		if node.URL == "/deferred/child" {
			t.Fatalf("expected deferred child not to be a root node after reconciliation")
		}
	}
}
