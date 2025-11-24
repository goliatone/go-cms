package menuscmd

import (
	"context"
	"errors"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

const invalidateMenuCacheMessageType = "cms.menus.cache.invalidate"

var ErrMenusModuleDisabled = errors.New("menus command: module disabled")

// FeatureGates exposes the runtime toggle required by menu command handlers.
type FeatureGates struct {
	MenusEnabled func() bool
}

func (g FeatureGates) menusEnabled() bool {
	if g.MenusEnabled == nil {
		return true
	}
	return g.MenusEnabled()
}

// InvalidateMenuCacheCommand clears cached menu lookups to force regeneration.
type InvalidateMenuCacheCommand struct{}

// Type implements command.Message.
func (InvalidateMenuCacheCommand) Type() string { return invalidateMenuCacheMessageType }

// Validate satisfies command.Message.
func (InvalidateMenuCacheCommand) Validate() error {
	return validation.ValidateStruct(&InvalidateMenuCacheCommand{})
}

// InvalidateMenuCacheHandler orchestrates menu cache invalidation.
type InvalidateMenuCacheHandler struct {
	service menus.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// InvalidateMenuCacheOption customises the cache invalidation handler.
type InvalidateMenuCacheOption func(*InvalidateMenuCacheHandler)

// InvalidateMenuCacheWithTimeout overrides the default execution timeout.
func InvalidateMenuCacheWithTimeout(timeout time.Duration) InvalidateMenuCacheOption {
	return func(h *InvalidateMenuCacheHandler) {
		h.timeout = timeout
	}
}

// NewInvalidateMenuCacheHandler constructs a handler wired to the provided menu service.
func NewInvalidateMenuCacheHandler(service menus.Service, logger interfaces.Logger, gates FeatureGates, opts ...InvalidateMenuCacheOption) *InvalidateMenuCacheHandler {
	handler := &InvalidateMenuCacheHandler{
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

// Execute satisfies command.Commander[InvalidateMenuCacheCommand].
func (h *InvalidateMenuCacheHandler) Execute(ctx context.Context, msg InvalidateMenuCacheCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if !h.gates.menusEnabled() {
		return commands.WrapExecuteError(ErrMenusModuleDisabled)
	}
	if err := h.service.InvalidateCache(ctx); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation": "menus.cache.invalidate",
	}).Info("menus.command.invalidate_cache.completed")
	return nil
}
