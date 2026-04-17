package widgets

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWidgetLocalizationRegressionSnapshot(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC)
	localePrimary := uuid.MustParse("00000000-0000-0000-0000-000000000301")
	localeFallback := uuid.MustParse("00000000-0000-0000-0000-000000000302")
	localeMissing := uuid.MustParse("00000000-0000-0000-0000-000000000303")
	actorID := uuid.MustParse("00000000-0000-0000-0000-000000000304")

	svc := newServiceWithAreas(
		WithClock(func() time.Time { return now }),
		WithIDGenerator(sequentialIDs(
			"00000000-0000-0000-0000-000000000401",
			"00000000-0000-0000-0000-000000000402",
			"00000000-0000-0000-0000-000000000403",
			"00000000-0000-0000-0000-000000000404",
		)),
	)

	definition, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name: "homepage.hero",
		Schema: map[string]any{
			"fields": []any{
				map[string]any{"name": "headline"},
				map[string]any{"name": "cta_label"},
				map[string]any{"name": "eyebrow"},
			},
		},
		Defaults: map[string]any{"eyebrow": "Featured"},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}
	if _, err := svc.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{
		Code: "homepage.hero",
		Name: "Homepage Hero",
	}); err != nil {
		t.Fatalf("register area definition: %v", err)
	}

	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID:  definition.ID,
		Configuration: map[string]any{"headline": "Base headline", "cta_label": "Base CTA"},
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if _, err := svc.AssignWidgetToArea(ctx, AssignWidgetToAreaInput{
		AreaCode:   "homepage.hero",
		InstanceID: instance.ID,
	}); err != nil {
		t.Fatalf("assign widget: %v", err)
	}
	if _, err := svc.AddTranslation(ctx, AddTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   localeFallback,
		Content: map[string]any{
			"headline": "Fallback headline",
		},
	}); err != nil {
		t.Fatalf("add fallback translation: %v", err)
	}
	if _, err := svc.AddTranslation(ctx, AddTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   localePrimary,
		Content: map[string]any{
			"headline":  "Primary headline",
			"cta_label": "Primary CTA",
		},
	}); err != nil {
		t.Fatalf("add primary translation: %v", err)
	}

	primary, err := svc.ResolveArea(ctx, ResolveAreaInput{
		AreaCode:          "homepage.hero",
		LocaleID:          &localePrimary,
		FallbackLocaleIDs: []uuid.UUID{localeFallback},
		Now:               now,
	})
	if err != nil {
		t.Fatalf("resolve primary: %v", err)
	}
	fallback, err := svc.ResolveArea(ctx, ResolveAreaInput{
		AreaCode:          "homepage.hero",
		LocaleID:          &localeMissing,
		FallbackLocaleIDs: []uuid.UUID{localeFallback},
		Now:               now,
	})
	if err != nil {
		t.Fatalf("resolve fallback: %v", err)
	}
	baseOnly, err := svc.ResolveArea(ctx, ResolveAreaInput{
		AreaCode: "homepage.hero",
		LocaleID: &localeMissing,
		Now:      now,
	})
	if err != nil {
		t.Fatalf("resolve base only: %v", err)
	}

	snapshot := map[string]any{
		"primary":   resolvedWidgetSnapshot(primary[0]),
		"fallback":  resolvedWidgetSnapshot(fallback[0]),
		"base_only": resolvedWidgetSnapshot(baseOnly[0]),
	}
	assertWidgetLocalizationSnapshot(t, snapshot, filepath.Join("testdata", "localization_regression_snapshot.json"))
}

func resolvedWidgetSnapshot(widget *ResolvedWidget) map[string]any {
	payload := map[string]any{
		"config":            widget.Config,
		"translation_count": len(widget.Instance.Translations),
	}
	if widget.ResolvedLocaleID != nil {
		payload["resolved_locale_id"] = widget.ResolvedLocaleID.String()
	} else {
		payload["resolved_locale_id"] = ""
	}
	return payload
}

func assertWidgetLocalizationSnapshot(t *testing.T, payload map[string]any, snapshotPath string) {
	t.Helper()
	got, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal widget localization snapshot: %v", err)
	}
	want, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot %q: %v", snapshotPath, err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Fatalf("widget localization snapshot mismatch\nexpected:\n%s\n\ngot:\n%s", string(want), string(got))
	}
}
