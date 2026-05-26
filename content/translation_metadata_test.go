package content

import (
	"encoding/json"
	"testing"
)

func TestTranslationMetadataValueUsesSQLNullForAbsentMetadata(t *testing.T) {
	var metadata TranslationMetadata
	value, err := metadata.Value()
	if err != nil {
		t.Fatalf("value nil metadata: %v", err)
	}
	if value != nil {
		t.Fatalf("expected SQL NULL for nil metadata, got %v", value)
	}
}

func TestTranslationMetadataValuePreservesEmptyObjectWhenPresent(t *testing.T) {
	metadata := TranslationMetadata{}
	value, err := metadata.Value()
	if err != nil {
		t.Fatalf("value empty metadata: %v", err)
	}
	text, ok := value.(string)
	if !ok {
		t.Fatalf("expected driver value string, got %T", value)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("unmarshal metadata value: %v", err)
	}
	if decoded == nil || len(decoded) != 0 {
		t.Fatalf("expected empty object, got %#v", decoded)
	}
}

func TestTranslationMetadataScanTreatsJSONNullAsAbsent(t *testing.T) {
	var metadata TranslationMetadata
	if err := metadata.Scan([]byte("null")); err != nil {
		t.Fatalf("scan JSON null: %v", err)
	}
	if metadata != nil {
		t.Fatalf("expected nil metadata, got %#v", metadata)
	}
}

func TestTranslationMetadataScanRejectsScalarJSON(t *testing.T) {
	var metadata TranslationMetadata
	err := metadata.Scan([]byte(`"null"`))
	if err == nil {
		t.Fatalf("expected scalar metadata scan error")
	}
}
