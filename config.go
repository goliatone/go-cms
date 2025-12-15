package cms

import "github.com/goliatone/go-cms/internal/runtimeconfig"

var (
	ErrThemesFeatureRequired                  = runtimeconfig.ErrThemesFeatureRequired
	ErrSchedulingFeatureRequiresVersioning    = runtimeconfig.ErrSchedulingFeatureRequiresVersioning
	ErrAdvancedCacheRequiresEnabledCache      = runtimeconfig.ErrAdvancedCacheRequiresEnabledCache
	ErrDefaultLocaleRequired                  = runtimeconfig.ErrDefaultLocaleRequired
	ErrLoggingProviderRequired                = runtimeconfig.ErrLoggingProviderRequired
	ErrLoggingProviderUnknown                 = runtimeconfig.ErrLoggingProviderUnknown
	ErrLoggingLevelInvalid                    = runtimeconfig.ErrLoggingLevelInvalid
	ErrLoggingFormatInvalid                   = runtimeconfig.ErrLoggingFormatInvalid
	ErrActivityFeatureRequired                = runtimeconfig.ErrActivityFeatureRequired
	ErrWorkflowProviderUnknown                = runtimeconfig.ErrWorkflowProviderUnknown
	ErrWorkflowProviderConfiguredWhenDisabled = runtimeconfig.ErrWorkflowProviderConfiguredWhenDisabled
)

type (
	Config                    = runtimeconfig.Config
	ContentConfig             = runtimeconfig.ContentConfig
	I18NConfig                = runtimeconfig.I18NConfig
	StorageConfig             = runtimeconfig.StorageConfig
	CacheConfig               = runtimeconfig.CacheConfig
	MenusConfig               = runtimeconfig.MenusConfig
	NavigationConfig          = runtimeconfig.NavigationConfig
	ThemeConfig               = runtimeconfig.ThemeConfig
	URLKitResolverConfig      = runtimeconfig.URLKitResolverConfig
	WidgetConfig              = runtimeconfig.WidgetConfig
	WidgetDefinitionConfig    = runtimeconfig.WidgetDefinitionConfig
	RetentionConfig           = runtimeconfig.RetentionConfig
	ShortcodeConfig           = runtimeconfig.ShortcodeConfig
	ShortcodeDefinitionConfig = runtimeconfig.ShortcodeDefinitionConfig
	ShortcodeSecurityConfig   = runtimeconfig.ShortcodeSecurityConfig
	ShortcodeCacheConfig      = runtimeconfig.ShortcodeCacheConfig
	Features                  = runtimeconfig.Features
	MarkdownConfig            = runtimeconfig.MarkdownConfig
	MarkdownParserConfig      = runtimeconfig.MarkdownParserConfig
	GeneratorConfig           = runtimeconfig.GeneratorConfig
	LoggingConfig             = runtimeconfig.LoggingConfig
	ActivityConfig            = runtimeconfig.ActivityConfig
	WorkflowConfig            = runtimeconfig.WorkflowConfig
	WorkflowDefinitionConfig  = runtimeconfig.WorkflowDefinitionConfig
	WorkflowStateConfig       = runtimeconfig.WorkflowStateConfig
	WorkflowTransitionConfig  = runtimeconfig.WorkflowTransitionConfig
)

func DefaultConfig() Config {
	return runtimeconfig.DefaultConfig()
}
