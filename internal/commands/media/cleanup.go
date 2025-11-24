package mediacmd

import (
	"context"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
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

// CleanupAssetsHandler invalidates media bindings.
type CleanupAssetsHandler struct {
	service media.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// CleanupAssetsOption customises the cleanup handler.
type CleanupAssetsOption func(*CleanupAssetsHandler)

// CleanupAssetsWithTimeout overrides the default execution timeout.
func CleanupAssetsWithTimeout(timeout time.Duration) CleanupAssetsOption {
	return func(h *CleanupAssetsHandler) {
		h.timeout = timeout
	}
}

// NewCleanupAssetsHandler constructs a handler wired to the provided media service.
func NewCleanupAssetsHandler(service media.Service, logger interfaces.Logger, gates FeatureGates, opts ...CleanupAssetsOption) *CleanupAssetsHandler {
	handler := &CleanupAssetsHandler{
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

// Execute satisfies command.Commander[CleanupAssetsCommand].
func (h *CleanupAssetsHandler) Execute(ctx context.Context, msg CleanupAssetsCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if !h.gates.mediaLibraryEnabled() {
		return commands.WrapExecuteError(media.ErrProviderUnavailable)
	}
	if msg.DryRun {
		return nil
	}
	if err := h.service.Invalidate(ctx, msg.Bindings); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation":      "media.asset.cleanup",
		"binding_groups": len(msg.Bindings),
		"dry_run":        msg.DryRun,
	}).Info("media.command.cleanup.completed")
	return nil
}
