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
	inner *commands.Handler[SchedulePageCommand]
}

// NewSchedulePageHandler constructs a handler wired to the provided page service.
func NewSchedulePageHandler(service pages.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[SchedulePageCommand]) *SchedulePageHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg SchedulePageCommand) error {
		if !gates.schedulingEnabled() {
			return pages.ErrSchedulingDisabled
		}

		fields := map[string]any{
			"page_id": msg.PageID,
			"locale":  strings.TrimSpace(msg.Locale),
		}
		if msg.TemplateID != nil && *msg.TemplateID != uuid.Nil {
			fields["template_id"] = *msg.TemplateID
		}
		if msg.PublishAt != nil {
			fields["publish_at"] = msg.PublishAt
		}
		if msg.UnpublishAt != nil {
			fields["unpublish_at"] = msg.UnpublishAt
		}
		operationLogger := logging.WithFields(baseLogger, fields)
		operationLogger.Debug("pages.command.schedule.dispatch")

		req := pages.SchedulePageRequest{
			PageID:      msg.PageID,
			PublishAt:   msg.PublishAt,
			UnpublishAt: msg.UnpublishAt,
			ScheduledBy: msg.ScheduledBy,
		}
		_, err := service.Schedule(ctx, req)
		return err
	}

	handlerOpts := []commands.HandlerOption[SchedulePageCommand]{
		commands.WithLogger[SchedulePageCommand](baseLogger),
		commands.WithOperation[SchedulePageCommand]("pages.schedule"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &SchedulePageHandler{
		inner: commands.NewHandler[SchedulePageCommand](exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[SchedulePageCommand].
func (h *SchedulePageHandler) Execute(ctx context.Context, msg SchedulePageCommand) error {
	return h.inner.Execute(ctx, msg)
}
