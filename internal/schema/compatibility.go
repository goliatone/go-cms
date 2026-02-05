package schema

import (
	"reflect"
	"sort"
	"strings"
)

// ChangeLevel captures the semantic impact of a schema update.
type ChangeLevel int

const (
	ChangeNone ChangeLevel = iota
	ChangePatch
	ChangeMinor
	ChangeMajor
)

func (c ChangeLevel) String() string {
	switch c {
	case ChangePatch:
		return "patch"
	case ChangeMinor:
		return "minor"
	case ChangeMajor:
		return "major"
	default:
		return "none"
	}
}

// CompatibilityResult summarizes compatibility of a schema change.
type CompatibilityResult struct {
	Compatible      bool
	ChangeLevel     ChangeLevel
	BreakingChanges []BreakingChange
	Warnings        []string
}

// BreakingChange describes a breaking schema update.
type BreakingChange struct {
	Type        string
	Field       string
	Description string
}

// CheckSchemaCompatibility compares schema changes for breaking updates.
func CheckSchemaCompatibility(oldSchema, newSchema map[string]any) CompatibilityResult {
	result := CompatibilityResult{Compatible: true, ChangeLevel: ChangeNone}
	oldNormalized := normalizeCompatibilitySchema(oldSchema)
	newNormalized := normalizeCompatibilitySchema(newSchema)

	oldFields := collectSchemaFields(oldNormalized)
	newFields := collectSchemaFields(newNormalized)

	hasMinor := false
	for path, oldField := range oldFields {
		newField, ok := newFields[path]
		if !ok {
			result.BreakingChanges = append(result.BreakingChanges, BreakingChange{
				Type:        "field_removed",
				Field:       path,
				Description: "field removed",
			})
			continue
		}
		switch compareTypeInfo(oldField.Type, newField.Type) {
		case typeChangeBreaking:
			result.BreakingChanges = append(result.BreakingChanges, BreakingChange{
				Type:        "type_changed",
				Field:       path,
				Description: "field type changed",
			})
		case typeChangeMinor:
			hasMinor = true
		}
		if !oldField.Required && newField.Required {
			result.BreakingChanges = append(result.BreakingChanges, BreakingChange{
				Type:        "required_added",
				Field:       path,
				Description: "required field added",
			})
		}
		if oldField.Required && !newField.Required {
			hasMinor = true
		}
	}

	for path, newField := range newFields {
		if _, ok := oldFields[path]; ok {
			continue
		}
		if newField.Required {
			result.BreakingChanges = append(result.BreakingChanges, BreakingChange{
				Type:        "required_added",
				Field:       path,
				Description: "required field added",
			})
			continue
		}
		hasMinor = true
	}

	changed := !reflect.DeepEqual(stripSchemaVersionMetadata(oldSchema), stripSchemaVersionMetadata(newSchema))
	if len(result.BreakingChanges) > 0 {
		result.Compatible = false
		result.ChangeLevel = ChangeMajor
		return result
	}
	if hasMinor {
		result.ChangeLevel = ChangeMinor
		return result
	}
	if changed {
		result.ChangeLevel = ChangePatch
	}
	return result
}

type fieldDescriptor struct {
	Type     typeInfo
	Required bool
}

type typeChange int

const (
	typeChangeNone typeChange = iota
	typeChangeMinor
	typeChangeBreaking
)

type typeInfo struct {
	kind      string
	scalars   map[string]struct{}
	items     *typeInfo
	signature string
}

func collectSchemaFields(schema map[string]any) map[string]fieldDescriptor {
	fields := map[string]fieldDescriptor{}
	walkSchemaFields(schema, "", fields)
	return fields
}

// FieldPaths returns the set of schema field paths derived from a JSON schema.
// The schema should be normalized (see validation.NormalizeSchema) before use.
func FieldPaths(schema map[string]any) map[string]struct{} {
	if schema == nil {
		return nil
	}
	fields := collectSchemaFields(schema)
	if len(fields) == 0 {
		return nil
	}
	paths := make(map[string]struct{}, len(fields))
	for path := range fields {
		paths[path] = struct{}{}
	}
	return paths
}

