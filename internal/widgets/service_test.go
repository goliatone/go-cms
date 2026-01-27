package widgets

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	shortcodepkg "github.com/goliatone/go-cms/internal/shortcode"
	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
)

func TestServiceRegisterDefinitionValidation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		WithClock(func() time.Time { return now }),
		WithIDGenerator(sequentialIDs(
			"00000000-0000-0000-0000-00000000d001",
		)),
	)

	if _, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{}); !errors.Is(err, ErrDefinitionNameRequired) {
		t.Fatalf("expected ErrDefinitionNameRequired, got %v", err)
	}

	if _, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{Name: "promo"}); !errors.Is(err, ErrDefinitionSchemaRequired) {
		t.Fatalf("expected ErrDefinitionSchemaRequired, got %v", err)
	}

	schema := map[string]any{
		"fields": []any{
			map[string]any{"name": "headline"},
		},
	}
	if _, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:     "promo",
		Schema:   schema,
		Defaults: map[string]any{"unknown": "value"},
	}); !errors.Is(err, ErrDefinitionDefaultsInvalid) {
		t.Fatalf("expected ErrDefinitionDefaultsInvalid, got %v", err)
	}

	definition, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:     "promo",
		Schema:   schema,
		Defaults: map[string]any{"headline": "Default"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if definition.ID != uuid.MustParse("00000000-0000-0000-0000-00000000d001") {
		t.Fatalf("unexpected definition ID: %s", definition.ID)
	}
	if !definition.CreatedAt.Equal(now) || !definition.UpdatedAt.Equal(now) {
		t.Fatalf("expected timestamps to equal now (%s), got %s / %s", now, definition.CreatedAt, definition.UpdatedAt)
	}

	if _, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:   "promo",
		Schema: schema,
	}); !errors.Is(err, ErrDefinitionExists) {
		t.Fatalf("expected ErrDefinitionExists, got %v", err)
	}
}

func TestServiceRegisterDefinitionRejectsInvalidSchema(t *testing.T) {
	ctx := context.Background()
	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
	)

	_, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:   "promo",
		Schema: map[string]any{"type": 123},
	})
	if !errors.Is(err, ErrDefinitionSchemaInvalid) {
		t.Fatalf("expected ErrDefinitionSchemaInvalid, got %v", err)
	}
}

func TestServiceCreateInstanceLifecycle(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 1, 2, 9, 0, 0, 0, time.UTC)
	idGen := sequentialIDs(
		"00000000-0000-0000-0000-00000000d010", // definition
		"00000000-0000-0000-0000-00000000a010", // instance
	)

	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		WithClock(func() time.Time { return now }),
		WithIDGenerator(idGen),
	)

	schema := map[string]any{
		"fields": []any{
			map[string]any{"name": "headline"},
			map[string]any{"name": "cta_text"},
		},
	}

	definition, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:     "newsletter_signup",
		Schema:   schema,
		Defaults: map[string]any{"cta_text": "Subscribe"},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000111")
	publishOn := now.Add(1 * time.Hour)

	visibility := map[string]any{
		"schedule": map[string]any{
			"starts_at": now.Format(time.RFC3339),
		},
	}

	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID:    definition.ID,
		Configuration:   map[string]any{"headline": "Join the list"},
		VisibilityRules: visibility,
		PublishOn:       &publishOn,
		Position:        0,
		CreatedBy:       userID,
		UpdatedBy:       userID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	expectedConfig := map[string]any{
		"cta_text": "Subscribe",
		"headline": "Join the list",
	}
	if !reflect.DeepEqual(instance.Configuration, expectedConfig) {
		t.Fatalf("expected configuration %v, got %v", expectedConfig, instance.Configuration)
	}

	if instance.PublishOn == nil || !instance.PublishOn.Equal(publishOn) {
		t.Fatalf("expected publish_on %s, got %v", publishOn, instance.PublishOn)
	}

	visibility["schedule"].(map[string]any)["starts_at"] = "mutated"
	if instance.VisibilityRules["schedule"].(map[string]any)["starts_at"] == "mutated" {
		t.Fatalf("expected visibility rules to be cloned")
	}
}

