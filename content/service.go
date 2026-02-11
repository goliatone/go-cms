package content

import (
	"context"
	"strings"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// TranslationCheckOptions configures translation completeness checks.
type TranslationCheckOptions = interfaces.TranslationCheckOptions

// Service exposes content management use cases.
type Service interface {
	Create(ctx context.Context, req CreateContentRequest) (*Content, error)
	Get(ctx context.Context, id uuid.UUID, opts ...ContentGetOption) (*Content, error)
	List(ctx context.Context, opts ...ContentListOption) ([]*Content, error)
	CheckTranslations(ctx context.Context, id uuid.UUID, required []string, opts TranslationCheckOptions) ([]string, error)
	AvailableLocales(ctx context.Context, id uuid.UUID, opts TranslationCheckOptions) ([]string, error)
	Update(ctx context.Context, req UpdateContentRequest) (*Content, error)
	Delete(ctx context.Context, req DeleteContentRequest) error
	UpdateTranslation(ctx context.Context, req UpdateContentTranslationRequest) (*ContentTranslation, error)
	DeleteTranslation(ctx context.Context, req DeleteContentTranslationRequest) error
	Schedule(ctx context.Context, req ScheduleContentRequest) (*Content, error)
	CreateDraft(ctx context.Context, req CreateContentDraftRequest) (*ContentVersion, error)
	PublishDraft(ctx context.Context, req PublishContentDraftRequest) (*ContentVersion, error)
	PreviewDraft(ctx context.Context, req PreviewContentDraftRequest) (*ContentPreview, error)
	ListVersions(ctx context.Context, contentID uuid.UUID) ([]*ContentVersion, error)
	RestoreVersion(ctx context.Context, req RestoreContentVersionRequest) (*ContentVersion, error)
}

// ContentTypeService provides CRUD operations for content types.
type ContentTypeService interface {
	Create(ctx context.Context, req CreateContentTypeRequest) (*ContentType, error)
	Update(ctx context.Context, req UpdateContentTypeRequest) (*ContentType, error)
	Delete(ctx context.Context, req DeleteContentTypeRequest) error
	Get(ctx context.Context, id uuid.UUID) (*ContentType, error)
	GetBySlug(ctx context.Context, slug string, env ...string) (*ContentType, error)
	List(ctx context.Context, env ...string) ([]*ContentType, error)
	Search(ctx context.Context, query string, env ...string) ([]*ContentType, error)
}

// ContentListOption configures content list behavior. It is an alias to string to
// preserve the existing List(ctx, env ...string) call pattern.
type ContentListOption = string

// ContentGetOption configures content get behavior. It reuses list option tokens.
type ContentGetOption = ContentListOption

// ProjectionTranslationMode controls how projection behaves when translations are
// not explicitly loaded for reads.
type ProjectionTranslationMode string

const (
	ProjectionTranslationModeAutoLoad ProjectionTranslationMode = "auto_load"
	ProjectionTranslationModeNoop     ProjectionTranslationMode = "noop"
	ProjectionTranslationModeError    ProjectionTranslationMode = "error"
)

const (
	ContentProjectionAdmin         = "admin"
	ContentProjectionDerivedFields = "derived_fields"
)

const (
	contentListWithTranslations     ContentListOption = "content:list:with_translations"
	contentListProjectionPrefix     ContentListOption = "content:list:projection:"
	contentListProjectionModePrefix ContentListOption = "content:list:projection_mode:"
)

// WithTranslations preloads translations when listing or fetching content records.
func WithTranslations() ContentListOption {
	return contentListWithTranslations
}

// WithProjection configures a named read projection for content list/get calls.
func WithProjection(name string) ContentListOption {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "":
		return ""
	case "derived", "derived-fields":
		normalized = ContentProjectionDerivedFields
	}
	return ContentListOption(string(contentListProjectionPrefix) + normalized)
}

// WithDerivedFields enables the canonical derived-content-fields projection.
func WithDerivedFields() ContentListOption {
	return WithProjection(ContentProjectionDerivedFields)
}

