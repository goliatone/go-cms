package interfaces

import "context"

// Logger defines the leveled logging contract expected by the CMS runtime.
// It mirrors the interface exposed by github.com/goliatone/go-logger so
// host applications can plug that package in without additional adapters.
type Logger interface {
	Trace(msg string, args ...any)
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Fatal(msg string, args ...any)
	WithContext(ctx context.Context) Logger
}

// LoggerProvider exposes named loggers. Implementations can return the same
// instance for every name or scope loggers (e.g. module-based children).
type LoggerProvider interface {
	GetLogger(name string) Logger
}

// FieldsLogger is an optional extension for attaching persistent structured
// fields to a logger. Providers that support this behaviour should return a
// new logger with the supplied fields applied on every log entry.
type FieldsLogger interface {
	WithFields(fields map[string]any) Logger
}
