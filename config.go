package cms

type Config struct {
	Enabled       bool
	DefaultLocale string
	Content       ContentConfig
	I18N          I18NConfig
	Storage       StorageConfig
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
	}
}
