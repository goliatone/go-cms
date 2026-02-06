package pages

import (
	"context"
	"fmt"
	"strings"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// CheckTranslations reports which required locales are missing page translations.
func (s *pageService) CheckTranslations(ctx context.Context, id uuid.UUID, required []string, opts interfaces.TranslationCheckOptions) ([]string, error) {
	if id == uuid.Nil {
		return nil, ErrPageRequired
	}
	requiredLocales, _ := normalizeTranslationCheckLocales(required)
	if len(requiredLocales) == 0 {
		return nil, nil
	}

	record, err := s.pages.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureTranslationCheckEnvironment(ctx, record.EnvironmentID, opts.Environment); err != nil {
		return nil, err
	}

	logger := s.opLogger(ctx, "pages.translations.check", map[string]any{
		"page_id": id,
	})

	requiredFields := normalizeRequiredFields(opts.RequiredFields)
	strategy := normalizeRequiredFieldsStrategy(opts.RequiredFieldsStrategy)

	missing := make([]string, 0)
	for _, locale := range requiredLocales {
		key := strings.ToLower(locale)
		fields := requiredFields[key]
		if len(fields) > 0 {
			known, unknown := splitUnknownPageFields(fields)
			if len(unknown) > 0 {
				if err := handleUnknownRequiredFields(logger, "pages", locale, unknown, strategy); err != nil {
					return nil, err
				}
			}
			fields = known
		}

		localeID, err := s.localeIDForCheck(ctx, locale)
		if err != nil {
			return nil, err
		}

		var translation *PageTranslation
		for _, tr := range record.Translations {
			if pageTranslationMatches(tr, localeID, locale) {
				translation = tr
				break
			}
		}
		if translation == nil {
			missing = append(missing, locale)
			continue
		}
		if len(fields) > 0 && !pageTranslationHasFields(translation, fields) {
			missing = append(missing, locale)
		}
	}

	if len(missing) == 0 {
		return nil, nil
	}
	return missing, nil
}

// AvailableLocales returns the locales available for the page translations.
func (s *pageService) AvailableLocales(ctx context.Context, id uuid.UUID, opts interfaces.TranslationCheckOptions) ([]string, error) {
	if id == uuid.Nil {
		return nil, ErrPageRequired
	}
	record, err := s.pages.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureTranslationCheckEnvironment(ctx, record.EnvironmentID, opts.Environment); err != nil {
		return nil, err
	}
	return collectPageTranslationLocales(ctx, s.locales, record), nil
}

func (s *pageService) ensureTranslationCheckEnvironment(ctx context.Context, envID uuid.UUID, key string) error {
	if err := s.ensureEnvironmentActive(ctx, envID); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return nil
	}
	resolvedID, _, err := s.resolveEnvironment(ctx, key)
	if err != nil {
		return err
	}
	if envID == uuid.Nil {
		if s.requireExplicitEnv {
			return cmsenv.ErrEnvironmentNotFound
		}
		defaultID, _, err := s.resolveEnvironment(ctx, "")
		if err != nil {
			return err
		}
		envID = defaultID
	}
	if envID != resolvedID {
		return cmsenv.ErrEnvironmentNotFound
	}
	return nil
}

func (s *pageService) localeIDForCheck(ctx context.Context, code string) (uuid.UUID, error) {
	if s.locales == nil {
		return uuid.Nil, ErrUnknownLocale
	}
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return uuid.Nil, ErrUnknownLocale
	}
	if parsed, err := uuid.Parse(trimmed); err == nil {
		locale, err := s.locales.GetByID(ctx, parsed)
		if err != nil || locale == nil {
			return uuid.Nil, ErrUnknownLocale
		}
		return locale.ID, nil
	}
	locale, err := s.locales.GetByCode(ctx, trimmed)
	if err != nil || locale == nil {
		return uuid.Nil, ErrUnknownLocale
	}
	return locale.ID, nil
}

func normalizeTranslationCheckLocales(required []string) ([]string, map[string]string) {
	if len(required) == 0 {
		return nil, nil
	}
	locales := make([]string, 0, len(required))
	seen := map[string]string{}
	for _, locale := range required {
		trimmed := strings.TrimSpace(locale)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = trimmed
		locales = append(locales, trimmed)
	}
	if len(locales) == 0 {
		return nil, nil
	}
	return locales, seen
}

