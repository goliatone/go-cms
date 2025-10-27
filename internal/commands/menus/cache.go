package menuscmd

import (
	"context"
	"errors"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/pkg/interfaces"
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
	inner *commands.Handler[InvalidateMenuCacheCommand]
}

// NewInvalidateMenuCacheHandler constructs a handler wired to the provided menu service.
func NewInvalidateMenuCacheHandler(service menus.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[InvalidateMenuCacheCommand]) *InvalidateMenuCacheHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, _ InvalidateMenuCacheCommand) error {
		if !gates.menusEnabled() {
			return ErrMenusModuleDisabled
		}
		if err := service.InvalidateCache(ctx); err != nil {
			return err
		}
		logging.WithFields(baseLogger, map[string]any{
			"operation": "invalidate",
		}).Info("menus.command.cache.invalidated")
		return nil
	}

	handlerOpts := []commands.HandlerOption[InvalidateMenuCacheCommand]{
		commands.WithLogger[InvalidateMenuCacheCommand](baseLogger),
		commands.WithOperation[InvalidateMenuCacheCommand]("menus.cache.invalidate"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &InvalidateMenuCacheHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[InvalidateMenuCacheCommand].
func (h *InvalidateMenuCacheHandler) Execute(ctx context.Context, msg InvalidateMenuCacheCommand) error {
	return h.inner.Execute(ctx, msg)
}
