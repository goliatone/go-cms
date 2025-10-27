package widgets

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
)

type widgetContractFixture struct {
	Definitions  []widgetDefinitionFixture  `json:"definitions"`
	Instances    []widgetInstanceFixture    `json:"instances"`
	Translations []widgetTranslationFixture `json:"translations"`
}

type widgetDefinitionFixture struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
	Defaults    map[string]any `json:"defaults"`
	Category    string         `json:"category"`
	Icon        string         `json:"icon"`
}

type widgetInstanceFixture struct {
	ID                string         `json:"id"`
	DefinitionID      string         `json:"definition_id"`
	AreaCode          string         `json:"area_code"`
	PlacementMetadata map[string]any `json:"placement_metadata"`
	Configuration     map[string]any `json:"configuration"`
	VisibilityRules   map[string]any `json:"visibility_rules"`
	Position          int            `json:"position"`
}

type widgetTranslationFixture struct {
	ID               string         `json:"id"`
	WidgetInstanceID string         `json:"widget_instance_id"`
	LocaleID         string         `json:"locale_id"`
	Content          map[string]any `json:"content"`
}

func TestWidgetServiceContract_BasicFixture(t *testing.T) {
	t.Parallel()

	fixture := loadWidgetContractFixture(t, "testdata/basic_widgets.json")
	ctx := context.Background()

	idSequence := make([]string, 0, len(fixture.Definitions)+len(fixture.Instances)+len(fixture.Translations))
	for _, def := range fixture.Definitions {
		idSequence = append(idSequence, def.ID)
	}
	for _, inst := range fixture.Instances {
		idSequence = append(idSequence, inst.ID)
	}
	for _, tr := range fixture.Translations {
		idSequence = append(idSequence, tr.ID)
	}

	svc := newServiceWithAreas(
		WithClock(func() time.Time {
			return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		}),
		WithIDGenerator(sequentialIDs(idSequence...)),
	)

	definitions := make(map[string]*Definition, len(fixture.Definitions))
	defFixtures := make(map[string]widgetDefinitionFixture, len(fixture.Definitions))
	for _, entry := range fixture.Definitions {
		input := RegisterDefinitionInput{
			Name:     entry.Name,
			Schema:   entry.Schema,
			Defaults: entry.Defaults,
		}
		if entry.Description != "" {
			input.Description = stringPtr(entry.Description)
		}
		if entry.Category != "" {
			input.Category = stringPtr(entry.Category)
		}
		if entry.Icon != "" {
			input.Icon = stringPtr(entry.Icon)
		}

		record, err := svc.RegisterDefinition(ctx, input)
		if err != nil {
			t.Fatalf("register definition %s: %v", entry.Name, err)
		}
		if record.ID.String() != entry.ID {
			t.Fatalf("unexpected definition id: want %s got %s", entry.ID, record.ID)
		}
		if !reflect.DeepEqual(record.Schema, entry.Schema) {
			t.Fatalf("definition schema mismatch for %s", entry.Name)
		}
		if !reflect.DeepEqual(record.Defaults, entry.Defaults) {
			t.Fatalf("definition defaults mismatch for %s", entry.Name)
		}
		definitions[entry.ID] = record
		defFixtures[entry.ID] = entry
	}

	listedDefinitions, err := svc.ListDefinitions(ctx)
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if len(listedDefinitions) != len(definitions) {
		t.Fatalf("expected %d definitions, got %d", len(definitions), len(listedDefinitions))
	}

	authorID := uuid.MustParse("00000000-0000-0000-0000-000000000999")
	instances := make(map[string]*Instance, len(fixture.Instances))
	for _, entry := range fixture.Instances {
		definition := definitions[entry.DefinitionID]
		if definition == nil {
			t.Fatalf("missing definition for instance fixture %s", entry.ID)
		}

		input := CreateInstanceInput{
			DefinitionID:    definition.ID,
			Placement:       entry.PlacementMetadata,
			Configuration:   entry.Configuration,
			VisibilityRules: entry.VisibilityRules,
			Position:        entry.Position,
			CreatedBy:       authorID,
			UpdatedBy:       authorID,
		}
		if entry.AreaCode != "" {
			input.AreaCode = stringPtr(entry.AreaCode)
		}

		record, err := svc.CreateInstance(ctx, input)
		if err != nil {
			t.Fatalf("create instance %s: %v", entry.ID, err)
		}
		if record.ID.String() != entry.ID {
			t.Fatalf("unexpected instance id: want %s got %s", entry.ID, record.ID)
		}
		if record.AreaCode == nil || *record.AreaCode != entry.AreaCode {
			t.Fatalf("unexpected area code for instance %s", entry.ID)
		}
		fixtureDefinition := defFixtures[entry.DefinitionID]
		expectedConfig := expectedConfiguration(fixtureDefinition.Defaults, entry.Configuration)
		if !reflect.DeepEqual(record.Configuration, expectedConfig) {
			t.Fatalf("instance configuration mismatch for %s\nwant: %#v\n got: %#v", entry.ID, expectedConfig, record.Configuration)
		}
		if record.Configuration["cta_text"] != entry.Configuration["cta_text"] {
			t.Fatalf("expected configuration override for cta_text on %s", entry.ID)
		}
		if fixtureDefinition.Defaults["success_message"] != nil {
			if record.Configuration["success_message"] != fixtureDefinition.Defaults["success_message"] {
				t.Fatalf("expected success_message default to be preserved on %s", entry.ID)
			}
		}
		if !reflect.DeepEqual(record.VisibilityRules, entry.VisibilityRules) {
			t.Fatalf("instance visibility mismatch for %s", entry.ID)
		}
		if !reflect.DeepEqual(record.Placement, entry.PlacementMetadata) {
			t.Fatalf("instance placement mismatch for %s", entry.ID)
		}
		instances[entry.ID] = record
	}

	for _, def := range definitions {
		byDefinition, err := svc.ListInstancesByDefinition(ctx, def.ID)
		if err != nil {
			t.Fatalf("list instances for definition %s: %v", def.Name, err)
		}
		if len(byDefinition) == 0 {
			t.Fatalf("expected instances for definition %s", def.Name)
		}
	}

	for _, entry := range fixture.Translations {
		instance := instances[entry.WidgetInstanceID]
		if instance == nil {
			t.Fatalf("missing instance %s for translation %s", entry.WidgetInstanceID, entry.ID)
		}

		localeID := mustParseUUID(t, entry.LocaleID)
		record, err := svc.AddTranslation(ctx, AddTranslationInput{
			InstanceID: instance.ID,
			LocaleID:   localeID,
			Content:    entry.Content,
		})
		if err != nil {
			t.Fatalf("add translation %s: %v", entry.ID, err)
		}
		if record.ID.String() != entry.ID {
			t.Fatalf("unexpected translation id: want %s got %s", entry.ID, record.ID)
		}

		got, err := svc.GetTranslation(ctx, instance.ID, localeID)
		if err != nil {
			t.Fatalf("get translation %s: %v", entry.ID, err)
		}
		if !reflect.DeepEqual(got.Content, entry.Content) {
			t.Fatalf("translation content mismatch for %s", entry.ID)
		}

		reloaded, err := svc.GetInstance(ctx, instance.ID)
		if err != nil {
			t.Fatalf("get instance with translations %s: %v", instance.ID, err)
		}
		if len(reloaded.Translations) == 0 {
			t.Fatalf("expected translations attached to instance %s", instance.ID)
		}
	}

	for _, entry := range fixture.Instances {
		instance := instances[entry.ID]
		if instance == nil {
			continue
		}
		// Ensure visibility schedule gates correctly before the start window.
		visible, err := svc.EvaluateVisibility(ctx, instance, VisibilityContext{
			Now:      time.Date(2023, 12, 31, 23, 0, 0, 0, time.UTC),
			Audience: []string{"guest"},
		})
		if err != nil {
			t.Fatalf("evaluate visibility pre-start: %v", err)
		}
		if visible {
			t.Fatalf("expected instance %s to be hidden before schedule", entry.ID)
		}

		visible, err = svc.EvaluateVisibility(ctx, instance, VisibilityContext{
			Now:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Audience: []string{"anonymous"},
		})
		if err != nil {
			t.Fatalf("evaluate visibility active: %v", err)
		}
		if !visible {
			t.Fatalf("expected instance %s to be visible within schedule", entry.ID)
		}
	}
}

func loadWidgetContractFixture(t *testing.T, path string) widgetContractFixture {
	t.Helper()

	raw, err := testsupport.LoadFixture(path)
	if err != nil {
		t.Fatalf("load widget fixture: %v", err)
	}
	var fx widgetContractFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("unmarshal widget fixture: %v", err)
	}
	return fx
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	clone := value
	return &clone
}

func mustParseUUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(value)
	if err != nil {
		t.Fatalf("parse uuid %s: %v", value, err)
	}
	return id
}

func expectedConfiguration(defaults map[string]any, configuration map[string]any) map[string]any {
	out := make(map[string]any)
	for key, value := range defaults {
		out[key] = value
	}
	for key, value := range configuration {
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