func TestServiceCreateInstanceValidationFailures(t *testing.T) {
	ctx := context.Background()
	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
	)

	definition, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name: "banner",
		Schema: map[string]any{
			"fields": []any{map[string]any{"name": "message"}},
		},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000222")

	if _, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID:  definition.ID,
		Configuration: map[string]any{"unknown": "value"},
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}); !errors.Is(err, ErrInstanceConfigurationInvalid) {
		t.Fatalf("expected ErrInstanceConfigurationInvalid, got %v", err)
	}

	publish := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	unpublish := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	if _, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: definition.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
		PublishOn:    &publish,
		UnpublishOn:  &unpublish,
	}); !errors.Is(err, ErrInstanceScheduleInvalid) {
		t.Fatalf("expected ErrInstanceScheduleInvalid, got %v", err)
	}

	if _, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: definition.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
		VisibilityRules: map[string]any{
			"invalid_key": true,
		},
	}); !errors.Is(err, ErrVisibilityRulesInvalid) {
		t.Fatalf("expected ErrVisibilityRulesInvalid, got %v", err)
	}
}

func TestServiceTranslationLifecycle(t *testing.T) {
	ctx := context.Background()
	idGen := sequentialIDs(
		"00000000-0000-0000-0000-00000000d020", // definition
		"00000000-0000-0000-0000-00000000a020", // instance
		"00000000-0000-0000-0000-00000000b020", // translation create
		"00000000-0000-0000-0000-00000000b021", // translation update (unused id)
	)

	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		WithIDGenerator(idGen),
	)

	definition, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name: "promo",
		Schema: map[string]any{
			"fields": []any{map[string]any{"name": "headline"}},
		},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000333")
	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: definition.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	localeID := uuid.MustParse("00000000-0000-0000-0000-000000000444")

	translation, err := svc.AddTranslation(ctx, AddTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   localeID,
		Content:    map[string]any{"headline": "Hola"},
	})
	if err != nil {
		t.Fatalf("add translation: %v", err)
	}

	if translation.ID != uuid.MustParse("00000000-0000-0000-0000-00000000b020") {
		t.Fatalf("unexpected translation ID: %s", translation.ID)
	}

	if _, err := svc.AddTranslation(ctx, AddTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   localeID,
		Content:    map[string]any{"headline": "Bonjour"},
	}); !errors.Is(err, ErrTranslationExists) {
		t.Fatalf("expected ErrTranslationExists, got %v", err)
	}

	updated, err := svc.UpdateTranslation(ctx, UpdateTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   localeID,
		Content:    map[string]any{"headline": "Salut"},
	})
	if err != nil {
		t.Fatalf("update translation: %v", err)
	}

	if updated.Content["headline"] != "Salut" {
		t.Fatalf("expected updated headline, got %v", updated.Content["headline"])
	}

}

func TestWidgetTranslationShortcodes(t *testing.T) {
	ctx := context.Background()
	svc := newServiceWithAreas(WithShortcodeService(newWidgetShortcodeService(t)))

	definition, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name: "notice",
		Schema: map[string]any{
			"fields": []any{map[string]any{"name": "body"}},
		},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: definition.ID,
		CreatedBy:    uuid.MustParse("00000000-0000-0000-0000-000000000555"),
		UpdatedBy:    uuid.MustParse("00000000-0000-0000-0000-000000000555"),
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	translation, err := svc.AddTranslation(ctx, AddTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   uuid.MustParse("00000000-0000-0000-0000-000000000556"),
		Content: map[string]any{
			"body": "Hi {{< alert type=\"info\" >}}Widgets{{< /alert >}}",
		},
	})
	if err != nil {
		t.Fatalf("add translation: %v", err)
	}

	body, ok := translation.Content["body"].(string)
	if !ok {
		t.Fatalf("expected body string, got %T", translation.Content["body"])
	}
	if !strings.Contains(body, "shortcode--alert") {
		t.Fatalf("expected shortcode to render, got %s", body)
	}
}

