package auditcmd

import (
	"context"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

const exportAuditMessageType = "cms.audit.export"

// AuditLog exposes read operations for recorded audit events.
type AuditLog interface {
	List(ctx context.Context) ([]jobs.AuditEvent, error)
}

// ExportAuditCommand retrieves recorded audit events and emits them through the logger.
type ExportAuditCommand struct {
	MaxRecords *int `json:"max_records,omitempty"`
}

// Type implements command.Message.
func (ExportAuditCommand) Type() string { return exportAuditMessageType }

// Validate ensures the command payload is well-formed.
func (m ExportAuditCommand) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.MaxRecords, validation.By(func(value any) error {
			if m.MaxRecords == nil {
				return nil
			}
			if *m.MaxRecords < 0 {
				return validation.NewError("cms.audit.export.max_records_invalid", "max_records must be zero or positive")
			}
			return nil
		})),
	)
}

// ExportAuditHandler logs recorded audit events up to the provided limit.
type ExportAuditHandler struct {
	inner *commands.Handler[ExportAuditCommand]
}

// NewExportAuditHandler constructs a handler wired to the provided audit log implementation.
func NewExportAuditHandler(log AuditLog, logger interfaces.Logger, opts ...commands.HandlerOption[ExportAuditCommand]) *ExportAuditHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg ExportAuditCommand) error {
		events, err := log.List(ctx)
		if err != nil {
			return err
		}
		limit := len(events)
		if msg.MaxRecords != nil && *msg.MaxRecords >= 0 && *msg.MaxRecords < limit {
			limit = *msg.MaxRecords
		}

		for idx := 0; idx < limit; idx++ {
			event := events[idx]
			logging.WithFields(baseLogger, map[string]any{
				"index":       idx,
				"entity_type": event.EntityType,
				"entity_id":   event.EntityID,
				"action":      event.Action,
				"occurred_at": event.OccurredAt.Format(time.RFC3339),
				"metadata":    event.Metadata,
			}).Debug("audit.command.export.event")
		}

		logging.WithFields(baseLogger, map[string]any{
			"exported": limit,
			"total":    len(events),
		}).Info("audit.command.export.completed")
		return nil
	}

	handlerOpts := []commands.HandlerOption[ExportAuditCommand]{
		commands.WithLogger[ExportAuditCommand](baseLogger),
		commands.WithOperation[ExportAuditCommand]("audit.export"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &ExportAuditHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[ExportAuditCommand].
func (h *ExportAuditHandler) Execute(ctx context.Context, msg ExportAuditCommand) error {
	return h.inner.Execute(ctx, msg)
}
