package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/environments"
	"github.com/google/uuid"
)

type contentProjectionFixture struct {
	svc      content.Service
	pageType *content.ContentType
	postType *content.ContentType
}

func newContentProjectionFixture(t *testing.T, opts ...content.ServiceOption) *contentProjectionFixture {
	t.Helper()

	ctx := context.Background()
	envRepo := environments.NewMemoryRepository()
	envSvc := environments.NewService(envRepo)

	defaultEnv, err := envSvc.CreateEnvironment(ctx, environments.CreateEnvironmentInput{Key: "default", IsDefault: true})
	if err != nil {
		t.Fatalf("create default environment: %v", err)
	}

	contentRepo := content.NewMemoryContentRepository()
	typeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()
	localeRepo.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"markdown": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
		"additionalProperties": true,
	}

	pageType := &content.ContentType{
		ID:            uuid.New(),
		Name:          "Page",
		Slug:          "page",
		Schema:        schema,
		EnvironmentID: defaultEnv.ID,
	}
	postType := &content.ContentType{
		ID:            uuid.New(),
		Name:          "Post",
		Slug:          "post",
		Schema:        schema,
		EnvironmentID: defaultEnv.ID,
	}
	seedContentType(t, typeRepo, pageType)
	seedContentType(t, typeRepo, postType)

	serviceOpts := append([]content.ServiceOption{content.WithEnvironmentService(envSvc)}, opts...)
	svc := content.NewService(contentRepo, typeRepo, localeRepo, serviceOpts...)

	return &contentProjectionFixture{
		svc:      svc,
		pageType: pageType,
		postType: postType,
	}
}

func (f *contentProjectionFixture) create(t *testing.T, contentTypeID uuid.UUID, slug string, payload map[string]any) uuid.UUID {
	t.Helper()
	record, err := f.svc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID:  contentTypeID,
		Slug:           slug,
		Status:         "draft",
		EnvironmentKey: "default",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Title " + slug,
				Content: payload,
			},
		},
	})
	if err != nil {
		t.Fatalf("create content %s: %v", slug, err)
	}
	return record.ID
}

func translationContent(t *testing.T, record *content.Content) map[string]any {
	t.Helper()
	if record == nil || len(record.Translations) == 0 || record.Translations[0] == nil {
		t.Fatalf("expected at least one translation")
	}
	return record.Translations[0].Content
}

func stringField(t *testing.T, payload map[string]any, key string) string {
	t.Helper()
	value, ok := payload[key]
	if !ok {
		t.Fatalf("expected %s key", key)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("expected %s to be string, got %T", key, value)
	}
	return text
}

func mapField(t *testing.T, payload map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := payload[key]
	if !ok {
		t.Fatalf("expected %s key", key)
	}
	record, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected %s to be map, got %T", key, value)
	}
	return record
}

func TestContentProjectionFrontmatterOnly(t *testing.T) {
	fixture := newContentProjectionFixture(t)

	id := fixture.create(t, fixture.pageType.ID, "about-frontmatter", map[string]any{
		"unknown_top": "keep",
		"markdown": map[string]any{
			"body": "We build composable admin products.",
			"frontmatter": map[string]any{
				"summary":        "How the team builds reliable admin tooling.",
				"path":           "/about",
				"published_at":   "2025-10-13T10:00:00Z",
				"featured_image": "/static/media/logo.png",
				"meta":           map[string]any{"audience": "customers"},
				"tags":           []any{"company", "mission"},
				"template_id":    "template-frontmatter",
				"parent_id":      "parent-frontmatter",
				"blocks":         []any{"hero"},
				"seo":            map[string]any{"title": "About Enterprise Admin", "description": "Meet the team."},
			},
		},
	})

	fetched, err := fixture.svc.Get(context.Background(), id, content.WithDerivedFields())
	if err != nil {
		t.Fatalf("get with derived fields: %v", err)
	}
	payload := translationContent(t, fetched)

	if got := stringField(t, payload, "content"); got != "We build composable admin products." {
		t.Fatalf("expected content body, got %q", got)
	}
	if got := stringField(t, payload, "summary"); got != "How the team builds reliable admin tooling." {
		t.Fatalf("expected summary from frontmatter, got %q", got)
	}
	if got := stringField(t, payload, "excerpt"); got != "How the team builds reliable admin tooling." {
		t.Fatalf("expected excerpt mirror summary, got %q", got)
	}
	if got := stringField(t, payload, "path"); got != "/about" {
		t.Fatalf("expected path /about, got %q", got)
	}
	if got := stringField(t, payload, "meta_title"); got != "About Enterprise Admin" {
		t.Fatalf("expected meta_title from seo, got %q", got)
	}
	if got := stringField(t, payload, "meta_description"); got != "Meet the team." {
		t.Fatalf("expected meta_description from seo, got %q", got)
	}
	if got := stringField(t, payload, "unknown_top"); got != "keep" {
		t.Fatalf("expected unknown_top preserved, got %q", got)
	}
	markdown := mapField(t, payload, "markdown")
	frontmatter := mapField(t, markdown, "frontmatter")
	if got := stringField(t, frontmatter, "summary"); got != "How the team builds reliable admin tooling." {
		t.Fatalf("expected markdown.frontmatter.summary preserved, got %q", got)
	}
	if _, ok := markdown["content"]; ok {
		t.Fatal("markdown payload should remain unchanged")
	}
}

