package content

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Locale represents supported languages for the CMS.
type Locale struct {
	bun.BaseModel `bun:"table:locales,alias:l"`

	ID         uuid.UUID      `bun:",pk,type:uuid"         json:"id"`
	Code       string         `bun:"code,notnull"          json:"code"`
	Display    string         `bun:"display_name,notnull"  json:"display_name"`
	NativeName *string        `bun:"native_name"           json:"native_name,omitempty"`
	IsActive   bool           `bun:"is_active,notnull,default:true"  json:"is_active"`
	IsDefault  bool           `bun:"is_default,notnull,default:false" json:"is_default"`
	Metadata   map[string]any `bun:"metadata,type:jsonb"   json:"metadata,omitempty"`
	DeletedAt  *time.Time     `bun:"deleted_at,nullzero"   json:"deleted_at,omitempty"`
	CreatedAt  time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
}

// ContentType defines available content schemas.
type ContentType struct {
	bun.BaseModel `bun:"table:content_types,alias:ct"`

	ID           uuid.UUID      `bun:",pk,type:uuid"                json:"id"`
	Name         string         `bun:"name,notnull"                 json:"name"`
	Description  *string        `bun:"description"                  json:"description,omitempty"`
	Schema       map[string]any `bun:"schema,type:jsonb,notnull"    json:"schema"`
	Capabilities map[string]any `bun:"capabilities,type:jsonb"      json:"capabilities,omitempty"`
	Icon         *string        `bun:"icon"                         json:"icon,omitempty"`
	CreatedAt    time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt    time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
}

// Content is the canonical record for translatable entries.
type Content struct {
	bun.BaseModel `bun:"table:contents,alias:c"`

	ID            uuid.UUID  `bun:",pk,type:uuid" json:"id"`
	ContentTypeID uuid.UUID  `bun:"content_type_id,notnull,type:uuid" json:"content_type_id"`
	Status        string     `bun:"status,notnull,default:'draft'" json:"status"`
	Slug          string     `bun:"slug,notnull" json:"slug"`
	PublishAt     *time.Time `bun:"publish_at,nullzero" json:"publish_at,omitempty"`
	UnpublishAt   *time.Time `bun:"unpublish_at,nullzero" json:"unpublish_at,omitempty"`
	CreatedBy     uuid.UUID  `bun:"created_by,notnull,type:uuid" json:"created_by"`
	UpdatedBy     uuid.UUID  `bun:"updated_by,notnull,type:uuid" json:"updated_by"`
	DeletedAt     *time.Time `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Type         *ContentType          `bun:"rel:belongs-to,join:content_type_id=id" json:"content_type,omitempty"`
	Translations []*ContentTranslation `bun:"rel:has-many,join:id=content_id"        json:"translations,omitempty"`
}

// ContentTranslation stores localized variants of a content entry.
type ContentTranslation struct {
	bun.BaseModel `bun:"table:content_translations,alias:ctn"`

	ID        uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	ContentID uuid.UUID      `bun:"content_id,notnull,type:uuid" json:"content_id"`
	LocaleID  uuid.UUID      `bun:"locale_id,notnull,type:uuid" json:"locale_id"`
	Title     string         `bun:"title,notnull" json:"title"`
	Summary   *string        `bun:"summary" json:"summary,omitempty"`
	Content   map[string]any `bun:"content,type:jsonb,notnull" json:"content"`
	DeletedAt *time.Time     `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Locale *Locale `bun:"rel:belongs-to,join:locale_id=id" json:"locale,omitempty"`
}
