package schema

import (
	"context"
	"fmt"
	"testing"
	"time"

	crud "github.com/goliatone/go-crud"
)

func TestNormalizeContentSchemaAppliesOverlay(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type": "string",
				"ui": map[string]any{
					"label":       "Inline",
					"placeholder": "inline placeholder",
				},
				"x-formgen": map[string]any{
					"help": "inline help",
				},
			},
		},
		"metadata": map[string]any{
			"schema_version": "article@v1.0.0",
		},
	}

	overlay := OverlayDocument{
		Schema: overlaySchemaVersion,
		Overrides: []OverlayOverride{
			{
				Path: "/properties/title",
				XFormgen: map[string]any{
					"label": "Overlay",
					"help":  "overlay help",
				},
			},
		},
	}

	normalized, err := NormalizeContentSchema(context.Background(), schema, NormalizeOptions{
		Slug:              "article",
		OverlayDocuments:  []OverlayDocument{overlay},
		FailOnUnsupported: true,
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	title, ok := normalized.Schema["properties"].(map[string]any)["title"].(map[string]any)
	if !ok {
		t.Fatalf("expected title schema")
	}
	meta := title["x-formgen"].(map[string]any)
	if meta["label"] != "Overlay" {
		t.Fatalf("expected overlay label, got %v", meta["label"])
	}
	if meta["help"] != "overlay help" {
		t.Fatalf("expected overlay help, got %v", meta["help"])
	}
	if meta["placeholder"] != "inline placeholder" {
		t.Fatalf("expected inline placeholder, got %v", meta["placeholder"])
	}
	if normalized.Version.String() != "article@v1.0.0" {
		t.Fatalf("expected version article@v1.0.0 got %s", normalized.Version.String())
	}
}

func TestProjectToOpenAPIRegistersSchemas(t *testing.T) {
	contentSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"body": map[string]any{"type": "string"},
		},
		"metadata": map[string]any{
			"schema_version": "article@v2.1.0",
		},
	}

	normalized, err := NormalizeContentSchema(context.Background(), contentSchema, NormalizeOptions{
		Slug: "article",
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	projection, err := ProjectToOpenAPI("article", "Article", normalized.Schema, normalized.Version, []BlockSchema{
		{
			Name: "Hero Banner",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"headline": map[string]any{"type": "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	registry := &captureRegistry{entries: map[string]map[string]any{}}
	if err := RegisterProjections(context.Background(), registry, []*Projection{projection}); err != nil {
		t.Fatalf("register: %v", err)
	}
	doc, ok := registry.entries["article"]
	if !ok {
		t.Fatalf("expected article projection registered")
	}
	components := doc["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	if _, ok := schemas["article"]; !ok {
		t.Fatalf("expected article schema component")
	}
	if _, ok := schemas["hero_banner"]; !ok {
		t.Fatalf("expected hero_banner schema component")
	}
}

func TestNormalizeContentSchemaResolvesOverlayRefs(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"headline": map[string]any{"type": "string"},
		},
		"metadata": map[string]any{
			"schema_version": "landing@v1.0.0",
			"ui_overlays":    []string{"landing.overlay.json"},
		},
	}

	resolver := &stubOverlayResolver{
		doc: OverlayDocument{
			Schema: overlaySchemaVersion,
			Overrides: []OverlayOverride{
				{
					Path: "/properties/headline",
					XFormgen: map[string]any{
						"label": "Hero Headline",
					},
				},
			},
		},
	}

	normalized, err := NormalizeContentSchema(context.Background(), schema, NormalizeOptions{
		Slug:            "landing",
		OverlayResolver: resolver,
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	headline := normalized.Schema["properties"].(map[string]any)["headline"].(map[string]any)
	meta := headline["x-formgen"].(map[string]any)
	if meta["label"] != "Hero Headline" {
		t.Fatalf("expected overlay label applied")
	}
	if resolver.calls != 1 {
		t.Fatalf("expected resolver called once, got %d", resolver.calls)
	}
}

func TestNormalizeContentSchemaReadsBlockAvailability(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"metadata": map[string]any{
			"schema_version": "article@v1.0.0",
			"block_availability": map[string]any{
				"allow": []any{"Hero", "gallery"},
				"deny":  []any{"promo"},
			},
		},
	}

	normalized, err := NormalizeContentSchema(context.Background(), schema, NormalizeOptions{
		Slug: "article",
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	availability := normalized.Metadata.BlockAvailability
	if len(availability.Allow) != 2 || availability.Allow[0] != "Hero" || availability.Allow[1] != "gallery" {
		t.Fatalf("unexpected allow list: %#v", availability.Allow)
	}
	if len(availability.Deny) != 1 || availability.Deny[0] != "promo" {
		t.Fatalf("unexpected deny list: %#v", availability.Deny)
	}
	if !availability.Allows("hero") {
		t.Fatalf("expected hero to be allowed")
	}
	if availability.Allows("promo") {
		t.Fatalf("expected promo to be denied")
	}
}

type captureRegistry struct {
	entries map[string]map[string]any
}

func (c *captureRegistry) Register(_ context.Context, name string, doc map[string]any) error {
	if c.entries == nil {
		c.entries = map[string]map[string]any{}
	}
	c.entries[name] = doc
	return nil
}

type stubOverlayResolver struct {
	calls int
	doc   OverlayDocument
}

func (s *stubOverlayResolver) Resolve(_ context.Context, _ string) (OverlayDocument, error) {
	s.calls++
	return s.doc, nil
}

func TestNormalizeProjectRegistersWithCRUDRegistry(t *testing.T) {
	resource := fmt.Sprintf("article_%d", time.Now().UnixNano())
	contentSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"body": map[string]any{"type": "string"},
		},
		"metadata": map[string]any{
			"schema_version": resource + "@v1.0.0",
		},
	}

	normalized, err := NormalizeContentSchema(context.Background(), contentSchema, NormalizeOptions{
		Slug:              resource,
		FailOnUnsupported: true,
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}

	projection, err := ProjectToOpenAPI(resource, "Article", normalized.Schema, normalized.Version, []BlockSchema{
		{
			Name: "Hero Banner",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"headline": map[string]any{"type": "string"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	adapter := crudRegistryAdapter{resource: resource}
	if err := RegisterProjections(context.Background(), adapter, []*Projection{projection}); err != nil {
		t.Fatalf("register: %v", err)
	}

	entry, ok := crud.GetSchema(resource)
	if !ok {
		t.Fatalf("expected schema %s registered", resource)
	}
	if entry.Document["openapi"] == nil {
		t.Fatalf("expected openapi document in registry")
	}
	components, ok := entry.Document["components"].(map[string]any)
	if !ok {
		t.Fatalf("expected components in document")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatalf("expected schemas in document")
	}
	if _, ok := schemas[componentName(resource)]; !ok {
		t.Fatalf("expected %s schema component", componentName(resource))
	}
	if _, ok := schemas[componentName("Hero Banner")]; !ok {
		t.Fatalf("expected hero banner schema component")
	}
	if cmsMeta, ok := entry.Document["x-cms"].(map[string]any); !ok || cmsMeta["content_type"] != resource {
		t.Fatalf("expected x-cms metadata for %s", resource)
	}
}

// crudRegistryAdapter bridges schema projections into the go-crud registry.
type crudRegistryAdapter struct {
	resource string
}

func (a crudRegistryAdapter) Register(_ context.Context, name string, doc map[string]any) error {
	resource := name
	if a.resource != "" {
		resource = a.resource
	}
	plural := resource + "s"
	if ok := crud.RegisterSchemaDocument(resource, plural, doc); !ok {
		return fmt.Errorf("crud registry rejected document")
	}
	return nil
}
