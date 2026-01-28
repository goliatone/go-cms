package schema

import "strings"

// DownlevelForValidation converts Draft 2020-12 constructs to Draft-07 equivalents.
func DownlevelForValidation(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	clone := cloneMap(schema)
	return downlevelNode(clone).(map[string]any)
}

func downlevelNode(node any) any {
	switch typed := node.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, value := range typed {
			switch key {
			case "$defs":
				out["definitions"] = downlevelNode(value)
			case "const":
				if _, ok := typed["enum"]; !ok {
					out["enum"] = []any{value}
				}
			default:
				out[key] = downlevelNode(value)
			}
		}
		if ref, ok := out["$ref"].(string); ok {
			out["$ref"] = strings.ReplaceAll(ref, "#/$defs/", "#/definitions/")
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, entry := range typed {
			out[i] = downlevelNode(entry)
		}
		return out
	default:
		return node
	}
}
