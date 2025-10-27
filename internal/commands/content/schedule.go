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
	inner *commands.Handler[ScheduleContentCommand]
}

// NewScheduleContentHandler constructs a handler wired to the provided content service.
func NewScheduleContentHandler(service content.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[ScheduleContentCommand]) *ScheduleContentHandler {
	exec := func(ctx context.Context, msg ScheduleContentCommand) error {
		if !gates.schedulingEnabled() {
			return content.ErrSchedulingDisabled
		}
		req := content.ScheduleContentRequest{
			ContentID:   msg.ContentID,
			PublishAt:   msg.PublishAt,
			UnpublishAt: msg.UnpublishAt,
			ScheduledBy: msg.ScheduledBy,
		}
		_, err := service.Schedule(ctx, req)
		return err
	}

	handlerOpts := []commands.HandlerOption[ScheduleContentCommand]{
		commands.WithLogger[ScheduleContentCommand](logger),
		commands.WithOperation[ScheduleContentCommand]("content.schedule"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &ScheduleContentHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[ScheduleContentCommand].
func (h *ScheduleContentHandler) Execute(ctx context.Context, msg ScheduleContentCommand) error {
	return h.inner.Execute(ctx, msg)
}
