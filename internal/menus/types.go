package menus

import (
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Menu represents a navigational container that groups hierarchical items.
type Menu struct {
	bun.BaseModel `bun:"table:menus,alias:m"`

	ID          uuid.UUID   `bun:",pk,type:uuid" json:"id"`
	Code        string      `bun:"code,notnull" json:"code"`
	Description *string     `bun:"description" json:"description,omitempty"`
	CreatedBy   uuid.UUID   `bun:"created_by,notnull,type:uuid" json:"created_by"`
	UpdatedBy   uuid.UUID   `bun:"updated_by,notnull,type:uuid" json:"updated_by"`
	CreatedAt   time.Time   `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt   time.Time   `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
	Items       []*MenuItem `bun:"rel:has-many,join:id=menu_id" json:"items,omitempty"`
}

// MenuItem describes a single navigational entry with optional hierarchy.
type MenuItem struct {
	bun.BaseModel `bun:"table:menu_items,alias:mi"`

	ID           uuid.UUID              `bun:",pk,type:uuid" json:"id"`
	MenuID       uuid.UUID              `bun:"menu_id,notnull,type:uuid" json:"menu_id"`
	ParentID     *uuid.UUID             `bun:"parent_id,type:uuid" json:"parent_id,omitempty"`
	Position     int                    `bun:"position,notnull,default:0" json:"position"`
	Target       map[string]any         `bun:"target,type:jsonb,notnull" json:"target"`
	CreatedBy    uuid.UUID              `bun:"created_by,notnull,type:uuid" json:"created_by"`
	UpdatedBy    uuid.UUID              `bun:"updated_by,notnull,type:uuid" json:"updated_by"`
	DeletedAt    *time.Time             `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt    time.Time              `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt    time.Time              `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
	Menu         *Menu                  `bun:"rel:belongs-to,join:menu_id=id" json:"menu,omitempty"`
	Parent       *MenuItem              `bun:"rel:belongs-to,join:parent_id=id" json:"parent,omitempty"`
	Children     []*MenuItem            `bun:"rel:has-many,join:id=parent_id" json:"children,omitempty"`
	Translations []*MenuItemTranslation `bun:"rel:has-many,join:id=menu_item_id" json:"translations,omitempty"`
}

// MenuItemTranslation stores localized metadata for menu items.
type MenuItemTranslation struct {
	bun.BaseModel `bun:"table:menu_item_translations,alias:mit"`

	ID          uuid.UUID       `bun:",pk,type:uuid" json:"id"`
	MenuItemID  uuid.UUID       `bun:"menu_item_id,notnull,type:uuid" json:"menu_item_id"`
	LocaleID    uuid.UUID       `bun:"locale_id,notnull,type:uuid" json:"locale_id"`
	Label       string          `bun:"label,notnull" json:"label"`
	URLOverride *string         `bun:"url_override" json:"url_override,omitempty"`
	DeletedAt   *time.Time      `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt   time.Time       `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt   time.Time       `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
	MenuItem    *MenuItem       `bun:"rel:belongs-to,join:menu_item_id=id" json:"menu_item,omitempty"`
	Locale      *content.Locale `bun:"rel:belongs-to,join:locale_id=id" json:"locale,omitempty"`
}
