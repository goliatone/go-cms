package logging

import (
	"maps"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// WithFields attaches structured fields to a logger when the implementation
// supports the optional FieldsLogger extension. Callers can pass nil or an
// empty map to skip allocation safely.
func WithFields(logger interfaces.Logger, fields map[string]any) interfaces.Logger {
	if logger == nil || len(fields) == 0 {
		return logger
	}

	if fieldsLogger, ok := logger.(interfaces.FieldsLogger); ok {
		copied := make(map[string]any, len(fields))
		maps.Copy(copied, fields)
		return fieldsLogger.WithFields(copied)
	}

	return logger
}
