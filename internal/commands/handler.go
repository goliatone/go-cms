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
	exec           command.CommandFunc[T]
	logger         interfaces.Logger
	timeout        time.Duration
	operation      string
	fieldExtractor func(T) map[string]any
	telemetry      Telemetry[T]
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
	if h.fieldExtractor != nil {
		for key, value := range h.fieldExtractor(msg) {
			if value == nil {
				continue
			}
			fields[key] = value
		}
	}
	logger := logging.WithFields(h.logger, fields)
	logger.Debug("command.execute.start")

	start := time.Now()

	if err := h.exec(ctx, msg); err != nil {
		h.dispatchTelemetry(ctx, msg, TelemetryInfo{
			Command:   messageType,
			Operation: h.operation,
			Fields:    copyFields(fields),
			Duration:  time.Since(start),
			Error:     err,
			Status:    TelemetryStatusFailed,
			Logger:    logger,
		}, func(info TelemetryInfo) {
			logger.Error("command.execute.failed", "error", info.Error)
		})
		return wrapExecuteError(err)
	}

	if err := ctx.Err(); err != nil {
		h.dispatchTelemetry(ctx, msg, TelemetryInfo{
			Command:   messageType,
			Operation: h.operation,
			Fields:    copyFields(fields),
			Duration:  time.Since(start),
			Error:     err,
			Status:    TelemetryStatusContextError,
			Logger:    logger,
		}, func(info TelemetryInfo) {
			logger.Error("command.execute.context_error", "error", info.Error)
		})
		return wrapContextError(err)
	}

	h.dispatchTelemetry(ctx, msg, TelemetryInfo{
		Command:   messageType,
		Operation: h.operation,
		Fields:    copyFields(fields),
		Duration:  time.Since(start),
		Status:    TelemetryStatusSuccess,
		Logger:    logger,
	}, func(info TelemetryInfo) {
		logger.Info("command.execute.success")
	})
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

// WithMessageFields attaches message-derived fields to every log entry for the command.
func WithMessageFields[T command.Message](extractor func(T) map[string]any) HandlerOption[T] {
	return func(h *Handler[T]) {
		h.fieldExtractor = extractor
	}
}

// WithTelemetry registers an optional telemetry callback invoked after execution completes.
func WithTelemetry[T command.Message](telemetry Telemetry[T]) HandlerOption[T] {
	return func(h *Handler[T]) {
		h.telemetry = telemetry
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

func (h *Handler[T]) dispatchTelemetry(ctx context.Context, msg T, info TelemetryInfo, fallback func(TelemetryInfo)) {
	if h.telemetry != nil {
		h.telemetry(ctx, msg, info)
		return
	}
	fallback(info)
}

func copyFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	clone := make(map[string]any, len(fields))
	for key, value := range fields {
		clone[key] = value
	}
	return clone
}