func TestServiceUpdateInstance(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 2, 1, 8, 0, 0, 0, time.UTC)

	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		WithClock(func() time.Time { return now }),
	)

	definition, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name: "promo",
		Schema: map[string]any{
			"fields": []any{
				map[string]any{"name": "headline"},
				map[string]any{"name": "cta_text"},
			},
		},
		Defaults: map[string]any{"cta_text": "Default CTA"},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000666")
	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: definition.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	newConfig := map[string]any{"headline": "Updated"}
	newArea := "  sidebar.secondary  "
	newPosition := 3
	updated, err := svc.UpdateInstance(ctx, UpdateInstanceInput{
		InstanceID:    instance.ID,
		Configuration: newConfig,
		UpdatedBy:     userID,
		AreaCode:      &newArea,
		Position:      &newPosition,
	})
	if err != nil {
		t.Fatalf("update instance: %v", err)
	}

	expectedConfig := map[string]any{
		"cta_text": "Default CTA",
		"headline": "Updated",
	}
	if !reflect.DeepEqual(updated.Configuration, expectedConfig) {
		t.Fatalf("expected configuration %v, got %v", expectedConfig, updated.Configuration)
	}

	if updated.AreaCode == nil || *updated.AreaCode != "sidebar.secondary" {
		t.Fatalf("expected trimmed area code, got %v", updated.AreaCode)
	}

	if updated.Position != newPosition {
		t.Fatalf("expected position %d, got %d", newPosition, updated.Position)
	}
}

func TestServiceUpdateInstanceRejectsInvalidConfiguration(t *testing.T) {
	ctx := context.Background()
	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
	)

	definition, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name: "promo",
		Schema: map[string]any{
			"fields": []any{
				map[string]any{"name": "headline"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000667")
	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: definition.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	_, err = svc.UpdateInstance(ctx, UpdateInstanceInput{
		InstanceID:    instance.ID,
		UpdatedBy:     userID,
		Configuration: map[string]any{"unknown": "value"},
	})
	if !errors.Is(err, ErrInstanceConfigurationInvalid) {
		t.Fatalf("expected ErrInstanceConfigurationInvalid, got %v", err)
	}
}

func TestServiceRegistryFactories(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	registry.RegisterFactory("newsletter", Registration{
		Definition: func() RegisterDefinitionInput {
			return RegisterDefinitionInput{
				Name: "newsletter",
				Schema: map[string]any{
					"fields": []any{
						map[string]any{"name": "headline"},
						map[string]any{"name": "cta_text"},
					},
				},
			}
		},
		InstanceFactory: func(_ context.Context, _ *Definition, _ CreateInstanceInput) (map[string]any, error) {
			return map[string]any{
				"headline": "Stay updated",
				"cta_text": "Subscribe now",
			}, nil
		},
	})

	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		WithRegistry(registry),
	)

	definitions, err := svc.ListDefinitions(ctx)
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("expected 1 definition from registry, got %d", len(definitions))
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000555")
	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: definitions[0].ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	expectedConfig := map[string]any{
		"cta_text": "Subscribe now",
		"headline": "Stay updated",
	}
	if !reflect.DeepEqual(instance.Configuration, expectedConfig) {
		t.Fatalf("expected configuration from factory %v, got %v", expectedConfig, instance.Configuration)
	}
}

func TestServiceRegisterAreaDefinitionValidation(t *testing.T) {
	ctx := context.Background()
	idGen := sequentialIDs(
		"00000000-0000-0000-0000-00000000ad01",
	)

	svc := newServiceWithAreas(WithIDGenerator(idGen))

	if _, err := svc.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{}); !errors.Is(err, ErrAreaCodeRequired) {
		t.Fatalf("expected ErrAreaCodeRequired, got %v", err)
	}

	if _, err := svc.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{Code: "invalid code"}); !errors.Is(err, ErrAreaCodeInvalid) {
		t.Fatalf("expected ErrAreaCodeInvalid, got %v", err)
	}

	if _, err := svc.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{Code: "sidebar.primary"}); !errors.Is(err, ErrAreaNameRequired) {
		t.Fatalf("expected ErrAreaNameRequired, got %v", err)
	}

	def, err := svc.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{Code: "sidebar.primary", Name: "Primary Sidebar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.Code != "sidebar.primary" {
		t.Fatalf("unexpected area code %s", def.Code)
	}

	if _, err := svc.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{Code: "sidebar.primary", Name: "Duplicate"}); !errors.Is(err, ErrAreaDefinitionExists) {
		t.Fatalf("expected ErrAreaDefinitionExists, got %v", err)
	}
}

