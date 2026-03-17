package util

import "maps"

// FirstNonEmpty returns the first non-empty string in values.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// CloneStringMap returns a shallow copy of input.
// It returns a non-nil map even when input is nil.
func CloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	maps.Copy(out, input)
	return out
}

// CloneAnyMap returns a shallow copy of supported raw map types.
// Unsupported inputs yield an empty map.
func CloneAnyMap(raw any) map[string]any {
	result := make(map[string]any)
	switch values := raw.(type) {
	case map[string]any:
		maps.Copy(result, values)
	case map[string]string:
		for k, v := range values {
			result[k] = v
		}
	}
	return result
}
