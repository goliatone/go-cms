package di

import (
	"context"
	"errors"
	"strings"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/interfaces"
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

func (a *markdownContentServiceAdapter) GetBySlug(ctx context.Context, slug string, env ...string) (*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}
	records, err := a.service.List(ctx, env...)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record != nil && strings.EqualFold(record.Slug, slug) {
			return toContentRecord(record), nil
		}
	}
	return nil, nil
}

func (a *markdownContentServiceAdapter) List(ctx context.Context, env ...string) ([]*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}
	records, err := a.service.List(ctx, env...)
	if err != nil {
		return nil, err
	}
	out := make([]*interfaces.ContentRecord, 0, len(records))
	for _, record := range records {
		out = append(out, toContentRecord(record))
	}
	return out, nil
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
			ID:      tr.ID,
			Locale:  locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Fields:  cloneFieldMap(tr.Content),
		})
	}
	return &interfaces.ContentRecord{
		ID:              record.ID,
		ContentType:     record.ContentTypeID,
		ContentTypeSlug: typeSlug,
		Slug:            record.Slug,
		Status:          record.Status,
		Translations:    translations,
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
		ID:      record.ID,
		Locale:  locale,
		Title:   record.Title,
		Summary: record.Summary,
		Fields:  cloneFieldMap(record.Content),
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