func TestServiceAssignAndReorderAreaWidgets(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 3, 1, 8, 0, 0, 0, time.UTC)
	idGen := sequentialIDs(
		"00000000-0000-0000-0000-00000000ad10", // area definition
		"00000000-0000-0000-0000-00000000bd10", // widget definition
		"00000000-0000-0000-0000-00000000b110", // instance A
		"00000000-0000-0000-0000-00000000b111", // instance B
		"00000000-0000-0000-0000-00000000b210", // placement 1
		"00000000-0000-0000-0000-00000000b211", // placement 2
	)

	svc := newServiceWithAreas(
		WithClock(func() time.Time { return now }),
		WithIDGenerator(idGen),
	)

	if _, err := svc.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{Code: "sidebar.primary", Name: "Primary Sidebar"}); err != nil {
		t.Fatalf("register area definition: %v", err)
	}

	def, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{map[string]any{"name": "headline"}}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000900")
	instA, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: def.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		t.Fatalf("create instance A: %v", err)
	}

	instB, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: def.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		t.Fatalf("create instance B: %v", err)
	}

	placements, err := svc.AssignWidgetToArea(ctx, AssignWidgetToAreaInput{
		AreaCode:   "sidebar.primary",
		InstanceID: instA.ID,
	})
	if err != nil {
		t.Fatalf("assign widget A: %v", err)
	}
	if len(placements) != 1 || placements[0].InstanceID != instA.ID {
		t.Fatalf("unexpected placements after first assign: %#v", placements)
	}

	position := 0
	placements, err = svc.AssignWidgetToArea(ctx, AssignWidgetToAreaInput{
		AreaCode:   "sidebar.primary",
		InstanceID: instB.ID,
		Position:   &position,
	})
	if err != nil {
		t.Fatalf("assign widget B: %v", err)
	}
	if len(placements) != 2 {
		t.Fatalf("expected 2 placements, got %d", len(placements))
	}
	if placements[0].InstanceID != instB.ID || placements[1].InstanceID != instA.ID {
		t.Fatalf("unexpected ordering: %#v", placements)
	}

	reorder := ReorderAreaWidgetsInput{
		AreaCode: "sidebar.primary",
		Items: []AreaWidgetOrder{
			{PlacementID: placements[1].ID, Position: 0},
			{PlacementID: placements[0].ID, Position: 1},
		},
	}

	placements, err = svc.ReorderAreaWidgets(ctx, reorder)
	if err != nil {
		t.Fatalf("reorder placements: %v", err)
	}
	if placements[0].InstanceID != instA.ID {
		t.Fatalf("expected instance A first after reorder")
	}

	if err := svc.RemoveWidgetFromArea(ctx, RemoveWidgetFromAreaInput{AreaCode: "sidebar.primary", InstanceID: instB.ID}); err != nil {
		t.Fatalf("remove widget: %v", err)
	}

	placements, err = svc.ReorderAreaWidgets(ctx, ReorderAreaWidgetsInput{AreaCode: "sidebar.primary", Items: []AreaWidgetOrder{{PlacementID: placements[0].ID, Position: 0}}})
	if err != nil {
		t.Fatalf("reorder single placement: %v", err)
	}
	if len(placements) != 1 {
		t.Fatalf("expected single placement after removal, got %d", len(placements))
	}
}

