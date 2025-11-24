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

// PublishPageHandler publishes drafts via the page service.
type PublishPageHandler struct {
	service pages.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// PublishPageOption customises the publish handler.
type PublishPageOption func(*PublishPageHandler)

// PublishPageWithTimeout overrides the default execution timeout.
func PublishPageWithTimeout(timeout time.Duration) PublishPageOption {
	return func(h *PublishPageHandler) {
		h.timeout = timeout
	}
}

// NewPublishPageHandler constructs a handler wired to the provided page service.
func NewPublishPageHandler(service pages.Service, logger interfaces.Logger, gates FeatureGates, opts ...PublishPageOption) *PublishPageHandler {
	handler := &PublishPageHandler{
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

// Execute satisfies command.Commander[PublishPageCommand].
func (h *PublishPageHandler) Execute(ctx context.Context, msg PublishPageCommand) error {
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

	req := pages.PublishPagePublishRequest{
		PageID:      msg.PageID,
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
		"operation":    "pages.publish",
		"page_id":      msg.PageID,
		"version":      msg.Version,
		"locale":       strings.TrimSpace(msg.Locale),
		"template_id":  msg.TemplateID,
		"published_by": msg.PublishedBy,
	}).Info("pages.command.publish.completed")
	return nil
}