func walkSchemaFields(node map[string]any, prefix string, fields map[string]fieldDescriptor) {
	if node == nil {
		return
	}
	required := requiredSet(node["required"])
	if props, ok := node["properties"].(map[string]any); ok {
		for name, raw := range props {
			child, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			path := joinFieldPath(prefix, name)
			fields[path] = fieldDescriptor{
				Type:     parseTypeInfo(child),
				Required: required[name],
			}
			walkSchemaFields(child, path, fields)
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		itemPath := prefix
		if itemPath == "" {
			itemPath = "[]"
		} else {
			itemPath = itemPath + "[]"
		}
		walkSchemaFields(items, itemPath, fields)
	}
	if oneOf, ok := node["oneOf"].([]any); ok {
		for idx, entry := range oneOf {
			child, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			walkSchemaFields(child, joinFieldPath(prefix, "oneOf", idx), fields)
		}
	}
	if allOf, ok := node["allOf"].([]any); ok {
		for idx, entry := range allOf {
			child, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			walkSchemaFields(child, joinFieldPath(prefix, "allOf", idx), fields)
		}
	}
	if defs, ok := node["$defs"].(map[string]any); ok {
		for name, entry := range defs {
			child, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			walkSchemaFields(child, joinFieldPath(prefix, "$defs", name), fields)
		}
	}
}

func requiredSet(value any) map[string]bool {
	set := map[string]bool{}
	switch typed := value.(type) {
	case []string:
		for _, name := range typed {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			set[name] = true
		}
	case []any:
		for _, entry := range typed {
			name, ok := entry.(string)
			if !ok {
				continue
			}
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			set[name] = true
		}
	}
	return set
}

func parseTypeInfo(node map[string]any) typeInfo {
	if node == nil {
		return typeInfo{kind: "unknown"}
	}
	types := readTypeList(node["type"])
	if len(types) > 0 {
		containsObject := containsType(types, "object")
		containsArray := containsType(types, "array")
		if containsObject || containsArray {
			if len(types) > 1 {
				return typeInfo{kind: "unknown", signature: "type:" + strings.Join(types, "|")}
			}
			if containsArray {
				items, _ := node["items"].(map[string]any)
				info := typeInfo{kind: "array"}
				if items != nil {
					itemInfo := parseTypeInfo(items)
					info.items = &itemInfo
				}
				return info
			}
			return typeInfo{kind: "object"}
		}
		return typeInfo{kind: "scalar", scalars: toSet(types)}
	}

	if info, ok := typeInfoFromConst(node["const"]); ok {
		return info
	}
	if info, ok := typeInfoFromEnum(node["enum"]); ok {
		return info
	}

	if props, ok := node["properties"].(map[string]any); ok && len(props) > 0 {
		return typeInfo{kind: "object"}
	}
	if items, ok := node["items"].(map[string]any); ok {
		info := typeInfo{kind: "array"}
		itemInfo := parseTypeInfo(items)
		info.items = &itemInfo
		return info
	}
	if oneOf, ok := node["oneOf"].([]any); ok {
		union := map[string]struct{}{}
		for _, entry := range oneOf {
			child, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			childInfo := parseTypeInfo(child)
			if childInfo.kind != "scalar" {
				return typeInfo{kind: "unknown", signature: "oneOf"}
			}
			for scalar := range childInfo.scalars {
				union[scalar] = struct{}{}
			}
		}
		if len(union) > 0 {
			return typeInfo{kind: "scalar", scalars: union}
		}
		return typeInfo{kind: "unknown", signature: "oneOf"}
	}
	if allOf, ok := node["allOf"].([]any); ok && len(allOf) > 0 {
		return typeInfo{kind: "unknown", signature: "allOf"}
	}

	return typeInfo{kind: "unknown"}
}

func typeInfoFromConst(value any) (typeInfo, bool) {
	if value == nil {
		return typeInfo{}, false
	}
	if kind := kindFromValue(value); kind != "" {
		return typeInfo{kind: "scalar", scalars: toSet([]string{kind})}, true
	}
	return typeInfo{}, false
}

func typeInfoFromEnum(value any) (typeInfo, bool) {
	if value == nil {
		return typeInfo{}, false
	}
	switch typed := value.(type) {
	case []any:
		types := make(map[string]struct{})
		for _, entry := range typed {
			if kind := kindFromValue(entry); kind != "" {
				types[kind] = struct{}{}
			}
		}
		if len(types) == 0 {
			return typeInfo{}, false
		}
		return typeInfo{kind: "scalar", scalars: types}, true
	case []string:
		if len(typed) == 0 {
			return typeInfo{}, false
		}
		return typeInfo{kind: "scalar", scalars: toSet([]string{"string"})}, true
	}
	return typeInfo{}, false
}

func kindFromValue(value any) string {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return ""
		}
		return "string"
	case bool:
		return "boolean"
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	default:
		return ""
	}
}

