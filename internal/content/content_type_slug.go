package content

import "strings"

// DeriveContentTypeSlug derives a deterministic slug for backfill/seed flows.
// Prefers explicit slug, then schema metadata, then normalized name.
func DeriveContentTypeSlug(ct *ContentType) string {
	if ct == nil {
		return ""
	}
	if slug := strings.TrimSpace(ct.Slug); slug != "" {
		return slug
	}
	if slug := strings.TrimSpace(extractSchemaSlug(ct.Schema)); slug != "" {
		return slug
	}
	return normalizeContentTypeSlug(strings.TrimSpace(ct.Name))
}

func extractSchemaSlug(schema map[string]any) string {
	if schema == nil {
		return ""
	}
	if value, ok := schema["slug"]; ok {
		if slug, ok := value.(string); ok {
			return slug
		}
	}
	if meta, ok := schema["metadata"].(map[string]any); ok {
		if value, ok := meta["slug"]; ok {
			if slug, ok := value.(string); ok {
				return slug
			}
		}
	}
	return ""
}

func normalizeContentTypeSlug(value string) string {
	if value == "" {
		return ""
	}
	return strings.ReplaceAll(strings.ToLower(value), " ", "-")
}
