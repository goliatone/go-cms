package pages_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

func TestPageServiceEnvironmentScopedSlugs(t *testing.T) {
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
		Name:          "Page",
		Slug:          "page",
		Schema:        map[string]any{"fields": []any{"body"}},
		EnvironmentID: defaultEnv.ID,
	}
	seedContentType(t, typeRepo, defaultType)

	stagingType := &content.ContentType{
		ID:            uuid.New(),
		Name:          "Page",
		Slug:          "page",
		Schema:        map[string]any{"fields": []any{"body"}},
		EnvironmentID: stagingEnv.ID,
	}
	seedContentType(t, typeRepo, stagingType)

	contentSvc := content.NewService(contentRepo, typeRepo, localeRepo, content.WithEnvironmentService(envSvc))

	defaultContent, err := contentSvc.Create(ctx, content.CreateContentRequest{
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

	stagingContent, err := contentSvc.Create(ctx, content.CreateContentRequest{
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

	pageRepo := pages.NewMemoryPageRepository()
	pageSvc := pages.NewService(pageRepo, contentRepo, localeRepo, pages.WithEnvironmentService(envSvc))

	defaultPage, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:      defaultContent.ID,
		TemplateID:     uuid.New(),
		Slug:           "home",
		EnvironmentKey: "default",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []pages.PageTranslationInput{
			{Locale: "en", Title: "Home", Path: "/home"},
		},
	})
	if err != nil {
		t.Fatalf("create default page: %v", err)
	}

	_, err = pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:      stagingContent.ID,
		TemplateID:     uuid.New(),
		Slug:           "home",
		EnvironmentKey: "staging",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []pages.PageTranslationInput{
			{Locale: "en", Title: "Home", Path: "/home"},
		},
	})
	if err != nil {
		t.Fatalf("create staging page: %v", err)
	}

	otherContent, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:  defaultType.ID,
		Slug:           "other",
		Status:         "draft",
		EnvironmentKey: "default",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Other", Content: map[string]any{"body": "hello"}},
		},
	})
	if err != nil {
		t.Fatalf("create other content: %v", err)
	}

	if _, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:      otherContent.ID,
		TemplateID:     uuid.New(),
		Slug:           "home",
		EnvironmentKey: "default",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []pages.PageTranslationInput{
			{Locale: "en", Title: "Home", Path: "/home-dup"},
		},
	}); !errors.Is(err, pages.ErrSlugExists) {
		t.Fatalf("expected ErrSlugExists, got %v", err)
	}

	defaultPages, err := pageSvc.List(ctx, "default")
	if err != nil {
		t.Fatalf("list default: %v", err)
	}
	if len(defaultPages) != 1 || defaultPages[0].ID != defaultPage.ID {
		t.Fatalf("expected default list to return default page")
	}

	stagingPages, err := pageSvc.List(ctx, "staging")
	if err != nil {
		t.Fatalf("list staging: %v", err)
	}
	if len(stagingPages) != 1 {
		t.Fatalf("expected 1 staging page, got %d", len(stagingPages))
	}
}
