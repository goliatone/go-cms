package shortcode

import (
	"errors"
	"testing"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

func TestValidator_CoerceParams(t *testing.T) {
	v := NewValidator()

	def := interfaces.ShortcodeDefinition{
		Name: "test",
		Schema: interfaces.ShortcodeSchema{
			Params: []interfaces.ShortcodeParam{
				{Name: "id", Type: interfaces.ShortcodeParamString, Required: true},
				{Name: "count", Type: interfaces.ShortcodeParamInt, Default: 1},
				{Name: "enabled", Type: interfaces.ShortcodeParamBool, Default: false},
			},
		},
	}

	input := map[string]any{
		"id":      "abc",
		"count":   "42",
		"enabled": "true",
	}

	got, err := v.CoerceParams(def, input)
	if err != nil {
		t.Fatalf("CoerceParams() unexpected error: %v", err)
	}

	if got["id"] != "abc" {
		t.Fatalf("id mismatch, got %v", got["id"])
	}
	if got["count"] != 42 {
		t.Fatalf("count mismatch, got %v", got["count"])
	}
	if got["enabled"] != true {
		t.Fatalf("enabled mismatch, got %v", got["enabled"])
	}
}

func TestValidator_MissingRequired(t *testing.T) {
	v := NewValidator()
	def := interfaces.ShortcodeDefinition{
		Name: "test",
		Schema: interfaces.ShortcodeSchema{
			Params: []interfaces.ShortcodeParam{
				{Name: "id", Type: interfaces.ShortcodeParamString, Required: true},
			},
		},
	}

	_, err := v.CoerceParams(def, map[string]any{})
	if !errors.Is(err, ErrMissingParameter) {
		t.Fatalf("expected ErrMissingParameter, got %v", err)
	}
}

func TestValidator_CustomValidation(t *testing.T) {
	v := NewValidator()
	def := interfaces.ShortcodeDefinition{
		Name: "test",
		Schema: interfaces.ShortcodeSchema{
			Params: []interfaces.ShortcodeParam{
				{
					Name:     "type",
					Type:     interfaces.ShortcodeParamString,
					Required: true,
					Validate: func(value any) error {
						if value.(string) != "allowed" {
							return errors.New("invalid type")
						}
						return nil
					},
				},
			},
		},
	}

	if _, err := v.CoerceParams(def, map[string]any{"type": "allowed"}); err != nil {
		t.Fatalf("CoerceParams() unexpected error: %v", err)
	}

	if _, err := v.CoerceParams(def, map[string]any{"type": "forbidden"}); err == nil {
		t.Fatal("CoerceParams() expected error for invalid validator")
	}
}
