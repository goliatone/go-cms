package blocks

import (
	"time"

	"github.com/goliatone/go-cms/internal/domain"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Definition represents a reusable block template and associated metadata.
type Definition struct {
	bun.BaseModel `bun:"table:block_definitions,alias:bd"`

	ID               uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	Name             string         `bun:"name,notnull" json:"name"`
	Description      *string        `bun:"description" json:"description,omitempty"`
	Icon             *string        `bun:"icon" json:"icon,omitempty"`
	Schema           map[string]any `bun:"schema,type:jsonb,notnull" json:"schema"`
	Defaults         map[string]any `bun:"defaults,type:jsonb" json:"defaults,omitempty"`
	EditorStyleURL   *string        `bun:"editor_style_url" json:"editor_style_url,omitempty"`
	FrontendStyleURL *string        `bun:"frontend_style_url" json:"frontend_style_url,omitempty"`
	DeletedAt        *time.Time     `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt        time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt        time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
}

// Instance captures a concrete usage of a block definition on a page or region.
type Instance struct {
	bun.BaseModel `bun:"table:block_instances,alias:bi"`

	ID               uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	PageID           *uuid.UUID     `bun:"page_id,type:uuid" json:"page_id,omitempty"`
	Region           string         `bun:"region,notnull" json:"region"`
	Position         int            `bun:"position,notnull,default:0" json:"position"`
	DefinitionID     uuid.UUID      `bun:"definition_id,notnull,type:uuid" json:"definition_id"`
	Configuration    map[string]any `bun:"configuration,type:jsonb,notnull,default:'{}'::jsonb" json:"configuration"`
	IsGlobal         bool           `bun:"is_global,notnull,default:false" json:"is_global"`
	CurrentVersion   int            `bun:"current_version,notnull,default:1" json:"current_version"`
	PublishedVersion *int           `bun:"published_version" json:"published_version,omitempty"`
	PublishedAt      *time.Time     `bun:"published_at,nullzero" json:"published_at,omitempty"`
	PublishedBy      *uuid.UUID     `bun:"published_by,type:uuid" json:"published_by,omitempty"`
	CreatedBy        uuid.UUID      `bun:"created_by,notnull,type:uuid" json:"created_by"`
	UpdatedBy        uuid.UUID      `bun:"updated_by,notnull,type:uuid" json:"updated_by"`
	DeletedAt        *time.Time     `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt        time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt        time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Definition   *Definition        `bun:"rel:belongs-to,join:definition_id=id" json:"definition,omitempty"`
	Translations []*Translation     `bun:"rel:has-many,join:id=block_instance_id" json:"translations,omitempty"`
	Versions     []*InstanceVersion `bun:"rel:has-many,join:id=block_instance_id" json:"versions,omitempty"`
}

// Translation stores localized block content and attribute overrides.
type Translation struct {
	bun.BaseModel `bun:"table:block_translations,alias:bt"`

	ID                uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	BlockInstanceID   uuid.UUID      `bun:"block_instance_id,notnull,type:uuid" json:"block_instance_id"`
	LocaleID          uuid.UUID      `bun:"locale_id,notnull,type:uuid" json:"locale_id"`
	Content           map[string]any `bun:"content,type:jsonb,notnull" json:"content"`
	AttributeOverride map[string]any `bun:"attribute_overrides,type:jsonb" json:"attribute_overrides,omitempty"`
	DeletedAt         *time.Time     `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt         time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt         time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
}

// InstanceVersion captures a snapshot of a block instance's configuration and translations.
type InstanceVersion struct {
	bun.BaseModel `bun:"table:block_versions,alias:bv"`

	ID              uuid.UUID            `bun:",pk,type:uuid" json:"id"`
	BlockInstanceID uuid.UUID            `bun:"block_instance_id,notnull,type:uuid" json:"block_instance_id"`
	Version         int                  `bun:"version,notnull" json:"version"`
	Status          domain.Status        `bun:"status,notnull,default:'draft'" json:"status"`
	Snapshot        BlockVersionSnapshot `bun:"snapshot,type:jsonb,notnull" json:"snapshot"`
	CreatedBy       uuid.UUID            `bun:"created_by,notnull,type:uuid" json:"created_by"`
	CreatedAt       time.Time            `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	PublishedAt     *time.Time           `bun:"published_at,nullzero" json:"published_at,omitempty"`
	PublishedBy     *uuid.UUID           `bun:"published_by,type:uuid" json:"published_by,omitempty"`

	Instance *Instance `bun:"rel:belongs-to,join:block_instance_id=id" json:"instance,omitempty"`
}

// BlockVersionSnapshot captures the persisted JSON snapshot for a block instance.
type BlockVersionSnapshot struct {
	Configuration map[string]any                    `json:"configuration,omitempty"`
	Translations  []BlockVersionTranslationSnapshot `json:"translations,omitempty"`
	Metadata      map[string]any                    `json:"metadata,omitempty"`
}

// BlockVersionTranslationSnapshot encodes localized payloads within a block snapshot.
type BlockVersionTranslationSnapshot struct {
	Locale             string         `json:"locale"`
	Content            map[string]any `json:"content"`
	AttributeOverrides map[string]any `json:"attribute_overrides,omitempty"`
}

// BlockVersionSnapshotSchema documents the JSON schema enforced for block snapshots.
var BlockVersionSnapshotSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"configuration": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
		"translations": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":     "object",
				"required": []string{"locale", "content"},
				"properties": map[string]any{
					"locale": map[string]any{"type": "string"},
					"content": map[string]any{
						"type":                 "object",
						"additionalProperties": true,
					},
					"attribute_overrides": map[string]any{
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
