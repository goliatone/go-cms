package cms

import "time"

type Config struct {
	Enabled       bool
	DefaultLocale string
	Content       ContentConfig
	I18N          I18NConfig
	Storage       StorageConfig
	Cache         CacheConfig
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
	}
}
