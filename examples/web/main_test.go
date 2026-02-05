package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
)

func TestDemoSeededContentTypesActiveAndSchemaFields(t *testing.T) {
	ctx := context.Background()

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}
	cfg.I18N.RequireTranslations = true
	cfg.I18N.DefaultLocaleRequired = true
	cfg.Features.Themes = true
	cfg.Features.Widgets = false
	cfg.Cache.Enabled = false

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("cms.New: %v", err)
	}

	themePath := filepath.Join("..", "..", "themes", "default")
	if _, err := setupDemoData(ctx, module, &cfg, themePath); err != nil {
		t.Fatalf("setupDemoData: %v", err)
	}

	typeRepo := module.Container().ContentTypeRepository()

	pageType, err := typeRepo.GetBySlug(ctx, "page")
	if err != nil {
		t.Fatalf("get page content type: %v", err)
	}
	if pageType.Status != content.ContentTypeStatusActive {
		t.Fatalf("expected page content type status %q got %q", content.ContentTypeStatusActive, pageType.Status)
	}
	if !capabilityBool(pageType.Capabilities, "structural_fields") {
		t.Fatalf("expected page content type structural_fields capability to be true")
	}

	blogType, err := typeRepo.GetBySlug(ctx, "blog_post")
	if err != nil {
		t.Fatalf("get blog_post content type: %v", err)
	}
	if blogType.Status != content.ContentTypeStatusActive {
		t.Fatalf("expected blog_post content type status %q got %q", content.ContentTypeStatusActive, blogType.Status)
	}

	pageFields := collectSchemaFieldNames(pageType.Schema)
	assertSchemaFields(t, "page", pageFields, []string{
		"title",
		"slug",
		"status",
		"summary",
		"body",
		"seo",
		"blocks",
		"path",
		"template_id",
		"parent_id",
		"sort_order",
	})

	blogFields := collectSchemaFieldNames(blogType.Schema)
	assertSchemaFields(t, "blog_post", blogFields, []string{
		"title",
		"slug",
		"status",
		"body",
		"excerpt",
		"tags",
		"category",
		"published_at",
		"seo",
		"blocks",
	})
}

func collectSchemaFieldNames(schema map[string]any) map[string]struct{} {
	names := map[string]struct{}{}
	if schema == nil {
		return names
	}

	raw, ok := schema["fields"]
	if !ok {
		return names
	}

	switch typed := raw.(type) {
	case []map[string]any:
		for _, field := range typed {
			if name, ok := field["name"].(string); ok {
				names[name] = struct{}{}
			}
		}
	case []any:
		for _, field := range typed {
			switch value := field.(type) {
			case string:
				names[value] = struct{}{}
			case map[string]any:
				if name, ok := value["name"].(string); ok {
					names[name] = struct{}{}
				}
			}
		}
	}

	return names
}

func assertSchemaFields(t *testing.T, slug string, fields map[string]struct{}, expected []string) {
	t.Helper()
	for _, name := range expected {
		if _, ok := fields[name]; !ok {
			t.Fatalf("expected %s schema to include field %q", slug, name)
		}
	}
}
