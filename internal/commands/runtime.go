package commands

import (
	"context"
	"time"

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

// DefaultCommandTimeout mirrors the historic handler timeout applied to commands.
const DefaultCommandTimeout = 30 * time.Second

// EnsureContext returns a non-nil context, falling back to context.Background when nil.
func EnsureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// WithCommandTimeout applies the provided timeout unless it is zero or negative.
func WithCommandTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

// EnsureLogger returns a usable logger, defaulting to a no-op logger when nil.
func EnsureLogger(logger interfaces.Logger) interfaces.Logger {
	if logger == nil {
		return logging.NoOp()
	}
	return logger
}