func compareTypeInfo(oldInfo, newInfo typeInfo) typeChange {
	if oldInfo.kind == "" && newInfo.kind == "" {
		return typeChangeNone
	}
	if oldInfo.kind != newInfo.kind {
		if oldInfo.kind == "scalar" && newInfo.kind == "scalar" {
			return compareScalarSets(oldInfo.scalars, newInfo.scalars)
		}
		return typeChangeBreaking
	}
	switch oldInfo.kind {
	case "scalar":
		return compareScalarSets(oldInfo.scalars, newInfo.scalars)
	case "array":
		if oldInfo.items == nil && newInfo.items == nil {
			return typeChangeNone
		}
		if oldInfo.items == nil && newInfo.items != nil {
			return typeChangeBreaking
		}
		if oldInfo.items != nil && newInfo.items == nil {
			return typeChangeMinor
		}
		return compareTypeInfo(*oldInfo.items, *newInfo.items)
	case "object":
		return typeChangeNone
	default:
		if oldInfo.signature == "" && newInfo.signature == "" {
			return typeChangeNone
		}
		if oldInfo.signature != "" || newInfo.signature != "" {
			if oldInfo.signature == newInfo.signature {
				return typeChangeNone
			}
		}
		return typeChangeBreaking
	}
}

func compareScalarSets(oldSet, newSet map[string]struct{}) typeChange {
	if len(oldSet) == 0 && len(newSet) == 0 {
		return typeChangeNone
	}
	if len(oldSet) == 0 || len(newSet) == 0 {
		return typeChangeBreaking
	}
	if isSuperset(newSet, oldSet) {
		if len(newSet) == len(oldSet) {
			return typeChangeNone
		}
		return typeChangeMinor
	}
	return typeChangeBreaking
}

func normalizeCompatibilitySchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	if isJSONSchema(schema) {
		return cloneMap(schema)
	}
	fields, ok := schema["fields"]
	if !ok {
		return cloneMap(schema)
	}
	props, required := normalizeFields(fields)
	normalized := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		normalized["required"] = required
	}
	if override, ok := schema["additionalProperties"]; ok {
		if allowed, ok := override.(bool); ok {
			normalized["additionalProperties"] = allowed
		}
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

func readTypeList(value any) []string {
	switch typed := value.(type) {
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(typed))
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	case []string:
		out := make([]string, 0, len(typed))
		for _, entry := range typed {
			trimmed := strings.ToLower(strings.TrimSpace(entry))
			if trimmed == "" {
				continue
			}
			out = append(out, trimmed)
		}
		sort.Strings(out)
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, entry := range typed {
			if name, ok := entry.(string); ok {
				trimmed := strings.ToLower(strings.TrimSpace(name))
				if trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		sort.Strings(out)
		return out
	default:
		return nil
	}
}

func toSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func containsType(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isSuperset(superset, subset map[string]struct{}) bool {
	for value := range subset {
		if _, ok := superset[value]; !ok {
			return false
		}
	}
	return true
}

func stripSchemaVersionMetadata(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	clean := cloneMap(schema)
	meta, ok := clean[metadataKey].(map[string]any)
	if !ok || meta == nil {
		return clean
	}
	metaCopy := cloneMap(meta)
	delete(metaCopy, metadataVersionKey)
	delete(metaCopy, metadataSlugKey)
	if len(metaCopy) == 0 {
		delete(clean, metadataKey)
		return clean
	}
	clean[metadataKey] = metaCopy
	return clean
}

func joinFieldPath(parts ...any) string {
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		switch value := part.(type) {
		case string:
			if value == "" {
				continue
			}
			segments = append(segments, value)
		case int:
			segments = append(segments, "["+intToString(value)+"]")
		}
	}
	return strings.Join(segments, ".")
}

func intToString(value int) string {
	if value == 0 {
		return "0"
	}
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	var digits [20]byte
	idx := len(digits)
	for value > 0 {
		idx--
		digits[idx] = byte('0' + value%10)
		value /= 10
	}
	return sign + string(digits[idx:])
}
