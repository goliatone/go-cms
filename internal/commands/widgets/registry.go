package widgetscmd

import (
	"context"
	"errors"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

const syncWidgetRegistryMessageType = "cms.widgets.registry.sync"

var ErrWidgetsModuleDisabled = errors.New("widgets command: module disabled")

// SyncWidgetRegistryCommand re-applies registered widget definitions.
type SyncWidgetRegistryCommand struct{}

// Type implements command.Message.
func (SyncWidgetRegistryCommand) Type() string { return syncWidgetRegistryMessageType }

// Validate satisfies command.Message.
func (SyncWidgetRegistryCommand) Validate() error {
	return validation.ValidateStruct(&SyncWidgetRegistryCommand{})
}

// SyncWidgetRegistryHandler wraps widget registry synchronisation.
type SyncWidgetRegistryHandler struct {
	service widgets.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// SyncWidgetRegistryOption customises the widget sync handler.
type SyncWidgetRegistryOption func(*SyncWidgetRegistryHandler)

// SyncWidgetRegistryWithTimeout overrides the default execution timeout.
func SyncWidgetRegistryWithTimeout(timeout time.Duration) SyncWidgetRegistryOption {
	return func(h *SyncWidgetRegistryHandler) {
		h.timeout = timeout
	}
}

// NewSyncWidgetRegistryHandler constructs a handler wired to the provided widget service.
func NewSyncWidgetRegistryHandler(service widgets.Service, logger interfaces.Logger, gates FeatureGates, opts ...SyncWidgetRegistryOption) *SyncWidgetRegistryHandler {
	handler := &SyncWidgetRegistryHandler{
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

// Execute satisfies command.Commander[SyncWidgetRegistryCommand].
func (h *SyncWidgetRegistryHandler) Execute(ctx context.Context, msg SyncWidgetRegistryCommand) error {
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
	if err := h.service.SyncRegistry(ctx); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation": "widgets.registry.sync",
	}).Info("widgets.command.registry.sync.completed")
	return nil
}
