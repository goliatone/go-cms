package shortcode

import (
	"testing"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

type noopValidator struct{}

func (noopValidator) ValidateDefinition(def interfaces.ShortcodeDefinition) error { return nil }

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := NewRegistry(noopValidator{})

	def := interfaces.ShortcodeDefinition{
		Name: "demo",
		Schema: interfaces.ShortcodeSchema{
			Params: []interfaces.ShortcodeParam{
				{Name: "id", Type: interfaces.ShortcodeParamString, Required: true},
			},
		},
	}

	if err := registry.Register(def); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	got, ok := registry.Get("demo")
	if !ok {
		t.Fatalf("Get() expected definition")
	}
	if got.Name != def.Name {
		t.Fatalf("Get() wrong definition, got %s", got.Name)
	}
}

func TestRegistry_Duplicate(t *testing.T) {
	registry := NewRegistry(noopValidator{})

	def := interfaces.ShortcodeDefinition{Name: "demo"}
	if err := registry.Register(def); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	if err := registry.Register(def); err != ErrDuplicateDefinition {
		t.Fatalf("Register() expected ErrDuplicateDefinition, got %v", err)
	}
}

func TestRegistry_ListSorted(t *testing.T) {
	registry := NewRegistry(noopValidator{})
	defs := []string{"beta", "alpha", "gamma"}
	for _, name := range defs {
		if err := registry.Register(interfaces.ShortcodeDefinition{Name: name}); err != nil {
			t.Fatalf("Register %s: %v", name, err)
		}
	}

	got := registry.List()
	if len(got) != len(defs) {
		t.Fatalf("List() expected %d definitions, got %d", len(defs), len(got))
	}

	expectOrder := []string{"alpha", "beta", "gamma"}
	for i, want := range expectOrder {
		if got[i].Name != want {
			t.Fatalf("List() order mismatch at %d: got %s, want %s", i, got[i].Name, want)
		}
	}
}
