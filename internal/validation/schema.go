package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	cmsschema "github.com/goliatone/go-cms/internal/schema"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

var (
	ErrSchemaInvalid    = errors.New("schema invalid")
	ErrSchemaValidation = errors.New("schema validation failed")
	ErrSchemaMigration  = errors.New("schema migration invalid")
)

// ValidationIssue captures a single validation failure.
type ValidationIssue struct {
	Location string
	Message  string
}

// PayloadValidationError surfaces validation issues with schema-aware context.
type PayloadValidationError struct {
	Issues []ValidationIssue
	Cause  error
}

func (e *PayloadValidationError) Error() string {
	if len(e.Issues) == 0 {
		if e.Cause != nil {
			return e.Cause.Error()
		}
		return ErrSchemaValidation.Error()
	}
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		location := strings.TrimSpace(issue.Location)
		if location == "" {
			location = "#"
		} else if !strings.HasPrefix(location, "#") {
			location = "#" + location
		}
		if issue.Message == "" {
			parts = append(parts, location)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", location, issue.Message))
	}
	return strings.Join(parts, "; ")
}

func (e *PayloadValidationError) Unwrap() error {
	return ErrSchemaValidation
}

// Issues extracts validation issues from an error.
func Issues(err error) []ValidationIssue {
	if err == nil {
		return nil
	}
	var payloadErr *PayloadValidationError
	if errors.As(err, &payloadErr) && payloadErr != nil {
		return payloadErr.Issues
	}
	var validationErr *jsonschema.ValidationError
	if errors.As(err, &validationErr) && validationErr != nil {
		return collectValidationIssues(validationErr)
	}
	return []ValidationIssue{{Message: err.Error()}}
}

// ValidateSchema ensures the schema can be compiled.
func ValidateSchema(schema map[string]any) error {
	normalized := NormalizeSchema(schema)
	if normalized == nil {
		return nil
	}
	if err := validateSchemaSubset(normalized); err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
	}
	if _, err := compileSchema(normalized); err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaInvalid, err)
	}
	return nil
}

// ValidatePayload validates payload against the provided schema.
func ValidatePayload(schema map[string]any, payload map[string]any) error {
	return validatePayloadWithSchema(NormalizeSchema(schema), payload)
}

// ValidateMigrationPayload validates migrated payloads against the target schema.
func ValidateMigrationPayload(schema map[string]any, payload map[string]any) error {
	if err := ValidatePayload(schema, payload); err != nil {
		return fmt.Errorf("%w: %s", ErrSchemaMigration, err)
	}
	return nil
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
	if err := validateSchemaSubset(schema); err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaValidation, err)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	compiled, err := compileSchema(schema)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSchemaValidation, err)
	}
	if err := compiled.Validate(payload); err != nil {
		issues := Issues(err)
		return &PayloadValidationError{
			Issues: issues,
			Cause:  err,
		}
	}
	return nil
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
				continue
			}
			if name, ok := entry.(string); ok {
				addField(properties, &required, map[string]any{"name": name})
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

func validateSchemaSubset(schema map[string]any) error {
	if schema == nil || !isJSONSchema(schema) {
		return nil
	}
	return cmsschema.ValidateSchemaSubset(schema)
}

func compileSchema(schema map[string]any) (*jsonschema.Schema, error) {
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	if err := compiler.AddResource("schema.json", bytes.NewReader(encoded)); err != nil {
		return nil, err
	}
	return compiler.Compile("schema.json")
}

func collectValidationIssues(err *jsonschema.ValidationError) []ValidationIssue {
	if err == nil {
		return nil
	}
	issues := []ValidationIssue{}
	var walk func(*jsonschema.ValidationError)
	walk = func(node *jsonschema.ValidationError) {
		if node == nil {
			return
		}
		if len(node.Causes) == 0 {
			issues = append(issues, ValidationIssue{
				Location: strings.TrimSpace(node.InstanceLocation),
				Message:  strings.TrimSpace(node.Message),
			})
			return
		}
		for _, cause := range node.Causes {
			walk(cause)
		}
	}
	walk(err)
	return issues
}