func TestServiceResolveAreaWithFallbacksAndVisibility(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 4, 1, 9, 0, 0, 0, time.UTC)
	localePrimary := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	idGen := sequentialIDs(
		"00000000-0000-0000-0000-00000000ad20", // area definition
		"00000000-0000-0000-0000-00000000bd20", // widget definition
		"00000000-0000-0000-0000-00000000b120", // locale instance
		"00000000-0000-0000-0000-00000000b121", // fallback instance
		"00000000-0000-0000-0000-00000000b220", // placement locale
		"00000000-0000-0000-0000-00000000b221", // placement fallback
	)

	svc := newServiceWithAreas(
		WithClock(func() time.Time { return now }),
		WithIDGenerator(idGen),
	)

	if _, err := svc.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{Code: "sidebar.primary", Name: "Primary Sidebar"}); err != nil {
		t.Fatalf("register area definition: %v", err)
	}

	def, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:   "alert",
		Schema: map[string]any{"fields": []any{map[string]any{"name": "message"}}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000910")
	localeInstance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: def.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		t.Fatalf("create locale instance: %v", err)
	}

	fallbackInstance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID:    def.ID,
		VisibilityRules: map[string]any{"audience": []any{"guest"}},
		CreatedBy:       userID,
		UpdatedBy:       userID,
	})
	if err != nil {
		t.Fatalf("create fallback instance: %v", err)
	}

	if _, err := svc.AssignWidgetToArea(ctx, AssignWidgetToAreaInput{AreaCode: "sidebar.primary", LocaleID: &localePrimary, InstanceID: localeInstance.ID}); err != nil {
		t.Fatalf("assign locale placement: %v", err)
	}
	if _, err := svc.AssignWidgetToArea(ctx, AssignWidgetToAreaInput{AreaCode: "sidebar.primary", InstanceID: fallbackInstance.ID}); err != nil {
		t.Fatalf("assign fallback placement: %v", err)
	}

	resolved, err := svc.ResolveArea(ctx, ResolveAreaInput{
		AreaCode: "sidebar.primary",
		LocaleID: &localePrimary,
		Audience: []string{"guest"},
		Now:      now,
	})
	if err != nil {
		t.Fatalf("resolve area: %v", err)
	}
	if len(resolved) != 1 || resolved[0].Instance.ID != localeInstance.ID {
		t.Fatalf("expected locale-specific widget, got %#v", resolved)
	}

	if err := svc.RemoveWidgetFromArea(ctx, RemoveWidgetFromAreaInput{AreaCode: "sidebar.primary", LocaleID: &localePrimary, InstanceID: localeInstance.ID}); err != nil {
		t.Fatalf("remove locale placement: %v", err)
	}

	resolved, err = svc.ResolveArea(ctx, ResolveAreaInput{
		AreaCode: "sidebar.primary",
		LocaleID: &localePrimary,
		Audience: []string{"guest"},
		Now:      now,
	})
	if err != nil {
		t.Fatalf("resolve fallback area: %v", err)
	}
	if len(resolved) != 1 || resolved[0].Instance.ID != fallbackInstance.ID {
		t.Fatalf("expected fallback widget, got %#v", resolved)
	}

	visible, err := svc.EvaluateVisibility(ctx, fallbackInstance, VisibilityContext{Now: now, Audience: []string{"guest"}})
	if err != nil {
		t.Fatalf("evaluate visibility: %v", err)
	}
	if !visible {
		t.Fatalf("expected fallback instance to be visible")
	}

	fixturePath := filepath.Join("testdata", "area_layout.json")
	var layout struct {
		Expected struct {
			Fallback []string `json:"fallback"`
		} `json:"expected"`
	}
	if err := testsupport.LoadGolden(fixturePath, &layout); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	ids := make([]string, len(resolved))
	for i, item := range resolved {
		ids[i] = item.Instance.ID.String()
	}
	if !reflect.DeepEqual(ids, layout.Expected.Fallback) {
		t.Fatalf("expected fallback order %v, got %v", layout.Expected.Fallback, ids)
	}
}

