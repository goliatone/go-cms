package di

import (
	"context"
	"errors"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type contentTypeServiceAdapter struct {
	service content.ContentTypeService
}

func newContentTypeServiceAdapter(service content.ContentTypeService) interfaces.ContentTypeService {
	if service == nil {
		return nil
	}
	return &contentTypeServiceAdapter{service: service}
}

func (a *contentTypeServiceAdapter) Create(ctx context.Context, req interfaces.ContentTypeCreateRequest) (*interfaces.ContentTypeRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content type service unavailable")
	}
	record, err := a.service.Create(ctx, content.CreateContentTypeRequest{
		Name:         req.Name,
		Slug:         req.Slug,
		Description:  req.Description,
		Schema:       cloneFieldMap(req.Schema),
		Capabilities: cloneFieldMap(req.Capabilities),
		Icon:         req.Icon,
	})
	if err != nil {
		return nil, err
	}
	return toContentTypeRecord(record), nil
}

func (a *contentTypeServiceAdapter) Update(ctx context.Context, req interfaces.ContentTypeUpdateRequest) (*interfaces.ContentTypeRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content type service unavailable")
	}
	record, err := a.service.Update(ctx, content.UpdateContentTypeRequest{
		ID:           req.ID,
		Name:         req.Name,
		Slug:         req.Slug,
		Description:  req.Description,
		Schema:       cloneFieldMap(req.Schema),
		Capabilities: cloneFieldMap(req.Capabilities),
		Icon:         req.Icon,
	})
	if err != nil {
		return nil, err
	}
	return toContentTypeRecord(record), nil
}

func (a *contentTypeServiceAdapter) Delete(ctx context.Context, req interfaces.ContentTypeDeleteRequest) error {
	if a == nil || a.service == nil {
		return errors.New("content type service unavailable")
	}
	return a.service.Delete(ctx, content.DeleteContentTypeRequest{
		ID:         req.ID,
		HardDelete: req.HardDelete,
	})
}

func (a *contentTypeServiceAdapter) Get(ctx context.Context, id uuid.UUID) (*interfaces.ContentTypeRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content type service unavailable")
	}
	record, err := a.service.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return toContentTypeRecord(record), nil
}

func (a *contentTypeServiceAdapter) GetBySlug(ctx context.Context, slug string) (*interfaces.ContentTypeRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content type service unavailable")
	}
	record, err := a.service.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return toContentTypeRecord(record), nil
}

func (a *contentTypeServiceAdapter) List(ctx context.Context) ([]*interfaces.ContentTypeRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content type service unavailable")
	}
	records, err := a.service.List(ctx)
	if err != nil {
		return nil, err
	}
	return toContentTypeRecords(records), nil
}

func (a *contentTypeServiceAdapter) Search(ctx context.Context, query string) ([]*interfaces.ContentTypeRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content type service unavailable")
	}
	records, err := a.service.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	return toContentTypeRecords(records), nil
}

func toContentTypeRecords(records []*content.ContentType) []*interfaces.ContentTypeRecord {
	out := make([]*interfaces.ContentTypeRecord, 0, len(records))
	for _, record := range records {
		out = append(out, toContentTypeRecord(record))
	}
	return out
}

func toContentTypeRecord(record *content.ContentType) *interfaces.ContentTypeRecord {
	if record == nil {
		return nil
	}
	return &interfaces.ContentTypeRecord{
		ID:           record.ID,
		Name:         record.Name,
		Slug:         record.Slug,
		Description:  record.Description,
		Schema:       cloneFieldMap(record.Schema),
		Capabilities: cloneFieldMap(record.Capabilities),
		Icon:         record.Icon,
	}
}
