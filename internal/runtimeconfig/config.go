package runtimeconfig

import (
	"errors"
	"strings"
	"time"

	urlkit "github.com/goliatone/go-urlkit"
)

// ErrThemesFeatureRequired indicates inconsistent theme configuration.
var ErrThemesFeatureRequired = errors.New("cms config: themes feature must be enabled to configure themes")

// ErrSchedulingFeatureRequiresVersioning ensures scheduling stays behind the versioning flag.
var ErrSchedulingFeatureRequiresVersioning = errors.New("cms config: scheduling feature requires versioning to be enabled")

// ErrAdvancedCacheRequiresEnabledCache ensures advanced cache builds only when cache is enabled.
var ErrAdvancedCacheRequiresEnabledCache = errors.New("cms config: advanced cache feature requires cache to be enabled")

// Config aggregates feature flags and adapter bindings for the CMS module.
// Fields intentionally use simple types so host applications can extend them later.
type Config struct {
	Enabled       bool
	DefaultLocale string
	Content       ContentConfig
	I18N          I18NConfig
	Storage       StorageConfig
	Cache         CacheConfig
	Navigation    NavigationConfig
	Themes        ThemeConfig
	Features      Features
}

// ContentConfig captures configuration for the core content module.
type ContentConfig struct {
	PageHierarchy bool
}

// I18NConfig wires go-i18n options through the CMS wrapper.
type I18NConfig struct {
	Enabled bool
	Locales []string
}

// StorageConfig lists identifiers for storage-related dependencies.
type StorageConfig struct {
	Provider string
}

// CacheConfig captures cache behaviour toggles.
type CacheConfig struct {
	Enabled    bool
	DefaultTTL time.Duration
}

// NavigationConfig captures routing configuration for menu URL resolution.
type NavigationConfig struct {
	RouteConfig *urlkit.Config
	URLKit      URLKitResolverConfig
}

// ThemeConfig captures configuration for the themes module.
type ThemeConfig struct {
	BasePath     string
	DefaultTheme string
}

// URLKitResolverConfig configures the go-urlkit based resolver.
type URLKitResolverConfig struct {
	DefaultGroup  string
	LocaleGroups  map[string]string
	DefaultRoute  string
	SlugParam     string
	LocaleParam   string
	LocaleIDParam string
	RouteField    string
	ParamsField   string
	QueryField    string
}

// Features toggles module functionality.
type Features struct {
	Widgets       bool
	Themes        bool
	Versioning    bool
	Scheduling    bool
	MediaLibrary  bool
	AdvancedCache bool
}

// DefaultConfig returns opinionated defaults matching Phase 1 expectations.
func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		DefaultLocale: "en",
		Content: ContentConfig{
			PageHierarchy: true,
		},
		I18N: I18NConfig{
			Enabled: true,
			Locales: []string{"en"},
		},
		Storage: StorageConfig{
			Provider: "bun",
		},
		Cache: CacheConfig{
			Enabled:    true,
			DefaultTTL: time.Minute,
		},
		Navigation: NavigationConfig{},
		Themes: ThemeConfig{
			BasePath: "themes",
		},
		Features: Features{},
	}
}

// Validate performs high-level consistency checks.
func (cfg Config) Validate() error {
	if !cfg.Features.Themes {
		if strings.TrimSpace(cfg.Themes.DefaultTheme) != "" {
			return ErrThemesFeatureRequired
		}
	}
	if cfg.Features.Scheduling && !cfg.Features.Versioning {
		return ErrSchedulingFeatureRequiresVersioning
	}
	if cfg.Features.AdvancedCache && !cfg.Cache.Enabled {
		return ErrAdvancedCacheRequiresEnabledCache
	}
	return nil
}
