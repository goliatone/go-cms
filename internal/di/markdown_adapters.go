package di

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
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
		Metadata:        nil,
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

type markdownPageServiceAdapter struct {
	service pages.Service

	mu              sync.RWMutex
	translationMeta map[uuid.UUID]map[uuid.UUID]pageTranslationMeta
	pageMetadata    map[uuid.UUID]map[string]any
}

type pageTranslationMeta struct {
	Locale string
	Fields map[string]any
}

func newMarkdownPageServiceAdapter(service pages.Service) interfaces.PageService {
	if service == nil {
		return nil
	}
	return &markdownPageServiceAdapter{
		service:         service,
		translationMeta: map[uuid.UUID]map[uuid.UUID]pageTranslationMeta{},
		pageMetadata:    map[uuid.UUID]map[string]any{},
	}
}

func (a *markdownPageServiceAdapter) Create(ctx context.Context, req interfaces.PageCreateRequest) (*interfaces.PageRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}

	translations := make([]pages.PageTranslationInput, 0, len(req.Translations))
	for _, tr := range req.Translations {
		translations = append(translations, pages.PageTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Path:    tr.Path,
			Summary: tr.Summary,
		})
	}

	record, err := a.service.Create(ctx, pages.CreatePageRequest{
		ContentID:                req.ContentID,
		TemplateID:               req.TemplateID,
		ParentID:                 req.ParentID,
		Slug:                     req.Slug,
		Status:                   req.Status,
		CreatedBy:                req.CreatedBy,
		UpdatedBy:                req.UpdatedBy,
		Translations:             translations,
		AllowMissingTranslations: req.AllowMissingTranslations,
	})
	if err != nil {
		return nil, err
	}

	a.setPageMetadata(record.ID, req.Metadata)
	a.storePageMeta(record, req.Translations)
	return a.toPageRecord(record), nil
}

func (a *markdownPageServiceAdapter) Update(ctx context.Context, req interfaces.PageUpdateRequest) (*interfaces.PageRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}
	if req.ID == uuid.Nil {
		return nil, errors.New("page id required")
	}

	existing, err := a.service.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("page %s not found", req.ID)
	}

	templateID := existing.TemplateID
	if req.TemplateID != nil && *req.TemplateID != uuid.Nil {
		templateID = *req.TemplateID
	}

	if err := a.service.Delete(ctx, pages.DeletePageRequest{
		ID:         req.ID,
		DeletedBy:  req.UpdatedBy,
		HardDelete: true,
	}); err != nil {
		return nil, err
	}
	a.clearPageMeta(req.ID)

	translations := make([]pages.PageTranslationInput, 0, len(req.Translations))
	for _, tr := range req.Translations {
		translations = append(translations, pages.PageTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Path:    tr.Path,
			Summary: tr.Summary,
		})
	}

	created, err := a.service.Create(ctx, pages.CreatePageRequest{
		ContentID:                existing.ContentID,
		TemplateID:               templateID,
		ParentID:                 existing.ParentID,
		Slug:                     existing.Slug,
		Status:                   req.Status,
		CreatedBy:                req.UpdatedBy,
		UpdatedBy:                req.UpdatedBy,
		Translations:             translations,
		AllowMissingTranslations: req.AllowMissingTranslations,
	})
	if err != nil {
		return nil, err
	}

	a.setPageMetadata(created.ID, req.Metadata)
	a.storePageMeta(created, req.Translations)
	return a.toPageRecord(created), nil
}

func (a *markdownPageServiceAdapter) GetBySlug(ctx context.Context, slug string, env ...string) (*interfaces.PageRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}
	records, err := a.service.List(ctx, env...)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record != nil && strings.EqualFold(record.Slug, slug) {
			return a.toPageRecord(record), nil
		}
	}
	return nil, nil
}

func (a *markdownPageServiceAdapter) List(ctx context.Context, env ...string) ([]*interfaces.PageRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}
	records, err := a.service.List(ctx, env...)
	if err != nil {
		return nil, err
	}
	out := make([]*interfaces.PageRecord, 0, len(records))
	for _, record := range records {
		out = append(out, a.toPageRecord(record))
	}
	return out, nil
}

func (a *markdownPageServiceAdapter) Delete(ctx context.Context, req interfaces.PageDeleteRequest) error {
	if a == nil || a.service == nil {
		return errors.New("page service unavailable")
	}
	if err := a.service.Delete(ctx, pages.DeletePageRequest{
		ID:         req.ID,
		DeletedBy:  req.DeletedBy,
		HardDelete: req.HardDelete,
	}); err != nil {
		return err
	}
	a.clearPageMeta(req.ID)
	return nil
}

func (a *markdownPageServiceAdapter) UpdateTranslation(ctx context.Context, req interfaces.PageUpdateTranslationRequest) (*interfaces.PageTranslation, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}
	translation, err := a.service.UpdateTranslation(ctx, pages.UpdatePageTranslationRequest{
		PageID:    req.PageID,
		Locale:    req.Locale,
		Title:     req.Title,
		Path:      req.Path,
		Summary:   req.Summary,
		UpdatedBy: req.UpdatedBy,
	})
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	if a.translationMeta == nil {
		a.translationMeta = map[uuid.UUID]map[uuid.UUID]pageTranslationMeta{}
	}
	meta := a.translationMeta[req.PageID]
	if meta == nil {
		meta = map[uuid.UUID]pageTranslationMeta{}
	}
	meta[translation.ID] = pageTranslationMeta{
		Locale: req.Locale,
		Fields: cloneFieldMap(req.Fields),
	}
	a.translationMeta[req.PageID] = meta
	a.mu.Unlock()

	return &interfaces.PageTranslation{
		ID:      translation.ID,
		Locale:  req.Locale,
		Title:   translation.Title,
		Path:    translation.Path,
		Summary: translation.Summary,
		Fields:  cloneFieldMap(req.Fields),
	}, nil
}

