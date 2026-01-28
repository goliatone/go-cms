package schema

import (
	"strconv"
	"strings"
)

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

func mergeMap(dest map[string]any, overlay map[string]any, override bool) map[string]any {
	if dest == nil {
		dest = map[string]any{}
	}
	if overlay == nil {
		return dest
	}
	for key, value := range overlay {
		if existing, ok := dest[key]; ok && !override {
			if existingMap, ok := existing.(map[string]any); ok {
				if valueMap, ok := value.(map[string]any); ok {
					dest[key] = mergeMap(existingMap, valueMap, override)
					continue
				}
			}
			continue
		}
		if valueMap, ok := value.(map[string]any); ok {
			dest[key] = cloneMap(valueMap)
			continue
		}
		if valueSlice, ok := value.([]any); ok {
			dest[key] = cloneSlice(valueSlice)
			continue
		}
		dest[key] = value
	}
	return dest
}

func pointerTokens(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, strconv.ErrSyntax
	}
	parts := strings.Split(pointer, "/")[1:]
	for i, part := range parts {
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")
		parts[i] = part
	}
	return parts, nil
}

func resolvePointer(root any, pointer string) (any, error) {
	tokens, err := pointerTokens(pointer)
	if err != nil {
		return nil, err
	}
	current := root
	for _, token := range tokens {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[token]
			if !ok {
				return nil, ErrOverlayPathNotFound
			}
			current = next
		case []any:
			idx, err := strconv.Atoi(token)
			if err != nil || idx < 0 || idx >= len(typed) {
				return nil, ErrOverlayPathNotFound
			}
			current = typed[idx]
		default:
			return nil, ErrOverlayPathNotFound
		}
	}
	return current, nil
}
