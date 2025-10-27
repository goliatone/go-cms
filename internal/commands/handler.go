package commands

import (
	"context"
	"time"

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

const defaultHandlerTimeout = 30 * time.Second

// HandlerOption configures a Handler instance.
type HandlerOption[T command.Message] func(*Handler[T])

// Handler wraps command execution with shared CMS concerns (context, logging, error tagging).
type Handler[T command.Message] struct {
	exec      command.CommandFunc[T]
	logger    interfaces.Logger
	timeout   time.Duration
	operation string
}

// NewHandler creates a handler that satisfies go-command's Commander interface while applying
// CMS-specific concerns (validation, logging, timeout enforcement).
func NewHandler[T command.Message](fn command.CommandFunc[T], opts ...HandlerOption[T]) *Handler[T] {
	if fn == nil {
		panic("commands: handler function cannot be nil")
	}
	h := &Handler[T]{
		exec:    fn,
		logger:  logging.NoOp(),
		timeout: defaultHandlerTimeout,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Execute conforms to command.Commander[T].Execute and applies validation, context management,
// logging, and error categorisation before delegating to the wrapped function.
func (h *Handler[T]) Execute(ctx context.Context, msg T) error {
	if err := command.ValidateMessage(msg); err != nil {
		return wrapValidationError(err)
	}

	ctx = ensureContext(ctx)
	ctx, cancel := h.withTimeout(ctx)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return wrapContextError(err)
	}

	messageType := command.GetMessageType(msg)
	fields := map[string]any{
		"command": messageType,
	}
	if h.operation != "" {
		fields["operation"] = h.operation
	}
	logger := logging.WithFields(h.logger, fields)
	logger.Debug("command.execute.start")

	if err := h.exec(ctx, msg); err != nil {
		logger.Error("command.execute.failed", "error", err)
		return wrapExecuteError(err)
	}

	if err := ctx.Err(); err != nil {
		logger.Error("command.execute.context_error", "error", err)
		return wrapContextError(err)
	}

	logger.Info("command.execute.success")
	return nil
}

// WithTimeout overrides the default execution timeout.
func WithTimeout[T command.Message](timeout time.Duration) HandlerOption[T] {
	return func(h *Handler[T]) {
		if timeout <= 0 {
			h.timeout = 0
			return
		}
		h.timeout = timeout
	}
}

// WithLogger injects the logger used during execution. Defaults to a no-op logger.
func WithLogger[T command.Message](logger interfaces.Logger) HandlerOption[T] {
	return func(h *Handler[T]) {
		if logger == nil {
			h.logger = logging.NoOp()
			return
		}
		h.logger = logger
	}
}

// WithOperation sets a human-friendly operation name emitted with every log entry.
func WithOperation[T command.Message](operation string) HandlerOption[T] {
	return func(h *Handler[T]) {
		h.operation = operation
	}
}

func (h *Handler[T]) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if h.timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, h.timeout)
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
