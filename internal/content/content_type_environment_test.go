package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/environments"
)

func TestContentTypeServiceEnvironmentScopedSlugs(t *testing.T) {
	ctx := context.Background()
	envRepo := environments.NewMemoryRepository()
	envSvc := environments.NewService(envRepo)

	if _, err := envSvc.CreateEnvironment(ctx, environments.CreateEnvironmentInput{Key: "default", IsDefault: true}); err != nil {
		t.Fatalf("create default environment: %v", err)
	}
	if _, err := envSvc.CreateEnvironment(ctx, environments.CreateEnvironmentInput{Key: "staging"}); err != nil {
		t.Fatalf("create staging environment: %v", err)
	}

	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo, content.WithContentTypeEnvironmentService(envSvc))

	defaultCT, err := svc.Create(ctx, content.CreateContentTypeRequest{
		Name:           "Article",
		Slug:           "article",
		Schema:         map[string]any{"fields": []any{"title"}},
		EnvironmentKey: "default",
	})
	if err != nil {
		t.Fatalf("create default content type: %v", err)
	}

	stagingCT, err := svc.Create(ctx, content.CreateContentTypeRequest{
		Name:           "Article",
		Slug:           "article",
		Schema:         map[string]any{"fields": []any{"title"}},
		EnvironmentKey: "staging",
	})
	if err != nil {
		t.Fatalf("create staging content type: %v", err)
	}

	if stagingCT.EnvironmentID == defaultCT.EnvironmentID {
		t.Fatalf("expected distinct environments")
	}

	if _, err := svc.Create(ctx, content.CreateContentTypeRequest{
		Name:           "Article Duplicate",
		Slug:           "article",
		Schema:         map[string]any{"fields": []any{"title"}},
		EnvironmentKey: "default",
	}); !errors.Is(err, content.ErrContentTypeSlugExists) {
		t.Fatalf("expected ErrContentTypeSlugExists, got %v", err)
	}

	got, err := svc.GetBySlug(ctx, "article", "staging")
	if err != nil {
		t.Fatalf("get by slug staging: %v", err)
	}
	if got.ID != stagingCT.ID {
		t.Fatalf("expected staging content type, got %s", got.ID)
	}

	defaultList, err := svc.List(ctx, "default")
	if err != nil {
		t.Fatalf("list default: %v", err)
	}
	if len(defaultList) != 1 || defaultList[0].ID != defaultCT.ID {
		t.Fatalf("expected default list to return default content type")
	}
}
