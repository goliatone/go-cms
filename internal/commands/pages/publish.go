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
	"github.com/google/uuid"
)

const publishPageMessageType = "cms.pages.publish"

// PublishPageCommand requests publication of a specific page draft version.
// Locale and template identifiers are carried for logging and hierarchy validation.
type PublishPageCommand struct {
	PageID      uuid.UUID  `json:"page_id"`
	Version     int        `json:"version"`
	Locale      string     `json:"locale"`
	TemplateID  *uuid.UUID `json:"template_id,omitempty"`
	PublishedBy *uuid.UUID `json:"published_by,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// Type implements command.Message.
func (PublishPageCommand) Type() string { return publishPageMessageType }

// Validate ensures the command captures the required identifiers before reaching handlers.
func (m PublishPageCommand) Validate() error {
	errs := validation.Errors{}
	if m.PageID == uuid.Nil {
		errs["page_id"] = validation.NewError("cms.pages.publish.page_id_required", "page_id is required")
	}
	if m.Version <= 0 {
		errs["version"] = validation.NewError("cms.pages.publish.version_invalid", "version must be greater than zero")
	}
	if strings.TrimSpace(m.Locale) == "" {
		errs["locale"] = validation.NewError("cms.pages.publish.locale_required", "locale is required")
	}
	if m.TemplateID != nil && *m.TemplateID == uuid.Nil {
		errs["template_id"] = validation.NewError("cms.pages.publish.template_id_invalid", "template_id must be a valid identifier when provided")
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// PublishPageHandler publishes drafts via the page service using the shared command handler foundation.
type PublishPageHandler struct {
	inner *commands.Handler[PublishPageCommand]
}

// NewPublishPageHandler constructs a handler wired to the provided page service.
func NewPublishPageHandler(service pages.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[PublishPageCommand]) *PublishPageHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg PublishPageCommand) error {
		if !gates.versioningEnabled() {
			return pages.ErrVersioningDisabled
		}

		req := pages.PublishPagePublishRequest{
			PageID:      msg.PageID,
			Version:     msg.Version,
			PublishedAt: msg.PublishedAt,
		}
		if msg.PublishedBy != nil {
			req.PublishedBy = *msg.PublishedBy
		}
		_, err := service.PublishDraft(ctx, req)
		return err
	}

	handlerOpts := []commands.HandlerOption[PublishPageCommand]{
		commands.WithLogger[PublishPageCommand](baseLogger),
		commands.WithOperation[PublishPageCommand]("pages.publish"),
		commands.WithMessageFields(func(msg PublishPageCommand) map[string]any {
			fields := map[string]any{}
			if msg.PageID != uuid.Nil {
				fields["page_id"] = msg.PageID
			}
			if msg.Version > 0 {
				fields["version"] = msg.Version
			}
			if trimmed := strings.TrimSpace(msg.Locale); trimmed != "" {
				fields["locale"] = trimmed
			}
			if msg.TemplateID != nil && *msg.TemplateID != uuid.Nil {
				fields["template_id"] = *msg.TemplateID
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
		commands.WithTelemetry(commands.DefaultTelemetry[PublishPageCommand](baseLogger)),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &PublishPageHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[PublishPageCommand].Execute.
func (h *PublishPageHandler) Execute(ctx context.Context, msg PublishPageCommand) error {
	return h.inner.Execute(ctx, msg)
}
