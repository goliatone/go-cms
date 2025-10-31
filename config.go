package cms

import "github.com/goliatone/go-cms/internal/runtimeconfig"

var (
	ErrThemesFeatureRequired               = runtimeconfig.ErrThemesFeatureRequired
	ErrSchedulingFeatureRequiresVersioning = runtimeconfig.ErrSchedulingFeatureRequiresVersioning
	ErrAdvancedCacheRequiresEnabledCache   = runtimeconfig.ErrAdvancedCacheRequiresEnabledCache
	ErrCommandsCronRequiresScheduling      = runtimeconfig.ErrCommandsCronRequiresScheduling
	ErrLoggingProviderRequired             = runtimeconfig.ErrLoggingProviderRequired
	ErrLoggingProviderUnknown              = runtimeconfig.ErrLoggingProviderUnknown
	ErrLoggingLevelInvalid                 = runtimeconfig.ErrLoggingLevelInvalid
	ErrLoggingFormatInvalid                = runtimeconfig.ErrLoggingFormatInvalid
)

type (
	Config                 = runtimeconfig.Config
	ContentConfig          = runtimeconfig.ContentConfig
	I18NConfig             = runtimeconfig.I18NConfig
	StorageConfig          = runtimeconfig.StorageConfig
	CacheConfig            = runtimeconfig.CacheConfig
	NavigationConfig       = runtimeconfig.NavigationConfig
	ThemeConfig            = runtimeconfig.ThemeConfig
	URLKitResolverConfig   = runtimeconfig.URLKitResolverConfig
	WidgetConfig           = runtimeconfig.WidgetConfig
	WidgetDefinitionConfig = runtimeconfig.WidgetDefinitionConfig
	Features               = runtimeconfig.Features
	CommandsConfig         = runtimeconfig.CommandsConfig
	MarkdownConfig         = runtimeconfig.MarkdownConfig
	MarkdownParserConfig   = runtimeconfig.MarkdownParserConfig
	GeneratorConfig        = runtimeconfig.GeneratorConfig
	LoggingConfig          = runtimeconfig.LoggingConfig
)

func DefaultConfig() Config {
	return runtimeconfig.DefaultConfig()
}
