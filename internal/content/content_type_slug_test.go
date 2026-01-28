package content

import "testing"

func TestDeriveContentTypeSlugPrefersExplicitSlug(t *testing.T) {
	ct := &ContentType{
		Name:   "Landing Page",
		Slug:   "Custom Slug",
		Schema: map[string]any{"metadata": map[string]any{"slug": "schema-slug"}},
	}
	got := DeriveContentTypeSlug(ct)
	if got != "custom-slug" {
		t.Fatalf("expected explicit slug to win, got %q", got)
	}
}

func TestDeriveContentTypeSlugUsesSchemaMetadata(t *testing.T) {
	ct := &ContentType{
		Name:   "Landing Page",
		Schema: map[string]any{"metadata": map[string]any{"slug": "Schema Slug"}},
	}
	got := DeriveContentTypeSlug(ct)
	if got != "schema-slug" {
		t.Fatalf("expected schema slug, got %q", got)
	}
}

func TestDeriveContentTypeSlugFallsBackToName(t *testing.T) {
	ct := &ContentType{
		Name: "Landing Page",
	}
	got := DeriveContentTypeSlug(ct)
	if got != "landing-page" {
		t.Fatalf("expected name slug, got %q", got)
	}
}

func TestExtractSchemaSlugReadsTopLevelAndMetadata(t *testing.T) {
	if got := extractSchemaSlug(map[string]any{"slug": "primary"}); got != "primary" {
		t.Fatalf("expected top-level slug, got %q", got)
	}
	if got := extractSchemaSlug(map[string]any{"metadata": map[string]any{"slug": "meta"}}); got != "meta" {
		t.Fatalf("expected metadata slug, got %q", got)
	}
}

func TestNormalizeContentTypeSlugHandlesEmpty(t *testing.T) {
	if got := normalizeContentTypeSlug(""); got != "" {
		t.Fatalf("expected empty normalization, got %q", got)
	}
}
