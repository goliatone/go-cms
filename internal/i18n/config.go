package i18n

import "github.com/goliatone/go-cms/pkg/interfaces"

type Config struct {
	DefaultLocale     string
	Locales           []string
	TemplateHelperKey string
	LocaleContextKey  string
}

func FromModuleConfig(defaultLocale string, locales []string) Config {
	return Config{
		DefaultLocale:     defaultLocale,
		Locales:           locales,
		TemplateHelperKey: "translate",
		LocaleContextKey:  "locale",
	}
}

func (c Config) HelperConfig() interfaces.HelperConfig {
	return interfaces.HelperConfig{
		LocaleKey:         c.LocaleContextKey,
		TemplateHelperKey: c.TemplateHelperKey,
	}
}
