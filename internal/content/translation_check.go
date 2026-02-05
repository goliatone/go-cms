package content

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/goliatone/go-cms/internal/domain"
	cmsenv "github.com/goliatone/go-cms/internal/environments"
	cmsschema "github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-cms/internal/validation"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// CheckTranslations reports which required locales are missing translations.
func (s *service) CheckTranslations(ctx context.Context, id uuid.UUID, required []string, opts interfaces.TranslationCheckOptions) ([]string, error) {
	if id == uuid.Nil {
		return nil, ErrContentIDRequired
	}
	requiredLocales, requiredIndex := normalizeTranslationCheckLocales(required)
	if len(requiredLocales) == 0 {
		return nil, nil
	}

	record, err := s.contents.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureTranslationCheckEnvironment(ctx, record.EnvironmentID, opts.Environment); err != nil {
		return nil, err
	}

	logger := s.opLogger(ctx, "content.translations.check", map[string]any{
		"content_id": id,
	})

	translations, snapshots, err := s.translationSourceForCheck(ctx, record, opts)
	if err != nil {
		return nil, err
	}

	requiredFields := normalizeRequiredFields(opts.RequiredFields)
	strategy := normalizeRequiredFieldsStrategy(opts.RequiredFieldsStrategy)

	var schemaFields map[string]struct{}
	if hasLocaleFieldRequirements(requiredIndex, requiredFields) {
		s.attachContentType(ctx, record)
		if record.Type != nil {
			normalized := validation.NormalizeSchema(record.Type.Schema)
			schemaFields = cmsschema.FieldPaths(normalized)
		}
	}

	missing := make([]string, 0)
	for _, locale := range requiredLocales {
		key := strings.ToLower(locale)
		fields := requiredFields[key]

		if len(fields) > 0 {
			known, unknown := splitUnknownFields(fields, schemaFields)
			if len(unknown) > 0 {
				if err := handleUnknownRequiredFields(logger, "content", locale, unknown, strategy); err != nil {
					return nil, err
				}
			}
			fields = known
		}

		found := false
		if snapshots != nil {
			var translation *ContentVersionTranslationSnapshot
			for idx, tr := range snapshots {
				if strings.EqualFold(strings.TrimSpace(tr.Locale), locale) {
					translation = &snapshots[idx]
					break
				}
			}
			if translation != nil {
				found = true
				if len(fields) > 0 && !contentFieldPathsHaveValue(translation.Content, fields) {
					missing = append(missing, locale)
				}
			}
		} else {
			localeID, err := s.localeIDForCheck(ctx, locale)
			if err != nil {
				return nil, err
			}
			var translation *ContentTranslation
			for _, tr := range translations {
				if contentTranslationMatches(tr, localeID, locale) {
					translation = tr
					break
				}
			}
			if translation != nil {
				found = true
				if len(fields) > 0 && !contentFieldPathsHaveValue(translation.Content, fields) {
					missing = append(missing, locale)
				}
			}
		}

		if !found {
			missing = append(missing, locale)
		}
	}

	if len(missing) == 0 {
		return nil, nil
	}
	return missing, nil
}

// AvailableLocales returns the locales available for the selected translation source.
func (s *service) AvailableLocales(ctx context.Context, id uuid.UUID, opts interfaces.TranslationCheckOptions) ([]string, error) {
	if id == uuid.Nil {
		return nil, ErrContentIDRequired
	}
	record, err := s.contents.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureTranslationCheckEnvironment(ctx, record.EnvironmentID, opts.Environment); err != nil {
		return nil, err
	}

	translations, snapshots, err := s.translationSourceForCheck(ctx, record, opts)
	if err != nil {
		return nil, err
	}

	if snapshots != nil {
		return collectSnapshotLocales(snapshots), nil
	}
	return collectContentTranslationLocales(translations), nil
}

func (s *service) translationSourceForCheck(ctx context.Context, record *Content, opts interfaces.TranslationCheckOptions) ([]*ContentTranslation, []ContentVersionTranslationSnapshot, error) {
	if record == nil {
		return nil, nil, nil
	}
	version, hasVersion, err := parseTranslationVersion(opts.Version)
	if err != nil {
		return nil, nil, err
	}
	if hasVersion {
		if !s.versioningEnabled {
			return nil, nil, ErrVersioningDisabled
		}
		versionRecord, err := s.contents.GetVersion(ctx, record.ID, version)
		if err != nil {
			return nil, nil, err
		}
		snapshot := append([]ContentVersionTranslationSnapshot(nil), versionRecord.Snapshot.Translations...)
		if snapshot == nil {
			snapshot = []ContentVersionTranslationSnapshot{}
		}
		return nil, snapshot, nil
	}

	state := domain.NormalizeWorkflowState(opts.State)
	if state == domain.WorkflowStatePublished && record.PublishedVersion != nil && s.versioningEnabled {
		versionRecord, err := s.contents.GetVersion(ctx, record.ID, *record.PublishedVersion)
		if err != nil {
			return nil, nil, err
		}
		snapshot := append([]ContentVersionTranslationSnapshot(nil), versionRecord.Snapshot.Translations...)
		if snapshot == nil {
			snapshot = []ContentVersionTranslationSnapshot{}
		}
		return nil, snapshot, nil
	}

	return record.Translations, nil, nil
}

