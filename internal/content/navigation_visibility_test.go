package content

import (
	"testing"

	"github.com/google/uuid"
)

func TestResolveNavigationVisibility_DefaultsAndOverrides(t *testing.T) {
	contentType := &ContentType{
		ID:   uuid.New(),
		Name: "page",
		Slug: "page",
		Capabilities: map[string]any{
			"navigation": map[string]any{
				"enabled":                 true,
				"eligible_locations":      []any{"site.main", "site.footer"},
				"default_locations":       []any{"site.main"},
				"default_visible":         true,
				"allow_instance_override": true,
			},
		},
	}
	result := ResolveNavigationVisibility(contentType, map[string]any{
		"_navigation": map[string]any{
			"site.main":   "hide",
			"site.footer": "show",
		},
	})

	if result.EffectiveVisibility["site.main"] {
		t.Fatalf("expected site.main hidden")
	}
	if !result.EffectiveVisibility["site.footer"] {
		t.Fatalf("expected site.footer visible")
	}
	if result.Origins["site.main"] != NavigationOriginOverride || result.Origins["site.footer"] != NavigationOriginOverride {
		t.Fatalf("expected override origins, got %#v", result.Origins)
	}
	if len(result.EffectiveMenuLocations) != 1 || result.EffectiveMenuLocations[0] != "site.footer" {
		t.Fatalf("unexpected effective menu locations: %#v", result.EffectiveMenuLocations)
	}
}

func TestResolveNavigationVisibility_OverrideGuardrail(t *testing.T) {
	contentType := &ContentType{
		ID:   uuid.New(),
		Name: "page",
		Slug: "page",
		Capabilities: map[string]any{
			"navigation": map[string]any{
				"enabled":                 true,
				"eligible_locations":      []any{"site.main", "site.footer"},
				"default_locations":       []any{"site.main"},
				"default_visible":         true,
				"allow_instance_override": false,
			},
		},
	}
	result := ResolveNavigationVisibility(contentType, map[string]any{
		"_navigation": map[string]any{
			"site.main":   "hide",
			"site.footer": "show",
		},
	})

	if !result.EffectiveVisibility["site.main"] || result.EffectiveVisibility["site.footer"] {
		t.Fatalf("expected defaults to win when overrides are disabled, got %#v", result.EffectiveVisibility)
	}
	if result.Origins["site.main"] != NavigationOriginDefault || result.Origins["site.footer"] != NavigationOriginDefault {
		t.Fatalf("expected default origins when overrides disabled, got %#v", result.Origins)
	}
	if result.EffectiveState["site.main"] != NavigationStateInherit || result.EffectiveState["site.footer"] != NavigationStateInherit {
		t.Fatalf("expected inherit effective state, got %#v", result.EffectiveState)
	}
}