// WithProjectionMode controls projection behavior when translations are not
// explicitly requested.
func WithProjectionMode(mode ProjectionTranslationMode) ContentListOption {
	normalized := strings.ToLower(strings.TrimSpace(string(mode)))
	if normalized == "" {
		return ""
	}
	return ContentListOption(string(contentListProjectionModePrefix) + normalized)
}

// CreateContentRequest captures the information required to create content.
type CreateContentRequest struct {
	ContentTypeID            uuid.UUID
	Slug                     string
	Status                   string
	EnvironmentKey           string
	CreatedBy                uuid.UUID
	UpdatedBy                uuid.UUID
	Metadata                 map[string]any
	Translations             []ContentTranslationInput
	AllowMissingTranslations bool
}

// ContentTranslationInput represents localized fields supplied during create or update.
type ContentTranslationInput struct {
	Locale  string
	Title   string
	Summary *string
	Content map[string]any
	Blocks  []map[string]any
}

// UpdateContentRequest captures mutable fields for an existing content entry.
type UpdateContentRequest struct {
	ID                       uuid.UUID
	Status                   string
	EnvironmentKey           string
	UpdatedBy                uuid.UUID
	Translations             []ContentTranslationInput
	Metadata                 map[string]any
	AllowMissingTranslations bool
}

// DeleteContentRequest captures the information required to remove a content entry.
type DeleteContentRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}

// UpdateContentTranslationRequest captures the payload required to mutate a single translation.
type UpdateContentTranslationRequest struct {
	ContentID uuid.UUID
	Locale    string
	Title     string
	Summary   *string
	Content   map[string]any
	Blocks    []map[string]any
	UpdatedBy uuid.UUID
}

// DeleteContentTranslationRequest captures the payload required to drop a translation.
type DeleteContentTranslationRequest struct {
	ContentID uuid.UUID
	Locale    string
	DeletedBy uuid.UUID
}

// CreateContentDraftRequest captures the payload needed to record a draft snapshot.
type CreateContentDraftRequest struct {
	ContentID   uuid.UUID
	Snapshot    ContentVersionSnapshot
	CreatedBy   uuid.UUID
	UpdatedBy   uuid.UUID
	BaseVersion *int
}

// PublishContentDraftRequest captures the information required to publish a content draft.
type PublishContentDraftRequest struct {
	ContentID   uuid.UUID
	Version     int
	PublishedBy uuid.UUID
	PublishedAt *time.Time
}

// PreviewContentDraftRequest captures the information required to preview a content draft.
type PreviewContentDraftRequest struct {
	ContentID uuid.UUID
	Version   int
}

// RestoreContentVersionRequest captures the request to restore a prior content version.
type RestoreContentVersionRequest struct {
	ContentID  uuid.UUID
	Version    int
	RestoredBy uuid.UUID
}

// ContentPreview bundles a preview snapshot with the derived content record.
type ContentPreview struct {
	Content *Content
	Version *ContentVersion
}

// ScheduleContentRequest captures details to schedule publish/unpublish events.
type ScheduleContentRequest struct {
	ContentID   uuid.UUID
	PublishAt   *time.Time
	UnpublishAt *time.Time
	ScheduledBy uuid.UUID
}

// CreateContentTypeRequest captures required fields to create a content type.
type CreateContentTypeRequest struct {
	Name           string
	Slug           string
	Description    *string
	Schema         map[string]any
	UISchema       map[string]any
	Capabilities   map[string]any
	Icon           *string
	Status         string
	EnvironmentKey string
	CreatedBy      uuid.UUID
	UpdatedBy      uuid.UUID
}

// UpdateContentTypeRequest captures mutable fields for a content type.
type UpdateContentTypeRequest struct {
	ID                   uuid.UUID
	Name                 *string
	Slug                 *string
	Description          *string
	Schema               map[string]any
	UISchema             map[string]any
	Capabilities         map[string]any
	Icon                 *string
	Status               *string
	EnvironmentKey       string
	UpdatedBy            uuid.UUID
	AllowBreakingChanges bool
}

// DeleteContentTypeRequest captures details required to delete a content type.
type DeleteContentTypeRequest struct {
	ID         uuid.UUID
	DeletedBy  uuid.UUID
	HardDelete bool
}
