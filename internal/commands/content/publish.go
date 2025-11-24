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

const publishContentMessageType = "cms.content.publish"

// PublishContentCommand requests publication of a specific content draft version.
type PublishContentCommand struct {
	ContentID   uuid.UUID  `json:"content_id"`
	Version     int        `json:"version"`
	PublishedBy *uuid.UUID `json:"published_by,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// Type implements command.Message.
func (PublishContentCommand) Type() string { return publishContentMessageType }

// Validate ensures the message carries the required fields before reaching handlers.
func (m PublishContentCommand) Validate() error {
	errs := validation.Errors{}
	if m.ContentID == uuid.Nil {
		errs["content_id"] = validation.NewError("cms.content.publish.content_id_required", "content_id is required")
	}
	if m.Version <= 0 {
		errs["version"] = validation.NewError("cms.content.publish.version_invalid", "version must be greater than zero")
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// PublishContentHandler publishes drafts via the content service.
type PublishContentHandler struct {
	service content.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// PublishContentOption customises the publish handler.
type PublishContentOption func(*PublishContentHandler)

// PublishContentWithTimeout overrides the default execution timeout.
func PublishContentWithTimeout(timeout time.Duration) PublishContentOption {
	return func(h *PublishContentHandler) {
		h.timeout = timeout
	}
}

// NewPublishContentHandler constructs a handler wired to the provided content service.
func NewPublishContentHandler(service content.Service, logger interfaces.Logger, gates FeatureGates, opts ...PublishContentOption) *PublishContentHandler {
	handler := &PublishContentHandler{
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

// Execute satisfies command.Commander[PublishContentCommand].Execute.
func (h *PublishContentHandler) Execute(ctx context.Context, msg PublishContentCommand) error {
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

	req := content.PublishContentDraftRequest{
		ContentID:   msg.ContentID,
		Version:     msg.Version,
		PublishedAt: msg.PublishedAt,
	}
	if msg.PublishedBy != nil {
		req.PublishedBy = *msg.PublishedBy
	}

	if _, err := h.service.PublishDraft(ctx, req); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation":    "content.publish",
		"content_id":   msg.ContentID,
		"version":      msg.Version,
		"published_by": msg.PublishedBy,
	}).Info("content.command.publish.completed")
	return nil
}
