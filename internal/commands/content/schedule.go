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

const scheduleContentMessageType = "cms.content.schedule"

// ScheduleContentCommand updates publish/unpublish windows for a content entry.
type ScheduleContentCommand struct {
	ContentID   uuid.UUID  `json:"content_id"`
	PublishAt   *time.Time `json:"publish_at,omitempty"`
	UnpublishAt *time.Time `json:"unpublish_at,omitempty"`
	ScheduledBy uuid.UUID  `json:"scheduled_by,omitempty"`
}

// Type implements command.Message.
func (ScheduleContentCommand) Type() string { return scheduleContentMessageType }

// Validate ensures required fields and basic payload consistency.
func (m ScheduleContentCommand) Validate() error {
	errs := validation.Errors{}
	if m.ContentID == uuid.Nil {
		errs["content_id"] = validation.NewError("cms.content.schedule.content_id_required", "content_id is required")
	}
	if m.PublishAt != nil && m.PublishAt.IsZero() {
		errs["publish_at"] = validation.NewError("cms.content.schedule.publish_at_invalid", "publish_at must be a valid timestamp when provided")
	}
	if m.UnpublishAt != nil && m.UnpublishAt.IsZero() {
		errs["unpublish_at"] = validation.NewError("cms.content.schedule.unpublish_at_invalid", "unpublish_at must be a valid timestamp when provided")
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ScheduleContentHandler coordinates scheduling changes via the content service.
type ScheduleContentHandler struct {
	service content.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// ScheduleContentOption customises the schedule handler.
type ScheduleContentOption func(*ScheduleContentHandler)

// ScheduleContentWithTimeout overrides the default execution timeout.
func ScheduleContentWithTimeout(timeout time.Duration) ScheduleContentOption {
	return func(h *ScheduleContentHandler) {
		h.timeout = timeout
	}
}

// NewScheduleContentHandler constructs a handler wired to the provided content service.
func NewScheduleContentHandler(service content.Service, logger interfaces.Logger, gates FeatureGates, opts ...ScheduleContentOption) *ScheduleContentHandler {
	handler := &ScheduleContentHandler{
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

// Execute satisfies command.Commander[ScheduleContentCommand].
func (h *ScheduleContentHandler) Execute(ctx context.Context, msg ScheduleContentCommand) error {
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
		return commands.WrapExecuteError(content.ErrSchedulingDisabled)
	}

	req := content.ScheduleContentRequest{
		ContentID:   msg.ContentID,
		PublishAt:   msg.PublishAt,
		UnpublishAt: msg.UnpublishAt,
		ScheduledBy: msg.ScheduledBy,
	}
	if _, err := h.service.Schedule(ctx, req); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation":    "content.schedule",
		"content_id":   msg.ContentID,
		"publish_at":   msg.PublishAt,
		"unpublish_at": msg.UnpublishAt,
		"scheduled_by": msg.ScheduledBy,
	}).Info("content.command.schedule.completed")
	return nil
}
