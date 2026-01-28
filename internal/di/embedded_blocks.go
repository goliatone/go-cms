package di

import (
	"context"

	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

type contentPageResolver struct {
	pages pages.PageRepository
}

func (r contentPageResolver) PageIDsForContent(ctx context.Context, contentID uuid.UUID) ([]uuid.UUID, error) {
	if r.pages == nil || contentID == uuid.Nil {
		return nil, nil
	}
	records, err := r.pages.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0)
	for _, record := range records {
		if record == nil {
			continue
		}
		if record.ContentID == contentID {
			ids = append(ids, record.ID)
		}
	}
	return ids, nil
}
