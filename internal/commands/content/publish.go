package contentcmd

import (
	"context"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/interfaces"
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

// PublishContentHandler publishes drafts via the content service using the shared command handler foundation.
type PublishContentHandler struct {
	inner *commands.Handler[PublishContentCommand]
}

// NewPublishContentHandler constructs a handler wired to the provided content service.
func NewPublishContentHandler(service content.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[PublishContentCommand]) *PublishContentHandler {
	exec := func(ctx context.Context, msg PublishContentCommand) error {
		if !gates.versioningEnabled() {
			return content.ErrVersioningDisabled
		}
		req := content.PublishContentDraftRequest{
			ContentID:   msg.ContentID,
			Version:     msg.Version,
			PublishedAt: msg.PublishedAt,
		}
		if msg.PublishedBy != nil {
			req.PublishedBy = *msg.PublishedBy
		}
		_, err := service.PublishDraft(ctx, req)
		return err
	}

	handlerOpts := []commands.HandlerOption[PublishContentCommand]{
		commands.WithLogger[PublishContentCommand](logger),
		commands.WithOperation[PublishContentCommand]("content.publish"),
		commands.WithMessageFields(func(msg PublishContentCommand) map[string]any {
			fields := map[string]any{}
			if msg.ContentID != uuid.Nil {
				fields["content_id"] = msg.ContentID
			}
			if msg.Version > 0 {
				fields["version"] = msg.Version
			}
			if msg.PublishedBy != nil && *msg.PublishedBy != uuid.Nil {
				fields["published_by"] = *msg.PublishedBy
			}
			if msg.PublishedAt != nil && !msg.PublishedAt.IsZero() {
				fields["published_at"] = msg.PublishedAt
			}
			if len(fields) == 0 {
				return nil
			}
			return fields
		}),
		commands.WithTelemetry(commands.DefaultTelemetry[PublishContentCommand](logger)),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &PublishContentHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[PublishContentCommand].Execute.
func (h *PublishContentHandler) Execute(ctx context.Context, msg PublishContentCommand) error {
	return h.inner.Execute(ctx, msg)
}
