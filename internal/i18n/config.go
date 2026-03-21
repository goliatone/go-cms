package i18n

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
	sharedi18n "github.com/goliatone/go-i18n"
)

// LocaleConfig describes a locale definition and its fallback behaviour.
type LocaleConfig struct {
	Code      string   `json:"code"`
	Fallbacks []string `json:"fallbacks,omitempty"`
}

// Config bridges CMS locale configuration to the i18n service.
type Config struct {
	DefaultLocale     string         `json:"default_locale"`
	Locales           []LocaleConfig `json:"locales"`
	TemplateHelperKey string         `json:"template_helper_key"`
	LocaleContextKey  string         `json:"locale_context_key"`
}

// FromModuleConfig produces the i18n config given CMS configuration values.
func FromModuleConfig(defaultLocale string, locales []string) Config {
	normalized := make([]LocaleConfig, 0, len(locales))
	seen := map[string]struct{}{}

	defaultCode := sharedi18n.NormalizeLocale(defaultLocale)
	if defaultCode == "" {
		defaultCode = "en"
	}

	for _, lc := range locales {
		code := sharedi18n.NormalizeLocale(lc)
		if code == "" {
			continue
		}

		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}

		normalized = append(normalized, LocaleConfig{Code: code})
	}

	// Ensure the default locale is always present.
	if _, ok := seen[defaultCode]; !ok {
		normalized = append([]LocaleConfig{{Code: defaultCode}}, normalized...)
	}

	return Config{
		DefaultLocale:     defaultCode,
		Locales:           normalized,
		TemplateHelperKey: "translate",
		LocaleContextKey:  "locale",
	}
}

// HelperConfig converts the module config into an interfaces.HelperConfig.
func (c Config) HelperConfig() interfaces.HelperConfig {
	return interfaces.HelperConfig{
		LocaleKey:         c.LocaleContextKey,
		TemplateHelperKey: c.TemplateHelperKey,
	}
}

// LocaleCodes returns the set of configured locale codes.
func (c Config) LocaleCodes() []string {
	if len(c.Locales) == 0 {
		return []string{c.DefaultLocale}
	}

	codes := make([]string, 0, len(c.Locales))
	for _, loc := range c.Locales {
		codes = append(codes, loc.Code)
	}
	codes = sharedi18n.NormalizeLocales(codes)

	if c.DefaultLocale != "" {
		if !containsLocale(codes, c.DefaultLocale) {
			codes = append([]string{c.DefaultLocale}, codes...)
		}
	}

	if len(codes) == 0 && c.DefaultLocale != "" {
		codes = append([]string{c.DefaultLocale}, codes...)
	}

	return codes
}

// Fallbacks returns the configured fallback chain for a locale.
func (c Config) Fallbacks(locale string) []string {
	code := sharedi18n.NormalizeLocale(locale)
	if code == "" {
		return []string{c.DefaultLocale}
	}

	for _, loc := range c.Locales {
		if sharedi18n.NormalizeLocale(loc.Code) != code {
			continue
		}

		if len(loc.Fallbacks) == 0 {
			if code == c.DefaultLocale {
				return []string{code}
			}

			return []string{code, c.DefaultLocale}
		}

		chain := append([]string{code}, loc.Fallbacks...)
		if c.DefaultLocale != "" {
			chain = append(chain, c.DefaultLocale)
		}
		return sharedi18n.NormalizeLocales(chain)
	}

	if c.DefaultLocale == "" {
		return []string{code}
	}

	return sharedi18n.NormalizeLocales([]string{code, c.DefaultLocale})
}

// WithFallbacks registers fallbacks for the provided locale code.
func (c *Config) WithFallbacks(locale string, fallbacks ...string) {
	target := sharedi18n.NormalizeLocale(locale)
	if target == "" {
		return
	}

	for i := range c.Locales {
		if sharedi18n.NormalizeLocale(c.Locales[i].Code) == target {
			c.Locales[i].Fallbacks = normalizeSlice(fallbacks)
			return
		}
	}

	c.Locales = append(c.Locales, LocaleConfig{
		Code:      target,
		Fallbacks: normalizeSlice(fallbacks),
	})
}

// MarshalJSON implements json.Marshaler ensuring we serialise unique locales.
func (c Config) MarshalJSON() ([]byte, error) {
	type alias Config
	return json.Marshal(alias(c))
}

// UnmarshalJSON implements json.Unmarshaler for Config.
func (c *Config) UnmarshalJSON(data []byte) error {
	type alias Config
	var aux alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("decode i18n config: %w", err)
	}

	aux.DefaultLocale = sharedi18n.NormalizeLocale(aux.DefaultLocale)
	aux.TemplateHelperKey = strings.TrimSpace(aux.TemplateHelperKey)
	aux.LocaleContextKey = strings.TrimSpace(aux.LocaleContextKey)

	if aux.DefaultLocale == "" && len(aux.Locales) > 0 {
		aux.DefaultLocale = sharedi18n.NormalizeLocale(aux.Locales[0].Code)
	}

	codes := map[string]struct{}{}
	locales := make([]LocaleConfig, 0, len(aux.Locales))

	for _, loc := range aux.Locales {
		code := sharedi18n.NormalizeLocale(loc.Code)
		if code == "" {
			continue
		}

		if _, ok := codes[code]; ok {
			continue
		}
		codes[code] = struct{}{}

		locales = append(locales, LocaleConfig{
			Code:      code,
			Fallbacks: normalizeSlice(loc.Fallbacks),
		})
	}

	aux.Locales = locales

	if aux.DefaultLocale == "" {
		aux.DefaultLocale = "en"
	}

	*c = Config(aux)
	return nil
}

func normalizeSlice(input []string) []string {
	return sharedi18n.NormalizeLocales(input)
}

func containsLocale(values []string, candidate string) bool {
	candidate = sharedi18n.NormalizeLocale(candidate)
	if candidate == "" {
		return false
	}
	for _, value := range values {
		if sharedi18n.NormalizeLocale(value) == candidate {
			return true
		}
	}
	return false
}
