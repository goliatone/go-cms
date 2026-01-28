package schema

import (
	"fmt"
	"strings"
)

var allowedKeywords = map[string]struct{}{
	"$schema":              {},
	"$id":                  {},
	"$ref":                 {},
	"$defs":                {},
	"$anchor":              {},
	"metadata":             {},
	"ui":                   {},
	"type":                 {},
	"properties":           {},
	"required":             {},
	"items":                {},
	"oneOf":                {},
	"allOf":                {},
	"const":                {},
	"enum":                 {},
	"default":              {},
	"title":                {},
	"description":          {},
	"format":               {},
	"additionalProperties": {},
}

// ValidateSchemaSubset ensures the schema only uses supported keywords.
func ValidateSchemaSubset(schema map[string]any) error {
	return validateSchemaNode(schema, "")
}

func validateSchemaNode(node map[string]any, path string) error {
	if node == nil {
		return nil
	}
	for key, value := range node {
		if strings.HasPrefix(key, "x-") {
			continue
		}
		if _, ok := allowedKeywords[key]; !ok {
			return fmt.Errorf("%w: %s at %s", ErrUnsupportedKeyword, key, path)
		}

		switch key {
		case "metadata", "ui":
			continue
		case "properties":
			props, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("%w: properties at %s", ErrUnsupportedKeyword, path)
			}
			for name, child := range props {
				childSchema, ok := child.(map[string]any)
				if !ok {
					return fmt.Errorf("%w: properties/%s at %s", ErrUnsupportedKeyword, name, path)
				}
				if err := validateSchemaNode(childSchema, joinPath(path, "properties", name)); err != nil {
					return err
				}
			}
		case "items":
			switch typed := value.(type) {
			case map[string]any:
				if err := validateSchemaNode(typed, joinPath(path, "items")); err != nil {
					return err
				}
			default:
				return fmt.Errorf("%w: items at %s", ErrUnsupportedKeyword, path)
			}
		case "oneOf":
			if err := validateSchemaArray(value, joinPath(path, "oneOf")); err != nil {
				return err
			}
		case "allOf":
			if err := validateAllOf(value, joinPath(path, "allOf")); err != nil {
				return err
			}
		case "$defs":
			if defs, ok := value.(map[string]any); ok {
				for name, child := range defs {
					childSchema, ok := child.(map[string]any)
					if !ok {
						return fmt.Errorf("%w: $defs/%s at %s", ErrUnsupportedKeyword, name, path)
					}
					if err := validateSchemaNode(childSchema, joinPath(path, "$defs", name)); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func validateSchemaArray(value any, path string) error {
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedKeyword, path)
	}
	for idx, entry := range items {
		child, ok := entry.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: %s/%d", ErrUnsupportedKeyword, path, idx)
		}
		if err := validateSchemaNode(child, fmt.Sprintf("%s/%d", path, idx)); err != nil {
			return err
		}
	}
	return nil
}

func validateAllOf(value any, path string) error {
	items, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedKeyword, path)
	}
	for idx, entry := range items {
		child, ok := entry.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: %s/%d", ErrUnsupportedKeyword, path, idx)
		}
		if err := validateAllOfEntry(child, fmt.Sprintf("%s/%d", path, idx)); err != nil {
			return err
		}
	}
	return nil
}

func validateAllOfEntry(entry map[string]any, path string) error {
	for key := range entry {
		if strings.HasPrefix(key, "x-") {
			continue
		}
		switch key {
		case "properties", "required", "additionalProperties", "title", "description":
			continue
		default:
			return fmt.Errorf("%w: %s at %s", ErrUnsupportedKeyword, key, path)
		}
	}
	if err := validateSchemaNode(entry, path); err != nil {
		return err
	}
	if t, ok := entry["type"].(string); ok && t != "object" {
		return fmt.Errorf("%w: allOf type at %s", ErrUnsupportedKeyword, path)
	}
	return nil
}

func joinPath(parts ...string) string {
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		trimmed = append(trimmed, part)
	}
	if len(trimmed) == 0 {
		return ""
	}
	return strings.Join(trimmed, "/")
}
