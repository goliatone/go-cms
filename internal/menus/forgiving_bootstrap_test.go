package menus_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/menus"
	"github.com/google/uuid"
)

func TestForgivingBootstrap_ChildBeforeParent_Reconciles(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	svc := newServiceWithLocales(t, fixture.locales(), func(menus.AddMenuItemInput) uuid.UUID { return uuid.New() }, nil, menus.WithForgivingMenuBootstrap(true))

	actor := uuid.New()
	menu, err := svc.GetOrCreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary_navigation",
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("GetOrCreateMenu: %v", err)
	}

	child, err := svc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:                   menu.ID,
		ParentCode:               "nav.group.main",
		ExternalCode:             "nav.item.child",
		Type:                     menus.MenuItemTypeItem,
		Target:                   map[string]any{"type": "url", "url": "/child"},
		AllowMissingTranslations: true,
		CreatedBy:                actor,
		UpdatedBy:                actor,
	})
	if err != nil {
		t.Fatalf("AddMenuItem child: %v", err)
	}
	if child.ParentID != nil {
		t.Fatalf("expected child ParentID to be nil, got %v", *child.ParentID)
	}
	if child.ParentRef == nil || *child.ParentRef != "nav.group.main" {
		t.Fatalf("expected child ParentRef %q, got %v", "nav.group.main", child.ParentRef)
	}

	if _, err := svc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:                   menu.ID,
		ExternalCode:             "nav.group.main",
		Type:                     menus.MenuItemTypeGroup,
		Collapsible:              true,
		AllowMissingTranslations: true,
		CreatedBy:                actor,
		UpdatedBy:                actor,
	}); err != nil {
		t.Fatalf("AddMenuItem parent: %v", err)
	}

	hydrated, err := svc.GetMenu(ctx, menu.ID)
	if err != nil {
		t.Fatalf("GetMenu: %v", err)
	}

	var (
		foundParent bool
		foundChild  bool
	)
	for _, item := range hydrated.Items {
		if item.ExternalCode != "nav.group.main" {
			continue
		}
		foundParent = true
		for _, kid := range item.Children {
			if kid.ExternalCode == "nav.item.child" {
				foundChild = true
			}
		}
	}

	if !foundParent {
		t.Fatalf("expected parent group to exist")
	}
	if !foundChild {
		t.Fatalf("expected child to be linked under parent after reconciliation")
	}
}

func TestForgivingBootstrap_CollapsibleBeforeChildren_NavigationDerivesState(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	svc := newServiceWithLocales(t, fixture.locales(), func(menus.AddMenuItemInput) uuid.UUID { return uuid.New() }, nil, menus.WithForgivingMenuBootstrap(true))

	actor := uuid.New()
	menu, err := svc.GetOrCreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary_navigation",
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("GetOrCreateMenu: %v", err)
	}

	if _, err := svc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ExternalCode: "group.content",
		Type:         menus.MenuItemTypeGroup,
		Collapsible:  true,
		Translations: fixture.translations("navigation_company"),
		CreatedBy:    actor,
		UpdatedBy:    actor,
	}); err != nil {
		t.Fatalf("AddMenuItem group: %v", err)
	}

	if _, err := svc.ResolveNavigation(ctx, menu.Code, "en"); err != nil {
		t.Fatalf("ResolveNavigation (no children): %v", err)
	}

	if _, err := svc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ParentCode:   "group.content",
		ExternalCode: "item.content.list",
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "url", "url": "/content"},
		Translations: fixture.translations("home"),
		CreatedBy:    actor,
		UpdatedBy:    actor,
	}); err != nil {
		t.Fatalf("AddMenuItem child: %v", err)
	}

	nodes, err := svc.ResolveNavigation(ctx, menu.Code, "en")
	if err != nil {
		t.Fatalf("ResolveNavigation (with children): %v", err)
	}
	if len(nodes) == 0 || len(nodes[0].Children) == 0 {
		t.Fatalf("expected group node with children")
	}
	if !nodes[0].Collapsible {
		t.Fatalf("expected collapsible to be true once children exist")
	}
}

func TestForgivingBootstrap_UpsertMenuItem_IdempotentAndMergesTranslations(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	svc := newServiceWithLocales(t, fixture.locales(), func(menus.AddMenuItemInput) uuid.UUID { return uuid.New() }, nil, menus.WithForgivingMenuBootstrap(true))

	actor := uuid.New()

	first, err := svc.UpsertMenuItem(ctx, menus.UpsertMenuItemInput{
		MenuCode:     "primary_navigation",
		ExternalCode: "nav.item.about",
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "url", "url": "/about"},
		Translations: fixture.translations("about"),
		Actor:        actor,
	})
	if err != nil {
		t.Fatalf("UpsertMenuItem first: %v", err)
	}

	second, err := svc.UpsertMenuItem(ctx, menus.UpsertMenuItemInput{
		MenuCode:     "primary_navigation",
		ExternalCode: "nav.item.about",
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "url", "url": "/about"},
		Translations: fixture.translations("home"), // en+es; should merge missing locale(s)
		Actor:        actor,
	})
	if err != nil {
		t.Fatalf("UpsertMenuItem second: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected idempotent upsert (same item ID), got %v and %v", first.ID, second.ID)
	}

	menu, err := svc.GetMenuByCode(ctx, "primary_navigation")
	if err != nil {
		t.Fatalf("GetMenuByCode: %v", err)
	}

	var about *menus.MenuItem
	var walk func([]*menus.MenuItem)
	walk = func(items []*menus.MenuItem) {
		for _, item := range items {
			if item.ExternalCode == "nav.item.about" {
				about = item
				return
			}
			if len(item.Children) > 0 {
				walk(item.Children)
			}
		}
	}
	walk(menu.Items)

	if about == nil {
		t.Fatalf("expected upserted item in menu")
	}
	if len(about.Translations) < 2 {
		t.Fatalf("expected merged translations to include multiple locales, got %d", len(about.Translations))
	}
}

func TestForgivingBootstrap_ReconcileMenu_DetectsCycles(t *testing.T) {
	ctx := context.Background()
	fixture := loadServiceFixture(t)
	svc := newServiceWithLocales(t, fixture.locales(), func(menus.AddMenuItemInput) uuid.UUID { return uuid.New() }, nil, menus.WithForgivingMenuBootstrap(true))

	actor := uuid.New()
	menu, err := svc.GetOrCreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary_navigation",
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("GetOrCreateMenu: %v", err)
	}

	if _, err := svc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:                   menu.ID,
		ParentCode:               "b",
		ExternalCode:             "a",
		Type:                     menus.MenuItemTypeGroup,
		AllowMissingTranslations: true,
		CreatedBy:                actor,
		UpdatedBy:                actor,
	}); err != nil {
		t.Fatalf("AddMenuItem a: %v", err)
	}

	// This insert triggers reconciliation which should detect the cycle.
	_, err = svc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:                   menu.ID,
		ParentCode:               "a",
		ExternalCode:             "b",
		Type:                     menus.MenuItemTypeGroup,
		AllowMissingTranslations: true,
		CreatedBy:                actor,
		UpdatedBy:                actor,
	})
	if err == nil {
		t.Fatalf("expected cycle error")
	}
	if !errors.Is(err, menus.ErrMenuItemCycle) {
		t.Fatalf("expected ErrMenuItemCycle, got %v", err)
	}
}
