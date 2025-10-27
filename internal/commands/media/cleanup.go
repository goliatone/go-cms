package mediacmd

import (
	"context"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

const cleanupAssetsMessageType = "cms.media.asset.cleanup"

// CleanupAssetsCommand invalidates cached media bindings so downstream resolvers refresh state.
type CleanupAssetsCommand struct {
	Bindings media.BindingSet `json:"bindings"`
	DryRun   bool             `json:"dry_run,omitempty"`
}

// Type implements command.Message.
func (CleanupAssetsCommand) Type() string { return cleanupAssetsMessageType }

// Validate ensures the command payload contains the references to invalidate.
func (m CleanupAssetsCommand) Validate() error {
	errs := validation.Errors{}
	if len(m.Bindings) == 0 {
		errs["bindings"] = validation.NewError("cms.media.asset.cleanup.bindings_required", "bindings must include at least one media reference")
	} else if refErr := validateBindingSet(m.Bindings); refErr != nil {
		errs["bindings"] = refErr
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// CleanupAssetsHandler invalidates media bindings using the shared command handler foundation.
type CleanupAssetsHandler struct {
	inner *commands.Handler[CleanupAssetsCommand]
}

// NewCleanupAssetsHandler constructs a handler wired to the provided media service.
func NewCleanupAssetsHandler(service media.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[CleanupAssetsCommand]) *CleanupAssetsHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg CleanupAssetsCommand) error {
		if !gates.mediaLibraryEnabled() {
			return media.ErrProviderUnavailable
		}
		if msg.DryRun {
			logging.WithFields(baseLogger, map[string]any{
				"binding_sets": len(msg.Bindings),
			}).Info("media.command.cleanup.dry_run")
			return nil
		}
		if err := service.Invalidate(ctx, msg.Bindings); err != nil {
			return err
		}
		logging.WithFields(baseLogger, map[string]any{
			"binding_sets": len(msg.Bindings),
		}).Info("media.command.cleanup.completed")
		return nil
	}

	handlerOpts := []commands.HandlerOption[CleanupAssetsCommand]{
		commands.WithLogger[CleanupAssetsCommand](baseLogger),
		commands.WithOperation[CleanupAssetsCommand]("media.asset.cleanup"),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &CleanupAssetsHandler{
		inner: commands.NewHandler[CleanupAssetsCommand](exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[CleanupAssetsCommand].
func (h *CleanupAssetsHandler) Execute(ctx context.Context, msg CleanupAssetsCommand) error {
	return h.inner.Execute(ctx, msg)
}
