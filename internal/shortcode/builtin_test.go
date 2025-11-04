package shortcode

import "testing"

func TestBuiltInDefinitions(t *testing.T) {
	defs := BuiltInDefinitions()
	if len(defs) == 0 {
		t.Fatal("expected built-in definitions")
	}

	reg := NewRegistry(NewValidator())
	for _, def := range defs {
		if err := reg.Register(def); err != nil {
			t.Fatalf("register built-in %s: %v", def.Name, err)
		}
	}

	// spot check
	if _, ok := reg.Get("youtube"); !ok {
		t.Fatal("youtube definition not registered")
	}
	if _, ok := reg.Get("alert"); !ok {
		t.Fatal("alert definition not registered")
	}
}

func TestGalleryDefinitionDefaults(t *testing.T) {
	gallery := galleryDefinition()
	v := NewValidator()
	params, err := v.CoerceParams(gallery, map[string]any{
		"images": []any{"a.jpg", "b.jpg"},
	})
	if err != nil {
		t.Fatalf("CoerceParams() unexpected error: %v", err)
	}
	if params["columns"] != 3 {
		t.Fatalf("expected default columns 3, got %v", params["columns"])
	}
}
