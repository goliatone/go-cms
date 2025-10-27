package contentcmd

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/interfaces"
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
	inner *commands.Handler[RestoreContentVersionCommand]
}

// NewRestoreContentVersionHandler constructs a handler wired to the provided content service.
func NewRestoreContentVersionHandler(service content.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[RestoreContentVersionCommand]) *RestoreContentVersionHandler {
	exec := func(ctx context.Context, msg RestoreContentVersionCommand) error {
		if !gates.versioningEnabled() {
			return content.ErrVersioningDisabled
		}
		req := content.RestoreContentVersionRequest{
			ContentID:  msg.ContentID,
			Version:    msg.Version,
			RestoredBy: msg.RestoredBy,
		}
		_, err := service.RestoreVersion(ctx, req)
		return err
	}

	handlerOpts := []commands.HandlerOption[RestoreContentVersionCommand]{
		commands.WithLogger[RestoreContentVersionCommand](logger),
		commands.WithOperation[RestoreContentVersionCommand]("content.version.restore"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &RestoreContentVersionHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[RestoreContentVersionCommand].
func (h *RestoreContentVersionHandler) Execute(ctx context.Context, msg RestoreContentVersionCommand) error {
	return h.inner.Execute(ctx, msg)
}
