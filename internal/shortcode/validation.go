package shortcode

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

var (
	// ErrUnknownParameter indicates the request supplied an unexpected parameter.
	ErrUnknownParameter = errors.New("shortcode: unknown parameter")
	// ErrMissingParameter indicates a required parameter was not provided.
	ErrMissingParameter = errors.New("shortcode: missing required parameter")
	// ErrParameterType indicates a parameter could not be coerced to the requested type.
	ErrParameterType = errors.New("shortcode: parameter type mismatch")
)

// Validator performs definition and parameter validation.
type Validator struct{}

// NewValidator returns a Validator instance.
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateDefinition ensures the definition contains a name, schema, and valid parameter definitions.
func (v *Validator) ValidateDefinition(def interfaces.ShortcodeDefinition) error {
	if strings.TrimSpace(def.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidDefinition)
	}

	if err := validateSchema(def.Schema); err != nil {
		return err
	}

	return nil
}

func validateSchema(schema interfaces.ShortcodeSchema) error {
	seen := make(map[string]struct{})
	for _, param := range schema.Params {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			return fmt.Errorf("%w: schema parameter name required", ErrInvalidDefinition)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("%w: duplicate schema parameter %q", ErrInvalidDefinition, name)
		}
		seen[name] = struct{}{}

		switch param.Type {
		case interfaces.ShortcodeParamString,
			interfaces.ShortcodeParamInt,
			interfaces.ShortcodeParamBool,
			interfaces.ShortcodeParamArray,
			interfaces.ShortcodeParamURL:
			// Valid types
		default:
			return fmt.Errorf("%w: parameter %q unknown type %q", ErrInvalidDefinition, name, param.Type)
		}
	}
	return nil
}

// CoerceParams validates user supplied parameters against the definition schema, returning a normalised map.
func (v *Validator) CoerceParams(def interfaces.ShortcodeDefinition, supplied map[string]any) (map[string]any, error) {
	if err := v.ValidateDefinition(def); err != nil {
		return nil, err
	}

	out := make(map[string]any, len(def.Schema.Params))
	allowed := make(map[string]interfaces.ShortcodeParam, len(def.Schema.Params))
	for _, param := range def.Schema.Params {
		allowed[param.Name] = param
		if def.Schema.Defaults != nil {
			if value, ok := def.Schema.Defaults[param.Name]; ok {
				out[param.Name] = value
			}
		} else if param.Default != nil {
			out[param.Name] = param.Default
		}
	}

	for key, value := range supplied {
		param, ok := allowed[key]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownParameter, key)
		}
		coerced, err := coerceValue(param.Type, value)
		if err != nil {
			return nil, fmt.Errorf("%w: %s %v", ErrParameterType, key, err)
		}
		if param.Validate != nil {
			if err := param.Validate(coerced); err != nil {
				return nil, err
			}
		}
		out[key] = coerced
	}

	for _, param := range def.Schema.Params {
		if param.Required {
			if _, ok := out[param.Name]; !ok {
				return nil, fmt.Errorf("%w: %s", ErrMissingParameter, param.Name)
			}
		}
	}

	return out, nil
}

func coerceValue(paramType interfaces.ShortcodeParamType, value any) (any, error) {
	switch paramType {
	case interfaces.ShortcodeParamString:
		return fmt.Sprintf("%v", value), nil
	case interfaces.ShortcodeParamInt:
		return coerceInt(value)
	case interfaces.ShortcodeParamBool:
		return coerceBool(value)
	case interfaces.ShortcodeParamArray:
		return coerceArray(value)
	case interfaces.ShortcodeParamURL:
		urlStr, err := coerceString(value)
		if err != nil {
			return nil, err
		}
		if _, err := url.ParseRequestURI(urlStr); err != nil {
			return nil, err
		}
		return urlStr, nil
	default:
		return nil, fmt.Errorf("unsupported parameter type %q", paramType)
	}
}

func coerceString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return fmt.Sprintf("%v", value), nil
	}
}

func coerceInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint, uint8, uint16, uint32, uint64:
		rv := reflect.ValueOf(v)
		return int(rv.Uint()), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, err
		}
		return i, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

func coerceBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "t", "yes", "y", "on":
			return true, nil
		case "0", "false", "f", "no", "n", "off":
			return false, nil
		default:
			return false, fmt.Errorf("cannot convert %q to bool", v)
		}
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

func coerceArray(value any) ([]any, error) {
	switch v := value.(type) {
	case []any:
		return v, nil
	case []string:
		out := make([]any, len(v))
		for i, s := range v {
			out[i] = s
		}
		return out, nil
	case string:
		parts := strings.Split(v, ",")
		out := make([]any, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out, nil
	default:
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			out := make([]any, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				out[i] = rv.Index(i).Interface()
			}
			return out, nil
		}
		return nil, fmt.Errorf("cannot convert %T to array", value)
	}
}
