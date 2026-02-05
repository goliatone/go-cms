package content

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func TestContentJSONUsesMetadataField(t *testing.T) {
	contentID := uuid.New()
	typeID := uuid.New()

	record := Content{
		ID:            contentID,
		ContentTypeID: typeID,
		Slug:          "about",
		Status:        "draft",
		Metadata: map[string]any{
			"path": "/about",
		},
	}

	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if _, ok := raw["metadata"]; !ok {
		t.Fatalf("expected metadata field in JSON payload")
	}
	if _, ok := raw["Metadata"]; ok {
		t.Fatalf("expected Metadata field to be absent from JSON payload")
	}

	input := fmt.Sprintf(`{"id":"%s","content_type_id":"%s","metadata":{"path":"/about"}}`, contentID, typeID)
	var decoded Content
	if err := json.Unmarshal([]byte(input), &decoded); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if decoded.Metadata == nil {
		t.Fatalf("expected metadata to decode")
	}
	if got, ok := decoded.Metadata["path"].(string); !ok || got != "/about" {
		t.Fatalf("expected metadata path %q got %v", "/about", decoded.Metadata["path"])
	}
}
