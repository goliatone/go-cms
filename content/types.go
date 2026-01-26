package content

import (
	"time"

	"github.com/goliatone/go-cms/domain"
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

	ID               uuid.UUID             `bun:",pk,type:uuid" json:"id"`
	ContentTypeID    uuid.UUID             `bun:"content_type_id,notnull,type:uuid" json:"content_type_id"`
	CurrentVersion   int                   `bun:"current_version,notnull,default:1" json:"current_version"`
	PublishedVersion *int                  `bun:"published_version" json:"published_version,omitempty"`
	Status           string                `bun:"status,notnull,default:'draft'" json:"status"`
	Slug             string                `bun:"slug,notnull" json:"slug"`
	PublishAt        *time.Time            `bun:"publish_at,nullzero" json:"publish_at,omitempty"`
	UnpublishAt      *time.Time            `bun:"unpublish_at,nullzero" json:"unpublish_at,omitempty"`
	PublishedAt      *time.Time            `bun:"published_at,nullzero" json:"published_at,omitempty"`
	PublishedBy      *uuid.UUID            `bun:"published_by,type:uuid" json:"published_by,omitempty"`
	CreatedBy        uuid.UUID             `bun:"created_by,notnull,type:uuid" json:"created_by"`
	UpdatedBy        uuid.UUID             `bun:"updated_by,notnull,type:uuid" json:"updated_by"`
	DeletedAt        *time.Time            `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt        time.Time             `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt        time.Time             `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
	Type             *ContentType          `bun:"rel:belongs-to,join:content_type_id=id" json:"content_type,omitempty"`
	Translations     []*ContentTranslation `bun:"rel:has-many,join:id=content_id"        json:"translations,omitempty"`
	Versions         []*ContentVersion     `bun:"rel:has-many,join:id=content_id"        json:"versions,omitempty"`
	EffectiveStatus  domain.Status         `bun:"-" json:"effective_status"`
	IsVisible        bool                  `bun:"-" json:"is_visible"`
}

// ContentTranslation stores localized variants of a content entry.
type ContentTranslation struct {
	bun.BaseModel `bun:"table:content_translations,alias:ctn"`

	ID                 uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	ContentID          uuid.UUID      `bun:"content_id,notnull,type:uuid" json:"content_id"`
	LocaleID           uuid.UUID      `bun:"locale_id,notnull,type:uuid" json:"locale_id"`
	TranslationGroupID *uuid.UUID     `bun:"translation_group_id,type:uuid,nullzero" json:"translation_group_id,omitempty"`
	Title              string         `bun:"title,notnull" json:"title"`
	Summary            *string        `bun:"summary" json:"summary,omitempty"`
	Content            map[string]any `bun:"content,type:jsonb,notnull" json:"content"`
	DeletedAt          *time.Time     `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt          time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt          time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Locale *Locale `bun:"rel:belongs-to,join:locale_id=id" json:"locale,omitempty"`
}

// ContentVersion captures immutable snapshots of content payloads.
type ContentVersion struct {
	bun.BaseModel `bun:"table:content_versions,alias:cv"`

	ID          uuid.UUID              `bun:",pk,type:uuid" json:"id"`
	ContentID   uuid.UUID              `bun:"content_id,notnull,type:uuid" json:"content_id"`
	Version     int                    `bun:"version,notnull" json:"version"`
	Status      domain.Status          `bun:"status,notnull,default:'draft'" json:"status"`
	Snapshot    ContentVersionSnapshot `bun:"snapshot,type:jsonb,notnull" json:"snapshot"`
	CreatedBy   uuid.UUID              `bun:"created_by,notnull,type:uuid" json:"created_by"`
	CreatedAt   time.Time              `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	PublishedAt *time.Time             `bun:"published_at,nullzero" json:"published_at,omitempty"`
	PublishedBy *uuid.UUID             `bun:"published_by,type:uuid" json:"published_by,omitempty"`
	Content     *Content               `bun:"rel:belongs-to,join:content_id=id" json:"content,omitempty"`
}

// ContentVersionSnapshot describes the persisted JSON snapshot for version history.
type ContentVersionSnapshot struct {
	Fields       map[string]any                      `json:"fields,omitempty"`
	Translations []ContentVersionTranslationSnapshot `json:"translations,omitempty"`
	Metadata     map[string]any                      `json:"metadata,omitempty"`
}

// ContentVersionTranslationSnapshot encodes a localized payload captured in a version.
type ContentVersionTranslationSnapshot struct {
	Locale  string         `json:"locale"`
	Title   string         `json:"title"`
	Summary *string        `json:"summary,omitempty"`
	Content map[string]any `json:"content"`
}

// ContentVersionSnapshotSchema captures the JSON schema used to validate snapshots.
var ContentVersionSnapshotSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"fields": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
		"translations": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":     "object",
				"required": []string{"locale", "title", "content"},
				"properties": map[string]any{
					"locale": map[string]any{"type": "string"},
					"title":  map[string]any{"type": "string"},
					"summary": map[string]any{
						"type": []any{"string", "null"},
					},
					"content": map[string]any{
						"type":                 "object",
						"additionalProperties": true,
					},
				},
			},
		},
		"metadata": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
	},
}
