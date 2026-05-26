package content

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// TranslationMetadata stores optional per-locale metadata. A nil value is
// persisted as SQL NULL; non-nil values are persisted as JSON objects.
type TranslationMetadata map[string]any

// Value implements driver.Valuer so Bun does not encode a nil map as JSON
// scalar null for jsonb columns.
func (m TranslationMetadata) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	payload, err := json.Marshal(map[string]any(m))
	if err != nil {
		return nil, err
	}
	if bytes.Equal(payload, []byte("null")) {
		return nil, nil
	}
	return string(payload), nil
}

// Scan implements sql.Scanner and treats existing JSON scalar null values as
// absent metadata while rejecting non-object JSON payloads.
func (m *TranslationMetadata) Scan(src any) error {
	if m == nil {
		return fmt.Errorf("content: scan translation metadata into nil pointer")
	}
	if src == nil {
		*m = nil
		return nil
	}

	var payload []byte
	switch value := src.(type) {
	case []byte:
		payload = value
	case string:
		payload = []byte(value)
	default:
		return fmt.Errorf("content: unsupported translation metadata source %T", src)
	}

	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || bytes.Equal(payload, []byte("null")) {
		*m = nil
		return nil
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return fmt.Errorf("content: translation metadata must be a JSON object: %w", err)
	}
	if decoded == nil {
		*m = nil
		return nil
	}
	*m = TranslationMetadata(decoded)
	return nil
}
