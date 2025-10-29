package interfaces

import (
	"context"

	"github.com/google/uuid"
)

// ContentService abstracts the CMS content service so markdown imports can
// provision or update records without depending on internal implementations.
type ContentService interface {
	Create(ctx context.Context, req ContentCreateRequest) (*ContentRecord, error)
	Update(ctx context.Context, req ContentUpdateRequest) (*ContentRecord, error)
	GetBySlug(ctx context.Context, slug string) (*ContentRecord, error)
	List(ctx context.Context) ([]*ContentRecord, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ContentCreateRequest captures the details required to create a content record.
type ContentCreateRequest struct {
	ContentTypeID uuid.UUID
	Slug          string
	Status        string
	CreatedBy     uuid.UUID
	UpdatedBy     uuid.UUID
	Translations  []ContentTranslationInput
	Metadata      map[string]any
}

// ContentUpdateRequest captures the mutable fields for an existing content record.
type ContentUpdateRequest struct {
	ID           uuid.UUID
	Status       string
	UpdatedBy    uuid.UUID
	Translations []ContentTranslationInput
	Metadata     map[string]any
}

// ContentTranslationInput represents localized fields provided during create/update.
type ContentTranslationInput struct {
	Locale  string
	Title   string
	Summary *string
	Fields  map[string]any
}

// ContentRecord reflects the persisted state returned by the content service.
type ContentRecord struct {
	ID           uuid.UUID
	ContentType  uuid.UUID
	Slug         string
	Status       string
	Translations []ContentTranslation
	Metadata     map[string]any
}

// ContentTranslation mirrors stored translation fields.
type ContentTranslation struct {
	ID      uuid.UUID
	Locale  string
	Title   string
	Summary *string
	Fields  map[string]any
}
