package schema

import "testing"

func TestCheckSchemaCompatibilityDetectsBreakingChanges(t *testing.T) {
	oldSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
		},
		"required": []any{"title"},
	}
	newSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "number"},
		},
		"required": []any{"title"},
	}

	result := CheckSchemaCompatibility(oldSchema, newSchema)
	if result.Compatible {
		t.Fatalf("expected incompatibility")
	}
	if result.ChangeLevel != ChangeMajor {
		t.Fatalf("expected major change, got %s", result.ChangeLevel.String())
	}
	if len(result.BreakingChanges) == 0 {
		t.Fatalf("expected breaking changes")
	}
}

func TestCheckSchemaCompatibilityDetectsMinorChanges(t *testing.T) {
	oldSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
		},
	}
	newSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":   map[string]any{"type": "string"},
			"summary": map[string]any{"type": "string"},
		},
	}

	result := CheckSchemaCompatibility(oldSchema, newSchema)
	if !result.Compatible {
		t.Fatalf("expected compatibility")
	}
	if result.ChangeLevel != ChangeMinor {
		t.Fatalf("expected minor change, got %s", result.ChangeLevel.String())
	}
}

func TestCheckSchemaCompatibilityDetectsPatchChanges(t *testing.T) {
	oldSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "Old",
			},
		},
	}
	newSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "New",
			},
		},
	}

	result := CheckSchemaCompatibility(oldSchema, newSchema)
	if !result.Compatible {
		t.Fatalf("expected compatibility")
	}
	if result.ChangeLevel != ChangePatch {
		t.Fatalf("expected patch change, got %s", result.ChangeLevel.String())
	}
}

func TestCheckSchemaCompatibilityTreatsTypeWideningAsMinor(t *testing.T) {
	oldSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
		},
	}
	newSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": []any{"string", "null"}},
		},
	}

	result := CheckSchemaCompatibility(oldSchema, newSchema)
	if !result.Compatible {
		t.Fatalf("expected compatibility")
	}
	if result.ChangeLevel != ChangeMinor {
		t.Fatalf("expected minor change, got %s", result.ChangeLevel.String())
	}
}
