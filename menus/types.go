package menus

import (
	"time"

	"github.com/goliatone/go-cms/content"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

const (
	MenuItemTypeItem      = "item"
	MenuItemTypeGroup     = "group"
	MenuItemTypeSeparator = "separator"
)

// Menu represents a navigational container that groups hierarchical items.
type Menu struct {
	bun.BaseModel `bun:"table:menus,alias:m"`

	ID          uuid.UUID   `bun:",pk,type:uuid" json:"id"`
	Code        string      `bun:"code,notnull" json:"code"`
	Location    string      `bun:"location" json:"location,omitempty"`
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
	ParentRef    *string                `bun:"parent_ref" json:"parent_ref,omitempty"`
	ExternalCode string                 `bun:"external_code" json:"external_code,omitempty"`
	Position     int                    `bun:"position,notnull,default:0" json:"position"`
	Type         string                 `bun:"type,notnull,default:item" json:"type,omitempty"`
	Target       map[string]any         `bun:"target,type:jsonb,notnull" json:"target,omitempty"`
	Icon         string                 `bun:"icon" json:"icon,omitempty"`
	Badge        map[string]any         `bun:"badge,type:jsonb" json:"badge,omitempty"`
	Permissions  []string               `bun:"permissions,type:text[]" json:"permissions,omitempty"`
	Classes      []string               `bun:"classes,type:text[]" json:"classes,omitempty"`
	Styles       map[string]string      `bun:"styles,type:jsonb" json:"styles,omitempty"`
	CanonicalKey *string                `bun:"canonical_key" json:"canonical_key,omitempty"`
	Collapsible  bool                   `bun:"collapsible,notnull,default:false" json:"collapsible,omitempty"`
	Collapsed    bool                   `bun:"collapsed,notnull,default:false" json:"collapsed,omitempty"`
	Metadata     map[string]any         `bun:"metadata,type:jsonb,notnull" json:"metadata,omitempty"`
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

	ID            uuid.UUID       `bun:",pk,type:uuid" json:"id"`
	MenuItemID    uuid.UUID       `bun:"menu_item_id,notnull,type:uuid" json:"menu_item_id"`
	LocaleID      uuid.UUID       `bun:"locale_id,notnull,type:uuid" json:"locale_id"`
	Label         string          `bun:"label,notnull" json:"label"`
	LabelKey      string          `bun:"label_key" json:"label_key,omitempty"`
	GroupTitle    string          `bun:"group_title" json:"group_title,omitempty"`
	GroupTitleKey string          `bun:"group_title_key" json:"group_title_key,omitempty"`
	URLOverride   *string         `bun:"url_override" json:"url_override,omitempty"`
	DeletedAt     *time.Time      `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt     time.Time       `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt     time.Time       `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
	MenuItem      *MenuItem       `bun:"rel:belongs-to,join:menu_item_id=id" json:"menu_item,omitempty"`
	Locale        *content.Locale `bun:"rel:belongs-to,join:locale_id=id" json:"locale,omitempty"`
}
