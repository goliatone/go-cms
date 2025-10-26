package pages

import (
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Page captures hierarchical page metadata.
type Page struct {
	bun.BaseModel `bun:"table:pages,alias:p"`

	ID               uuid.UUID          `bun:",pk,type:uuid" json:"id"`
	ContentID        uuid.UUID          `bun:"content_id,notnull,type:uuid" json:"content_id"`
	CurrentVersion   int                `bun:"current_version,notnull,default:1" json:"current_version"`
	PublishedVersion *int               `bun:"published_version" json:"published_version,omitempty"`
	ParentID         *uuid.UUID         `bun:"parent_id,type:uuid" json:"parent_id,omitempty"`
	TemplateID       uuid.UUID          `bun:"template_id,notnull,type:uuid" json:"template_id"`
	Slug             string             `bun:"slug,notnull" json:"slug"`
	Status           string             `bun:"status,notnull,default:'draft'" json:"status"`
	PublishAt        *time.Time         `bun:"publish_at,nullzero" json:"publish_at,omitempty"`
	UnpublishAt      *time.Time         `bun:"unpublish_at,nullzero" json:"unpublish_at,omitempty"`
	PublishedAt      *time.Time         `bun:"published_at,nullzero" json:"published_at,omitempty"`
	PublishedBy      *uuid.UUID         `bun:"published_by,type:uuid" json:"published_by,omitempty"`
	CreatedBy        uuid.UUID          `bun:"created_by,notnull,type:uuid" json:"created_by"`
	UpdatedBy        uuid.UUID          `bun:"updated_by,notnull,type:uuid" json:"updated_by"`
	DeletedAt        *time.Time         `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt        time.Time          `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt        time.Time          `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
	Content          *content.Content   `bun:"rel:belongs-to,join:content_id=id" json:"content,omitempty"`
	Translations     []*PageTranslation `bun:"rel:has-many,join:id=page_id" json:"translations,omitempty"`
	Versions         []*PageVersion     `bun:"rel:has-many,join:id=page_id" json:"versions,omitempty"`
	Blocks           []*blocks.Instance `bun:"-" json:"blocks,omitempty"`

	Widgets map[string][]*widgets.ResolvedWidget `bun:"-" json:"widgets,omitempty"`
}

// PageVersion snapshots structural layout for history/versioning.
type PageVersion struct {
	bun.BaseModel `bun:"table:page_versions,alias:pv"`

	ID          uuid.UUID           `bun:",pk,type:uuid" json:"id"`
	PageID      uuid.UUID           `bun:"page_id,notnull,type:uuid" json:"page_id"`
	Version     int                 `bun:"version,notnull" json:"version"`
	Status      domain.Status       `bun:"status,notnull,default:'draft'" json:"status"`
	Snapshot    PageVersionSnapshot `bun:"snapshot,type:jsonb,notnull" json:"snapshot"`
	DeletedAt   *time.Time          `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedBy   uuid.UUID           `bun:"created_by,notnull,type:uuid" json:"created_by"`
	CreatedAt   time.Time           `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	PublishedAt *time.Time          `bun:"published_at,nullzero" json:"published_at,omitempty"`
	PublishedBy *uuid.UUID          `bun:"published_by,type:uuid" json:"published_by,omitempty"`
	Page        *Page               `bun:"rel:belongs-to,join:page_id=id" json:"page,omitempty"`
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

// PageVersionSnapshot captures layout, block, and widget placements at publish time.
type PageVersionSnapshot struct {
	Regions  map[string][]PageBlockPlacement      `json:"regions,omitempty"`
	Blocks   []PageBlockPlacement                 `json:"blocks,omitempty"`
	Widgets  map[string][]WidgetPlacementSnapshot `json:"widgets,omitempty"`
	Metadata map[string]any                       `json:"metadata,omitempty"`
}

// PageBlockPlacement describes a block instance captured in a snapshot.
type PageBlockPlacement struct {
	Region     string         `json:"region"`
	Position   int            `json:"position"`
	BlockID    uuid.UUID      `json:"block_id"`
	InstanceID uuid.UUID      `json:"instance_id"`
	Version    *int           `json:"version,omitempty"`
	Snapshot   map[string]any `json:"snapshot,omitempty"`
}

// WidgetPlacementSnapshot describes widget placement state for a snapshot.
type WidgetPlacementSnapshot struct {
	Area          string         `json:"area"`
	WidgetID      uuid.UUID      `json:"widget_id"`
	InstanceID    uuid.UUID      `json:"instance_id"`
	Configuration map[string]any `json:"configuration,omitempty"`
}

// PageVersionSnapshotSchema documents the JSON schema enforced for page snapshots.
var PageVersionSnapshotSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"regions": map[string]any{
			"type": "object",
			"additionalProperties": map[string]any{
				"type": "array",
				"items": map[string]any{
					"$ref": "#/$defs/blockPlacement",
				},
			},
		},
		"blocks": map[string]any{
			"type":  "array",
			"items": map[string]any{"$ref": "#/$defs/blockPlacement"},
		},
		"widgets": map[string]any{
			"type": "object",
			"additionalProperties": map[string]any{
				"type":  "array",
				"items": map[string]any{"$ref": "#/$defs/widgetPlacement"},
			},
		},
		"metadata": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
	},
	"$defs": map[string]any{
		"blockPlacement": map[string]any{
			"type":     "object",
			"required": []string{"region", "position", "block_id", "instance_id"},
			"properties": map[string]any{
				"region": map[string]any{"type": "string"},
				"position": map[string]any{
					"type":    "integer",
					"minimum": 0,
				},
				"block_id":    map[string]any{"type": "string", "format": "uuid"},
				"instance_id": map[string]any{"type": "string", "format": "uuid"},
				"version": map[string]any{
					"type": []any{"integer", "null"},
				},
				"snapshot": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		},
		"widgetPlacement": map[string]any{
			"type":     "object",
			"required": []string{"area", "widget_id", "instance_id"},
			"properties": map[string]any{
				"area":        map[string]any{"type": "string"},
				"widget_id":   map[string]any{"type": "string", "format": "uuid"},
				"instance_id": map[string]any{"type": "string", "format": "uuid"},
				"configuration": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		},
	},
}
