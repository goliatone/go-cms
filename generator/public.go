package generator

import internal "github.com/goliatone/go-cms/internal/generator"

type (
	Service              = internal.Service
	Config               = internal.Config
	ThemingConfig        = internal.ThemingConfig
	Dependencies         = internal.Dependencies
	Hooks                = internal.Hooks
	BuildOptions         = internal.BuildOptions
	BuildResult          = internal.BuildResult
	BuildMetrics         = internal.BuildMetrics
	RenderedPage         = internal.RenderedPage
	RenderDiagnostic     = internal.RenderDiagnostic
	TemplateContext      = internal.TemplateContext
	SiteMetadata         = internal.SiteMetadata
	BuildMetadata        = internal.BuildMetadata
	PageRenderingContext = internal.PageRenderingContext
	ThemeContext         = internal.ThemeContext
	TemplateHelpers      = internal.TemplateHelpers
	LocaleSpec           = internal.LocaleSpec
	DependencyMetadata   = internal.DependencyMetadata
	AssetResolver        = internal.AssetResolver
	NoOpAssetResolver    = internal.NoOpAssetResolver
)

var (
	ErrNotImplemented  = internal.ErrNotImplemented
	ErrServiceDisabled = internal.ErrServiceDisabled
)

func NewService(cfg Config, deps Dependencies) Service {
	return internal.NewService(cfg, deps)
}

func NewDisabledService() Service {
	return internal.NewDisabledService()
}
