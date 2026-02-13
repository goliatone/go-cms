package di

import (
	"context"
	"errors"
	"strings"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type markdownContentServiceAdapter struct {
	service content.Service
}

func newMarkdownContentServiceAdapter(service content.Service) interfaces.ContentService {
	if service == nil {
		return nil
	}
	return &markdownContentServiceAdapter{service: service}
}

func (a *markdownContentServiceAdapter) Create(ctx context.Context, req interfaces.ContentCreateRequest) (*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}

	translations := make([]content.ContentTranslationInput, 0, len(req.Translations))
	for _, tr := range req.Translations {
		translations = append(translations, content.ContentTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Content: cloneFieldMap(tr.Fields),
			Blocks:  cloneBlockSlice(tr.Blocks),
		})
	}

	record, err := a.service.Create(ctx, content.CreateContentRequest{
		ContentTypeID:            req.ContentTypeID,
		Slug:                     req.Slug,
		Status:                   req.Status,
		CreatedBy:                req.CreatedBy,
		UpdatedBy:                req.UpdatedBy,
		Metadata:                 cloneFieldMap(req.Metadata),
		Translations:             translations,
		AllowMissingTranslations: req.AllowMissingTranslations,
	})
	if err != nil {
		return nil, err
	}
	return toContentRecord(record), nil
}

func (a *markdownContentServiceAdapter) Update(ctx context.Context, req interfaces.ContentUpdateRequest) (*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}

	translations := make([]content.ContentTranslationInput, 0, len(req.Translations))
	for _, tr := range req.Translations {
		translations = append(translations, content.ContentTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Content: cloneFieldMap(tr.Fields),
			Blocks:  cloneBlockSlice(tr.Blocks),
		})
	}

	record, err := a.service.Update(ctx, content.UpdateContentRequest{
		ID:                       req.ID,
		Status:                   req.Status,
		UpdatedBy:                req.UpdatedBy,
		Translations:             translations,
		Metadata:                 cloneFieldMap(req.Metadata),
		AllowMissingTranslations: req.AllowMissingTranslations,
	})
	if err != nil {
		return nil, err
	}
	return toContentRecord(record), nil
}

func (a *markdownContentServiceAdapter) GetBySlug(ctx context.Context, slug string, opts interfaces.ContentReadOptions) (*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}
	envKey := strings.TrimSpace(opts.EnvironmentKey)
	var records []*content.Content
	var err error
	if envKey == "" {
		records, err = a.service.List(ctx)
	} else {
		records, err = a.service.List(ctx, envKey)
	}
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record != nil && strings.EqualFold(record.Slug, slug) {
			var availableLocales []string
			if opts.IncludeAvailableLocales && len(record.Translations) == 0 {
				locales, err := a.service.AvailableLocales(ctx, record.ID, interfaces.TranslationCheckOptions{
					Environment: opts.EnvironmentKey,
				})
				if err != nil {
					return nil, err
				}
				availableLocales = locales
			}
			result := toContentRecordWithOptions(record, opts, availableLocales)
			if err := validateTranslationAllowed(result, opts.AllowMissingTranslations); err != nil {
				return nil, err
			}
			return result, nil
		}
	}
	return nil, nil
}

func (a *markdownContentServiceAdapter) List(ctx context.Context, opts interfaces.ContentReadOptions) ([]*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}
	envKey := strings.TrimSpace(opts.EnvironmentKey)
	var records []*content.Content
	var err error
	if envKey == "" {
		records, err = a.service.List(ctx)
	} else {
		records, err = a.service.List(ctx, envKey)
	}
	if err != nil {
		return nil, err
	}
	out := make([]*interfaces.ContentRecord, 0, len(records))
	for _, record := range records {
		var availableLocales []string
		if opts.IncludeAvailableLocales && record != nil && len(record.Translations) == 0 {
			locales, err := a.service.AvailableLocales(ctx, record.ID, interfaces.TranslationCheckOptions{
				Environment: opts.EnvironmentKey,
			})
			if err != nil {
				return nil, err
			}
			availableLocales = locales
		}
		out = append(out, toContentRecordWithOptions(record, opts, availableLocales))
	}
	return out, nil
}

