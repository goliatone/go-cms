package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

func TestContentEntryCRUDForPageAndBlogPost(t *testing.T) {
	ctx := context.Background()

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en"}
	cfg.I18N.RequireTranslations = true
	cfg.I18N.DefaultLocaleRequired = true
	cfg.Cache.Enabled = false

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("cms.New: %v", err)
	}

	typeRepo := module.Container().ContentTypeRepository()
	seeder, ok := typeRepo.(interface {
		Put(*content.ContentType) error
	})
	if !ok {
		t.Fatalf("expected seedable content type repository, got %T", typeRepo)
	}

	pageTypeID := uuid.New()
	if err := seeder.Put(&content.ContentType{
		ID:   pageTypeID,
		Name: "page",
		Slug: "page",
		Schema: map[string]any{
			"fields": []any{map[string]any{"name": "body"}},
		},
	}); err != nil {
		t.Fatalf("seed page content type: %v", err)
	}

	blogTypeID := uuid.New()
	if err := seeder.Put(&content.ContentType{
		ID:   blogTypeID,
		Name: "blog_post",
		Slug: "blog_post",
		Schema: map[string]any{
			"fields": []any{map[string]any{"name": "body"}},
		},
	}); err != nil {
		t.Fatalf("seed blog_post content type: %v", err)
	}

	contentSvc := module.Content()
	pageRepo := module.Container().PageRepository()
	authorID := uuid.New()

	pageEntry := createContentEntry(t, ctx, contentSvc, pageTypeID, "page-entry", authorID)
	assertNoPages(t, ctx, pageRepo)

	if _, err := contentSvc.Update(ctx, content.UpdateContentRequest{
		ID:        pageEntry.ID,
		Status:    "published",
		UpdatedBy: authorID,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Page Updated", Content: map[string]any{"body": "updated"}},
		},
	}); err != nil {
		t.Fatalf("update page content entry: %v", err)
	}
	assertNoPages(t, ctx, pageRepo)

	if err := contentSvc.Delete(ctx, content.DeleteContentRequest{
		ID:         pageEntry.ID,
		DeletedBy:  authorID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("delete page content entry: %v", err)
	}
	assertContentDeleted(t, ctx, contentSvc, pageEntry.ID)

	blogEntry := createContentEntry(t, ctx, contentSvc, blogTypeID, "blog-entry", authorID)
	assertNoPages(t, ctx, pageRepo)

	if _, err := contentSvc.Update(ctx, content.UpdateContentRequest{
		ID:        blogEntry.ID,
		Status:    "published",
		UpdatedBy: authorID,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Blog Updated", Content: map[string]any{"body": "updated"}},
		},
	}); err != nil {
		t.Fatalf("update blog_post content entry: %v", err)
	}
	assertNoPages(t, ctx, pageRepo)

	if err := contentSvc.Delete(ctx, content.DeleteContentRequest{
		ID:         blogEntry.ID,
		DeletedBy:  authorID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("delete blog_post content entry: %v", err)
	}
	assertContentDeleted(t, ctx, contentSvc, blogEntry.ID)
	assertNoPages(t, ctx, pageRepo)
}

func createContentEntry(t *testing.T, ctx context.Context, svc content.Service, typeID uuid.UUID, slug string, authorID uuid.UUID) *content.Content {
	t.Helper()

	record, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          slug,
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{Locale: "en", Title: "Entry", Content: map[string]any{"body": "content"}},
		},
	})
	if err != nil {
		t.Fatalf("create content entry %q: %v", slug, err)
	}
	return record
}

func assertContentDeleted(t *testing.T, ctx context.Context, svc content.Service, id uuid.UUID) {
	t.Helper()

	_, err := svc.Get(ctx, id)
	if err == nil {
		t.Fatalf("expected content entry %s to be deleted", id)
	}
	var notFound *content.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected NotFoundError after delete, got %v", err)
	}
}

func assertNoPages(t *testing.T, ctx context.Context, repo pages.PageRepository) {
	t.Helper()

	pagesList, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(pagesList) != 0 {
		t.Fatalf("expected no page records, got %d", len(pagesList))
	}
}
