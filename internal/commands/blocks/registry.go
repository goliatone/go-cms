package blockscmd

import (
	"context"
	"errors"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

const syncBlockRegistryMessageType = "cms.blocks.registry.sync"

var ErrBlocksModuleDisabled = errors.New("blocks command: module disabled")

// FeatureGates exposes the toggle required by block command handlers.
type FeatureGates struct {
	BlocksEnabled func() bool
}

func (g FeatureGates) blocksEnabled() bool {
	if g.BlocksEnabled == nil {
		return true
	}
	return g.BlocksEnabled()
}

// SyncBlockRegistryCommand re-applies registered block definitions to the persistence layer.
type SyncBlockRegistryCommand struct{}

// Type implements command.Message.
func (SyncBlockRegistryCommand) Type() string { return syncBlockRegistryMessageType }

// Validate satisfies command.Message.
func (SyncBlockRegistryCommand) Validate() error {
	return validation.ValidateStruct(&SyncBlockRegistryCommand{})
}

// SyncBlockRegistryHandler wraps block registry synchronisation.
type SyncBlockRegistryHandler struct {
	inner *commands.Handler[SyncBlockRegistryCommand]
}

// NewSyncBlockRegistryHandler constructs a handler wired to the provided block service.
func NewSyncBlockRegistryHandler(service blocks.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[SyncBlockRegistryCommand]) *SyncBlockRegistryHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, _ SyncBlockRegistryCommand) error {
		if !gates.blocksEnabled() {
			return ErrBlocksModuleDisabled
		}
		if err := service.SyncRegistry(ctx); err != nil {
			return err
		}
		logging.WithFields(baseLogger, map[string]any{
			"operation": "sync",
		}).Info("blocks.command.registry.completed")
		return nil
	}

	handlerOpts := []commands.HandlerOption[SyncBlockRegistryCommand]{
		commands.WithLogger[SyncBlockRegistryCommand](baseLogger),
		commands.WithOperation[SyncBlockRegistryCommand]("blocks.registry.sync"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &SyncBlockRegistryHandler{
		inner: commands.NewHandler[SyncBlockRegistryCommand](exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[SyncBlockRegistryCommand].
func (h *SyncBlockRegistryHandler) Execute(ctx context.Context, msg SyncBlockRegistryCommand) error {
	return h.inner.Execute(ctx, msg)
}
