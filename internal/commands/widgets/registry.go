package widgetscmd

import (
	"context"
	"errors"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
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
	inner *commands.Handler[SyncWidgetRegistryCommand]
}

// NewSyncWidgetRegistryHandler constructs a handler wired to the provided widget service.
func NewSyncWidgetRegistryHandler(service widgets.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[SyncWidgetRegistryCommand]) *SyncWidgetRegistryHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, _ SyncWidgetRegistryCommand) error {
		if !gates.widgetsEnabled() {
			return ErrWidgetsModuleDisabled
		}
		if err := service.SyncRegistry(ctx); err != nil {
			return err
		}
		logging.WithFields(baseLogger, map[string]any{
			"operation": "sync",
		}).Info("widgets.command.registry.completed")
		return nil
	}

	handlerOpts := []commands.HandlerOption[SyncWidgetRegistryCommand]{
		commands.WithLogger[SyncWidgetRegistryCommand](baseLogger),
		commands.WithOperation[SyncWidgetRegistryCommand]("widgets.registry.sync"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &SyncWidgetRegistryHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[SyncWidgetRegistryCommand].
func (h *SyncWidgetRegistryHandler) Execute(ctx context.Context, msg SyncWidgetRegistryCommand) error {
	return h.inner.Execute(ctx, msg)
}
