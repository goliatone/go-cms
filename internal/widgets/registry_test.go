package widgets

import (
	"context"
	"testing"
)

func TestRegistryRegisterFactoryCanonicalKey(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterFactory(" Newsletter ", Registration{
		Definition: func() RegisterDefinitionInput {
			return RegisterDefinitionInput{
				Name: "Newsletter",
			}
		},
		InstanceFactory: func(ctx context.Context, def *Definition, input CreateInstanceInput) (map[string]any, error) {
			if def == nil {
				t.Fatalf("expected definition in instance factory")
			}
			return map[string]any{"headline": "Stay in the loop"}, nil
		},
	})

	defs := registry.List()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}
	if defs[0].Name != "Newsletter" {
		t.Fatalf("unexpected definition name %s", defs[0].Name)
	}

	factory := registry.InstanceFactory(" newsletter ")
	if factory == nil {
		t.Fatalf("expected instance factory lookup to succeed")
	}
	config, err := factory(context.Background(), &Definition{Name: "Newsletter"}, CreateInstanceInput{})
	if err != nil {
		t.Fatalf("instance factory error: %v", err)
	}
	if config["headline"] != "Stay in the loop" {
		t.Fatalf("unexpected factory configuration %#v", config)
	}
}

func TestRegistryRegisterFactoryDerivesName(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterFactory("", Registration{
		Definition: func() RegisterDefinitionInput {
			return RegisterDefinitionInput{
				Name: "Promo Banner",
			}
		},
	})

	if len(registry.List()) != 1 {
		t.Fatalf("expected derived registration to be stored")
	}
	if registry.InstanceFactory("promo banner") != nil {
		t.Fatalf("unexpected instance factory for name-only registration")
	}
}

func TestRegistryIgnoresEmptyRegistration(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.RegisterFactory("ignored", Registration{})
	registry.RegisterFactory("  ", Registration{
		Definition: nil,
	})
	registry.Register(RegisterDefinitionInput{})

	if len(registry.List()) != 0 {
		t.Fatalf("expected registry to ignore invalid registrations")
	}
}
