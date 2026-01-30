package blocks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/environments"
)

func TestBlockServiceEnvironmentScopedDefinitions(t *testing.T) {
	ctx := context.Background()
	envRepo := environments.NewMemoryRepository()
	envSvc := environments.NewService(envRepo)

	if _, err := envSvc.CreateEnvironment(ctx, environments.CreateEnvironmentInput{Key: "default", IsDefault: true}); err != nil {
		t.Fatalf("create default environment: %v", err)
	}
	if _, err := envSvc.CreateEnvironment(ctx, environments.CreateEnvironmentInput{Key: "staging"}); err != nil {
		t.Fatalf("create staging environment: %v", err)
	}

	svc := newBlockService(blocks.WithEnvironmentService(envSvc))

	defaultDef, err := svc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:           "Hero",
		Slug:           "hero",
		Schema:         map[string]any{"fields": []any{"title"}},
		EnvironmentKey: "default",
	})
	if err != nil {
		t.Fatalf("register default definition: %v", err)
	}

	if _, err := svc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:           "Hero",
		Slug:           "hero",
		Schema:         map[string]any{"fields": []any{"title"}},
		EnvironmentKey: "staging",
	}); err != nil {
		t.Fatalf("register staging definition: %v", err)
	}

	if _, err := svc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:           "Hero Duplicate",
		Slug:           "hero",
		Schema:         map[string]any{"fields": []any{"title"}},
		EnvironmentKey: "default",
	}); !errors.Is(err, blocks.ErrDefinitionSlugExists) {
		t.Fatalf("expected ErrDefinitionSlugExists, got %v", err)
	}

	defs, err := svc.ListDefinitions(ctx, "default")
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if len(defs) != 1 || defs[0].ID != defaultDef.ID {
		t.Fatalf("expected default list to return default definition")
	}
}
