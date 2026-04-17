package menus_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/goliatone/go-cms/internal/menus"
	"github.com/google/uuid"
)

func TestMenuLocalizationRegressionSnapshot(t *testing.T) {
	ctx := context.Background()
	service := newService(t)

	menu, err := service.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: uuid.Nil,
		UpdatedBy: uuid.Nil,
	})
	if err != nil {
		t.Fatalf("create menu: %v", err)
	}

	group, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 0,
		Type:     menus.MenuItemTypeGroup,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", GroupTitleKey: "menu.group.main"},
		},
	})
	if err != nil {
		t.Fatalf("add group: %v", err)
	}
	if _, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		ParentID: &group.ID,
		Position: 0,
		Target: map[string]any{
			"type": "page",
			"slug": "products",
		},
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", LabelKey: "menu.products"},
		},
	}); err != nil {
		t.Fatalf("add key-only child: %v", err)
	}
	if _, err := service.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menu.ID,
		Position: 1,
		Target: map[string]any{
			"type": "page",
			"slug": "legacy",
		},
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "menu.legacy", LabelKey: "menu.legacy"},
		},
	}); err != nil {
		t.Fatalf("add legacy mirrored item: %v", err)
	}

	nav, err := service.ResolveNavigation(ctx, "primary", "en")
	if err != nil {
		t.Fatalf("resolve navigation: %v", err)
	}
	if len(nav) != 2 {
		t.Fatalf("expected two root navigation nodes, got %d", len(nav))
	}

	snapshot := map[string]any{
		"group": map[string]any{
			"type":                nav[0].Type,
			"group_title_key":     nav[0].GroupTitleKey,
			"display_label":       nav[0].DisplayLabel,
			"display_group_title": nav[0].DisplayGroupTitle,
			"child": map[string]any{
				"label":         nav[0].Children[0].Label,
				"label_key":     nav[0].Children[0].LabelKey,
				"display_label": nav[0].Children[0].DisplayLabel,
				"url":           nav[0].Children[0].URL,
			},
		},
		"legacy_item": map[string]any{
			"label":         nav[1].Label,
			"label_key":     nav[1].LabelKey,
			"display_label": nav[1].DisplayLabel,
			"url":           nav[1].URL,
		},
	}
	assertMenuLocalizationSnapshot(t, snapshot, filepath.Join("testdata", "localization_regression_snapshot.json"))
}

func assertMenuLocalizationSnapshot(t *testing.T, payload map[string]any, snapshotPath string) {
	t.Helper()
	got, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal menu localization snapshot: %v", err)
	}
	want, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot %q: %v", snapshotPath, err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Fatalf("menu localization snapshot mismatch\nexpected:\n%s\n\ngot:\n%s", string(want), string(got))
	}
}
