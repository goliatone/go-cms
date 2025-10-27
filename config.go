package cms

import "github.com/goliatone/go-cms/internal/runtimeconfig"

var (
	ErrThemesFeatureRequired               = runtimeconfig.ErrThemesFeatureRequired
	ErrSchedulingFeatureRequiresVersioning = runtimeconfig.ErrSchedulingFeatureRequiresVersioning
	ErrAdvancedCacheRequiresEnabledCache   = runtimeconfig.ErrAdvancedCacheRequiresEnabledCache
)

type (
	Config               = runtimeconfig.Config
	ContentConfig        = runtimeconfig.ContentConfig
	I18NConfig           = runtimeconfig.I18NConfig
	StorageConfig        = runtimeconfig.StorageConfig
	CacheConfig          = runtimeconfig.CacheConfig
	NavigationConfig     = runtimeconfig.NavigationConfig
	ThemeConfig          = runtimeconfig.ThemeConfig
	URLKitResolverConfig = runtimeconfig.URLKitResolverConfig
	Features             = runtimeconfig.Features
)

func DefaultConfig() Config {
	return runtimeconfig.DefaultConfig()
}