func TestServiceEvaluateVisibilityInvalidScheduleFormat(t *testing.T) {
	ctx := context.Background()
	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
	)

	instance := &Instance{
		VisibilityRules: map[string]any{
			"schedule": map[string]any{
				"starts_at": "not-a-timestamp",
			},
		},
	}

	visible, err := svc.EvaluateVisibility(ctx, instance, VisibilityContext{Now: time.Now()})
	if !errors.Is(err, ErrVisibilityScheduleInvalid) {
		t.Fatalf("expected ErrVisibilityScheduleInvalid, got %v", err)
	}
	if visible {
		t.Fatalf("expected visibility to be false on invalid schedule")
	}
}

func TestServiceEvaluateVisibilityLocaleRestrictions(t *testing.T) {
	ctx := context.Background()
	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
	)

	allowed := uuid.MustParse("00000000-0000-0000-0000-000000000123")
	instance := &Instance{
		VisibilityRules: map[string]any{
			"locales": []any{allowed.String()},
		},
	}

	visible, err := svc.EvaluateVisibility(ctx, instance, VisibilityContext{})
	if !errors.Is(err, ErrVisibilityLocaleRestricted) {
		t.Fatalf("expected ErrVisibilityLocaleRestricted, got %v", err)
	}
	if visible {
		t.Fatalf("expected hidden widget without locale match")
	}

	visible, err = svc.EvaluateVisibility(ctx, instance, VisibilityContext{LocaleID: &allowed})
	if err != nil {
		t.Fatalf("evaluate visibility with allowed locale: %v", err)
	}
	if !visible {
		t.Fatalf("expected widget to be visible for allowed locale")
	}
}

func TestServiceEvaluateVisibilityAudienceAndSegments(t *testing.T) {
	ctx := context.Background()
	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
	)

	instance := &Instance{
		VisibilityRules: map[string]any{
			"audience": []any{"guest"},
			"segments": []string{"beta"},
		},
	}

	visible, err := svc.EvaluateVisibility(ctx, instance, VisibilityContext{
		Audience: []string{"member"},
		Segments: []string{"control"},
	})
	if err != nil {
		t.Fatalf("evaluate visibility without matches: %v", err)
	}
	if visible {
		t.Fatalf("expected widget to be hidden without audience/segment match")
	}

	visible, err = svc.EvaluateVisibility(ctx, instance, VisibilityContext{
		Audience: []string{"member", "guest"},
		Segments: []string{"beta"},
	})
	if err != nil {
		t.Fatalf("evaluate visibility with matches: %v", err)
	}
	if !visible {
		t.Fatalf("expected widget to be visible when audience and segments match")
	}
}

func TestEnsureDefinitions(t *testing.T) {
	ctx := context.Background()
	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
	)

	definitions := []RegisterDefinitionInput{
		{
			Name: "announcement",
			Schema: map[string]any{
				"fields": []any{
					map[string]any{"name": "message"},
				},
			},
		},
	}

	if err := EnsureDefinitions(ctx, svc, definitions); err != nil {
		t.Fatalf("ensure definitions: %v", err)
	}
	if err := EnsureDefinitions(ctx, svc, definitions); err != nil {
		t.Fatalf("ensure definitions second run: %v", err)
	}

	list, err := svc.ListDefinitions(ctx)
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one definition, got %d", len(list))
	}
	if list[0].Name != "announcement" {
		t.Fatalf("unexpected definition name %s", list[0].Name)
	}
}

