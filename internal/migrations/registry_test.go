package migrations

import "testing"

func TestRegistryRegistersAndMigrates(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("article", "article@v1.0.0", "article@v1.1.0", func(payload map[string]any) (map[string]any, error) {
		payload["step"] = "v1.1.0"
		return payload, nil
	}); err != nil {
		t.Fatalf("register migration: %v", err)
	}
	if err := registry.Register("article", "article@v1.1.0", "article@v2.0.0", func(payload map[string]any) (map[string]any, error) {
		payload["step"] = "v2.0.0"
		return payload, nil
	}); err != nil {
		t.Fatalf("register migration 2: %v", err)
	}

	result, err := registry.Migrate("article", "article@v1.0.0", "article@v2.0.0", map[string]any{"value": "start"})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result["step"] != "v2.0.0" {
		t.Fatalf("expected final step v2.0.0, got %v", result["step"])
	}
}

func TestRegistryRejectsSlugMismatch(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("article", "page@v1.0.0", "page@v1.1.0", func(payload map[string]any) (map[string]any, error) {
		return payload, nil
	}); err == nil {
		t.Fatalf("expected slug mismatch error")
	}
}
