package cms_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/google/uuid"
)

func TestCRUDRegression_BlockWidgetMenu(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en"}
	cfg.Features.Versioning = true
	cfg.Features.Widgets = true

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("new cms module: %v", err)
	}

	t.Run("blocks", func(t *testing.T) {
		exerciseBlockCRUD(t, ctx, module.Blocks())
	})
	t.Run("widgets", func(t *testing.T) {
		exerciseWidgetCRUD(t, ctx, module.Widgets())
	})
	t.Run("menus", func(t *testing.T) {
		exerciseMenuCRUD(t, ctx, module.Menus())
	})
}

func exerciseBlockCRUD(t *testing.T, ctx context.Context, svc cms.BlockService) {
	t.Helper()
	actor := uuid.New()

	definition, err := svc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero-" + uuid.NewString(),
		Schema: map[string]any{"type": "object"},
		Defaults: map[string]any{
			"headline": "Initial hero",
		},
	})
	if err != nil {
		t.Fatalf("register block definition: %v", err)
	}

	instance, err := svc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: definition.ID,
		Region:       "hero",
		Position:     0,
		Configuration: map[string]any{
			"headline": "Initial hero",
		},
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("create block instance: %v", err)
	}

	localeID := uuid.New()
	if _, err := svc.AddTranslation(ctx, blocks.AddTranslationInput{
		BlockInstanceID: instance.ID,
		LocaleID:        localeID,
		Content:         map[string]any{"headline": "Hola"},
	}); err != nil {
		t.Fatalf("add block translation: %v", err)
	}

	if _, err := svc.UpdateDefinition(ctx, blocks.UpdateDefinitionInput{
		ID:          definition.ID,
		Description: stringPtr("Updated hero block"),
		Defaults:    map[string]any{"headline": "Updated hero"},
	}); err != nil {
		t.Fatalf("update block definition: %v", err)
	}

	if _, err := svc.UpdateInstance(ctx, blocks.UpdateInstanceInput{
		InstanceID:    instance.ID,
		Configuration: map[string]any{"headline": "Updated configuration"},
		Position:      intPtr(1),
		UpdatedBy:     actor,
	}); err != nil {
		t.Fatalf("update block instance: %v", err)
	}

	if _, err := svc.UpdateTranslation(ctx, blocks.UpdateTranslationInput{
		BlockInstanceID: instance.ID,
		LocaleID:        localeID,
		Content:         map[string]any{"headline": "Updated hola"},
		UpdatedBy:       actor,
	}); err != nil {
		t.Fatalf("update block translation: %v", err)
	}

	if err := svc.DeleteTranslation(ctx, blocks.DeleteTranslationRequest{
		BlockInstanceID:          instance.ID,
		LocaleID:                 localeID,
		DeletedBy:                actor,
		AllowMissingTranslations: true,
	}); err != nil {
		t.Fatalf("delete block translation: %v", err)
	}

	if err := svc.DeleteInstance(ctx, blocks.DeleteInstanceRequest{
		ID:         instance.ID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("delete block instance: %v", err)
	}

	if err := svc.DeleteDefinition(ctx, blocks.DeleteDefinitionRequest{
		ID:         definition.ID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("delete block definition: %v", err)
	}
}

func exerciseWidgetCRUD(t *testing.T, ctx context.Context, svc cms.WidgetService) {
	t.Helper()
	actor := uuid.New()

	definition, err := svc.RegisterDefinition(ctx, widgets.RegisterDefinitionInput{
		Name:        "announcement-" + uuid.NewString(),
		Description: stringPtr("Used for hero banners"),
		Schema: map[string]any{
			"fields": []any{
				map[string]any{"name": "headline", "type": "text"},
			},
		},
		Defaults: map[string]any{"headline": "Initial announcement"},
	})
	if err != nil {
		t.Fatalf("register widget definition: %v", err)
	}

	instance, err := svc.CreateInstance(ctx, widgets.CreateInstanceInput{
		DefinitionID:  definition.ID,
		Configuration: map[string]any{"headline": "Initial announcement"},
		VisibilityRules: map[string]any{
			"schedule": map[string]any{},
		},
		Position:  0,
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("create widget instance: %v", err)
	}

	if _, err := svc.UpdateInstance(ctx, widgets.UpdateInstanceInput{
		InstanceID:    instance.ID,
		Configuration: map[string]any{"headline": "Updated announcement"},
		UpdatedBy:     actor,
	}); err != nil {
		t.Fatalf("update widget instance: %v", err)
	}

	localeID := uuid.New()
	if _, err := svc.AddTranslation(ctx, widgets.AddTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   localeID,
		Content:    map[string]any{"headline": "Hola anuncio"},
	}); err != nil {
		t.Fatalf("add widget translation: %v", err)
	}

	if err := svc.DeleteTranslation(ctx, widgets.DeleteTranslationRequest{
		InstanceID: instance.ID,
		LocaleID:   localeID,
	}); err != nil {
		t.Fatalf("delete widget translation: %v", err)
	}

	if err := svc.DeleteInstance(ctx, widgets.DeleteInstanceRequest{
		InstanceID: instance.ID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("delete widget instance: %v", err)
	}

	if err := svc.DeleteDefinition(ctx, widgets.DeleteDefinitionRequest{
		ID:         definition.ID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("delete widget definition: %v", err)
	}
}

func exerciseMenuCRUD(t *testing.T, ctx context.Context, svc cms.MenuService) {
	t.Helper()
	actor := uuid.New()

	menu, err := svc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary-" + uuid.NewString(),
		CreatedBy: actor,
		UpdatedBy: actor,
	})
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}

	first, err := svc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Target: map[string]any{
			"type": "url",
			"url":  "/home",
		},
		CreatedBy: actor,
		UpdatedBy: actor,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Home"},
		},
	})
	if err != nil {
		t.Fatalf("add first menu item: %v", err)
	}

	second, err := svc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 1,
		Target: map[string]any{
			"type": "url",
			"url":  "/about",
		},
		CreatedBy: actor,
		UpdatedBy: actor,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "About"},
		},
	})
	if err != nil {
		t.Fatalf("add second menu item: %v", err)
	}

	if _, err := svc.BulkReorderMenuItems(ctx, menus.BulkReorderMenuItemsInput{
		MenuID: menu.ID,
		Items: []menus.ItemOrder{
			{ItemID: first.ID, Position: 1},
			{ItemID: second.ID, Position: 0},
		},
		UpdatedBy: actor,
	}); err != nil {
		t.Fatalf("bulk reorder menu items: %v", err)
	}

	if err := svc.DeleteMenuItem(ctx, menus.DeleteMenuItemRequest{
		ItemID:          second.ID,
		DeletedBy:       actor,
		CascadeChildren: true,
	}); err != nil {
		t.Fatalf("delete second menu item: %v", err)
	}

	if err := svc.DeleteMenu(ctx, menus.DeleteMenuRequest{
		MenuID:    menu.ID,
		DeletedBy: actor,
	}); err != nil {
		t.Fatalf("delete menu: %v", err)
	}
}

func stringPtr(v string) *string {
	return &v
}

func intPtr(v int) *int {
	return &v
}
