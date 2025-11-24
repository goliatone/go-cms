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

const schedulePageMessageType = "cms.pages.schedule"

// SchedulePageCommand updates publish/unpublish windows for a page entry.
type SchedulePageCommand struct {
	PageID      uuid.UUID  `json:"page_id"`
	Locale      string     `json:"locale"`
	TemplateID  *uuid.UUID `json:"template_id,omitempty"`
	PublishAt   *time.Time `json:"publish_at,omitempty"`
	UnpublishAt *time.Time `json:"unpublish_at,omitempty"`
	ScheduledBy uuid.UUID  `json:"scheduled_by,omitempty"`
}

// Type implements command.Message.
func (SchedulePageCommand) Type() string { return schedulePageMessageType }

// Validate ensures the message carries the required fields before reaching handlers.
func (m SchedulePageCommand) Validate() error {
	errs := validation.Errors{}
	if m.PageID == uuid.Nil {
		errs["page_id"] = validation.NewError("cms.pages.schedule.page_id_required", "page_id is required")
	}
	if strings.TrimSpace(m.Locale) == "" {
		errs["locale"] = validation.NewError("cms.pages.schedule.locale_required", "locale is required")
	}
	if m.TemplateID != nil && *m.TemplateID == uuid.Nil {
		errs["template_id"] = validation.NewError("cms.pages.schedule.template_id_invalid", "template_id must be a valid identifier when provided")
	}
	if m.PublishAt != nil && m.PublishAt.IsZero() {
		errs["publish_at"] = validation.NewError("cms.pages.schedule.publish_at_invalid", "publish_at must be a valid timestamp when provided")
	}
	if m.UnpublishAt != nil && m.UnpublishAt.IsZero() {
		errs["unpublish_at"] = validation.NewError("cms.pages.schedule.unpublish_at_invalid", "unpublish_at must be a valid timestamp when provided")
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// SchedulePageHandler coordinates scheduling changes via the page service.
type SchedulePageHandler struct {
	service pages.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// SchedulePageOption customises the schedule handler.
type SchedulePageOption func(*SchedulePageHandler)

// SchedulePageWithTimeout overrides the default execution timeout.
func SchedulePageWithTimeout(timeout time.Duration) SchedulePageOption {
	return func(h *SchedulePageHandler) {
		h.timeout = timeout
	}
}

// NewSchedulePageHandler constructs a handler wired to the provided page service.
func NewSchedulePageHandler(service pages.Service, logger interfaces.Logger, gates FeatureGates, opts ...SchedulePageOption) *SchedulePageHandler {
	handler := &SchedulePageHandler{
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

// Execute satisfies command.Commander[SchedulePageCommand].
func (h *SchedulePageHandler) Execute(ctx context.Context, msg SchedulePageCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if !h.gates.schedulingEnabled() {
		return commands.WrapExecuteError(pages.ErrSchedulingDisabled)
	}

	req := pages.SchedulePageRequest{
		PageID:      msg.PageID,
		PublishAt:   msg.PublishAt,
		UnpublishAt: msg.UnpublishAt,
		ScheduledBy: msg.ScheduledBy,
	}
	if _, err := h.service.Schedule(ctx, req); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation":    "pages.schedule",
		"page_id":      msg.PageID,
		"locale":       strings.TrimSpace(msg.Locale),
		"template_id":  msg.TemplateID,
		"publish_at":   msg.PublishAt,
		"unpublish_at": msg.UnpublishAt,
		"scheduled_by": msg.ScheduledBy,
	}).Info("pages.command.schedule.completed")
	return nil
}
