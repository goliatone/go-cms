package pages

import (
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Page captures hierarchical page metadata.
type Page struct {
	bun.BaseModel `bun:"table:pages,alias:p"`

	ID          uuid.UUID  `bun:",pk,type:uuid" json:"id"`
	ContentID   uuid.UUID  `bun:"content_id,notnull,type:uuid" json:"content_id"`
	ParentID    *uuid.UUID `bun:"parent_id,type:uuid" json:"parent_id,omitempty"`
	TemplateID  uuid.UUID  `bun:"template_id,notnull,type:uuid" json:"template_id"`
	Slug        string     `bun:"slug,notnull" json:"slug"`
	Status      string     `bun:"status,notnull,default:'draft'" json:"status"`
	PublishAt   *time.Time `bun:"publish_at,nullzero" json:"publish_at,omitempty"`
	UnpublishAt *time.Time `bun:"unpublish_at,nullzero" json:"unpublish_at,omitempty"`
	CreatedBy   uuid.UUID  `bun:"created_by,notnull,type:uuid" json:"created_by"`
	UpdatedBy   uuid.UUID  `bun:"updated_by,notnull,type:uuid" json:"updated_by"`
	DeletedAt   *time.Time `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt   time.Time  `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt   time.Time  `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Content      *content.Content   `bun:"rel:belongs-to,join:content_id=id" json:"content,omitempty"`
	Translations []*PageTranslation `bun:"rel:has-many,join:id=page_id" json:"translations,omitempty"`
	Versions     []*PageVersion     `bun:"rel:has-many,join:id=page_id" json:"versions,omitempty"`
}

// PageVersion snapshots structural layout for history/versioning.
type PageVersion struct {
	bun.BaseModel `bun:"table:page_versions,alias:pv"`

	ID        uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	PageID    uuid.UUID      `bun:"page_id,notnull,type:uuid" json:"page_id"`
	Version   int            `bun:"version,notnull" json:"version"`
	Snapshot  map[string]any `bun:"snapshot,type:jsonb,notnull" json:"snapshot"`
	DeletedAt *time.Time     `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedBy uuid.UUID      `bun:"created_by,notnull,type:uuid" json:"created_by"`
	CreatedAt time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
}

// PageTranslation stores localized page metadata.
type PageTranslation struct {
	bun.BaseModel `bun:"table:page_translations,alias:pt"`

	ID             uuid.UUID  `bun:",pk,type:uuid" json:"id"`
	PageID         uuid.UUID  `bun:"page_id,notnull,type:uuid" json:"page_id"`
	LocaleID       uuid.UUID  `bun:"locale_id,notnull,type:uuid" json:"locale_id"`
	Title          string     `bun:"title,notnull" json:"title"`
	Path           string     `bun:"path,notnull" json:"path"`
	SEOTitle       *string    `bun:"seo_title" json:"seo_title,omitempty"`
	SEODescription *string    `bun:"seo_description" json:"seo_description,omitempty"`
	Summary        *string    `bun:"summary" json:"summary,omitempty"`
	DeletedAt      *time.Time `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt      time.Time  `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt      time.Time  `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
}