func (a *markdownPageServiceAdapter) DeleteTranslation(ctx context.Context, req interfaces.PageDeleteTranslationRequest) error {
	if a == nil || a.service == nil {
		return errors.New("page service unavailable")
	}
	if err := a.service.DeleteTranslation(ctx, pages.DeletePageTranslationRequest{
		PageID:    req.PageID,
		Locale:    req.Locale,
		DeletedBy: req.DeletedBy,
	}); err != nil {
		return err
	}

	a.mu.Lock()
	if a.translationMeta != nil {
		meta := a.translationMeta[req.PageID]
		for id, info := range meta {
			if strings.EqualFold(info.Locale, req.Locale) {
				delete(meta, id)
			}
		}
		a.translationMeta[req.PageID] = meta
	}
	a.mu.Unlock()
	return nil
}

func (a *markdownPageServiceAdapter) Move(ctx context.Context, req interfaces.PageMoveRequest) (*interfaces.PageRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}
	record, err := a.service.Move(ctx, pages.MovePageRequest{
		PageID:      req.PageID,
		NewParentID: req.NewParentID,
		ActorID:     req.ActorID,
	})
	if err != nil {
		return nil, err
	}
	return a.toPageRecord(record), nil
}

func (a *markdownPageServiceAdapter) Duplicate(ctx context.Context, req interfaces.PageDuplicateRequest) (*interfaces.PageRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}
	record, err := a.service.Duplicate(ctx, pages.DuplicatePageRequest{
		PageID:    req.PageID,
		Slug:      req.Slug,
		ParentID:  req.ParentID,
		Status:    req.Status,
		CreatedBy: req.CreatedBy,
		UpdatedBy: req.UpdatedBy,
	})
	if err != nil {
		return nil, err
	}
	// No translation metadata available for duplicates; ensure clean slate.
	a.clearPageMeta(record.ID)
	return a.toPageRecord(record), nil
}

func (a *markdownPageServiceAdapter) toPageRecord(record *pages.Page) *interfaces.PageRecord {
	if record == nil {
		return nil
	}
	meta := a.metaForPage(record.ID)
	translations := make([]interfaces.PageTranslation, 0, len(record.Translations))
	for _, tr := range record.Translations {
		if tr == nil {
			continue
		}
		info := meta[tr.ID]
		translations = append(translations, interfaces.PageTranslation{
			ID:      tr.ID,
			Locale:  info.Locale,
			Title:   tr.Title,
			Path:    tr.Path,
			Summary: tr.Summary,
			Fields:  cloneFieldMap(info.Fields),
		})
	}
	return &interfaces.PageRecord{
		ID:           record.ID,
		ContentID:    record.ContentID,
		TemplateID:   record.TemplateID,
		Slug:         record.Slug,
		Status:       record.Status,
		Translations: translations,
		Metadata:     a.pageMetaClone(record.ID),
	}
}

func (a *markdownPageServiceAdapter) setPageMetadata(pageID uuid.UUID, metadata map[string]any) {
	if pageID == uuid.Nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pageMetadata == nil {
		a.pageMetadata = map[uuid.UUID]map[string]any{}
	}
	if len(metadata) == 0 {
		delete(a.pageMetadata, pageID)
		return
	}
	a.pageMetadata[pageID] = cloneFieldMap(metadata)
}

func (a *markdownPageServiceAdapter) pageMetaClone(pageID uuid.UUID) map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.pageMetadata == nil {
		return nil
	}
	return cloneFieldMap(a.pageMetadata[pageID])
}

func (a *markdownPageServiceAdapter) clearPageMeta(pageID uuid.UUID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.translationMeta != nil {
		delete(a.translationMeta, pageID)
	}
	if a.pageMetadata != nil {
		delete(a.pageMetadata, pageID)
	}
}

func (a *markdownPageServiceAdapter) metaForPage(pageID uuid.UUID) map[uuid.UUID]pageTranslationMeta {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.translationMeta == nil {
		return map[uuid.UUID]pageTranslationMeta{}
	}
	meta, ok := a.translationMeta[pageID]
	if !ok {
		return map[uuid.UUID]pageTranslationMeta{}
	}
	out := make(map[uuid.UUID]pageTranslationMeta, len(meta))
	for id, info := range meta {
		out[id] = pageTranslationMeta{
			Locale: info.Locale,
			Fields: cloneFieldMap(info.Fields),
		}
	}
	return out
}

func (a *markdownPageServiceAdapter) storePageMeta(record *pages.Page, inputs []interfaces.PageTranslationInput) {
	if record == nil || len(record.Translations) == 0 {
		return
	}
	index := make(map[string]interfaces.PageTranslationInput, len(inputs))
	for _, in := range inputs {
		key := strings.ToLower(strings.TrimSpace(in.Path))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(in.Locale))
		}
		index[key] = in
	}
	meta := make(map[uuid.UUID]pageTranslationMeta, len(record.Translations))
	for _, tr := range record.Translations {
		if tr == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(tr.Path))
		in := index[key]
		meta[tr.ID] = pageTranslationMeta{
			Locale: strings.TrimSpace(in.Locale),
			Fields: cloneFieldMap(in.Fields),
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.translationMeta == nil {
		a.translationMeta = map[uuid.UUID]map[uuid.UUID]pageTranslationMeta{}
	}
	a.translationMeta[record.ID] = meta
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
