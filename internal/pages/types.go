package pages

import (
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Page struct {
	ID           uuid.UUID
	ContentID    uuid.UUID
	ParentID     *uuid.UUID
	Slug         string
	Status       string
	PublishAt    *time.Time
	UnpublishAt  *time.Time
	CreatedBy    uuid.UUID
	UpdatedBy    uuid.UUID
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Content      *content.Content
	Translations []*PageTranslation
	Versions     []*PageVersion
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
