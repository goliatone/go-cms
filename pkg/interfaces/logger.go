package interfaces

import "context"

// Logger defines the leveled logging contract expected by the CMS runtime.
// The interface is promoted during Phase 7 (Observability) so advanced
// deployments can supply structured logging while keeping console fallbacks
// viable for simpler setups. It mirrors github.com/goliatone/go-logger,
// allowing host applications to forward their preferred provider without
// additional adapters.
type Logger interface {
	Trace(msg string, args ...any)
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Fatal(msg string, args ...any)
	// WithFields returns a child logger that will attach the provided fields
	// to every emitted log entry. Implementations should avoid mutating the
	// current instance so callers can safely reuse loggers across requests.
	WithFields(fields map[string]any) Logger
	// WithContext returns a logger that binds the supplied context and should
	// propagate correlation identifiers when available.
	WithContext(ctx context.Context) Logger
}

// LoggerProvider exposes named loggers. Implementations can return the same
// instance for every name or scope loggers (e.g. module-based children). Phase 7
// expects container wiring to request names like "cms.pages" or "cms.scheduler"
// so operators can filter logs per module.
type LoggerProvider interface {
	GetLogger(name string) Logger
}

// FieldsLogger is an optional extension for attaching persistent structured
// fields to a logger. Providers that support this behaviour should return a
// new logger with the supplied fields applied on every log entry. Logger
// implementations exported during the Observability phase are expected to
// satisfy this interface even though WithFields is promoted onto Logger.
type FieldsLogger interface {
	WithFields(fields map[string]any) Logger
}
