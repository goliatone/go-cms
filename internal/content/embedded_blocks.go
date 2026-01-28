package content

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	cmsschema "github.com/goliatone/go-cms/internal/schema"
	"github.com/google/uuid"
)

const (
	// EmbeddedBlocksKey is the payload key holding embedded blocks.
	EmbeddedBlocksKey = "blocks"
	// EmbeddedBlockTypeKey identifies the block type discriminator.
	EmbeddedBlockTypeKey = "_type"
	// EmbeddedBlockSchemaKey tracks the block schema version.
	EmbeddedBlockSchemaKey = "_schema"
	// EmbeddedBlockMetaKey stores system metadata alongside embedded blocks.
	EmbeddedBlockMetaKey = "_cms"
)

const (
	ConflictEmbeddedMissing = "embedded_missing"
	ConflictLegacyMissing   = "legacy_missing"
	ConflictTypeMismatch    = "type_mismatch"
	ConflictSchemaMismatch  = "schema_mismatch"
	ConflictContentMismatch = "content_mismatch"
	ConflictConfigMismatch  = "configuration_mismatch"
	ConflictAttrsMismatch   = "attribute_overrides_mismatch"
	ConflictMediaMismatch   = "media_bindings_mismatch"
)

// EmbeddedBlocksResolver bridges embedded blocks to legacy block instances.
type EmbeddedBlocksResolver interface {
	SyncEmbeddedBlocks(ctx context.Context, contentID uuid.UUID, translations []ContentTranslationInput, actor uuid.UUID) error
	MergeLegacyBlocks(ctx context.Context, record *Content) error
	MigrateEmbeddedBlocks(ctx context.Context, locale string, blocks []map[string]any) ([]map[string]any, error)
	ValidateEmbeddedBlocks(ctx context.Context, locale string, blocks []map[string]any, mode EmbeddedBlockValidationMode) error
	ValidateBlockAvailability(ctx context.Context, contentType string, availability cmsschema.BlockAvailability, blocks []map[string]any) error
}

// EmbeddedBlockValidationMode controls how strict validation should be.
type EmbeddedBlockValidationMode string

const (
	EmbeddedBlockValidationDraft  EmbeddedBlockValidationMode = "draft"
	EmbeddedBlockValidationStrict EmbeddedBlockValidationMode = "strict"
)

// EmbeddedBlockValidationIssue captures validation failures with block context.
type EmbeddedBlockValidationIssue struct {
	Locale  string `json:"locale,omitempty"`
	Index   int    `json:"index,omitempty"`
	Type    string `json:"type,omitempty"`
	Schema  string `json:"schema,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message,omitempty"`
}

// EmbeddedBlockValidationError aggregates embedded block validation failures.
type EmbeddedBlockValidationError struct {
	Mode   EmbeddedBlockValidationMode
	Issues []EmbeddedBlockValidationIssue
}

func (e *EmbeddedBlockValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "embedded block validation failed"
	}
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		scope := make([]string, 0, 4)
		if issue.Locale != "" {
			scope = append(scope, "locale="+issue.Locale)
		}
		if issue.Index >= 0 {
			scope = append(scope, fmt.Sprintf("block[%d]", issue.Index))
		}
		if issue.Type != "" {
			scope = append(scope, "type="+issue.Type)
		}
		if issue.Field != "" {
			scope = append(scope, "field="+issue.Field)
		}
		prefix := strings.Join(scope, " ")
		if prefix != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", prefix, issue.Message))
			continue
		}
		parts = append(parts, issue.Message)
	}
	return strings.Join(parts, "; ")
}

// ContentTranslationReader exposes translation lookups without replacing the core repository interface.
type ContentTranslationReader interface {
	ListTranslations(ctx context.Context, contentID uuid.UUID) ([]*ContentTranslation, error)
}

// EmbeddedBlockConflict describes mismatches between embedded and legacy block payloads.
type EmbeddedBlockConflict struct {
	ContentID        uuid.UUID      `json:"content_id"`
	PageID           uuid.UUID      `json:"page_id"`
	Locale           string         `json:"locale,omitempty"`
	Region           string         `json:"region,omitempty"`
	Index            int            `json:"index,omitempty"`
	Issue            string         `json:"issue"`
	EmbeddedType     string         `json:"embedded_type,omitempty"`
	LegacyType       string         `json:"legacy_type,omitempty"`
	EmbeddedSchema   string         `json:"embedded_schema,omitempty"`
	LegacySchema     string         `json:"legacy_schema,omitempty"`
	LegacyInstanceID uuid.UUID      `json:"legacy_instance_id,omitempty"`
	Details          map[string]any `json:"details,omitempty"`
}

// ExtractEmbeddedBlocks returns embedded block payloads from a translation content map.
func ExtractEmbeddedBlocks(payload map[string]any) ([]map[string]any, bool) {
	if payload == nil {
		return nil, false
	}
	raw, ok := payload[EmbeddedBlocksKey]
	if !ok || raw == nil {
		return nil, false
	}
	switch typed := raw.(type) {
	case []map[string]any:
		return typed, true
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, entry := range typed {
			block, ok := entry.(map[string]any)
			if !ok {
				return nil, false
			}
			out = append(out, block)
		}
		return out, true
	default:
		return nil, false
	}
}

// MergeEmbeddedBlocks applies a blocks slice to the content payload, returning a copy.
func MergeEmbeddedBlocks(payload map[string]any, blocks []map[string]any) map[string]any {
	merged := cloneMap(payload)
	if merged == nil {
		merged = map[string]any{}
	}
	if blocks == nil {
		delete(merged, EmbeddedBlocksKey)
		return merged
	}
	copied := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		copied = append(copied, cloneMap(block))
	}
	merged[EmbeddedBlocksKey] = copied
	return merged
}

// SanitizeEmbeddedBlocks removes system metadata keys from embedded blocks for validation.
func SanitizeEmbeddedBlocks(payload map[string]any) map[string]any {
	clean := deepCloneMap(payload)
	blocks, ok := ExtractEmbeddedBlocks(clean)
	if !ok {
		return clean
	}
	sanitized := make([]any, 0, len(blocks))
	for _, block := range blocks {
		copied := deepCloneMap(block)
		delete(copied, EmbeddedBlockMetaKey)
		sanitized = append(sanitized, copied)
	}
	clean[EmbeddedBlocksKey] = sanitized
	return clean
}

// NormalizeLocale trims and lowercases locale codes for comparison.
func NormalizeLocale(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func deepCloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	raw, err := json.Marshal(src)
	if err != nil {
		return cloneMap(src)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return cloneMap(src)
	}
	return out
}
