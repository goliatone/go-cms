package auditcmd

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
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
	inner *commands.Handler[ReplayAuditCommand]
}

// NewReplayAuditHandler constructs a handler that delegates to the provided worker instance.
func NewReplayAuditHandler(worker Worker, logger interfaces.Logger, opts ...commands.HandlerOption[ReplayAuditCommand]) *ReplayAuditHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, _ ReplayAuditCommand) error {
		if err := worker.Process(ctx); err != nil {
			return err
		}
		logging.WithFields(baseLogger, map[string]any{
			"operation": "replay",
		}).Info("audit.command.replay.completed")
		return nil
	}

	handlerOpts := []commands.HandlerOption[ReplayAuditCommand]{
		commands.WithLogger[ReplayAuditCommand](baseLogger),
		commands.WithOperation[ReplayAuditCommand]("audit.replay"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &ReplayAuditHandler{
		inner: commands.NewHandler[ReplayAuditCommand](exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[ReplayAuditCommand].
func (h *ReplayAuditHandler) Execute(ctx context.Context, msg ReplayAuditCommand) error {
	return h.inner.Execute(ctx, msg)
}
