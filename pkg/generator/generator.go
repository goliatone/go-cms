package generator

import internal "github.com/goliatone/go-cms/internal/generator"

// Re-export generator contracts for external consumers. The implementation
// lives in internal/generator; this package provides a stable import path.
type (
	Service           = internal.Service
	Config            = internal.Config
	ThemingConfig     = internal.ThemingConfig
	BuildOptions      = internal.BuildOptions
	BuildResult       = internal.BuildResult
	BuildMetrics      = internal.BuildMetrics
	RenderedPage      = internal.RenderedPage
	RenderDiagnostic  = internal.RenderDiagnostic
	Dependencies      = internal.Dependencies
	Hooks             = internal.Hooks
	LocaleLookup      = internal.LocaleLookup
	AssetResolver     = internal.AssetResolver
	NoOpAssetResolver = internal.NoOpAssetResolver
)

var (
	ErrNotImplemented  = internal.ErrNotImplemented
	ErrServiceDisabled = internal.ErrServiceDisabled
)

// NewService wires a static site generator with the supplied configuration and dependencies.
func NewService(cfg Config, deps Dependencies) Service {
	return internal.NewService(cfg, deps)
}

// NewDisabledService returns a Service that fails all operations with ErrServiceDisabled.
func NewDisabledService() Service {
	return internal.NewDisabledService()
}
