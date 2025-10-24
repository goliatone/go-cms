package di_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/widgets"
)

func TestContainerWidgetServiceDisabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	container := di.NewContainer(cfg)

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

	container := di.NewContainer(cfg)
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
