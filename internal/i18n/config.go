package i18n

type Config struct {
	DefaultLocale string
	Locales       []string
}

func FromModuleConfig(defaultLocale string, locales []string) Config {
	return Config{
		DefaultLocale: defaultLocale,
		Locales:       locales,
	}
}
