package exampledata

import "github.com/goliatone/go-cms/internal/blocks"

// DemoBlockDefinitions returns the block definitions seeded by the web example app.
func DemoBlockDefinitions() []blocks.RegisterDefinitionInput {
	return []blocks.RegisterDefinitionInput{
		{
			Name: "hero",
			Schema: map[string]any{
				"fields": []any{"title", "subtitle", "cta_text", "cta_url", "background_image"},
			},
		},
		{
			Name: "features_grid",
			Schema: map[string]any{
				"fields": []any{"title", "features"},
			},
		},
		{
			Name: "call_to_action",
			Schema: map[string]any{
				"fields": []any{"headline", "description", "button_text", "button_url"},
			},
		},
	}
}