func TestContentProjectionCustomOnly(t *testing.T) {
	fixture := newContentProjectionFixture(t)

	_ = fixture.create(t, fixture.postType.ID, "custom-only", map[string]any{
		"markdown": map[string]any{
			"body": "custom body",
			"custom": map[string]any{
				"summary": "Custom summary",
				"path":    "/custom",
			},
		},
	})

	listed, err := fixture.svc.List(context.Background(), "default", content.WithDerivedFields())
	if err != nil {
		t.Fatalf("list with derived fields: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 record got %d", len(listed))
	}
	payload := translationContent(t, listed[0])
	if got := stringField(t, payload, "summary"); got != "Custom summary" {
		t.Fatalf("expected summary from custom, got %q", got)
	}
	if got := stringField(t, payload, "excerpt"); got != "Custom summary" {
		t.Fatalf("expected excerpt mirror summary, got %q", got)
	}
	if got := stringField(t, payload, "path"); got != "/custom" {
		t.Fatalf("expected path from custom, got %q", got)
	}
}

func TestContentProjectionLegacyNestedFrontmatter(t *testing.T) {
	fixture := newContentProjectionFixture(t)

	id := fixture.create(t, fixture.pageType.ID, "legacy-shape", map[string]any{
		"markdown": map[string]any{
			"custom": map[string]any{
				"markdown": map[string]any{
					"frontmatter": map[string]any{
						"summary": "Legacy summary",
						"path":    "/legacy",
						"tags":    []any{"legacy"},
					},
				},
			},
		},
	})

	fetched, err := fixture.svc.Get(context.Background(), id, content.WithProjection("admin"))
	if err != nil {
		t.Fatalf("get with admin projection: %v", err)
	}
	payload := translationContent(t, fetched)
	if got := stringField(t, payload, "summary"); got != "Legacy summary" {
		t.Fatalf("expected summary from legacy frontmatter, got %q", got)
	}
	if got := stringField(t, payload, "excerpt"); got != "Legacy summary" {
		t.Fatalf("expected excerpt from legacy frontmatter, got %q", got)
	}
	if got := stringField(t, payload, "path"); got != "/legacy" {
		t.Fatalf("expected path from legacy frontmatter, got %q", got)
	}
}

func TestContentProjectionPreservesTopLevelValues(t *testing.T) {
	fixture := newContentProjectionFixture(t)

	id := fixture.create(t, fixture.postType.ID, "preserve-top-level", map[string]any{
		"content":          "top-level content",
		"summary":          "top-level summary",
		"excerpt":          "top-level excerpt",
		"path":             "/top-level",
		"featured_image":   "/top-level.png",
		"meta":             map[string]any{"source": "top"},
		"tags":             []any{"top"},
		"template_id":      "template-top",
		"parent_id":        "parent-top",
		"meta_title":       "Top Meta Title",
		"meta_description": "Top Meta Description",
		"markdown": map[string]any{
			"body": "source body",
			"custom": map[string]any{
				"summary":          "source summary",
				"path":             "/source",
				"featured_image":   "/source.png",
				"meta":             map[string]any{"source": "custom"},
				"tags":             []any{"source"},
				"template_id":      "template-source",
				"parent_id":        "parent-source",
				"meta_title":       "Source Meta Title",
				"meta_description": "Source Meta Description",
				"seo":              map[string]any{"title": "Source SEO", "description": "Source SEO Desc"},
			},
		},
	})

	fetched, err := fixture.svc.Get(context.Background(), id, content.WithDerivedFields())
	if err != nil {
		t.Fatalf("get with derived fields: %v", err)
	}
	payload := translationContent(t, fetched)

	if got := stringField(t, payload, "content"); got != "top-level content" {
		t.Fatalf("expected top-level content preserved, got %q", got)
	}
	if got := stringField(t, payload, "summary"); got != "top-level summary" {
		t.Fatalf("expected top-level summary preserved, got %q", got)
	}
	if got := stringField(t, payload, "excerpt"); got != "top-level excerpt" {
		t.Fatalf("expected top-level excerpt preserved, got %q", got)
	}
	if got := stringField(t, payload, "path"); got != "/top-level" {
		t.Fatalf("expected top-level path preserved, got %q", got)
	}
	if got := stringField(t, payload, "meta_title"); got != "Top Meta Title" {
		t.Fatalf("expected top-level meta_title preserved, got %q", got)
	}
	if got := stringField(t, payload, "meta_description"); got != "Top Meta Description" {
		t.Fatalf("expected top-level meta_description preserved, got %q", got)
	}
}