func (a *markdownContentServiceAdapter) CheckTranslations(ctx context.Context, id uuid.UUID, required []string, opts interfaces.TranslationCheckOptions) ([]string, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}
	return a.service.CheckTranslations(ctx, id, required, opts)
}

func (a *markdownContentServiceAdapter) AvailableLocales(ctx context.Context, id uuid.UUID, opts interfaces.TranslationCheckOptions) ([]string, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}
	return a.service.AvailableLocales(ctx, id, opts)
}

func (a *markdownContentServiceAdapter) Delete(ctx context.Context, req interfaces.ContentDeleteRequest) error {
	if a == nil || a.service == nil {
		return errors.New("content service unavailable")
	}
	return a.service.Delete(ctx, content.DeleteContentRequest{
		ID:         req.ID,
		DeletedBy:  req.DeletedBy,
		HardDelete: req.HardDelete,
	})
}

func (a *markdownContentServiceAdapter) CreateTranslation(ctx context.Context, req interfaces.ContentCreateTranslationRequest) (*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}
	creator, ok := a.service.(content.TranslationCreator)
	if !ok {
		return nil, errors.New("content translation create unsupported")
	}
	record, err := creator.CreateTranslation(ctx, content.CreateContentTranslationRequest{
		SourceID:       req.SourceID,
		SourceLocale:   req.SourceLocale,
		TargetLocale:   req.TargetLocale,
		EnvironmentKey: req.EnvironmentKey,
		ActorID:        req.ActorID,
		Status:         req.Status,
	})
	if err != nil {
		return nil, err
	}
	return toContentRecordWithOptions(record, interfaces.ContentReadOptions{
		Locale:                   req.TargetLocale,
		IncludeAvailableLocales:  true,
		AllowMissingTranslations: true,
		EnvironmentKey:           req.EnvironmentKey,
	}, nil), nil
}

func (a *markdownContentServiceAdapter) UpdateTranslation(ctx context.Context, req interfaces.ContentUpdateTranslationRequest) (*interfaces.ContentTranslation, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}

	translation, err := a.service.UpdateTranslation(ctx, content.UpdateContentTranslationRequest{
		ContentID: req.ContentID,
		Locale:    req.Locale,
		Title:     req.Title,
		Summary:   req.Summary,
		Content:   cloneFieldMap(req.Fields),
		Blocks:    cloneBlockSlice(req.Blocks),
		UpdatedBy: req.UpdatedBy,
	})
	if err != nil {
		return nil, err
	}
	return toInterfacesContentTranslation(translation), nil
}

func (a *markdownContentServiceAdapter) DeleteTranslation(ctx context.Context, req interfaces.ContentDeleteTranslationRequest) error {
	if a == nil || a.service == nil {
		return errors.New("content service unavailable")
	}
	return a.service.DeleteTranslation(ctx, content.DeleteContentTranslationRequest{
		ContentID: req.ContentID,
		Locale:    req.Locale,
		DeletedBy: req.DeletedBy,
	})
}

func toContentRecord(record *content.Content) *interfaces.ContentRecord {
	return toContentRecordWithOptions(record, interfaces.ContentReadOptions{}, nil)
}

func toContentRecordWithOptions(record *content.Content, opts interfaces.ContentReadOptions, availableLocales []string) *interfaces.ContentRecord {
	if record == nil {
		return nil
	}
	typeSlug := ""
	if record.Type != nil {
		typeSlug = record.Type.Slug
	}
	translations := make([]interfaces.ContentTranslation, 0, len(record.Translations))
	for _, tr := range record.Translations {
		if tr == nil {
			continue
		}
		locale := ""
		if tr.Locale != nil {
			locale = tr.Locale.Code
		}
		translations = append(translations, interfaces.ContentTranslation{
			ID:                 tr.ID,
			TranslationGroupID: tr.TranslationGroupID,
			Locale:             locale,
			Title:              tr.Title,
			Summary:            tr.Summary,
			Fields:             cloneFieldMap(tr.Content),
		})
	}
	return &interfaces.ContentRecord{
		ID:              record.ID,
		ContentType:     record.ContentTypeID,
		ContentTypeSlug: typeSlug,
		Slug:            record.Slug,
		Status:          record.Status,
		Translation:     buildContentTranslationBundle(translations, opts, record.PrimaryLocale, availableLocales),
		Metadata:        cloneFieldMap(record.Metadata),
	}
}

