package staticcmd

import (
	"context"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
	"github.com/google/uuid"
)

// BuildSiteHandler orchestrates generator builds.
type BuildSiteHandler struct {
	service generator.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// BuildSiteOption customises the build handler.
type BuildSiteOption func(*BuildSiteHandler)

// BuildSiteWithTimeout overrides the default execution timeout.
func BuildSiteWithTimeout(timeout time.Duration) BuildSiteOption {
	return func(h *BuildSiteHandler) {
		h.timeout = timeout
	}
}

// NewBuildSiteHandler constructs a handler wired to the provided generator service.
func NewBuildSiteHandler(service generator.Service, logger interfaces.Logger, gates FeatureGates, opts ...BuildSiteOption) *BuildSiteHandler {
	handler := &BuildSiteHandler{
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

// Execute satisfies command.Commander[BuildSiteCommand].
func (h *BuildSiteHandler) Execute(ctx context.Context, msg BuildSiteCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if h.service == nil || !h.gates.generatorEnabled() {
		return commands.WrapExecuteError(generator.ErrServiceDisabled)
	}

	logger := logging.WithFields(h.logger, map[string]any{"operation": "static.build"})

	if msg.AssetsOnly {
		if err := h.service.BuildAssets(ctx); err != nil {
			return commands.WrapExecuteError(err)
		}
		invokeCallback(msg.ResultCallback, ResultEnvelope{
			Result: nil,
			Metadata: map[string]any{
				"operation": "build_assets",
			},
		})
		logger.Info("static.command.assets.completed")
		return nil
	}

	if len(msg.PageIDs) == 1 && len(msg.Locales) == 1 {
		pageID := msg.PageIDs[0]
		locale := strings.TrimSpace(msg.Locales[0])
		if err := h.service.BuildPage(ctx, pageID, locale); err != nil {
			return commands.WrapExecuteError(err)
		}
		invokeCallback(msg.ResultCallback, ResultEnvelope{
			Result: nil,
			Metadata: map[string]any{
				"operation": "build_page",
				"page_id":   pageID,
				"locale":    locale,
			},
		})
		logger.WithFields(map[string]any{"page_id": pageID, "locale": locale}).Info("static.command.page.completed")
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

	result, err := h.service.Build(ctx, options)
	invokeCallback(msg.ResultCallback, ResultEnvelope{
		Result: result,
		Metadata: map[string]any{
			"operation": "build",
		},
	})
	if err != nil {
		return commands.WrapExecuteError(err)
	}

	logger.Info("static.command.build.completed")
	return nil
}

// CLIHandler exposes the build handler for CLI registration.
func (h *BuildSiteHandler) CLIHandler() any {
	return h
}

// CLIOptions describes the CLI metadata for static build.
func (h *BuildSiteHandler) CLIOptions() command.CLIConfig {
	return command.CLIConfig{
		Path:        []string{"static", "build"},
		Group:       "static",
		Description: "Build static site assets and pages",
	}
}

// DiffSiteHandler performs dry-run builds for diffing workflows.
type DiffSiteHandler struct {
	service generator.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// DiffSiteOption customises the diff handler.
type DiffSiteOption func(*DiffSiteHandler)

// DiffSiteWithTimeout overrides the default execution timeout.
func DiffSiteWithTimeout(timeout time.Duration) DiffSiteOption {
	return func(h *DiffSiteHandler) {
		h.timeout = timeout
	}
}

// NewDiffSiteHandler constructs a handler that executes generator dry-runs.
func NewDiffSiteHandler(service generator.Service, logger interfaces.Logger, gates FeatureGates, opts ...DiffSiteOption) *DiffSiteHandler {
	handler := &DiffSiteHandler{
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

// Execute satisfies command.Commander[DiffSiteCommand].
func (h *DiffSiteHandler) Execute(ctx context.Context, msg DiffSiteCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if h.service == nil || !h.gates.generatorEnabled() {
		return commands.WrapExecuteError(generator.ErrServiceDisabled)
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

	result, err := h.service.Build(ctx, options)
	invokeCallback(msg.ResultCallback, ResultEnvelope{
		Result: result,
		Metadata: map[string]any{
			"operation": "diff",
		},
	})
	if err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation": "static.diff",
		"page_ids":  len(msg.PageIDs),
		"locales":   len(msg.Locales),
		"force":     msg.Force,
	}).Info("static.command.diff.completed")
	return nil
}

// CLIHandler exposes the diff handler for CLI registration.
func (h *DiffSiteHandler) CLIHandler() any {
	return h
}

// CLIOptions describes the CLI metadata for static diff.
func (h *DiffSiteHandler) CLIOptions() command.CLIConfig {
	return command.CLIConfig{
		Path:        []string{"static", "diff"},
		Group:       "static",
		Description: "Run a dry-run static build to produce a diff",
	}
}

// CleanSiteHandler clears generator artifacts.
type CleanSiteHandler struct {
	service generator.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// CleanSiteOption customises the clean handler.
type CleanSiteOption func(*CleanSiteHandler)

// CleanSiteWithTimeout overrides the default execution timeout.
func CleanSiteWithTimeout(timeout time.Duration) CleanSiteOption {
	return func(h *CleanSiteHandler) {
		h.timeout = timeout
	}
}

// NewCleanSiteHandler constructs a handler that cleans generator output.
func NewCleanSiteHandler(service generator.Service, logger interfaces.Logger, gates FeatureGates, opts ...CleanSiteOption) *CleanSiteHandler {
	handler := &CleanSiteHandler{
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

// Execute satisfies command.Commander[CleanSiteCommand].
func (h *CleanSiteHandler) Execute(ctx context.Context, msg CleanSiteCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if h.service == nil || !h.gates.generatorEnabled() {
		return commands.WrapExecuteError(generator.ErrServiceDisabled)
	}
	if err := h.service.Clean(ctx); err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation": "static.clean",
	}).Info("static.command.clean.completed")
	return nil
}

// CLIHandler exposes the clean handler for CLI registration.
func (h *CleanSiteHandler) CLIHandler() any {
	return h
}

// CLIOptions describes the CLI metadata for static clean.
func (h *CleanSiteHandler) CLIOptions() command.CLIConfig {
	return command.CLIConfig{
		Path:        []string{"static", "clean"},
		Group:       "static",
		Description: "Clean generator output",
	}
}

// BuildSitemapHandler regenerates sitemap artifacts via the generator service.
type BuildSitemapHandler struct {
	service generator.Service
	logger  interfaces.Logger
	gates   FeatureGates
	timeout time.Duration
}

// BuildSitemapOption customises the sitemap handler.
type BuildSitemapOption func(*BuildSitemapHandler)

// BuildSitemapWithTimeout overrides the default execution timeout.
func BuildSitemapWithTimeout(timeout time.Duration) BuildSitemapOption {
	return func(h *BuildSitemapHandler) {
		h.timeout = timeout
	}
}

// NewBuildSitemapHandler constructs a handler that invokes generator sitemap builds.
func NewBuildSitemapHandler(service generator.Service, logger interfaces.Logger, gates FeatureGates, opts ...BuildSitemapOption) *BuildSitemapHandler {
	handler := &BuildSitemapHandler{
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

// Execute satisfies command.Commander[BuildSitemapCommand].
func (h *BuildSitemapHandler) Execute(ctx context.Context, msg BuildSitemapCommand) error {
	if err := commands.WrapValidationError(command.ValidateMessage(msg)); err != nil {
		return err
	}
	ctx = commands.EnsureContext(ctx)
	ctx, cancel := commands.WithCommandTimeout(ctx, h.timeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return commands.WrapContextError(err)
	}
	if h.service == nil || !h.gates.generatorEnabled() {
		return commands.WrapExecuteError(generator.ErrServiceDisabled)
	}
	if !h.gates.sitemapEnabled() {
		return commands.WrapExecuteError(generator.ErrServiceDisabled)
	}

	err := h.service.BuildSitemap(ctx)
	invokeCallback(msg.ResultCallback, ResultEnvelope{
		Result: nil,
		Metadata: map[string]any{
			"operation": "build_sitemap",
		},
	})
	if err != nil {
		return commands.WrapExecuteError(err)
	}

	logging.WithFields(h.logger, map[string]any{
		"operation": "static.sitemap",
	}).Info("static.command.sitemap.completed")
	return nil
}

// CLIHandler exposes the sitemap handler for CLI registration.
func (h *BuildSitemapHandler) CLIHandler() any {
	return h
}

// CLIOptions describes the CLI metadata for sitemap builds.
func (h *BuildSitemapHandler) CLIOptions() command.CLIConfig {
	return command.CLIConfig{
		Path:        []string{"static", "sitemap"},
		Group:       "static",
		Description: "Build static sitemaps",
	}
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
