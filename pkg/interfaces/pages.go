package interfaces

import (
	"context"

	"github.com/google/uuid"
)

// PageService abstracts page orchestration for markdown imports.
type PageService interface {
	Create(ctx context.Context, req PageCreateRequest) (*PageRecord, error)
	Update(ctx context.Context, req PageUpdateRequest) (*PageRecord, error)
	GetBySlug(ctx context.Context, slug string) (*PageRecord, error)
	List(ctx context.Context) ([]*PageRecord, error)
	Delete(ctx context.Context, req PageDeleteRequest) error
}

// PageCreateRequest captures the required fields to create a page backed by content.
type PageCreateRequest struct {
	ContentID                uuid.UUID
	TemplateID               uuid.UUID
	ParentID                 *uuid.UUID
	Slug                     string
	Status                   string
	CreatedBy                uuid.UUID
	UpdatedBy                uuid.UUID
	Translations             []PageTranslationInput
	Metadata                 map[string]any
	AllowMissingTranslations bool
}

// PageUpdateRequest mutates an existing page.
type PageUpdateRequest struct {
	ID                       uuid.UUID
	TemplateID               *uuid.UUID
	Status                   string
	UpdatedBy                uuid.UUID
	Translations             []PageTranslationInput
	Metadata                 map[string]any
	AllowMissingTranslations bool
}

// PageDeleteRequest captures the information required to remove a page.
type PageDeleteRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}

// PageTranslationInput describes localized routing attributes.
type PageTranslationInput struct {
	Locale  string
	Title   string
	Path    string
	Summary *string
	Fields  map[string]any
}

// PageRecord reflects stored page details.
type PageRecord struct {
	ID           uuid.UUID
	ContentID    uuid.UUID
	TemplateID   uuid.UUID
	Slug         string
	Status       string
	Translations []PageTranslation
	Metadata     map[string]any
}

// PageTranslation mirrors persisted page translations.
type PageTranslation struct {
	ID      uuid.UUID
	Locale  string
	Title   string
	Path    string
	Summary *string
	Fields  map[string]any
}