func (s *service) ensureTranslationCheckEnvironment(ctx context.Context, envID uuid.UUID, key string) error {
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
	if envID != uuid.Nil && envID != resolvedID {
		return cmsenv.ErrEnvironmentNotFound
	}
	return nil
}

func (s *service) localeIDForCheck(ctx context.Context, code string) (uuid.UUID, error) {
	if s.locales == nil {
		return uuid.Nil, ErrUnknownLocale
	}
	locale, err := s.locales.GetByCode(ctx, code)
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
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			clean = append(clean, trimmed)
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

func hasLocaleFieldRequirements(requiredLocales map[string]string, requiredFields map[string][]string) bool {
	if len(requiredLocales) == 0 || len(requiredFields) == 0 {
		return false
	}
	for locale := range requiredLocales {
		if len(requiredFields[locale]) > 0 {
			return true
		}
	}
	return false
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

func splitUnknownFields(fields []string, known map[string]struct{}) ([]string, []string) {
	if len(fields) == 0 {
		return nil, nil
	}
	if len(known) == 0 {
		return nil, append([]string(nil), fields...)
	}
	knownFields := make([]string, 0, len(fields))
	unknownFields := make([]string, 0)
	for _, field := range fields {
		if _, ok := known[field]; ok {
			knownFields = append(knownFields, field)
		} else {
			unknownFields = append(unknownFields, field)
		}
	}
	return knownFields, unknownFields
}

func parseTranslationVersion(raw string) (int, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return 0, true, fmt.Errorf("content: invalid version %q", raw)
	}
	return value, true, nil
}

func contentTranslationMatches(tr *ContentTranslation, localeID uuid.UUID, code string) bool {
	if tr == nil {
		return false
	}
	if tr.Locale != nil && strings.EqualFold(strings.TrimSpace(tr.Locale.Code), strings.TrimSpace(code)) {
		return true
	}
	return localeID != uuid.Nil && tr.LocaleID == localeID
}

func collectSnapshotLocales(translations []ContentVersionTranslationSnapshot) []string {
	if len(translations) == 0 {
		return nil
	}
	locales := make([]string, 0, len(translations))
	seen := map[string]struct{}{}
	for _, tr := range translations {
		code := strings.TrimSpace(tr.Locale)
		if code == "" {
			continue
		}
		key := strings.ToLower(code)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		locales = append(locales, code)
	}
	if len(locales) == 0 {
		return nil
	}
	return locales
}

func collectContentTranslationLocales(translations []*ContentTranslation) []string {
	if len(translations) == 0 {
		return nil
	}
	locales := make([]string, 0, len(translations))
	seen := map[string]struct{}{}
	for _, tr := range translations {
		if tr == nil {
			continue
		}
		code := ""
		if tr.Locale != nil {
			code = strings.TrimSpace(tr.Locale.Code)
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
		locales = append(locales, code)
	}
	if len(locales) == 0 {
		return nil
	}
	return locales
}

type fieldSegment struct {
	name    string
	isArray bool
	index   *int
}

func parseFieldSegment(segment string) fieldSegment {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return fieldSegment{}
	}
	if strings.HasSuffix(segment, "[]") {
		return fieldSegment{name: strings.TrimSuffix(segment, "[]"), isArray: true}
	}
	open := strings.Index(segment, "[")
	close := strings.LastIndex(segment, "]")
	if open >= 0 && close > open {
		name := segment[:open]
		indexStr := strings.TrimSpace(segment[open+1 : close])
		if indexStr == "" {
			return fieldSegment{name: name, isArray: true}
		}
		if idx, err := strconv.Atoi(indexStr); err == nil {
			return fieldSegment{name: name, isArray: true, index: &idx}
		}
		return fieldSegment{name: name}
	}
	return fieldSegment{name: segment}
}

func parseFieldPath(path string) []fieldSegment {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
	segments := make([]fieldSegment, 0, len(parts))
	for _, part := range parts {
		segment := parseFieldSegment(part)
		if segment.name == "" && !segment.isArray {
			continue
		}
		segments = append(segments, segment)
	}
	return segments
}

func contentFieldPathsHaveValue(payload map[string]any, fields []string) bool {
	if len(fields) == 0 {
		return true
	}
	for _, field := range fields {
		segments := parseFieldPath(field)
		if !fieldSegmentsHaveValue(payload, segments) {
			return false
		}
	}
	return true
}

func fieldSegmentsHaveValue(value any, segments []fieldSegment) bool {
	if len(segments) == 0 {
		return hasMeaningfulValue(value)
	}
	segment := segments[0]
	if segment.name != "" {
		typed, ok := value.(map[string]any)
		if !ok {
			return false
		}
		next, ok := typed[segment.name]
		if !ok {
			return false
		}
		value = next
	}
	if segment.isArray {
		list, ok := value.([]any)
		if !ok {
			return false
		}
		if segment.index != nil {
			if *segment.index < 0 || *segment.index >= len(list) {
				return false
			}
			return fieldSegmentsHaveValue(list[*segment.index], segments[1:])
		}
		if len(list) == 0 {
			return false
		}
		if len(segments) == 1 {
			return hasMeaningfulValue(list)
		}
		for _, entry := range list {
			if fieldSegmentsHaveValue(entry, segments[1:]) {
				return true
			}
		}
		return false
	}
	return fieldSegmentsHaveValue(value, segments[1:])
}

func hasMeaningfulValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case []string:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}