func TestEnsureAreaDefinitions(t *testing.T) {
	ctx := context.Background()
	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		WithAreaDefinitionRepository(NewMemoryAreaDefinitionRepository()),
		WithAreaPlacementRepository(NewMemoryAreaPlacementRepository()),
	)

	areas := []RegisterAreaDefinitionInput{
		{
			Code:  "footer.global",
			Name:  "Footer",
			Scope: AreaScopeGlobal,
		},
	}

	if err := EnsureAreaDefinitions(ctx, svc, areas); err != nil {
		t.Fatalf("ensure areas: %v", err)
	}
	if err := EnsureAreaDefinitions(ctx, svc, areas); err != nil {
		t.Fatalf("ensure areas second run: %v", err)
	}

	list, err := svc.ListAreaDefinitions(ctx)
	if err != nil {
		t.Fatalf("list areas: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one area definition, got %d", len(list))
	}
	if list[0].Code != "footer.global" {
		t.Fatalf("unexpected area code %s", list[0].Code)
	}
}

func TestServiceDeleteDefinitionPreventsInUse(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	def, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	userID := uuid.New()
	if _, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: def.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	}); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	err = svc.DeleteDefinition(ctx, DeleteDefinitionRequest{ID: def.ID, HardDelete: true})
	if !errors.Is(err, ErrDefinitionInUse) {
		t.Fatalf("expected ErrDefinitionInUse, got %v", err)
	}
}

func TestServiceDeleteInstanceRemovesPlacements(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	idGen := sequentialIDs(
		"00000000-0000-0000-0000-00000000aa10",
		"00000000-0000-0000-0000-00000000bb10",
		"00000000-0000-0000-0000-00000000cc10",
		"00000000-0000-0000-0000-00000000dd10",
	)

	svc := newServiceWithAreas(WithIDGenerator(idGen))

	if _, err := svc.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{Code: "sidebar.primary", Name: "Primary Sidebar"}); err != nil {
		t.Fatalf("register area: %v", err)
	}
	def, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}
	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: def.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if _, err := svc.AssignWidgetToArea(ctx, AssignWidgetToAreaInput{
		AreaCode:   "sidebar.primary",
		InstanceID: instance.ID,
	}); err != nil {
		t.Fatalf("assign widget: %v", err)
	}

	if err := svc.DeleteInstance(ctx, DeleteInstanceRequest{InstanceID: instance.ID, HardDelete: true}); err != nil {
		t.Fatalf("delete instance: %v", err)
	}

	resolved, err := svc.ResolveArea(ctx, ResolveAreaInput{AreaCode: "sidebar.primary", Now: time.Now()})
	if err != nil {
		t.Fatalf("resolve area: %v", err)
	}
	if len(resolved) != 0 {
		t.Fatalf("expected placements cleared, got %d", len(resolved))
	}
}

func TestServiceDeleteInstanceRequiresHardDelete(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	def, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}
	userID := uuid.New()
	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: def.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	err = svc.DeleteInstance(ctx, DeleteInstanceRequest{InstanceID: instance.ID, HardDelete: false})
	if !errors.Is(err, ErrInstanceSoftDeleteUnsupported) {
		t.Fatalf("expected ErrInstanceSoftDeleteUnsupported, got %v", err)
	}
}

func TestServiceDeleteTranslation(t *testing.T) {
	ctx := context.Background()
	svc := newService()

	def, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}
	userID := uuid.New()
	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: def.ID,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	localeID := uuid.New()
	if _, err := svc.AddTranslation(ctx, AddTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   localeID,
		Content: map[string]any{
			"title": "Hero",
		},
	}); err != nil {
		t.Fatalf("add translation: %v", err)
	}

	if err := svc.DeleteTranslation(ctx, DeleteTranslationRequest{InstanceID: instance.ID, LocaleID: localeID}); err != nil {
		t.Fatalf("delete translation: %v", err)
	}
	if _, err := svc.GetTranslation(ctx, instance.ID, localeID); err == nil {
		t.Fatalf("expected translation removal to return error")
	}
}

