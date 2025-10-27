package cms

import (
	"errors"
	"strings"
	"time"

	urlkit "github.com/goliatone/go-urlkit"
)

// ErrThemesFeatureRequired indicates inconsistent theme configuration.
var ErrThemesFeatureRequired = errors.New("cms config: themes feature must be enabled to configure themes")

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

type ContentConfig struct {
	PageHierarchy bool
}

type I18NConfig struct {
	Enabled bool
	Locales []string
}

type StorageConfig struct {
	Provider string
}

type CacheConfig struct {
	Enabled    bool
	DefaultTTL time.Duration
}

// NavigationConfig captures routing configuration for menu URL resolution.
type NavigationConfig struct {
	RouteConfig *urlkit.Config
	URLKit      URLKitResolverConfig
}

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

// Features toggles module functionality
type Features struct {
	Widgets      bool
	Themes       bool
	MediaLibrary bool
}

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
	return nil
}
