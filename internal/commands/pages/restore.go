package pagescmd

import (
	"context"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

const restorePageVersionMessageType = "cms.pages.version.restore"

// RestorePageVersionCommand requests that a historical page version be restored as a draft.
type RestorePageVersionCommand struct {
	PageID     uuid.UUID  `json:"page_id"`
	Version    int        `json:"version"`
	Locale     string     `json:"locale"`
	TemplateID *uuid.UUID `json:"template_id,omitempty"`
	RestoredBy uuid.UUID  `json:"restored_by"`
}

// Type implements command.Message.
func (RestorePageVersionCommand) Type() string { return restorePageVersionMessageType }

// Validate ensures the command carries the required identifiers.
func (m RestorePageVersionCommand) Validate() error {
	errs := validation.Errors{}
	if m.PageID == uuid.Nil {
		errs["page_id"] = validation.NewError("cms.pages.version.restore.page_id_required", "page_id is required")
	}
	if m.Version <= 0 {
		errs["version"] = validation.NewError("cms.pages.version.restore.version_invalid", "version must be greater than zero")
	}
	if strings.TrimSpace(m.Locale) == "" {
		errs["locale"] = validation.NewError("cms.pages.version.restore.locale_required", "locale is required")
	}
	if m.TemplateID != nil && *m.TemplateID == uuid.Nil {
		errs["template_id"] = validation.NewError("cms.pages.version.restore.template_id_invalid", "template_id must be a valid identifier when provided")
	}
	if m.RestoredBy == uuid.Nil {
		errs["restored_by"] = validation.NewError("cms.pages.version.restore.restored_by_required", "restored_by is required")
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// RestorePageVersionHandler restores historical page versions via the page service.
type RestorePageVersionHandler struct {
	inner *commands.Handler[RestorePageVersionCommand]
}

// NewRestorePageVersionHandler constructs a handler wired to the provided page service.
func NewRestorePageVersionHandler(service pages.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[RestorePageVersionCommand]) *RestorePageVersionHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg RestorePageVersionCommand) error {
		if !gates.versioningEnabled() {
			return pages.ErrVersioningDisabled
		}

		fields := map[string]any{
			"page_id": msg.PageID,
			"locale":  strings.TrimSpace(msg.Locale),
			"version": msg.Version,
		}
		if msg.TemplateID != nil && *msg.TemplateID != uuid.Nil {
			fields["template_id"] = *msg.TemplateID
		}
		operationLogger := logging.WithFields(baseLogger, fields)
		operationLogger.Debug("pages.command.version.restore.dispatch")

		req := pages.RestorePageVersionRequest{
			PageID:     msg.PageID,
			Version:    msg.Version,
			RestoredBy: msg.RestoredBy,
		}
		_, err := service.RestoreVersion(ctx, req)
		return err
	}

	handlerOpts := []commands.HandlerOption[RestorePageVersionCommand]{
		commands.WithLogger[RestorePageVersionCommand](baseLogger),
		commands.WithOperation[RestorePageVersionCommand]("pages.version.restore"),
		commands.WithMessageFields(func(msg RestorePageVersionCommand) map[string]any {
			fields := map[string]any{}
			if msg.PageID != uuid.Nil {
				fields["page_id"] = msg.PageID
			}
			if msg.Version > 0 {
				fields["version"] = msg.Version
			}
			if msg.Locale != "" {
				fields["locale"] = msg.Locale
			}
			if msg.RestoredBy != uuid.Nil {
				fields["restored_by"] = msg.RestoredBy
			}
			if len(fields) == 0 {
				return nil
			}
			return fields
		}),
		commands.WithTelemetry(commands.DefaultTelemetry[RestorePageVersionCommand](baseLogger)),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &RestorePageVersionHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[RestorePageVersionCommand].
func (h *RestorePageVersionHandler) Execute(ctx context.Context, msg RestorePageVersionCommand) error {
	return h.inner.Execute(ctx, msg)
}
