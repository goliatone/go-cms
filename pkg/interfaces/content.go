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
	GetBySlug(ctx context.Context, slug string, opts ContentReadOptions) (*ContentRecord, error)
	List(ctx context.Context, opts ContentReadOptions) ([]*ContentRecord, error)
	CheckTranslations(ctx context.Context, id uuid.UUID, required []string, opts TranslationCheckOptions) ([]string, error)
	AvailableLocales(ctx context.Context, id uuid.UUID, opts TranslationCheckOptions) ([]string, error)
	Delete(ctx context.Context, req ContentDeleteRequest) error
	UpdateTranslation(ctx context.Context, req ContentUpdateTranslationRequest) (*ContentTranslation, error)
	DeleteTranslation(ctx context.Context, req ContentDeleteTranslationRequest) error
}

// ContentReadOptions defines read-time locale resolution and metadata behaviour.
//
// Behavior contract:
//   - RequestedLocale always echoes Locale, even when missing.
//   - If a translation exists for Locale, Translation.Requested/Resolved point to it and ResolvedLocale = Locale.
//   - If missing and FallbackLocale exists, Translation.Requested is nil, Resolved uses the fallback locale, and FallbackUsed=true.
//   - If missing with no fallback, Translation.Requested/Resolved are nil and ResolvedLocale is empty.
//   - AllowMissingTranslations=false should return ErrTranslationMissing when Locale is set and missing.
//   - List reads never hard-fail on missing translations; they return bundle metadata instead.
//   - IncludeAvailableLocales populates Translation.Meta.AvailableLocales for the content bundle only.
type ContentReadOptions struct {
	Locale                   string
	FallbackLocale           string
	AllowMissingTranslations bool
	IncludeAvailableLocales  bool
	EnvironmentKey           string
}

// ContentCreateRequest captures the details required to create a content record.
type ContentCreateRequest struct {
	ContentTypeID            uuid.UUID
	Slug                     string
	Status                   string
	CreatedBy                uuid.UUID
	UpdatedBy                uuid.UUID
	Translations             []ContentTranslationInput
	Metadata                 map[string]any
	AllowMissingTranslations bool
}

// ContentUpdateRequest captures the mutable fields for an existing content record.
type ContentUpdateRequest struct {
	ID                       uuid.UUID
	Status                   string
	UpdatedBy                uuid.UUID
	Translations             []ContentTranslationInput
	Metadata                 map[string]any
	AllowMissingTranslations bool
}

// ContentDeleteRequest captures the information required to remove content. When
// HardDelete is false, implementations may opt for soft-deletion where supported.
type ContentDeleteRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}

// ContentUpdateTranslationRequest updates a single locale entry.
type ContentUpdateTranslationRequest struct {
	ContentID uuid.UUID
	Locale    string
	Title     string
	Summary   *string
	Fields    map[string]any
	Blocks    []map[string]any
	UpdatedBy uuid.UUID
}

// ContentDeleteTranslationRequest removes a locale entry.
type ContentDeleteTranslationRequest struct {
	ContentID uuid.UUID
	Locale    string
	DeletedBy uuid.UUID
}

// ContentTranslationInput represents localized fields provided during create/update.
type ContentTranslationInput struct {
	Locale  string
	Title   string
	Summary *string
	Fields  map[string]any
	Blocks  []map[string]any
}

// ContentRecord reflects the persisted state returned by the content service.
type ContentRecord struct {
	ID              uuid.UUID
	ContentType     uuid.UUID
	ContentTypeSlug string
	Slug            string
	Status          string
	Translation     TranslationBundle[ContentTranslation] `json:"translation"`
	Metadata        map[string]any
}

// ContentTranslation mirrors stored translation fields.
type ContentTranslation struct {
	ID      uuid.UUID
	Locale  string
	Title   string
	Summary *string
	Fields  map[string]any
}

// ContentTypeService abstracts content type management for adapters.
type ContentTypeService interface {
	Create(ctx context.Context, req ContentTypeCreateRequest) (*ContentTypeRecord, error)
	Update(ctx context.Context, req ContentTypeUpdateRequest) (*ContentTypeRecord, error)
	Delete(ctx context.Context, req ContentTypeDeleteRequest) error
	Get(ctx context.Context, id uuid.UUID) (*ContentTypeRecord, error)
	GetBySlug(ctx context.Context, slug string, env ...string) (*ContentTypeRecord, error)
	List(ctx context.Context, env ...string) ([]*ContentTypeRecord, error)
	Search(ctx context.Context, query string, env ...string) ([]*ContentTypeRecord, error)
}

// ContentTypeCreateRequest captures fields required to create a content type.
type ContentTypeCreateRequest struct {
	Name           string
	Slug           string
	Description    *string
	Schema         map[string]any
	Capabilities   map[string]any
	Icon           *string
	EnvironmentKey string
}

// ContentTypeUpdateRequest captures fields for updating a content type.
type ContentTypeUpdateRequest struct {
	ID           uuid.UUID
	Name         *string
	Slug         *string
	Description  *string
	Schema       map[string]any
	Capabilities map[string]any
	Icon         *string
}

// ContentTypeDeleteRequest captures data to delete a content type.
type ContentTypeDeleteRequest struct {
	ID         uuid.UUID
	HardDelete bool
}

// ContentTypeRecord reflects a stored content type.
type ContentTypeRecord struct {
	ID           uuid.UUID
	Name         string
	Slug         string
	Description  *string
	Schema       map[string]any
	Capabilities map[string]any
	Icon         *string
}
