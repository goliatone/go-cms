package auditcmd

import (
	"context"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
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
	log     AuditLog
	logger  interfaces.Logger
	timeout time.Duration
}

// ExportHandlerOption customises the export handler.
type ExportHandlerOption func(*ExportAuditHandler)

// ExportWithTimeout overrides the default execution timeout.
func ExportWithTimeout(timeout time.Duration) ExportHandlerOption {
	return func(h *ExportAuditHandler) {
		h.timeout = timeout
	}
}

// NewExportAuditHandler constructs a handler wired to the provided audit log implementation.
func NewExportAuditHandler(log AuditLog, logger interfaces.Logger, opts ...ExportHandlerOption) *ExportAuditHandler {
	handler := &ExportAuditHandler{
		log:     log,
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

// Execute satisfies command.Commander[ExportAuditCommand].
func (h *ExportAuditHandler) Execute(ctx context.Context, msg ExportAuditCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}

	events, err := h.log.List(ctx)
	if err != nil {
		return commands.WrapExecuteError(err)
	}
	limit := len(events)
	if msg.MaxRecords != nil && *msg.MaxRecords >= 0 && *msg.MaxRecords < limit {
		limit = *msg.MaxRecords
	}

	baseLogger := logging.WithFields(h.logger, map[string]any{
		"operation": "audit.export",
	})

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

// CLIHandler satisfies command.CLICommand by returning the handler.
func (h *ExportAuditHandler) CLIHandler() any {
	return h
}

// CLIOptions describes the CLI metadata for audit export.
func (h *ExportAuditHandler) CLIOptions() command.CLIConfig {
	return command.CLIConfig{
		Path:        []string{"audit", "export"},
		Group:       "audit",
		Description: "Export audit events to the configured logger",
	}
}
