package interfaces

import (
	"context"

	"github.com/google/uuid"
)

// PageService abstracts page orchestration for CMS page records.
type PageService interface {
	Create(ctx context.Context, req PageCreateRequest) (*PageRecord, error)
	Update(ctx context.Context, req PageUpdateRequest) (*PageRecord, error)
	GetBySlug(ctx context.Context, slug string, opts PageReadOptions) (*PageRecord, error)
	List(ctx context.Context, opts PageReadOptions) ([]*PageRecord, error)
	Delete(ctx context.Context, req PageDeleteRequest) error
	UpdateTranslation(ctx context.Context, req PageUpdateTranslationRequest) (*PageTranslation, error)
	DeleteTranslation(ctx context.Context, req PageDeleteTranslationRequest) error
	Move(ctx context.Context, req PageMoveRequest) (*PageRecord, error)
	Duplicate(ctx context.Context, req PageDuplicateRequest) (*PageRecord, error)
}

// PageReadOptions defines read-time locale resolution and metadata behaviour.
//
// Behavior contract:
//   - RequestedLocale always echoes Locale, even when missing.
//   - If a translation exists for Locale, Translation.Requested/Resolved point to it and ResolvedLocale = Locale.
//   - If missing and FallbackLocale exists, Translation.Requested is nil, Resolved uses the fallback locale, and FallbackUsed=true.
//   - If missing with no fallback, Translation.Requested/Resolved are nil and ResolvedLocale is empty.
//   - AllowMissingTranslations=false should return ErrTranslationMissing when Locale is set and missing.
//   - List reads never hard-fail on missing translations; they return bundle metadata instead.
//   - IncludeAvailableLocales populates Translation.Meta.AvailableLocales for the page bundle only.
//   - Page and content translation bundles resolve independently (no forced cross-bundle fallback).
type PageReadOptions struct {
	Locale                   string
	FallbackLocale           string
	AllowMissingTranslations bool
	IncludeAvailableLocales  bool
	EnvironmentKey           string
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

// PageUpdateTranslationRequest mutates a single translation entry.
type PageUpdateTranslationRequest struct {
	PageID    uuid.UUID
	Locale    string
	Title     string
	Path      string
	Summary   *string
	Fields    map[string]any
	UpdatedBy uuid.UUID
}

// PageDeleteTranslationRequest removes a translation entry.
type PageDeleteTranslationRequest struct {
	PageID    uuid.UUID
	Locale    string
	DeletedBy uuid.UUID
}

// PageMoveRequest updates the parent relationship.
type PageMoveRequest struct {
	PageID      uuid.UUID
	NewParentID *uuid.UUID
	ActorID     uuid.UUID
}

// PageDuplicateRequest clones an existing page with overrides.
type PageDuplicateRequest struct {
	PageID    uuid.UUID
	Slug      string
	ParentID  *uuid.UUID
	Status    string
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
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
	ID                 uuid.UUID
	ContentID          uuid.UUID
	TemplateID         uuid.UUID
	Slug               string
	Status             string
	Translation        TranslationBundle[PageTranslation]    `json:"translation"`
	ContentTranslation TranslationBundle[ContentTranslation] `json:"content_translation"`
	Translations       []PageTranslation
	Metadata           map[string]any
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
