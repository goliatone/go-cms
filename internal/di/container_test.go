package di_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
)

func TestContainerWidgetServiceDisabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	svc := container.WidgetService()
	if svc == nil {
		t.Fatalf("expected widget service, got nil")
	}

	if _, err := svc.RegisterDefinition(context.Background(), widgets.RegisterDefinitionInput{Name: "any"}); err == nil {
		t.Fatalf("expected error when widget feature disabled")
	} else if err != widgets.ErrFeatureDisabled {
		t.Fatalf("expected ErrFeatureDisabled, got %v", err)
	}
}

func TestContainerWidgetServiceEnabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Widgets = true

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}
	svc := container.WidgetService()

	if svc == nil {
		t.Fatalf("expected widget service")
	}

	definitions, err := svc.ListDefinitions(context.Background())
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if len(definitions) == 0 {
		t.Fatalf("expected built-in widget definitions to be registered")
	}

	found := false
	for _, def := range definitions {
		if def != nil && def.Name == "newsletter_signup" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected newsletter_signup definition to be registered")
	}
}

func TestContainerThemeServiceDisabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	svc := container.ThemeService()
	if svc == nil {
		t.Fatalf("expected theme service, got nil")
	}

	if _, err := svc.ListThemes(context.Background()); !errors.Is(err, themes.ErrFeatureDisabled) {
		t.Fatalf("expected ErrFeatureDisabled, got %v", err)
	}
}

func TestContainerThemeServiceEnabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Themes = true

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}
	svc := container.ThemeService()

	theme, err := svc.RegisterTheme(context.Background(), themes.RegisterThemeInput{
		Name:      "aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
		Config: themes.ThemeConfig{
			WidgetAreas: []themes.ThemeWidgetArea{{Code: "hero", Name: "Hero"}},
		},
	})
	if err != nil {
		t.Fatalf("register theme: %v", err)
	}

	_, err = svc.RegisterTemplate(context.Background(), themes.RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "landing",
		Slug:         "landing",
		TemplatePath: "templates/landing.html",
		Regions: map[string]themes.TemplateRegion{
			"hero": {Name: "Hero", AcceptsBlocks: true},
		},
	})
	if err != nil {
		t.Fatalf("register template: %v", err)
	}

	if _, err := svc.ActivateTheme(context.Background(), theme.ID); err != nil {
		t.Fatalf("activate theme: %v", err)
	}

	summaries, err := svc.ListActiveSummaries(context.Background())
	if err != nil {
		t.Fatalf("list active summaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 active theme, got %d", len(summaries))
	}
}
