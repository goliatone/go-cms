package pagescmd

import (
	"context"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
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
	service pages.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// RestorePageOption customises the restore handler.
type RestorePageOption func(*RestorePageVersionHandler)

// RestorePageWithTimeout overrides the default execution timeout.
func RestorePageWithTimeout(timeout time.Duration) RestorePageOption {
	return func(h *RestorePageVersionHandler) {
		h.timeout = timeout
	}
}

// NewRestorePageVersionHandler constructs a handler wired to the provided page service.
func NewRestorePageVersionHandler(service pages.Service, logger interfaces.Logger, gates FeatureGates, opts ...RestorePageOption) *RestorePageVersionHandler {
	handler := &RestorePageVersionHandler{
		service: service,
		logger:  commands.EnsureLogger(logger),
		gates:   gates,
		timeout: commands.DefaultCommandTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(handler)
		}
	}
	return handler
}

// Execute satisfies command.Commander[RestorePageVersionCommand].
func (h *RestorePageVersionHandler) Execute(ctx context.Context, msg RestorePageVersionCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if !h.gates.versioningEnabled() {
		return commands.WrapExecuteError(pages.ErrVersioningDisabled)
	}

	req := pages.RestorePageVersionRequest{
		PageID:     msg.PageID,
		Version:    msg.Version,
		RestoredBy: msg.RestoredBy,
	}
	if _, err := h.service.RestoreVersion(ctx, req); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation":   "pages.version.restore",
		"page_id":     msg.PageID,
		"version":     msg.Version,
		"locale":      strings.TrimSpace(msg.Locale),
		"template_id": msg.TemplateID,
		"restored_by": msg.RestoredBy,
	}).Info("pages.command.restore.completed")
	return nil
}
