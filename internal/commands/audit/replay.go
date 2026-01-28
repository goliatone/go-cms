package auditcmd

import (
	"context"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

const replayAuditMessageType = "cms.audit.replay"

// Worker exposes the subset of jobs.Worker behaviour required by the audit commands.
type Worker interface {
	Process(ctx context.Context) error
}

// ReplayAuditCommand triggers scheduler job replay through the worker pipeline.
type ReplayAuditCommand struct{}

// Type implements command.Message.
func (ReplayAuditCommand) Type() string { return replayAuditMessageType }

// Validate satisfies command.Message.
func (ReplayAuditCommand) Validate() error {
	return validation.ValidateStruct(&ReplayAuditCommand{})
}

// ReplayAuditHandler runs pending scheduler jobs via the supplied worker.
type ReplayAuditHandler struct {
	worker  Worker
	logger  interfaces.Logger
	timeout time.Duration
}

// ReplayHandlerOption customises the replay handler.
type ReplayHandlerOption func(*ReplayAuditHandler)

// ReplayWithTimeout overrides the default execution timeout.
func ReplayWithTimeout(timeout time.Duration) ReplayHandlerOption {
	return func(h *ReplayAuditHandler) {
		h.timeout = timeout
	}
}

// NewReplayAuditHandler constructs a handler that delegates to the provided worker instance.
func NewReplayAuditHandler(worker Worker, logger interfaces.Logger, opts ...ReplayHandlerOption) *ReplayAuditHandler {
	handler := &ReplayAuditHandler{
		worker:  worker,
		logger:  commands.EnsureLogger(logger),
		timeout: commands.DefaultCommandTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(handler)
		}
	}
	return handler
}

// Execute satisfies command.Commander[ReplayAuditCommand].
func (h *ReplayAuditHandler) Execute(ctx context.Context, msg ReplayAuditCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}

	if err := h.worker.Process(ctx); err != nil {
		return commands.WrapExecuteError(err)
	}
	logging.WithFields(h.logger, map[string]any{
		"operation": "audit.replay",
	}).Info("audit.command.replay.completed")
	return nil
}

// CLIHandler satisfies command.CLICommand by returning the handler.
func (h *ReplayAuditHandler) CLIHandler() any {
	return h
}

// CLIOptions describes the CLI metadata for audit replay.
func (h *ReplayAuditHandler) CLIOptions() command.CLIConfig {
	return command.CLIConfig{
		Path:        []string{"audit", "replay"},
		Group:       "audit",
		Description: "Replay scheduled audit jobs through the worker pipeline",
	}
}
