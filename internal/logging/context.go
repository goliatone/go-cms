package logging

import "context"

type contextKey string

const contextFieldsKey contextKey = "cms.logging.fields"

// ContextWithFields returns a context carrying structured logging fields that
// console loggers can merge into subsequent entries. Existing fields on the
// context are preserved and merged with the provided values.
func ContextWithFields(ctx context.Context, fields map[string]any) context.Context {
	if ctx == nil || len(fields) == 0 {
		return ctx
	}

	existing := ContextFields(ctx)
	if len(existing) == 0 {
		copied := make(map[string]any, len(fields))
		for key, value := range fields {
			copied[key] = value
		}
		return context.WithValue(ctx, contextFieldsKey, copied)
	}

	merged := make(map[string]any, len(existing)+len(fields))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range fields {
		merged[key] = value
	}
	return context.WithValue(ctx, contextFieldsKey, merged)
}

// ContextFields extracts previously annotated logging fields from the context.
// A defensive copy is returned so callers can mutate the map without affecting
// future log entries.
func ContextFields(ctx context.Context) map[string]any {
	if ctx == nil {
		return nil
	}
	value := ctx.Value(contextFieldsKey)
	if value == nil {
		return nil
	}

	fields, ok := value.(map[string]any)
	if !ok || len(fields) == 0 {
		return nil
	}

	copied := make(map[string]any, len(fields))
	for key, val := range fields {
		copied[key] = val
	}
	return copied
}
