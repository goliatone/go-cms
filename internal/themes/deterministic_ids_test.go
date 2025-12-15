package themes

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/internal/identity"
)

func TestDeterministicThemeIDs(t *testing.T) {
	ctx := context.Background()
	input := RegisterThemeInput{
		Name:      "Aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
	}

	svc1 := NewService(NewMemoryThemeRepository(), NewMemoryTemplateRepository())
	theme1, err := svc1.RegisterTheme(ctx, input)
	if err != nil {
		t.Fatalf("register theme 1: %v", err)
	}

	svc2 := NewService(NewMemoryThemeRepository(), NewMemoryTemplateRepository())
	theme2, err := svc2.RegisterTheme(ctx, input)
	if err != nil {
		t.Fatalf("register theme 2: %v", err)
	}

	expected := identity.ThemeUUID(input.ThemePath)
	if theme1.ID != expected {
		t.Fatalf("unexpected theme id: got %s want %s", theme1.ID, expected)
	}
	if theme2.ID != expected {
		t.Fatalf("unexpected theme id: got %s want %s", theme2.ID, expected)
	}
}

func TestDeterministicTemplateIDs(t *testing.T) {
	ctx := context.Background()

	themeInput := RegisterThemeInput{
		Name:      "Aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
	}

	templateInput := RegisterTemplateInput{
		Name:         "Landing",
		Slug:         "landing",
		TemplatePath: "templates/landing.html.tmpl",
		Regions: map[string]TemplateRegion{
			"hero": {Name: "Hero", AcceptsBlocks: true},
		},
	}

	svc1 := NewService(NewMemoryThemeRepository(), NewMemoryTemplateRepository())
	theme1, err := svc1.RegisterTheme(ctx, themeInput)
	if err != nil {
		t.Fatalf("register theme 1: %v", err)
	}
	templateInput.ThemeID = theme1.ID
	template1, err := svc1.RegisterTemplate(ctx, templateInput)
	if err != nil {
		t.Fatalf("register template 1: %v", err)
	}

	svc2 := NewService(NewMemoryThemeRepository(), NewMemoryTemplateRepository())
	theme2, err := svc2.RegisterTheme(ctx, themeInput)
	if err != nil {
		t.Fatalf("register theme 2: %v", err)
	}
	templateInput.ThemeID = theme2.ID
	template2, err := svc2.RegisterTemplate(ctx, templateInput)
	if err != nil {
		t.Fatalf("register template 2: %v", err)
	}

	expectedThemeID := identity.ThemeUUID(themeInput.ThemePath)
	if theme1.ID != expectedThemeID || theme2.ID != expectedThemeID {
		t.Fatalf("expected theme ids to be deterministic: got %s and %s", theme1.ID, theme2.ID)
	}

	expectedTemplateID := identity.TemplateUUID(expectedThemeID, templateInput.Slug)
	if template1.ID != expectedTemplateID {
		t.Fatalf("unexpected template id: got %s want %s", template1.ID, expectedTemplateID)
	}
	if template2.ID != expectedTemplateID {
		t.Fatalf("unexpected template id: got %s want %s", template2.ID, expectedTemplateID)
	}
}

