package schema

import (
	"errors"
	"testing"
)

func TestValidateSchemaSubsetAcceptsStandardConstraints(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 120,
				"pattern":   "^[A-Za-z0-9 _-]+$",
			},
			"count": map[string]any{
				"type":             "number",
				"minimum":          0,
				"maximum":          100,
				"exclusiveMinimum": -1,
				"exclusiveMaximum": 101,
			},
			"blocks": map[string]any{
				"type":     "array",
				"minItems": 1,
				"maxItems": 6,
				"items": map[string]any{
					"type": "string",
				},
			},
		},
	}

	if err := ValidateSchemaSubset(schema); err != nil {
		t.Fatalf("expected constraints to be accepted, got %v", err)
	}
}

func TestValidateSchemaSubsetRejectsUnsupportedKeyword(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":         "string",
				"minProperties": 1,
			},
		},
	}

	err := ValidateSchemaSubset(schema)
	if err == nil {
		t.Fatalf("expected unsupported keyword error")
	}
	if !errors.Is(err, ErrUnsupportedKeyword) {
		t.Fatalf("expected ErrUnsupportedKeyword, got %v", err)
	}
}
