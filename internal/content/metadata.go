package content

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
)

const (
	entryFieldParentID   = "parent_id"
	entryFieldTemplateID = "template_id"
	entryFieldPath       = "path"
	entryFieldSortOrder  = "sort_order"
	entryFieldOrder      = "order"
)

const (
	maxIntValue = int(^uint(0) >> 1)
	minIntValue = -maxIntValue - 1
)

func normalizeEntryMetadata(metadata map[string]any) (map[string]any, error) {
	if metadata == nil {
		return nil, nil
	}
	normalized := cloneMap(metadata)
	if err := normalizeUUIDField(normalized, entryFieldParentID); err != nil {
		return nil, err
	}
	if err := normalizeUUIDField(normalized, entryFieldTemplateID); err != nil {
		return nil, err
	}
	if err := normalizePathField(normalized); err != nil {
		return nil, err
	}
	if err := normalizeSortOrderField(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func normalizeUUIDField(metadata map[string]any, key string) error {
	value, ok := metadata[key]
	if !ok {
		return nil
	}
	if value == nil {
		delete(metadata, key)
		return nil
	}
	switch typed := value.(type) {
	case uuid.UUID:
		metadata[key] = typed.String()
		return nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			delete(metadata, key)
			return nil
		}
		parsed, err := uuid.Parse(trimmed)
		if err != nil {
			return fmt.Errorf("%w: %s must be a valid UUID", ErrContentMetadataInvalid, key)
		}
		metadata[key] = parsed.String()
		return nil
	default:
		return fmt.Errorf("%w: %s must be a valid UUID", ErrContentMetadataInvalid, key)
	}
}

func normalizePathField(metadata map[string]any) error {
	value, ok := metadata[entryFieldPath]
	if !ok {
		return nil
	}
	if value == nil {
		delete(metadata, entryFieldPath)
		return nil
	}
	path, ok := value.(string)
	if !ok {
		return fmt.Errorf("%w: %s must be a string", ErrContentMetadataInvalid, entryFieldPath)
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("%w: %s cannot be empty", ErrContentMetadataInvalid, entryFieldPath)
	}
	metadata[entryFieldPath] = trimmed
	return nil
}

func normalizeSortOrderField(metadata map[string]any) error {
	if value, ok := metadata[entryFieldOrder]; ok {
		if _, exists := metadata[entryFieldSortOrder]; !exists {
			metadata[entryFieldSortOrder] = value
		}
		delete(metadata, entryFieldOrder)
	}
	value, ok := metadata[entryFieldSortOrder]
	if !ok {
		return nil
	}
	if value == nil {
		delete(metadata, entryFieldSortOrder)
		return nil
	}
	normalized, ok := normalizeIntValue(value)
	if !ok {
		return fmt.Errorf("%w: %s must be an integer", ErrContentMetadataInvalid, entryFieldSortOrder)
	}
	metadata[entryFieldSortOrder] = normalized
	return nil
}

func normalizeIntValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		if typed > uint64(maxIntValue) {
			return 0, false
		}
		return int(typed), true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		if math.Mod(typed, 1) != 0 {
			return 0, false
		}
		return int(typed), true
	case float32:
		value := float64(typed)
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
		if math.Mod(value, 1) != 0 {
			return 0, false
		}
		return int(value), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		if parsed > int64(maxIntValue) || parsed < int64(minIntValue) {
			return 0, false
		}
		return int(parsed), true
	default:
		return 0, false
	}
}
