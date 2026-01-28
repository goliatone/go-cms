package schema

import "strings"

const (
	metadataKey           = "metadata"
	metadataSlugKey       = "slug"
	metadataVersionKey    = "schema_version"
	metadataUIOverlaysKey = "ui_overlays"
	metadataBlockAvailKey = "block_availability"
)

// Metadata captures schema-level metadata persisted alongside JSON Schema docs.
type Metadata struct {
	Slug              string
	SchemaVersion     string
	UIOverlays        []string
	BlockAvailability BlockAvailability
}

// BlockAvailability defines allow/deny rules for block types.
type BlockAvailability struct {
	Allow []string
	Deny  []string
}

func (b BlockAvailability) Empty() bool {
	return len(b.Allow) == 0 && len(b.Deny) == 0
}

func (b BlockAvailability) Allows(value string) bool {
	candidate := normalizeAvailabilityToken(value)
	if candidate == "" {
		return false
	}
	for _, entry := range b.Deny {
		if normalizeAvailabilityToken(entry) == candidate {
			return false
		}
	}
	if len(b.Allow) == 0 {
		return true
	}
	for _, entry := range b.Allow {
		if normalizeAvailabilityToken(entry) == candidate {
			return true
		}
	}
	return false
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
	meta.BlockAvailability = readBlockAvailability(raw[metadataBlockAvailKey])
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
	if !meta.BlockAvailability.Empty() {
		availability := map[string]any{}
		if len(meta.BlockAvailability.Allow) > 0 {
			availability["allow"] = append([]string(nil), meta.BlockAvailability.Allow...)
		}
		if len(meta.BlockAvailability.Deny) > 0 {
			availability["deny"] = append([]string(nil), meta.BlockAvailability.Deny...)
		}
		if len(availability) > 0 {
			target[metadataBlockAvailKey] = availability
		}
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

func readBlockAvailability(value any) BlockAvailability {
	availability := BlockAvailability{}
	if value == nil {
		return availability
	}
	if list := readStringList(value); len(list) > 0 {
		availability.Allow = normalizeAvailabilityList(list)
		return availability
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return availability
	}
	allow := readStringList(raw["allow"])
	if len(allow) == 0 {
		allow = readStringList(raw["allowed"])
	}
	deny := readStringList(raw["deny"])
	if len(deny) == 0 {
		deny = readStringList(raw["denied"])
	}
	availability.Allow = normalizeAvailabilityList(allow)
	availability.Deny = normalizeAvailabilityList(deny)
	return availability
}

func normalizeAvailabilityList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func normalizeAvailabilityToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
