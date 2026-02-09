package pages

import (
	"context"
	"time"

	"github.com/goliatone/go-cms/content"
	"github.com/goliatone/go-cms/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// TranslationCheckOptions configures translation completeness checks.
type TranslationCheckOptions = interfaces.TranslationCheckOptions

// Service describes page management capabilities.
type Service interface {
	Create(ctx context.Context, req CreatePageRequest) (*Page, error)
	Get(ctx context.Context, id uuid.UUID) (*Page, error)
	List(ctx context.Context, env ...string) ([]*Page, error)
	CheckTranslations(ctx context.Context, id uuid.UUID, required []string, opts TranslationCheckOptions) ([]string, error)
	AvailableLocales(ctx context.Context, id uuid.UUID, opts TranslationCheckOptions) ([]string, error)
	Update(ctx context.Context, req UpdatePageRequest) (*Page, error)
	Delete(ctx context.Context, req DeletePageRequest) error
	UpdateTranslation(ctx context.Context, req UpdatePageTranslationRequest) (*PageTranslation, error)
	DeleteTranslation(ctx context.Context, req DeletePageTranslationRequest) error
	Move(ctx context.Context, req MovePageRequest) (*Page, error)
	Duplicate(ctx context.Context, req DuplicatePageRequest) (*Page, error)
	Schedule(ctx context.Context, req SchedulePageRequest) (*Page, error)
	CreateDraft(ctx context.Context, req CreatePageDraftRequest) (*PageVersion, error)
	PublishDraft(ctx context.Context, req PublishPageDraftRequest) (*PageVersion, error)
	PreviewDraft(ctx context.Context, req PreviewPageDraftRequest) (*PagePreview, error)
	ListVersions(ctx context.Context, pageID uuid.UUID) ([]*PageVersion, error)
	RestoreVersion(ctx context.Context, req RestorePageVersionRequest) (*PageVersion, error)
}

// CreatePageRequest captures the payload required to create a page.
type CreatePageRequest struct {
	ContentID                uuid.UUID
	TemplateID               uuid.UUID
	ParentID                 *uuid.UUID
	Slug                     string
	Status                   string
	EnvironmentKey           string
	CreatedBy                uuid.UUID
	UpdatedBy                uuid.UUID
	Translations             []PageTranslationInput
	AllowMissingTranslations bool
}

// PageTranslationInput represents localized routing information.
type PageTranslationInput struct {
	Locale        string
	Title         string
	Path          string
	Summary       *string
	MediaBindings media.BindingSet
}

// UpdatePageRequest captures the mutable fields for an existing page.
type UpdatePageRequest struct {
	ID                       uuid.UUID
	TemplateID               *uuid.UUID
	Status                   string
	EnvironmentKey           string
	UpdatedBy                uuid.UUID
	Translations             []PageTranslationInput
	AllowMissingTranslations bool
}

// DeletePageRequest captures the information required to delete a page.
type DeletePageRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}

// UpdatePageTranslationRequest mutates a specific translation for a page.
type UpdatePageTranslationRequest struct {
	PageID        uuid.UUID
	Locale        string
	Title         string
	Path          string
	Summary       *string
	MediaBindings media.BindingSet
	UpdatedBy     uuid.UUID
}

// DeletePageTranslationRequest removes a locale from a page.
type DeletePageTranslationRequest struct {
	PageID    uuid.UUID
	Locale    string
	DeletedBy uuid.UUID
}

// MovePageRequest updates the hierarchical parent for a page.
type MovePageRequest struct {
	PageID      uuid.UUID
	NewParentID *uuid.UUID
	ActorID     uuid.UUID
}

// DuplicatePageRequest clones a page, allowing optional overrides.
type DuplicatePageRequest struct {
	PageID    uuid.UUID
	Slug      string
	ParentID  *uuid.UUID
	Status    string
	CreatedBy uuid.UUID
	UpdatedBy uuid.UUID
}

// CreatePageDraftRequest captures the data required to create a page version draft.
type CreatePageDraftRequest struct {
	PageID      uuid.UUID
	Snapshot    PageVersionSnapshot
	CreatedBy   uuid.UUID
	UpdatedBy   uuid.UUID
	BaseVersion *int
}

// PublishPageDraftRequest captures the inputs required to publish a draft version.
type PublishPageDraftRequest struct {
	PageID      uuid.UUID
	Version     int
	PublishedBy uuid.UUID
	PublishedAt *time.Time
}

// PreviewPageDraftRequest captures the inputs required to preview a draft version.
type PreviewPageDraftRequest struct {
	PageID          uuid.UUID
	Version         int
	ContentSnapshot *content.ContentVersionSnapshot
}

// RestorePageVersionRequest captures the request to restore a historical version as a draft.
type RestorePageVersionRequest struct {
	PageID     uuid.UUID
	Version    int
	RestoredBy uuid.UUID
}

// PagePreview bundles a preview record with the requested version snapshot.
type PagePreview struct {
	Page    *Page
	Version *PageVersion
}

// SchedulePageRequest captures scheduling input for page publish/unpublish windows.
type SchedulePageRequest struct {
	PageID      uuid.UUID
	PublishAt   *time.Time
	UnpublishAt *time.Time
	ScheduledBy uuid.UUID
}
