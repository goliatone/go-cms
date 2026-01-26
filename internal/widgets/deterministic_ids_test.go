package widgets

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/internal/identity"
)

func TestDeterministicWidgetDefinitionIDs(t *testing.T) {
	ctx := context.Background()

	schema := map[string]any{
		"fields": []any{
			map[string]any{"name": "headline"},
		},
	}

	svc1 := NewService(NewMemoryDefinitionRepository(), NewMemoryInstanceRepository(), NewMemoryTranslationRepository())
	def1, err := svc1.RegisterDefinition(ctx, RegisterDefinitionInput{Name: "promo", Schema: schema})
	if err != nil {
		t.Fatalf("register definition 1: %v", err)
	}

	svc2 := NewService(NewMemoryDefinitionRepository(), NewMemoryInstanceRepository(), NewMemoryTranslationRepository())
	def2, err := svc2.RegisterDefinition(ctx, RegisterDefinitionInput{Name: "promo", Schema: schema})
	if err != nil {
		t.Fatalf("register definition 2: %v", err)
	}

	expected := identity.WidgetDefinitionUUID("promo")
	if def1.ID != expected {
		t.Fatalf("unexpected definition id: got %s want %s", def1.ID, expected)
	}
	if def2.ID != expected {
		t.Fatalf("unexpected definition id: got %s want %s", def2.ID, expected)
	}
}

func TestDeterministicWidgetAreaDefinitionIDs(t *testing.T) {
	ctx := context.Background()

	svc1 := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		WithAreaDefinitionRepository(NewMemoryAreaDefinitionRepository()),
		WithAreaPlacementRepository(NewMemoryAreaPlacementRepository()),
	)
	area1, err := svc1.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{Code: "sidebar.primary", Name: "Primary Sidebar"})
	if err != nil {
		t.Fatalf("register area definition 1: %v", err)
	}

	svc2 := NewService(
		NewMemoryDefinitionRepository(),
		NewMemoryInstanceRepository(),
		NewMemoryTranslationRepository(),
		WithAreaDefinitionRepository(NewMemoryAreaDefinitionRepository()),
		WithAreaPlacementRepository(NewMemoryAreaPlacementRepository()),
	)
	area2, err := svc2.RegisterAreaDefinition(ctx, RegisterAreaDefinitionInput{Code: "sidebar.primary", Name: "Primary Sidebar"})
	if err != nil {
		t.Fatalf("register area definition 2: %v", err)
	}

	expected := identity.WidgetAreaDefinitionUUID("sidebar.primary")
	if area1.ID != expected {
		t.Fatalf("unexpected area definition id: got %s want %s", area1.ID, expected)
	}
	if area2.ID != expected {
		t.Fatalf("unexpected area definition id: got %s want %s", area2.ID, expected)
	}
}
