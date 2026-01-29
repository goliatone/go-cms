package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/migrations"
	"github.com/google/uuid"
)

func TestContentCreatePersistsSchemaVersion(t *testing.T) {
	contentRepo := content.NewMemoryContentRepository()
	typeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeRepo.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentTypeID := uuid.New()
	seedContentType(t, typeRepo, &content.ContentType{
		ID:     contentTypeID,
		Name:   "Page",
		Slug:   "page",
		Schema: map[string]any{"fields": []any{"title"}},
	})

	svc := content.NewService(contentRepo, typeRepo, localeRepo)

	author := uuid.New()
	created, err := svc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "welcome",
		CreatedBy:     author,
		UpdatedBy:     author,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Welcome",
				Content: map[string]any{"title": "Hello"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}
	if len(created.Translations) != 1 {
		t.Fatalf("expected 1 translation got %d", len(created.Translations))
	}
	if created.Translations[0].Content["_schema"] != "page@v1.0.0" {
		t.Fatalf("expected _schema page@v1.0.0 got %v", created.Translations[0].Content["_schema"])
	}
}

func TestPublishDraftMigratesContentSchema(t *testing.T) {
	contentRepo := content.NewMemoryContentRepository()
	typeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeRepo.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentTypeID := uuid.New()
	seedContentType(t, typeRepo, &content.ContentType{
		ID:   contentTypeID,
		Name: "Article",
		Slug: "article",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "string"},
			},
			"required": []any{"title"},
			"metadata": map[string]any{"schema_version": "article@v1.0.0"},
		},
	})

	registry := migrations.NewRegistry()
	if err := registry.Register("article", "article@v1.0.0", "article@v2.0.0", func(payload map[string]any) (map[string]any, error) {
		if title, ok := payload["title"]; ok {
			payload["headline"] = title
			delete(payload, "title")
		}
		return payload, nil
	}); err != nil {
		t.Fatalf("register migration: %v", err)
	}

	svc := content.NewService(
		contentRepo,
		typeRepo,
		localeRepo,
		content.WithVersioningEnabled(true),
		content.WithSchemaMigrator(registry.Migrator()),
	)

	author := uuid.New()
	created, err := svc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "news",
		CreatedBy:     author,
		UpdatedBy:     author,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "News", Content: map[string]any{"title": "Hello"}},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	draftSnapshot := content.ContentVersionSnapshot{
		Translations: []content.ContentVersionTranslationSnapshot{
			{
				Locale:  "en",
				Title:   "Draft",
				Content: map[string]any{"_schema": "article@v1.0.0", "title": "Hello"},
			},
		},
	}

	draft, err := svc.CreateDraft(context.Background(), content.CreateContentDraftRequest{
		ContentID: created.ID,
		Snapshot:  draftSnapshot,
		CreatedBy: author,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	ctx := context.Background()
	ct, err := typeRepo.GetByID(ctx, contentTypeID)
	if err != nil {
		t.Fatalf("get content type: %v", err)
	}
	ct.Schema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"headline": map[string]any{"type": "string"},
		},
		"required": []any{"headline"},
		"metadata": map[string]any{"schema_version": "article@v2.0.0"},
	}
	ct.SchemaVersion = "article@v2.0.0"
	if _, err := typeRepo.Update(ctx, ct); err != nil {
		t.Fatalf("update content type: %v", err)
	}

	published, err := svc.PublishDraft(ctx, content.PublishContentDraftRequest{
		ContentID:   created.ID,
		Version:     draft.Version,
		PublishedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}

	if len(published.Snapshot.Translations) != 1 {
		t.Fatalf("expected 1 translation in published snapshot")
	}
	contentPayload := published.Snapshot.Translations[0].Content
	if contentPayload["_schema"] != "article@v2.0.0" {
		t.Fatalf("expected _schema article@v2.0.0 got %v", contentPayload["_schema"])
	}
	if contentPayload["headline"] != "Hello" {
		t.Fatalf("expected headline Hello got %v", contentPayload["headline"])
	}
	if _, ok := contentPayload["title"]; ok {
		t.Fatalf("expected title to be removed after migration")
	}
}

func TestPublishDraftFailsOnInvalidMigrationPayload(t *testing.T) {
	contentRepo := content.NewMemoryContentRepository()
	typeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeRepo.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	contentTypeID := uuid.New()
	seedContentType(t, typeRepo, &content.ContentType{
		ID:   contentTypeID,
		Name: "Article",
		Slug: "article",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "string"},
			},
			"required": []any{"title"},
			"metadata": map[string]any{"schema_version": "article@v1.0.0"},
		},
	})

	registry := migrations.NewRegistry()
	if err := registry.Register("article", "article@v1.0.0", "article@v2.0.0", func(payload map[string]any) (map[string]any, error) {
		delete(payload, "title")
		return payload, nil
	}); err != nil {
		t.Fatalf("register migration: %v", err)
	}

	svc := content.NewService(
		contentRepo,
		typeRepo,
		localeRepo,
		content.WithVersioningEnabled(true),
		content.WithSchemaMigrator(registry.Migrator()),
	)

	author := uuid.New()
	created, err := svc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "news",
		CreatedBy:     author,
		UpdatedBy:     author,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "News", Content: map[string]any{"title": "Hello"}},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	draftSnapshot := content.ContentVersionSnapshot{
		Translations: []content.ContentVersionTranslationSnapshot{
			{
				Locale:  "en",
				Title:   "Draft",
				Content: map[string]any{"_schema": "article@v1.0.0", "title": "Hello"},
			},
		},
	}

	draft, err := svc.CreateDraft(context.Background(), content.CreateContentDraftRequest{
		ContentID: created.ID,
		Snapshot:  draftSnapshot,
		CreatedBy: author,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	ctx := context.Background()
	ct, err := typeRepo.GetByID(ctx, contentTypeID)
	if err != nil {
		t.Fatalf("get content type: %v", err)
	}
	ct.Schema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"headline": map[string]any{"type": "string"},
		},
		"required": []any{"headline"},
		"metadata": map[string]any{"schema_version": "article@v2.0.0"},
	}
	ct.SchemaVersion = "article@v2.0.0"
	if _, err := typeRepo.Update(ctx, ct); err != nil {
		t.Fatalf("update content type: %v", err)
	}

	_, err = svc.PublishDraft(ctx, content.PublishContentDraftRequest{
		ContentID:   created.ID,
		Version:     draft.Version,
		PublishedBy: uuid.New(),
	})
	if !errors.Is(err, content.ErrContentSchemaInvalid) {
		t.Fatalf("expected ErrContentSchemaInvalid got %v", err)
	}
}
