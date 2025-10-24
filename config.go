package cms

import (
	"time"

	urlkit "github.com/goliatone/go-urlkit"
)

type Config struct {
	Enabled       bool
	DefaultLocale string
	Content       ContentConfig
	I18N          I18NConfig
	Storage       StorageConfig
	Cache         CacheConfig
	Navigation    NavigationConfig
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
	Widgets bool
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
		Features:   Features{},
	}
}
