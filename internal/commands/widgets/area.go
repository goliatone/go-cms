package widgetscmd

import (
	"context"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

const refreshWidgetAreaMessageType = "cms.widgets.area.refresh"

// RefreshWidgetAreaCommand resolves widget placements for a specific area/locale combination.
type RefreshWidgetAreaCommand struct {
	AreaCode          string      `json:"area_code"`
	LocaleID          *uuid.UUID  `json:"locale_id,omitempty"`
	FallbackLocaleIDs []uuid.UUID `json:"fallback_locale_ids,omitempty"`
	Audience          []string    `json:"audience,omitempty"`
	Segments          []string    `json:"segments,omitempty"`
}

// Type implements command.Message.
func (RefreshWidgetAreaCommand) Type() string { return refreshWidgetAreaMessageType }

// Validate ensures required fields are present.
func (m RefreshWidgetAreaCommand) Validate() error {
	errs := validation.Errors{}
	if strings.TrimSpace(m.AreaCode) == "" {
		errs["area_code"] = validation.NewError("cms.widgets.area.refresh.area_code_required", "area_code is required")
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// RefreshWidgetAreaHandler wraps widget area refresh operations.
type RefreshWidgetAreaHandler struct {
	inner *commands.Handler[RefreshWidgetAreaCommand]
}

// NewRefreshWidgetAreaHandler constructs a handler wired to the provided widget service.
func NewRefreshWidgetAreaHandler(service widgets.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[RefreshWidgetAreaCommand]) *RefreshWidgetAreaHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg RefreshWidgetAreaCommand) error {
		if !gates.widgetsEnabled() {
			return ErrWidgetsModuleDisabled
		}

		input := widgets.ResolveAreaInput{
			AreaCode: strings.TrimSpace(msg.AreaCode),
			Audience: append([]string(nil), msg.Audience...),
			Segments: append([]string(nil), msg.Segments...),
			Now:      time.Now().UTC(),
		}
		if msg.LocaleID != nil {
			input.LocaleID = msg.LocaleID
		}
		if len(msg.FallbackLocaleIDs) > 0 {
			input.FallbackLocaleIDs = append([]uuid.UUID(nil), msg.FallbackLocaleIDs...)
		}

		resolved, err := service.ResolveArea(ctx, input)
		if err != nil {
			return err
		}
		logging.WithFields(baseLogger, map[string]any{
			"area_code": input.AreaCode,
			"resolved":  len(resolved),
		}).Info("widgets.command.area.refreshed")
		return nil
	}

	handlerOpts := []commands.HandlerOption[RefreshWidgetAreaCommand]{
		commands.WithLogger[RefreshWidgetAreaCommand](baseLogger),
		commands.WithOperation[RefreshWidgetAreaCommand]("widgets.area.refresh"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &RefreshWidgetAreaHandler{
		inner: commands.NewHandler[RefreshWidgetAreaCommand](exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[RefreshWidgetAreaCommand].
func (h *RefreshWidgetAreaHandler) Execute(ctx context.Context, msg RefreshWidgetAreaCommand) error {
	return h.inner.Execute(ctx, msg)
}
