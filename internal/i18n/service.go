package i18n

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Service aggregates translator + helper wiring.
type Service interface {
	interfaces.Service
}

// ServiceOptions configures the concrete i18n service.
type ServiceOptions struct {
	Translator      interfaces.Translator
	Culture         interfaces.CultureService
	Helpers         TemplateHelpersProvider
	DefaultLocale   string
	FallbackOnError MissingTranslationStrategy
}

// TemplateHelpersProvider builds helper maps for template renderers.
type TemplateHelpersProvider interface {
	TemplateHelpers(translator interfaces.Translator, cfg interfaces.HelperConfig, fallback MissingTranslationStrategy) map[string]any
}

// TemplateHelpersFunc adapts a function into a TemplateHelpersProvider.
type TemplateHelpersFunc func(interfaces.Translator, interfaces.HelperConfig, MissingTranslationStrategy) map[string]any

// TemplateHelpers satisfies TemplateHelpersProvider.
func (fn TemplateHelpersFunc) TemplateHelpers(tr interfaces.Translator, cfg interfaces.HelperConfig, fallback MissingTranslationStrategy) map[string]any {
	return fn(tr, cfg, fallback)
}

// MissingTranslationStrategy defines how helpers handle missing translations.
type MissingTranslationStrategy func(locale, key string, args []any, err error) string

// ErrMissingTranslation is returned when no translation can be resolved.
var ErrMissingTranslation = errors.New("i18n: missing translation")

type service struct {
	translator    interfaces.Translator
	culture       interfaces.CultureService
	helpers       TemplateHelpersProvider
	defaultLocale string
	onMissing     MissingTranslationStrategy
}

// NewService constructs a concrete Service from the provided options.
func NewService(opts ServiceOptions) Service {
	if opts.Translator == nil {
		return NewNoOpService()
	}

	defaultLocale := normalizeLocale(opts.DefaultLocale)
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	helpers := opts.Helpers
	if helpers == nil {
		helpers = TemplateHelpersFunc(defaultTemplateHelpers)
	}

	return &service{
		translator:    opts.Translator,
		culture:       opts.Culture,
		helpers:       helpers,
		defaultLocale: defaultLocale,
		onMissing:     opts.FallbackOnError,
	}
}

func (s *service) Translator() interfaces.Translator {
	return s.translator
}

func (s *service) Culture() interfaces.CultureService {
	return s.culture
}

func (s *service) TemplateHelpers(cfg interfaces.HelperConfig) map[string]any {
	fallback := s.onMissing
	if fallback == nil && cfg.OnMissing != nil {
		fallback = MissingTranslationStrategy(cfg.OnMissing)
	}
	if fallback == nil {
		fallback = defaultMissingTranslationStrategy
	}
	return s.helpers.TemplateHelpers(s.translator, cfg, fallback)
}

func (s *service) DefaultLocale() string {
	return s.defaultLocale
}

// NewInMemoryService creates a service backed by the provided translations.
func NewInMemoryService(cfg Config, translations map[string]map[string]string) (Service, error) {
	if len(translations) == 0 {
		return nil, fmt.Errorf("i18n: translations map cannot be empty")
	}

	tr := newStaticTranslator(cfg, translations)
	return NewService(ServiceOptions{
		Translator:    tr,
		DefaultLocale: cfg.DefaultLocale,
	}), nil
}

// staticTranslator stores translations in memory and performs fallback resolution.
type staticTranslator struct {
	defaultLocale string
	translations  map[string]map[string]string
	fallbacks     map[string][]string
	mu            sync.RWMutex
}

func newStaticTranslator(cfg Config, translations map[string]map[string]string) *staticTranslator {
	fallbacks := make(map[string][]string, len(cfg.Locales))
	for _, loc := range cfg.Locales {
		fallbacks[normalizeLocale(loc.Code)] = cfg.Fallbacks(loc.Code)
	}

	sanitised := make(map[string]map[string]string, len(translations))
	for rawLocale, entries := range translations {
		locale := normalizeLocale(rawLocale)
		if locale == "" {
			continue
		}
		catalog := make(map[string]string, len(entries))
		for key, value := range entries {
			catalog[strings.TrimSpace(key)] = value
		}
		sanitised[locale] = catalog
	}

	return &staticTranslator{
		defaultLocale: cfg.DefaultLocale,
		translations:  sanitised,
		fallbacks:     fallbacks,
	}
}

func (t *staticTranslator) Translate(locale, key string, args ...any) (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("i18n: key cannot be empty")
	}

	candidates := t.candidateLocales(locale)
	for _, candidate := range candidates {
		if catalog, ok := t.translations[candidate]; ok {
			if template, found := catalog[key]; found {
				if len(args) == 0 {
					return template, nil
				}
				return fmt.Sprintf(template, args...), nil
			}
		}
	}

	return "", fmt.Errorf("%w: key %q locale %q", ErrMissingTranslation, key, locale)
}

func (t *staticTranslator) candidateLocales(locale string) []string {
	normalized := normalizeLocale(locale)
	if normalized == "" {
		normalized = t.defaultLocale
	}
	if normalized == "" {
		normalized = "en"
	}

	sequence := append([]string{}, t.fallbacks[normalized]...)
	if len(sequence) == 0 || sequence[0] != normalized {
		sequence = append([]string{normalized}, sequence...)
	}

	// Always append default locale as final fallback.
	if t.defaultLocale != "" {
		sequence = append(sequence, t.defaultLocale)
	}

	return dedupePreserveOrder(sequence)
}

// NoOpService is a placeholder that satisfies the contract without performing translations.
type NoOpService struct{}

// NewNoOpService constructs a stub i18n service.
func NewNoOpService() Service {
	return NoOpService{}
}

func (NoOpService) Translator() interfaces.Translator {
	return noopTranslator{}
}

func (NoOpService) Culture() interfaces.CultureService {
	return nil
}

func (NoOpService) TemplateHelpers(cfg interfaces.HelperConfig) map[string]any {
	var handler MissingTranslationStrategy
	if cfg.OnMissing != nil {
		handler = MissingTranslationStrategy(cfg.OnMissing)
	} else {
		handler = defaultMissingTranslationStrategy
	}

	key := cfg.TemplateHelperKey
	if key == "" {
		key = "translate"
	}

	return map[string]any{
		key: func(locale, messageKey string, args ...any) string {
			return handler(locale, messageKey, args, ErrMissingTranslation)
		},
	}
}

func (NoOpService) DefaultLocale() string {
	return ""
}

type noopTranslator struct{}

func (noopTranslator) Translate(_ string, key string, _ ...any) (string, error) {
	return key, nil
}

func defaultTemplateHelpers(tr interfaces.Translator, cfg interfaces.HelperConfig, fallback MissingTranslationStrategy) map[string]any {
	helperKey := cfg.TemplateHelperKey
	if helperKey == "" {
		helperKey = "translate"
	}

	return map[string]any{
		helperKey: func(locale, key string, args ...any) string {
			msg, err := tr.Translate(locale, key, args...)
			if err != nil {
				return fallback(locale, key, args, err)
			}
			return msg
		},
	}
}

func defaultMissingTranslationStrategy(locale, key string, args []any, err error) string {
	if errors.Is(err, ErrMissingTranslation) {
		return key
	}
	return fmt.Sprintf("missing translation: %s (%v)", key, err)
}