func TestServiceWidgetTranslationsEmitActivity(t *testing.T) {
	ctx := context.Background()
	actor := uuid.New()
	localeID := uuid.New()

	hook := &activity.CaptureHook{}
	emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{Enabled: true, Channel: "cms"})

	svc := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		WithActivityEmitter(emitter),
		WithIDGenerator(sequentialIDs(
			"00000000-0000-0000-0000-000000000501", // definition
			"00000000-0000-0000-0000-000000000502", // instance
			"00000000-0000-0000-0000-000000000503", // translation
		)),
	)

	def, err := svc.RegisterDefinition(ctx, RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	instance, err := svc.CreateInstance(ctx, CreateInstanceInput{
		DefinitionID: def.ID,
		CreatedBy:    actor,
		UpdatedBy:    actor,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	tr, err := svc.AddTranslation(ctx, AddTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   localeID,
		Content: map[string]any{
			"title": "One",
		},
	})
	if err != nil {
		t.Fatalf("add translation: %v", err)
	}

	if _, err := svc.UpdateTranslation(ctx, UpdateTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   localeID,
		Content: map[string]any{
			"title": "Updated",
		},
	}); err != nil {
		t.Fatalf("update translation: %v", err)
	}

	if err := svc.DeleteTranslation(ctx, DeleteTranslationRequest{
		InstanceID: instance.ID,
		LocaleID:   localeID,
	}); err != nil {
		t.Fatalf("delete translation: %v", err)
	}

	var translationEvents []activity.Event
	for _, event := range hook.Events {
		if event.ObjectType == "widget_translation" {
			translationEvents = append(translationEvents, event)
		}
	}
	if len(translationEvents) != 3 {
		t.Fatalf("expected 3 widget_translation events, got %d", len(translationEvents))
	}

	expectedVerbs := []string{"create", "update", "delete"}
	for i, verb := range expectedVerbs {
		if translationEvents[i].Verb != verb {
			t.Fatalf("expected verb %s got %s", verb, translationEvents[i].Verb)
		}
		if translationEvents[i].ObjectID != tr.ID.String() {
			t.Fatalf("expected object id %s got %s", tr.ID, translationEvents[i].ObjectID)
		}
		if translationEvents[i].Metadata["locale_id"] != localeID.String() {
			t.Fatalf("expected locale_id metadata %s got %v", localeID, translationEvents[i].Metadata["locale_id"])
		}
	}
}

func newService(options ...ServiceOption) Service {
	return NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		options...,
	)
}

func sequentialIDs(values ...string) IDGenerator {
	ids := make([]uuid.UUID, len(values))
	for i, value := range values {
		ids[i] = uuid.MustParse(value)
	}

	var idx int
	return func() uuid.UUID {
		if idx >= len(ids) {
			return uuid.New()
		}
		value := ids[idx]
		idx++
		return value
	}
}

func newServiceWithAreas(options ...ServiceOption) Service {
	memDef := NewMemoryDefinitionRepository()
	memInst := NewMemoryInstanceRepository()
	memTr := NewMemoryTranslationRepository()
	memAreas := NewMemoryAreaDefinitionRepository()
	memPlacements := NewMemoryAreaPlacementRepository()

	opts := append([]ServiceOption{}, options...)
	opts = append(opts,
		WithAreaDefinitionRepository(memAreas),
		WithAreaPlacementRepository(memPlacements),
	)

	return NewService(memDef, memInst, memTr, opts...)
}

func newWidgetShortcodeService(tb testing.TB) interfaces.ShortcodeService {
	validator := shortcodepkg.NewValidator()
	registry := shortcodepkg.NewRegistry(validator)
	if err := shortcodepkg.RegisterBuiltIns(registry, nil); err != nil {
		tb.Fatalf("RegisterBuiltIns: %v", err)
	}
	renderer := shortcodepkg.NewRenderer(registry, validator)
	return shortcodepkg.NewService(registry, renderer)
}