func normalizeRequiredFields(fields map[string][]string) map[string][]string {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string][]string, len(fields))
	for locale, entries := range fields {
		key := strings.ToLower(strings.TrimSpace(locale))
		if key == "" {
			continue
		}
		seen := map[string]struct{}{}
		clean := make([]string, 0, len(entries))
		for _, field := range entries {
			trimmed := strings.TrimSpace(field)
			if trimmed == "" {
				continue
			}
			lowered := strings.ToLower(trimmed)
			if _, ok := seen[lowered]; ok {
				continue
			}
			seen[lowered] = struct{}{}
			clean = append(clean, lowered)
		}
		if len(clean) > 0 {
			out[key] = clean
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeRequiredFieldsStrategy(strategy interfaces.RequiredFieldsValidationStrategy) interfaces.RequiredFieldsValidationStrategy {
	switch strings.ToLower(strings.TrimSpace(string(strategy))) {
	case string(interfaces.RequiredFieldsValidationWarn):
		return interfaces.RequiredFieldsValidationWarn
	case string(interfaces.RequiredFieldsValidationIgnore):
		return interfaces.RequiredFieldsValidationIgnore
	case string(interfaces.RequiredFieldsValidationError):
		return interfaces.RequiredFieldsValidationError
	default:
		return interfaces.RequiredFieldsValidationError
	}
}

func handleUnknownRequiredFields(logger interfaces.Logger, entity string, locale string, unknown []string, strategy interfaces.RequiredFieldsValidationStrategy) error {
	if len(unknown) == 0 {
		return nil
	}
	message := fmt.Sprintf("%s: unknown required fields for locale %q: %s", entity, locale, strings.Join(unknown, ", "))
	switch strategy {
	case interfaces.RequiredFieldsValidationWarn:
		if logger != nil {
			logger.Warn(message)
		}
		return nil
	case interfaces.RequiredFieldsValidationIgnore:
		return nil
	default:
		return fmt.Errorf("%s", message)
	}
}

func pageTranslationMatches(tr *PageTranslation, localeID uuid.UUID, code string) bool {
	if tr == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(tr.Locale), strings.TrimSpace(code)) {
		return true
	}
	return localeID != uuid.Nil && tr.LocaleID == localeID
}

func splitUnknownPageFields(fields []string) ([]string, []string) {
	if len(fields) == 0 {
		return nil, nil
	}
	known := make([]string, 0, len(fields))
	unknown := make([]string, 0)
	for _, field := range fields {
		if _, ok := pageRequiredFieldKeys[field]; ok {
			known = append(known, field)
		} else {
			unknown = append(unknown, field)
		}
	}
	return known, unknown
}

var pageRequiredFieldKeys = map[string]struct{}{
	"title":           {},
	"path":            {},
	"summary":         {},
	"seo_title":       {},
	"seo_description": {},
}

func pageTranslationHasFields(tr *PageTranslation, fields []string) bool {
	if tr == nil {
		return false
	}
	for _, field := range fields {
		switch field {
		case "title":
			if strings.TrimSpace(tr.Title) == "" {
				return false
			}
		case "path":
			if strings.TrimSpace(tr.Path) == "" {
				return false
			}
		case "summary":
			if tr.Summary == nil || strings.TrimSpace(*tr.Summary) == "" {
				return false
			}
		case "seo_title":
			if tr.SEOTitle == nil || strings.TrimSpace(*tr.SEOTitle) == "" {
				return false
			}
		case "seo_description":
			if tr.SEODescription == nil || strings.TrimSpace(*tr.SEODescription) == "" {
				return false
			}
		}
	}
	return true
}

func collectPageTranslationLocales(ctx context.Context, locales LocaleRepository, record *Page) []string {
	if record == nil || len(record.Translations) == 0 {
		return nil
	}
	localesList := make([]string, 0, len(record.Translations))
	seen := map[string]struct{}{}
	for _, tr := range record.Translations {
		if tr == nil {
			continue
		}
		code := strings.TrimSpace(tr.Locale)
		if code == "" && locales != nil && tr.LocaleID != uuid.Nil {
			code = localeCodeByID(ctx, locales, tr.LocaleID)
		}
		if code == "" {
			code = tr.LocaleID.String()
		}
		if code == "" {
			continue
		}
		key := strings.ToLower(code)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		localesList = append(localesList, code)
	}
	if len(localesList) == 0 {
		return nil
	}
	return localesList
}

func localeCodeByID(ctx context.Context, locales LocaleRepository, id uuid.UUID) string {
	if locales == nil || id == uuid.Nil {
		return ""
	}
	locale, err := locales.GetByID(ctx, id)
	if err != nil || locale == nil {
		return ""
	}
	return strings.TrimSpace(locale.Code)
}
