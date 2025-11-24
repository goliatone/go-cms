package blockscmd

import (
	"context"
	"errors"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
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
	service blocks.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// SyncBlockRegistryOption customises the block sync handler.
type SyncBlockRegistryOption func(*SyncBlockRegistryHandler)

// SyncBlockRegistryWithTimeout overrides the default execution timeout.
func SyncBlockRegistryWithTimeout(timeout time.Duration) SyncBlockRegistryOption {
	return func(h *SyncBlockRegistryHandler) {
		h.timeout = timeout
	}
}

// NewSyncBlockRegistryHandler constructs a handler wired to the provided block service.
func NewSyncBlockRegistryHandler(service blocks.Service, logger interfaces.Logger, gates FeatureGates, opts ...SyncBlockRegistryOption) *SyncBlockRegistryHandler {
	handler := &SyncBlockRegistryHandler{
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

// Execute satisfies command.Commander[SyncBlockRegistryCommand].
func (h *SyncBlockRegistryHandler) Execute(ctx context.Context, msg SyncBlockRegistryCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if !h.gates.blocksEnabled() {
		return commands.WrapExecuteError(ErrBlocksModuleDisabled)
	}
	if err := h.service.SyncRegistry(ctx); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation": "blocks.registry.sync",
	}).Info("blocks.command.registry.sync.completed")
	return nil
}
