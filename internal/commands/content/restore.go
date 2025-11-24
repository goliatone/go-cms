package contentcmd

import (
	"context"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
	"github.com/google/uuid"
)

const restoreContentVersionMessageType = "cms.content.version.restore"

// RestoreContentVersionCommand requests that a historical version be restored as a draft.
type RestoreContentVersionCommand struct {
	ContentID  uuid.UUID `json:"content_id"`
	Version    int       `json:"version"`
	RestoredBy uuid.UUID `json:"restored_by"`
}

// Type implements command.Message.
func (RestoreContentVersionCommand) Type() string { return restoreContentVersionMessageType }

// Validate ensures the command carries the required identifiers.
func (m RestoreContentVersionCommand) Validate() error {
	errs := validation.Errors{}
	if m.ContentID == uuid.Nil {
		errs["content_id"] = validation.NewError("cms.content.version.restore.content_id_required", "content_id is required")
	}
	if m.Version <= 0 {
		errs["version"] = validation.NewError("cms.content.version.restore.version_invalid", "version must be greater than zero")
	}
	if m.RestoredBy == uuid.Nil {
		errs["restored_by"] = validation.NewError("cms.content.version.restore.restored_by_required", "restored_by is required")
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// RestoreContentVersionHandler restores historical content versions via the content service.
type RestoreContentVersionHandler struct {
	service content.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// RestoreContentOption customises the restore handler.
type RestoreContentOption func(*RestoreContentVersionHandler)

// RestoreContentWithTimeout overrides the default execution timeout.
func RestoreContentWithTimeout(timeout time.Duration) RestoreContentOption {
	return func(h *RestoreContentVersionHandler) {
		h.timeout = timeout
	}
}

// NewRestoreContentVersionHandler constructs a handler wired to the provided content service.
func NewRestoreContentVersionHandler(service content.Service, logger interfaces.Logger, gates FeatureGates, opts ...RestoreContentOption) *RestoreContentVersionHandler {
	handler := &RestoreContentVersionHandler{
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

// Execute satisfies command.Commander[RestoreContentVersionCommand].
func (h *RestoreContentVersionHandler) Execute(ctx context.Context, msg RestoreContentVersionCommand) error {
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
		return commands.WrapExecuteError(content.ErrVersioningDisabled)
	}

	req := content.RestoreContentVersionRequest{
		ContentID:  msg.ContentID,
		Version:    msg.Version,
		RestoredBy: msg.RestoredBy,
	}
	if _, err := h.service.RestoreVersion(ctx, req); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation":   "content.version.restore",
		"content_id":  msg.ContentID,
		"version":     msg.Version,
		"restored_by": msg.RestoredBy,
	}).Info("content.command.restore.completed")
	return nil
}
