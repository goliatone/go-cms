package blocks

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/internal/identity"
)

func TestDeterministicBlockDefinitionIDs(t *testing.T) {
	ctx := context.Background()

	schema := map[string]any{
		"fields": []any{
			map[string]any{"name": "headline"},
		},
	}

	svc1 := NewService(NewMemoryDefinitionRepository(), NewMemoryInstanceRepository(), NewMemoryTranslationRepository())
	def1, err := svc1.RegisterDefinition(ctx, RegisterDefinitionInput{Name: "hero", Schema: schema})
	if err != nil {
		t.Fatalf("register definition 1: %v", err)
	}

	svc2 := NewService(NewMemoryDefinitionRepository(), NewMemoryInstanceRepository(), NewMemoryTranslationRepository())
	def2, err := svc2.RegisterDefinition(ctx, RegisterDefinitionInput{Name: "hero", Schema: schema})
	if err != nil {
		t.Fatalf("register definition 2: %v", err)
	}

	expected := identity.BlockDefinitionUUID("hero")
	if def1.ID != expected {
		t.Fatalf("unexpected definition id: got %s want %s", def1.ID, expected)
	}
	if def2.ID != expected {
		t.Fatalf("unexpected definition id: got %s want %s", def2.ID, expected)
	}
}
