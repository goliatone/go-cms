package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/environments"
	"github.com/google/uuid"
)

func TestContentServiceEnvironmentScopedSlugs(t *testing.T) {
	ctx := context.Background()
	envRepo := environments.NewMemoryRepository()
	envSvc := environments.NewService(envRepo)

	defaultEnv, err := envSvc.CreateEnvironment(ctx, environments.CreateEnvironmentInput{Key: "default", IsDefault: true})
	if err != nil {
		t.Fatalf("create default environment: %v", err)
	}
	stagingEnv, err := envSvc.CreateEnvironment(ctx, environments.CreateEnvironmentInput{Key: "staging"})
	if err != nil {
		t.Fatalf("create staging environment: %v", err)
	}

	contentRepo := content.NewMemoryContentRepository()
	typeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeRepo.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	defaultType := &content.ContentType{
		ID:            uuid.New(),
		Name:          "Article",
		Slug:          "article",
		Schema:        map[string]any{"fields": []any{"body"}},
		EnvironmentID: defaultEnv.ID,
	}
	seedContentType(t, typeRepo, defaultType)

	stagingType := &content.ContentType{
		ID:            uuid.New(),
		Name:          "Article",
		Slug:          "article",
		Schema:        map[string]any{"fields": []any{"body"}},
		EnvironmentID: stagingEnv.ID,
	}
	seedContentType(t, typeRepo, stagingType)

	svc := content.NewService(contentRepo, typeRepo, localeRepo, content.WithEnvironmentService(envSvc))

	defaultContent, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:  defaultType.ID,
		Slug:           "welcome",
		Status:         "draft",
		EnvironmentKey: "default",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Welcome", Content: map[string]any{"body": "hello"}},
		},
	})
	if err != nil {
		t.Fatalf("create default content: %v", err)
	}

	stagingContent, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:  stagingType.ID,
		Slug:           "welcome",
		Status:         "draft",
		EnvironmentKey: "staging",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Welcome", Content: map[string]any{"body": "hello"}},
		},
	})
	if err != nil {
		t.Fatalf("create staging content: %v", err)
	}

	if _, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:  defaultType.ID,
		Slug:           "welcome",
		Status:         "draft",
		EnvironmentKey: "default",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Welcome", Content: map[string]any{"body": "hello"}},
		},
	}); !errors.Is(err, content.ErrSlugExists) {
		t.Fatalf("expected ErrSlugExists, got %v", err)
	}

	defaultEntries, err := svc.List(ctx, "default")
	if err != nil {
		t.Fatalf("list default: %v", err)
	}
	if len(defaultEntries) != 1 || defaultEntries[0].ID != defaultContent.ID {
		t.Fatalf("expected default list to return default content")
	}

	stagingEntries, err := svc.List(ctx, "staging")
	if err != nil {
		t.Fatalf("list staging: %v", err)
	}
	if len(stagingEntries) != 1 || stagingEntries[0].ID != stagingContent.ID {
		t.Fatalf("expected staging list to return staging content")
	}
}
