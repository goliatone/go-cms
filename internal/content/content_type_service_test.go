package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
)

type staticNormalizer struct {
	value string
	err   error
}

func (n staticNormalizer) Normalize(string) (string, error) {
	return n.value, n.err
}

func TestContentTypeServiceCreateDerivesSlugFromName(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	ct, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Landing Page",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	if ct.Slug != "landing-page" {
		t.Fatalf("expected slug landing-page, got %q", ct.Slug)
	}
}

func TestContentTypeServiceCreateUsesSchemaSlug(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	ct, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article",
		Schema: map[string]any{"metadata": map[string]any{"slug": "Story"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	if ct.Slug != "story" {
		t.Fatalf("expected schema slug story, got %q", ct.Slug)
	}
}

func TestContentTypeServiceCreateNormalizesSlugRules(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	ct, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Blog_Post Draft",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	if ct.Slug != "blog-post-draft" {
		t.Fatalf("expected normalized slug blog-post-draft, got %q", ct.Slug)
	}
}

func TestContentTypeServiceCreateRejectsDuplicateSlug(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	_, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article",
		Slug:   "article",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	_, err = svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article Two",
		Slug:   "article",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if !errors.Is(err, content.ErrContentTypeSlugExists) {
		t.Fatalf("expected ErrContentTypeSlugExists, got %v", err)
	}
}

func TestContentTypeServiceGetBySlugNormalizesInput(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	created, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Landing Page",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	got, err := svc.GetBySlug(context.Background(), "Landing Page")
	if err != nil {
		t.Fatalf("get by slug: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected slug lookup to return created content type")
	}
}

func TestContentTypeServiceUpdateRejectsSlugConflict(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo)

	first, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article",
		Slug:   "article",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	second, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "News",
		Slug:   "news",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	_, err = svc.Update(context.Background(), content.UpdateContentTypeRequest{
		ID:   second.ID,
		Slug: &first.Slug,
	})
	if !errors.Is(err, content.ErrContentTypeSlugExists) {
		t.Fatalf("expected ErrContentTypeSlugExists, got %v", err)
	}
}

func TestContentTypeServiceCreateRejectsInvalidSlug(t *testing.T) {
	repo := content.NewMemoryContentTypeRepository()
	svc := content.NewContentTypeService(repo, content.WithContentTypeSlugNormalizer(staticNormalizer{value: "bad@slug"}))

	_, err := svc.Create(context.Background(), content.CreateContentTypeRequest{
		Name:   "Article",
		Slug:   "Article",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if !errors.Is(err, content.ErrContentTypeSlugInvalid) {
		t.Fatalf("expected ErrContentTypeSlugInvalid, got %v", err)
	}
}
