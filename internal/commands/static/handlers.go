package staticcmd

import (
	"context"
	"strings"

	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// BuildSiteHandler orchestrates generator builds using the shared command handler foundation.
type BuildSiteHandler struct {
	inner *commands.Handler[BuildSiteCommand]
}

// NewBuildSiteHandler constructs a handler wired to the provided generator service.
func NewBuildSiteHandler(service generator.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[BuildSiteCommand]) *BuildSiteHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg BuildSiteCommand) error {
		if service == nil || !gates.generatorEnabled() {
			return generator.ErrServiceDisabled
		}

		if msg.AssetsOnly {
			if err := service.BuildAssets(ctx); err != nil {
				return err
			}
			invokeCallback(msg.ResultCallback, ResultEnvelope{
				Result: nil,
				Metadata: map[string]any{
					"operation": "build_assets",
				},
			})
			return nil
		}

		if len(msg.PageIDs) == 1 && len(msg.Locales) == 1 {
			if err := service.BuildPage(ctx, msg.PageIDs[0], strings.TrimSpace(msg.Locales[0])); err != nil {
				return err
			}
			invokeCallback(msg.ResultCallback, ResultEnvelope{
				Result: nil,
				Metadata: map[string]any{
					"operation": "build_page",
					"page_id":   msg.PageIDs[0],
					"locale":    strings.TrimSpace(msg.Locales[0]),
				},
			})
			return nil
		}

		options := generator.BuildOptions{
			Force:      msg.Force,
			DryRun:     msg.DryRun,
			AssetsOnly: msg.AssetsOnly,
		}
		if len(msg.PageIDs) > 0 {
			options.PageIDs = append([]uuid.UUID(nil), msg.PageIDs...)
		}
		if len(msg.Locales) > 0 {
			options.Locales = normalizeLocales(msg.Locales)
		}

		result, err := service.Build(ctx, options)
		invokeCallback(msg.ResultCallback, ResultEnvelope{
			Result: result,
			Metadata: map[string]any{
				"operation": "build",
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	handlerOpts := []commands.HandlerOption[BuildSiteCommand]{
		commands.WithLogger[BuildSiteCommand](baseLogger),
		commands.WithOperation[BuildSiteCommand]("static.build"),
		commands.WithMessageFields(func(msg BuildSiteCommand) map[string]any {
			fields := map[string]any{}
			if len(msg.PageIDs) > 0 {
				fields["page_ids"] = len(msg.PageIDs)
			}
			if len(msg.Locales) > 0 {
				fields["locales"] = len(msg.Locales)
			}
			if msg.Force {
				fields["force"] = true
			}
			if msg.DryRun {
				fields["dry_run"] = true
			}
			if msg.AssetsOnly {
				fields["assets_only"] = true
			}
			return fields
		}),
		commands.WithTelemetry(commands.DefaultTelemetry[BuildSiteCommand](baseLogger)),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &BuildSiteHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[BuildSiteCommand].
func (h *BuildSiteHandler) Execute(ctx context.Context, msg BuildSiteCommand) error {
	return h.inner.Execute(ctx, msg)
}

// DiffSiteHandler performs dry-run builds for diffing workflows.
type DiffSiteHandler struct {
	inner *commands.Handler[DiffSiteCommand]
}

// NewDiffSiteHandler constructs a handler that executes generator dry-runs.
func NewDiffSiteHandler(service generator.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[DiffSiteCommand]) *DiffSiteHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg DiffSiteCommand) error {
		if service == nil || !gates.generatorEnabled() {
			return generator.ErrServiceDisabled
		}

		options := generator.BuildOptions{
			Force:  msg.Force,
			DryRun: true,
		}
		if len(msg.PageIDs) > 0 {
			options.PageIDs = append([]uuid.UUID(nil), msg.PageIDs...)
		}
		if len(msg.Locales) > 0 {
			options.Locales = normalizeLocales(msg.Locales)
		}

		result, err := service.Build(ctx, options)
		invokeCallback(msg.ResultCallback, ResultEnvelope{
			Result: result,
			Metadata: map[string]any{
				"operation": "diff",
			},
		})
		if err != nil {
			return err
		}
		return nil
	}

	handlerOpts := []commands.HandlerOption[DiffSiteCommand]{
		commands.WithLogger[DiffSiteCommand](baseLogger),
		commands.WithOperation[DiffSiteCommand]("static.diff"),
		commands.WithMessageFields(func(msg DiffSiteCommand) map[string]any {
			fields := map[string]any{}
			if len(msg.PageIDs) > 0 {
				fields["page_ids"] = len(msg.PageIDs)
			}
			if len(msg.Locales) > 0 {
				fields["locales"] = len(msg.Locales)
			}
			if msg.Force {
				fields["force"] = true
			}
			return fields
		}),
		commands.WithTelemetry(commands.DefaultTelemetry[DiffSiteCommand](baseLogger)),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &DiffSiteHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[DiffSiteCommand].
func (h *DiffSiteHandler) Execute(ctx context.Context, msg DiffSiteCommand) error {
	return h.inner.Execute(ctx, msg)
}

// CleanSiteHandler clears generator artifacts.
type CleanSiteHandler struct {
	inner *commands.Handler[CleanSiteCommand]
}

// NewCleanSiteHandler constructs a handler that cleans generator output.
func NewCleanSiteHandler(service generator.Service, logger interfaces.Logger, gates FeatureGates, opts ...commands.HandlerOption[CleanSiteCommand]) *CleanSiteHandler {
	baseLogger := logger
	if baseLogger == nil {
		baseLogger = logging.NoOp()
	}

	exec := func(ctx context.Context, msg CleanSiteCommand) error {
		if service == nil || !gates.generatorEnabled() {
			return generator.ErrServiceDisabled
		}
		return service.Clean(ctx)
	}

	handlerOpts := []commands.HandlerOption[CleanSiteCommand]{
		commands.WithLogger[CleanSiteCommand](baseLogger),
		commands.WithOperation[CleanSiteCommand]("static.clean"),
		commands.WithTelemetry(commands.DefaultTelemetry[CleanSiteCommand](baseLogger)),
	}
	handlerOpts = append(handlerOpts, opts...)

	return &CleanSiteHandler{
		inner: commands.NewHandler(exec, handlerOpts...),
	}
}

// Execute satisfies command.Commander[CleanSiteCommand].
func (h *CleanSiteHandler) Execute(ctx context.Context, msg CleanSiteCommand) error {
	return h.inner.Execute(ctx, msg)
}

func normalizeLocales(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, locale := range values {
		trimmed := strings.TrimSpace(locale)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func invokeCallback(cb ResultCallback, envelope ResultEnvelope) {
	if cb == nil {
		return
	}
	cb(envelope)
}
