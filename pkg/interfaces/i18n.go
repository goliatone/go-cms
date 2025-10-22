package interfaces

type Translator interface {
	Translate(locale, key string, args ...any) (string, error)
}

type TranslatorWithMetadata interface {
	TranslatorWithMetadata(locale, key string, args ...any) (string, map[string]any, error)
}

type Formatter interface {
	Format(template string, args ...any) (string, error)
}

type CultureService interface {
	GetCurrencyCode(locale string) (string, error)
	GetCurrency(locale string) (any, error)
	GetSupportNumber(locale string) (string, error)
	GetList(locale, name string) ([]string, error)
	GetMeasurementPreference(locale, measurementType string) (any, error)
	ConvertMeasurement(locale string, value float64, fromUnit, measurementType string) (float64, string, string, error)
}

type FormatterRegistry interface {
	FuncMap(locale string) map[string]any
}

type MissingTranslationHandler func(locale, key string, args []any, err error) string

type HelperConfig struct {
	LocaleKey         string
	TemplateHelperKey string
	OnMissing         MissingTranslationHandler
	Registry          FormatterRegistry
}

type TemplateHelperProvider interface {
	TemplateHelpers(translator Translator, cfg HelperConfig) map[string]any
}

type Service interface {
	Translator() Translator
	Culture() CultureService
	TemplateHelpers(cfg HelperConfig) map[string]any
	DefaultLocale() string
}