func toInterfacesContentTranslation(record *content.ContentTranslation) *interfaces.ContentTranslation {
	if record == nil {
		return nil
	}
	locale := ""
	if record.Locale != nil {
		locale = record.Locale.Code
	}
	return &interfaces.ContentTranslation{
		ID:                 record.ID,
		TranslationGroupID: record.TranslationGroupID,
		Locale:             locale,
		Title:              record.Title,
		Summary:            record.Summary,
		Fields:             cloneFieldMap(record.Content),
	}
}

func cloneFieldMap(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		out[k] = v
	}
	return out
}

func cloneBlockSlice(blocks []map[string]any) []map[string]any {
	if len(blocks) == 0 {
		return nil
	}
	out := make([]map[string]any, len(blocks))
	for i, block := range blocks {
		out[i] = cloneFieldMap(block)
	}
	return out
}

func buildContentTranslationBundle(translations []interfaces.ContentTranslation, opts interfaces.ContentReadOptions, primaryLocale string, availableLocales []string) interfaces.TranslationBundle[interfaces.ContentTranslation] {
	requestedLocale := strings.TrimSpace(opts.Locale)
	fallbackLocale := strings.TrimSpace(opts.FallbackLocale)
	meta := interfaces.TranslationMeta{
		RequestedLocale: requestedLocale,
		PrimaryLocale:   strings.TrimSpace(primaryLocale),
	}

	if opts.IncludeAvailableLocales {
		if len(availableLocales) > 0 {
			meta.AvailableLocales = append([]string(nil), availableLocales...)
		} else {
			meta.AvailableLocales = collectLocalesFromContentTranslations(translations)
		}
	}

	var requested *interfaces.ContentTranslation
	var resolved *interfaces.ContentTranslation
	resolvedLocale := ""

	if requestedLocale != "" {
		if tr := findContentTranslation(translations, requestedLocale); tr != nil {
			requested = tr
			resolved = tr
			resolvedLocale = tr.Locale
		}
	}
	if requested == nil && requestedLocale != "" && fallbackLocale != "" {
		if tr := findContentTranslation(translations, fallbackLocale); tr != nil {
			resolved = tr
			resolvedLocale = tr.Locale
			meta.FallbackUsed = true
		}
	}

	meta.ResolvedLocale = strings.TrimSpace(resolvedLocale)
	meta.MissingRequestedLocale = requestedLocale != "" && requested == nil

	return interfaces.TranslationBundle[interfaces.ContentTranslation]{
		Meta:      meta,
		Requested: requested,
		Resolved:  resolved,
	}
}

func validateTranslationAllowed(record *interfaces.ContentRecord, allowMissing bool) error {
	if allowMissing || record == nil {
		return nil
	}
	meta := record.Translation.Meta
	if meta.RequestedLocale != "" && meta.MissingRequestedLocale {
		return interfaces.ErrTranslationMissing
	}
	return nil
}

func findContentTranslation(translations []interfaces.ContentTranslation, locale string) *interfaces.ContentTranslation {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return nil
	}
	for _, tr := range translations {
		if strings.EqualFold(strings.TrimSpace(tr.Locale), locale) {
			copy := tr
			return &copy
		}
	}
	return nil
}

func collectLocalesFromContentTranslations(translations []interfaces.ContentTranslation) []string {
	if len(translations) == 0 {
		return nil
	}
	locales := make([]string, 0, len(translations))
	for _, tr := range translations {
		code := strings.TrimSpace(tr.Locale)
		if code == "" {
			continue
		}
		locales = append(locales, code)
	}
	if len(locales) == 0 {
		return nil
	}
	return locales
}
