package menus_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/identity"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/google/uuid"
)

func TestService_DeterministicIDs_AcrossInstances(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	makeService := func() menus.Service {
		menuRepo := menus.NewMemoryMenuRepository()
		itemRepo := menus.NewMemoryMenuItemRepository()
		trRepo := menus.NewMemoryMenuItemTranslationRepository()
		localeRepo := content.NewMemoryLocaleRepository()
		localeRepo.Put(&content.Locale{
			ID:        uuid.New(),
			Code:      "en",
			Display:   "English",
			IsActive:  true,
			IsDefault: true,
			CreatedAt: time.Now().UTC(),
		})

		return menus.NewService(
			menuRepo,
			itemRepo,
			trRepo,
			localeRepo,
			menus.WithMenuIDDeriver(identity.MenuUUID),
			menus.WithIDGenerator(func(input menus.AddMenuItemInput) uuid.UUID {
				if canonicalKey := strings.TrimSpace(input.CanonicalKey); canonicalKey != "" {
					return identity.MenuItemUUID(input.MenuID, canonicalKey)
				}
				if external := strings.TrimSpace(input.ExternalCode); external != "" {
					return identity.UUID("go-cms:menu_item_path:" + external)
				}
				return identity.UUID("go-cms:menu_item_fallback:" + input.MenuID.String())
			}),
		)
	}

	svc1 := makeService()
	menu1, err := svc1.CreateMenu(ctx, menus.CreateMenuInput{Code: "primary"})
	if err != nil {
		t.Fatalf("create menu 1: %v", err)
	}
	if menu1.ID != identity.MenuUUID("primary") {
		t.Fatalf("expected deterministic menu id %s got %s", identity.MenuUUID("primary"), menu1.ID)
	}

	item1, err := svc1.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu1.ID,
		ExternalCode: "primary.home",
		Position:     0,
		Target:       map[string]any{"type": "url", "url": "/"},
		Translations: []menus.MenuItemTranslationInput{{Locale: "en", Label: "Home"}},
	})
	if err != nil {
		t.Fatalf("add item 1: %v", err)
	}
	if item1.CanonicalKey == nil || strings.TrimSpace(*item1.CanonicalKey) == "" {
		t.Fatalf("expected canonical key to be set")
	}

	svc2 := makeService()
	menu2, err := svc2.CreateMenu(ctx, menus.CreateMenuInput{Code: "primary"})
	if err != nil {
		t.Fatalf("create menu 2: %v", err)
	}
	if menu2.ID != menu1.ID {
		t.Fatalf("expected same menu id across instances, got %s and %s", menu1.ID, menu2.ID)
	}

	item2, err := svc2.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:       menu2.ID,
		ExternalCode: "primary.home",
		Position:     0,
		Target:       map[string]any{"type": "url", "url": "/"},
		Translations: []menus.MenuItemTranslationInput{{Locale: "en", Label: "Home"}},
	})
	if err != nil {
		t.Fatalf("add item 2: %v", err)
	}

	expectedItemID := identity.MenuItemUUID(menu1.ID, strings.TrimSpace(*item1.CanonicalKey))
	if item1.ID != expectedItemID {
		t.Fatalf("expected deterministic item id %s got %s", expectedItemID, item1.ID)
	}
	if item2.ID != expectedItemID {
		t.Fatalf("expected same item id across instances, got %s and %s", item1.ID, item2.ID)
	}
}
