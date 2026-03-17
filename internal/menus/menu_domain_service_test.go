package menus_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/google/uuid"
)

func TestService_MenuByCode_AppliesViewProfileProjectionAndOrdering(t *testing.T) {
	ctx := context.Background()
	actor := uuid.New()
	service := newServiceWithLocales(t, []content.Locale{{
		ID:        uuid.New(),
		Code:      "en",
		Display:   "English",
		IsActive:  true,
		IsDefault: true,
	}}, func(menus.AddMenuItemInput) uuid.UUID { return uuid.New() }, nil)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{Code: "primary", CreatedBy: actor, UpdatedBy: actor})
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}

	home, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ExternalCode: "home",
		Position:     0,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": "/home"},
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Home"},
		},
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("add home item: %v", err)
	}

	products, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ExternalCode: "products",
		Position:     1,
		Type:         menus.MenuItemTypeGroup,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Products"},
		},
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("add products item: %v", err)
	}

	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ExternalCode: "products.software",
		ParentID:     &products.ID,
		Position:     0,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": "/products/software"},
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Software"},
		},
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("add software item: %v", err)
	}

	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ExternalCode: "products.hardware",
		ParentID:     &products.ID,
		Position:     1,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": "/products/hardware"},
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Hardware"},
		},
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("add hardware item: %v", err)
	}

	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ExternalCode: "contact",
		Position:     2,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": "/contact"},
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Contact"},
		},
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("add contact item: %v", err)
	}

	profile, err := service.UpsertMenuViewProfile(ctx, menus.UpsertMenuViewProfileInput{
		Code:        "footer_compact",
		Name:        "Footer Compact",
		Mode:        menus.MenuViewModeComposed,
		MaxTopLevel: new(2),
		MaxDepth:    new(2),
		ExcludeItemIDs: []string{
			home.ExternalCode,
		},
		Status: "published",
		Actor:  actor,
	})
	if err != nil {
		t.Fatalf("upsert menu view profile: %v", err)
	}
	if profile.Code != "footer_compact" {
		t.Fatalf("expected profile code footer_compact got %q", profile.Code)
	}

	resolved, err := service.MenuByCode(ctx, "primary", "en", menus.MenuQueryOptions{ViewProfile: "footer_compact"})
	if err != nil {
		t.Fatalf("resolve menu by code: %v", err)
	}

	if resolved.ViewProfile == nil || resolved.ViewProfile.Code != "footer_compact" {
		t.Fatalf("expected resolved view profile footer_compact")
	}
	if len(resolved.Items) != 2 {
		t.Fatalf("expected 2 projected root items, got %d", len(resolved.Items))
	}
	if resolved.Items[0].Label != "Products" || resolved.Items[1].Label != "Contact" {
		t.Fatalf("unexpected projected ordering: %#v", resolved.Items)
	}
	if len(resolved.Items[0].Children) != 2 {
		t.Fatalf("expected products children to be preserved at depth 2")
	}
}

