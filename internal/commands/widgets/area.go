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
	command "github.com/goliatone/go-command"
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
	service widgets.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// RefreshWidgetAreaOption customises the widget area handler.
type RefreshWidgetAreaOption func(*RefreshWidgetAreaHandler)

// RefreshWidgetAreaWithTimeout overrides the default execution timeout.
func RefreshWidgetAreaWithTimeout(timeout time.Duration) RefreshWidgetAreaOption {
	return func(h *RefreshWidgetAreaHandler) {
		h.timeout = timeout
	}
}

// NewRefreshWidgetAreaHandler constructs a handler wired to the provided widget service.
func NewRefreshWidgetAreaHandler(service widgets.Service, logger interfaces.Logger, gates FeatureGates, opts ...RefreshWidgetAreaOption) *RefreshWidgetAreaHandler {
	handler := &RefreshWidgetAreaHandler{
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

// Execute satisfies command.Commander[RefreshWidgetAreaCommand].
func (h *RefreshWidgetAreaHandler) Execute(ctx context.Context, msg RefreshWidgetAreaCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if !h.gates.widgetsEnabled() {
		return commands.WrapExecuteError(ErrWidgetsModuleDisabled)
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

	resolved, err := h.service.ResolveArea(ctx, input)
	if err != nil {
		return commands.WrapExecuteError(err)
	}
	logging.WithFields(h.logger, map[string]any{
		"operation": "widgets.area.refresh",
		"area_code": input.AreaCode,
		"resolved":  len(resolved),
	}).Info("widgets.command.area.refreshed")
	return nil
}
