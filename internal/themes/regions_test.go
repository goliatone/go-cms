package themes

import (
	"testing"

	"github.com/google/uuid"
)

func TestInspectTemplateRegions(t *testing.T) {
	regions := map[string]TemplateRegion{
		"hero": {
			Name:           "Hero",
			AcceptsBlocks:  true,
			AcceptsWidgets: false,
		},
		"sidebar": {
			Name:           "Sidebar",
			AcceptsWidgets: true,
		},
	}
	template := &Template{
		ID:      uuid.New(),
		ThemeID: uuid.New(),
		Slug:    "landing",
		Regions: regions,
	}

	summary := InspectTemplateRegions(template)
	if len(summary) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(summary))
	}
	if summary[0].Key != "hero" {
		t.Fatalf("expected sorted output by key")
	}
}