func TestService_MenuByLocation_ResolvesLocaleAndBindingPolicy(t *testing.T) {
	ctx := context.Background()
	actor := uuid.New()
	service := newServiceWithLocales(t, []content.Locale{{
		ID:        uuid.New(),
		Code:      "en",
		Display:   "English",
		IsActive:  true,
		IsDefault: true,
	}, {
		ID:       uuid.New(),
		Code:     "es",
		Display:  "Spanish",
		IsActive: true,
	}}, func(menus.AddMenuItemInput) uuid.UUID { return uuid.New() }, nil)

	defaultMenu, err := service.CreateMenu(ctx, menus.CreateMenuInput{Code: "main_default", CreatedBy: actor, UpdatedBy: actor})
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
		t.Fatalf("add default item: %v", err)
	}

	spanishMenu, err := service.CreateMenu(ctx, menus.CreateMenuInput{Code: "main_es", Status: "draft", CreatedBy: actor, UpdatedBy: actor})
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
		t.Fatalf("add spanish item: %v", err)
	}

	_, err = service.UpsertMenuLocationBinding(ctx, menus.UpsertMenuLocationBindingInput{
		Location: "site.main",
		MenuCode: "main_default",
		Priority: 10,
		Status:   "published",
		Actor:    actor,
	})
	if err != nil {
		t.Fatalf("upsert default binding: %v", err)
	}
	localeES := "es"
	_, err = service.UpsertMenuLocationBinding(ctx, menus.UpsertMenuLocationBindingInput{
		Location: "site.main",
		MenuCode: "main_es",
		Locale:   &localeES,
		Priority: 20,
		Status:   "draft",
		Actor:    actor,
	})
	if err != nil {
		t.Fatalf("upsert es binding: %v", err)
	}

	fallbackResolved, err := service.MenuByLocation(ctx, "site.main", "fr", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve fallback binding: %v", err)
	}
	if fallbackResolved.Binding == nil || fallbackResolved.Binding.MenuCode != "main_default" {
		t.Fatalf("expected default binding fallback, got %#v", fallbackResolved.Binding)
	}

	previewResolved, err := service.MenuByLocation(ctx, "site.main", "es", menus.MenuQueryOptions{IncludeDrafts: true, PreviewToken: "preview-token"})
	if err != nil {
		t.Fatalf("resolve draft preview binding: %v", err)
	}
	if previewResolved.Binding == nil || previewResolved.Binding.MenuCode != "main_es" {
		t.Fatalf("expected spanish draft binding, got %#v", previewResolved.Binding)
	}

	menuA, err := service.CreateMenu(ctx, menus.CreateMenuInput{Code: "a", CreatedBy: actor, UpdatedBy: actor})
	if err != nil {
		t.Fatalf("create menu A: %v", err)
	}
	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menuA.ID,
		Position:     0,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": "/a"},
		Translations: []menus.MenuItemTranslationInput{{Locale: "en", Label: "A"}},
		CreatedBy:    actor,
		UpdatedBy:    actor,
	})
	if err != nil {
		t.Fatalf("add menu A item: %v", err)
	}

	menuB, err := service.CreateMenu(ctx, menus.CreateMenuInput{Code: "b", CreatedBy: actor, UpdatedBy: actor})
	if err != nil {
		t.Fatalf("create menu B: %v", err)
	}
	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menuB.ID,
		Position:     0,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": "/b"},
		Translations: []menus.MenuItemTranslationInput{{Locale: "en", Label: "B"}},
		CreatedBy:    actor,
		UpdatedBy:    actor,
	})
	if err != nil {
		t.Fatalf("add menu B item: %v", err)
	}

	_, err = service.UpsertMenuLocationBinding(ctx, menus.UpsertMenuLocationBindingInput{
		Location: "site.footer",
		MenuCode: "a",
		Priority: 5,
		Status:   "published",
		Actor:    actor,
	})
	if err != nil {
		t.Fatalf("upsert footer binding A: %v", err)
	}
	localeEN := "en"
	_, err = service.UpsertMenuLocationBinding(ctx, menus.UpsertMenuLocationBindingInput{
		Location: "site.footer",
		MenuCode: "b",
		Locale:   &localeEN,
		Priority: 10,
		Status:   "published",
		Actor:    actor,
	})
	if err != nil {
		t.Fatalf("upsert footer binding B: %v", err)
	}

	single, err := service.MenuByLocation(ctx, "site.footer", "en", menus.MenuQueryOptions{})
	if err != nil {
		t.Fatalf("resolve single policy footer menu: %v", err)
	}
	if len(single.Items) != 1 || single.Items[0].Label != "B" {
		t.Fatalf("expected highest-priority single binding, got %#v", single.Items)
	}

	multi, err := service.MenuByLocation(ctx, "site.footer", "en", menus.MenuQueryOptions{BindingPolicy: menus.MenuBindingPolicyPriorityMulti})
	if err != nil {
		t.Fatalf("resolve multi policy footer menu: %v", err)
	}
	if len(multi.Items) != 2 {
		t.Fatalf("expected merged multi-binding items, got %d", len(multi.Items))
	}
	if multi.Items[0].Label != "B" || multi.Items[1].Label != "A" {
		t.Fatalf("unexpected multi-binding order: %#v", multi.Items)
	}
}

func TestService_MenuDepthLimitGuard(t *testing.T) {
	ctx := context.Background()
	actor := uuid.New()
	service := newServiceWithLocales(t, []content.Locale{{
		ID:        uuid.New(),
		Code:      "en",
		Display:   "English",
		IsActive:  true,
		IsDefault: true,
	}}, func(menus.AddMenuItemInput) uuid.UUID { return uuid.New() }, nil, menus.WithMaxMenuDepth(2))

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{Code: "depth_menu", CreatedBy: actor, UpdatedBy: actor})
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}
	parent, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		Position:     0,
		Type:         menus.MenuItemTypeGroup,
		Translations: []menus.MenuItemTranslationInput{{Locale: "en", Label: "Root"}},
		CreatedBy:    actor,
		UpdatedBy:    actor,
	})
	if err != nil {
		t.Fatalf("add parent: %v", err)
	}
	child, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ParentID:     &parent.ID,
		Position:     0,
		Type:         menus.MenuItemTypeGroup,
		Translations: []menus.MenuItemTranslationInput{{Locale: "en", Label: "Child"}},
		CreatedBy:    actor,
		UpdatedBy:    actor,
	})
	if err != nil {
		t.Fatalf("add child: %v", err)
	}

	_, err = service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu.ID,
		ParentID:     &child.ID,
		Position:     0,
		Type:         menus.MenuItemTypeItem,
		Target:       map[string]any{"type": "external", "url": "/grandchild"},
		Translations: []menus.MenuItemTranslationInput{{Locale: "en", Label: "Grandchild"}},
		CreatedBy:    actor,
		UpdatedBy:    actor,
	})
	if !errors.Is(err, menus.ErrMenuItemDepthExceeded) {
		t.Fatalf("expected ErrMenuItemDepthExceeded, got %v", err)
	}
}

//go:fix inline
func ptrInt(v int) *int {
	return new(v)
}
