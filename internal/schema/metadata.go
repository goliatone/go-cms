package schema

import "strings"

const (
	metadataKey           = "metadata"
	metadataSlugKey       = "slug"
	metadataVersionKey    = "schema_version"
	metadataUIOverlaysKey = "ui_overlays"
)

// Metadata captures schema-level metadata persisted alongside JSON Schema docs.
type Metadata struct {
	Slug          string
	SchemaVersion string
	UIOverlays    []string
}

// ExtractMetadata reads the schema metadata object when present.
func ExtractMetadata(schema map[string]any) Metadata {
	meta := Metadata{}
	if schema == nil {
		return meta
	}
	raw, ok := schema[metadataKey].(map[string]any)
	if !ok || raw == nil {
		return meta
	}
	if slug, ok := raw[metadataSlugKey].(string); ok {
		meta.Slug = strings.TrimSpace(slug)
	}
	if version, ok := raw[metadataVersionKey].(string); ok {
		meta.SchemaVersion = strings.TrimSpace(version)
	}
	meta.UIOverlays = readStringList(raw[metadataUIOverlaysKey])
	return meta
}

// ApplyMetadata updates the schema metadata object with the provided fields.
func ApplyMetadata(schema map[string]any, meta Metadata) map[string]any {
	if schema == nil {
		return nil
	}
	out := cloneMap(schema)
	target, _ := out[metadataKey].(map[string]any)
	if target == nil {
		target = map[string]any{}
	}
	if strings.TrimSpace(meta.Slug) != "" {
		target[metadataSlugKey] = strings.TrimSpace(meta.Slug)
	}
	if strings.TrimSpace(meta.SchemaVersion) != "" {
		target[metadataVersionKey] = strings.TrimSpace(meta.SchemaVersion)
	}
	if len(meta.UIOverlays) > 0 {
		target[metadataUIOverlaysKey] = append([]string(nil), meta.UIOverlays...)
	}
	out[metadataKey] = target
	return out
}

func readStringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, entry := range typed {
			trimmed := strings.TrimSpace(entry)
			if trimmed == "" {
				continue
			}
			out = append(out, trimmed)
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, entry := range typed {
			switch item := entry.(type) {
			case string:
				trimmed := strings.TrimSpace(item)
				if trimmed != "" {
					out = append(out, trimmed)
				}
			case map[string]any:
				if ref, ok := item["ref"].(string); ok {
					trimmed := strings.TrimSpace(ref)
					if trimmed != "" {
						out = append(out, trimmed)
					}
				}
				if path, ok := item["path"].(string); ok {
					trimmed := strings.TrimSpace(path)
					if trimmed != "" {
						out = append(out, trimmed)
					}
				}
			}
		}
		return out
	default:
		return nil
	}
}
