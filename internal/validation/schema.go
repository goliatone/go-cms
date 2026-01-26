package validation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

var (
	ErrSchemaInvalid    = errors.New("schema invalid")
	ErrSchemaValidation = errors.New("schema validation failed")
)

// ValidateSchema ensures the schema can be compiled.
func ValidateSchema(schema map[string]any) error {
	normalized := NormalizeSchema(schema)
	if normalized == nil {
		return nil
	}
	if _, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(normalized)); err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
	}
	return nil
}

// ValidatePayload validates payload against the provided schema.
func ValidatePayload(schema map[string]any, payload map[string]any) error {
	return validatePayloadWithSchema(NormalizeSchema(schema), payload)
}

// ValidatePartialPayload validates payload without enforcing required fields.
func ValidatePartialPayload(schema map[string]any, payload map[string]any) error {
	normalized := NormalizeSchema(schema)
	if normalized == nil {
		return nil
	}
	normalized = cloneMap(normalized)
	delete(normalized, "required")
	return validatePayloadWithSchema(normalized, payload)
}

func validatePayloadWithSchema(schema map[string]any, payload map[string]any) error {
	if schema == nil {
		return nil
	}
	if payload == nil {
		payload = map[string]any{}
	}
	result, err := gojsonschema.Validate(gojsonschema.NewGoLoader(schema), gojsonschema.NewGoLoader(payload))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaValidation, err)
	}
	if result.Valid() {
		return nil
	}
	var parts []string
	for _, err := range result.Errors() {
		parts = append(parts, err.String())
	}
	return fmt.Errorf("%w: %s", ErrSchemaValidation, strings.Join(parts, "; "))
}

// NormalizeSchema converts a schema definition into a JSON schema.
func NormalizeSchema(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return nil
	}
	if isJSONSchema(schema) {
		return cloneMap(schema)
	}
	fields, ok := schema["fields"]
	if !ok {
		return nil
	}
	properties, required := normalizeFields(fields)
	if len(properties) == 0 {
		return nil
	}
	normalized := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if override, ok := schema["additionalProperties"]; ok {
		if allowed, ok := override.(bool); ok {
			normalized["additionalProperties"] = allowed
		}
	}
	if len(required) > 0 {
		normalized["required"] = required
	}
	return normalized
}

func isJSONSchema(schema map[string]any) bool {
	if _, ok := schema["$schema"]; ok {
		return true
	}
	if _, ok := schema["type"]; ok {
		return true
	}
	if _, ok := schema["properties"]; ok {
		return true
	}
	if _, ok := schema["oneOf"]; ok {
		return true
	}
	if _, ok := schema["anyOf"]; ok {
		return true
	}
	if _, ok := schema["allOf"]; ok {
		return true
	}
	return false
}

func normalizeFields(fields any) (map[string]any, []string) {
	properties := make(map[string]any)
	required := make([]string, 0)

	switch typed := fields.(type) {
	case []any:
		for _, entry := range typed {
			if fieldMap, ok := entry.(map[string]any); ok {
				addField(properties, &required, fieldMap)
			}
		}
	case []map[string]any:
		for _, fieldMap := range typed {
			addField(properties, &required, fieldMap)
		}
	}

	return properties, required
}

func addField(properties map[string]any, required *[]string, field map[string]any) {
	if field == nil {
		return
	}
	name, _ := field["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if schema, ok := field["schema"].(map[string]any); ok {
		properties[name] = cloneMap(schema)
	} else if fieldType, ok := field["type"].(string); ok {
		if jsonType := normalizeJSONType(fieldType); jsonType != "" {
			properties[name] = map[string]any{"type": jsonType}
		} else {
			properties[name] = map[string]any{}
		}
	} else {
		properties[name] = map[string]any{}
	}
	if required != nil {
		if flag, ok := field["required"].(bool); ok && flag {
			*required = append(*required, name)
		}
	}
}

func normalizeJSONType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "string", "number", "integer", "boolean", "object", "array", "null":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = cloneMap(typed)
		case []any:
			out[key] = cloneSlice(typed)
		default:
			out[key] = value
		}
	}
	return out
}

func cloneSlice(input []any) []any {
	if input == nil {
		return nil
	}
	out := make([]any, len(input))
	for i, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			out[i] = cloneMap(typed)
		case []any:
			out[i] = cloneSlice(typed)
		default:
			out[i] = value
		}
	}
	return out
}