func TestContentProjectionUsesTopLevelSEOForMetaFallback(t *testing.T) {
	fixture := newContentProjectionFixture(t)

	id := fixture.create(t, fixture.pageType.ID, "seo-fallback", map[string]any{
		"seo": map[string]any{
			"title":       "Top SEO Title",
			"description": "Top SEO Description",
		},
		"markdown": map[string]any{
			"body": "seo body",
		},
	})

	fetched, err := fixture.svc.Get(context.Background(), id, content.WithDerivedFields())
	if err != nil {
		t.Fatalf("get with derived fields: %v", err)
	}
	payload := translationContent(t, fetched)
	if got := stringField(t, payload, "meta_title"); got != "Top SEO Title" {
		t.Fatalf("expected meta_title from top-level seo, got %q", got)
	}
	if got := stringField(t, payload, "meta_description"); got != "Top SEO Description" {
		t.Fatalf("expected meta_description from top-level seo, got %q", got)
	}
}

func TestContentProjectionModeNoopAndError(t *testing.T) {
	fixture := newContentProjectionFixture(t)
	id := fixture.create(t, fixture.postType.ID, "mode-behavior", map[string]any{
		"markdown": map[string]any{
			"body": "mode body",
			"custom": map[string]any{
				"summary": "Mode summary",
			},
		},
	})

	noopRecord, err := fixture.svc.Get(context.Background(), id, content.WithDerivedFields(), content.WithProjectionMode(content.ProjectionTranslationModeNoop))
	if err != nil {
		t.Fatalf("get with noop projection mode: %v", err)
	}
	if len(noopRecord.Translations) != 0 {
		t.Fatalf("expected translations to remain omitted in noop mode, got %d", len(noopRecord.Translations))
	}

	_, err = fixture.svc.Get(context.Background(), id, content.WithDerivedFields(), content.WithProjectionMode(content.ProjectionTranslationModeError))
	if !errors.Is(err, content.ErrContentProjectionRequiresTranslations) {
		t.Fatalf("expected ErrContentProjectionRequiresTranslations, got %v", err)
	}
}

func TestContentProjectionModeConfigurableAtServiceLevel(t *testing.T) {
	fixture := newContentProjectionFixture(t, content.WithProjectionTranslationMode(content.ProjectionTranslationModeError))
	id := fixture.create(t, fixture.postType.ID, "service-mode", map[string]any{
		"markdown": map[string]any{
			"body": "service mode body",
		},
	})

	_, err := fixture.svc.Get(context.Background(), id, content.WithDerivedFields())
	if !errors.Is(err, content.ErrContentProjectionRequiresTranslations) {
		t.Fatalf("expected ErrContentProjectionRequiresTranslations from service-level mode, got %v", err)
	}

	record, err := fixture.svc.Get(context.Background(), id, content.WithDerivedFields(), content.WithProjectionMode(content.ProjectionTranslationModeNoop))
	if err != nil {
		t.Fatalf("expected per-call mode override to succeed, got %v", err)
	}
	if len(record.Translations) != 0 {
		t.Fatalf("expected translations omitted with noop override, got %d", len(record.Translations))
	}
}

func TestContentProjectionWorksForPageAndPostTypes(t *testing.T) {
	fixture := newContentProjectionFixture(t)

	pageID := fixture.create(t, fixture.pageType.ID, "page-projection", map[string]any{
		"markdown": map[string]any{
			"body": "page body",
			"custom": map[string]any{
				"path": "/page",
			},
		},
	})
	postID := fixture.create(t, fixture.postType.ID, "post-projection", map[string]any{
		"markdown": map[string]any{
			"body": "post body",
			"custom": map[string]any{
				"path": "/post",
			},
		},
	})

	listed, err := fixture.svc.List(context.Background(), "default", content.WithDerivedFields())
	if err != nil {
		t.Fatalf("list with derived fields: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 records got %d", len(listed))
	}

	seen := map[uuid.UUID]bool{}
	for _, record := range listed {
		if record == nil {
			continue
		}
		if record.ID != pageID && record.ID != postID {
			continue
		}
		seen[record.ID] = true
		payload := translationContent(t, record)
		if stringField(t, payload, "content") == "" {
			t.Fatalf("expected derived content for %s", record.ID)
		}
		if stringField(t, payload, "path") == "" {
			t.Fatalf("expected derived path for %s", record.ID)
		}
	}
	if !seen[pageID] || !seen[postID] {
		t.Fatalf("expected both page and post records, seen=%v", seen)
	}
}
