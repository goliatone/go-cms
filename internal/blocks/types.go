package blocks

import (
	"time"

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

	ID            uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	PageID        *uuid.UUID     `bun:"page_id,type:uuid" json:"page_id,omitempty"`
	Region        string         `bun:"region,notnull" json:"region"`
	Position      int            `bun:"position,notnull,default:0" json:"position"`
	DefinitionID  uuid.UUID      `bun:"definition_id,notnull,type:uuid" json:"definition_id"`
	Configuration map[string]any `bun:"configuration,type:jsonb,notnull,default:'{}'::jsonb" json:"configuration"`
	IsGlobal      bool           `bun:"is_global,notnull,default:false" json:"is_global"`
	CreatedBy     uuid.UUID      `bun:"created_by,notnull,type:uuid" json:"created_by"`
	UpdatedBy     uuid.UUID      `bun:"updated_by,notnull,type:uuid" json:"updated_by"`
	DeletedAt     *time.Time     `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt     time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Definition   *Definition    `bun:"rel:belongs-to,join:definition_id=id" json:"definition,omitempty"`
	Translations []*Translation `bun:"rel:has-many,join:id=block_instance_id" json:"translations,omitempty"`
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
