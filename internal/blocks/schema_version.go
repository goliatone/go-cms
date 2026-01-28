package blocks

import (
	"strings"

	cmsschema "github.com/goliatone/go-cms/internal/schema"
)

func resolveDefinitionSchemaVersion(schema map[string]any, slug string) (cmsschema.Version, error) {
	if len(schema) == 0 {
		if strings.TrimSpace(slug) == "" {
			return cmsschema.Version{}, cmsschema.ErrInvalidSchemaVersion
		}
		return cmsschema.DefaultVersion(slug), nil
	}
	_, version, err := cmsschema.EnsureSchemaVersion(schema, slug)
	return version, err
}

func stripSchemaVersion(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	clean := cloneMap(payload)
	delete(clean, cmsschema.RootSchemaKey)
	return clean
}

func applySchemaVersion(payload map[string]any, version cmsschema.Version) map[string]any {
	result := cloneMap(payload)
	if result == nil {
		result = map[string]any{}
	}
	result[cmsschema.RootSchemaKey] = version.String()
	return result
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
