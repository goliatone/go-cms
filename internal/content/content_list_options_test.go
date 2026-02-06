package content_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/environments"
	"github.com/google/uuid"
)

type contentListFixture struct {
	svc         content.Service
	defaultType *content.ContentType
	stagingType *content.ContentType
}

func newContentListFixture(t *testing.T) *contentListFixture {
	t.Helper()

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

	return &contentListFixture{
		svc:         svc,
		defaultType: defaultType,
		stagingType: stagingType,
	}
}

func TestContentServiceListDefaultOmitsTranslations(t *testing.T) {
	ctx := context.Background()
	fixture := newContentListFixture(t)

	if _, err := fixture.svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:  fixture.defaultType.ID,
		Slug:           "welcome",
		Status:         "draft",
		EnvironmentKey: "default",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Welcome", Content: map[string]any{"body": "hello"}},
		},
	}); err != nil {
		t.Fatalf("create content: %v", err)
	}

	listed, err := fixture.svc.List(ctx, "default")
	if err != nil {
		t.Fatalf("list content: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 record got %d", len(listed))
	}
	if len(listed[0].Translations) != 0 {
		t.Fatalf("expected translations to be omitted, got %d", len(listed[0].Translations))
	}
}

func TestContentServiceListWithTranslationsIncludesLocale(t *testing.T) {
	ctx := context.Background()
	fixture := newContentListFixture(t)

	if _, err := fixture.svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:  fixture.defaultType.ID,
		Slug:           "welcome",
		Status:         "draft",
		EnvironmentKey: "default",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Welcome", Content: map[string]any{"body": "hello"}},
		},
	}); err != nil {
		t.Fatalf("create content: %v", err)
	}

	listed, err := fixture.svc.List(ctx, "default", content.WithTranslations())
	if err != nil {
		t.Fatalf("list content with translations: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 record got %d", len(listed))
	}
	if len(listed[0].Translations) == 0 {
		t.Fatal("expected translations to be populated")
	}
	tr := listed[0].Translations[0]
	if tr.Title == "" {
		t.Fatal("expected translation title to be populated")
	}
	if len(tr.Content) == 0 {
		t.Fatal("expected translation content to be populated")
	}
	if tr.Locale == nil || tr.Locale.Code != "en" {
		t.Fatalf("expected locale code en, got %+v", tr.Locale)
	}
}

func TestContentServiceListWithTranslationsRespectsEnvironment(t *testing.T) {
	ctx := context.Background()
	fixture := newContentListFixture(t)

	if _, err := fixture.svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:  fixture.defaultType.ID,
		Slug:           "welcome",
		Status:         "draft",
		EnvironmentKey: "default",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Welcome", Content: map[string]any{"body": "hello"}},
		},
	}); err != nil {
		t.Fatalf("create default content: %v", err)
	}

	if _, err := fixture.svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:  fixture.stagingType.ID,
		Slug:           "welcome",
		Status:         "draft",
		EnvironmentKey: "staging",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Welcome", Content: map[string]any{"body": "hello"}},
		},
	}); err != nil {
		t.Fatalf("create staging content: %v", err)
	}

	listed, err := fixture.svc.List(ctx, "staging", content.WithTranslations())
	if err != nil {
		t.Fatalf("list staging with translations: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 record got %d", len(listed))
	}
	if listed[0].EnvironmentID != fixture.stagingType.EnvironmentID {
		t.Fatalf("expected staging environment %s got %s", fixture.stagingType.EnvironmentID, listed[0].EnvironmentID)
	}
	if len(listed[0].Translations) == 0 {
		t.Fatal("expected staging translations to be populated")
	}
}
