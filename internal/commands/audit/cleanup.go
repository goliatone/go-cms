package auditcmd

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

const cleanupAuditMessageType = "cms.audit.cleanup"

// AuditCleaner extends AuditLog with cleanup capabilities.
type AuditCleaner interface {
	AuditLog
	Clear(ctx context.Context) error
}

// CleanupAuditCommand removes recorded audit events. When DryRun is true only the event count is reported.
type CleanupAuditCommand struct {
	DryRun bool `json:"dry_run,omitempty"`
}

// Type implements command.Message.
func (CleanupAuditCommand) Type() string { return cleanupAuditMessageType }

// Validate satisfies command.Message.
func (CleanupAuditCommand) Validate() error {
	return validation.ValidateStruct(&CleanupAuditCommand{})
}

// CleanupAuditHandler clears audit logs via the supplied cleaner implementation.
type CleanupAuditHandler struct {
	inner *commands.Handler[CleanupAuditCommand]
}

// NewCleanupAuditHandler constructs a handler that delegates to the provided cleaner instance.
func NewCleanupAuditHandler(cleaner AuditCleaner, logger interfaces.Logger, opts ...commands.HandlerOption[CleanupAuditCommand]) *CleanupAuditHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg CleanupAuditCommand) error {
		events, err := cleaner.List(ctx)
		if err != nil {
			return err
		}
		if msg.DryRun {
			logging.WithFields(baseLogger, map[string]any{
				"operation": "cleanup",
				"dry_run":   true,
				"count":     len(events),
			}).Info("audit.command.cleanup.dry_run")
			return nil
		}
		if err := cleaner.Clear(ctx); err != nil {
			return err
		}
		logging.WithFields(baseLogger, map[string]any{
			"operation": "cleanup",
			"removed":   len(events),
		}).Info("audit.command.cleanup.completed")
		return nil
	}

	handlerOpts := []commands.HandlerOption[CleanupAuditCommand]{
		commands.WithLogger[CleanupAuditCommand](baseLogger),
		commands.WithOperation[CleanupAuditCommand]("audit.cleanup"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &CleanupAuditHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[CleanupAuditCommand].
func (h *CleanupAuditHandler) Execute(ctx context.Context, msg CleanupAuditCommand) error {
	return h.inner.Execute(ctx, msg)
}
