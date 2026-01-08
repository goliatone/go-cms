package themes

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Theme captures a complete site design (templates, assets, metadata).
type Theme struct {
	bun.BaseModel `bun:"table:themes,alias:t"`

	ID          uuid.UUID   `bun:",pk,type:uuid" json:"id"`
	Name        string      `bun:"name,notnull,unique" json:"name"`
	Description *string     `bun:"description" json:"description,omitempty"`
	Version     string      `bun:"version,notnull" json:"version"`
	Author      *string     `bun:"author" json:"author,omitempty"`
	IsActive    bool        `bun:"is_active,notnull,default:false" json:"is_active"`
	ThemePath   string      `bun:"theme_path,notnull" json:"theme_path"`
	Config      ThemeConfig `bun:"config,type:jsonb" json:"config"`
	CreatedAt   time.Time   `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt   time.Time   `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Templates []*Template `bun:"rel:has-many,join:id=theme_id" json:"templates,omitempty"`
}

// Template defines the layout surface for pages within a theme.
type Template struct {
	bun.BaseModel `bun:"table:templates,alias:tp"`

	ID           uuid.UUID                 `bun:",pk,type:uuid" json:"id"`
	ThemeID      uuid.UUID                 `bun:"theme_id,notnull,type:uuid" json:"theme_id"`
	Name         string                    `bun:"name,notnull" json:"name"`
	Slug         string                    `bun:"slug,notnull" json:"slug"`
	Description  *string                   `bun:"description" json:"description,omitempty"`
	TemplatePath string                    `bun:"template_path,notnull" json:"template_path"`
	Regions      map[string]TemplateRegion `bun:"regions,type:jsonb,notnull" json:"regions"`
	Metadata     map[string]any            `bun:"metadata,type:jsonb" json:"metadata,omitempty"`
	CreatedAt    time.Time                 `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt    time.Time                 `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Theme *Theme `bun:"rel:belongs-to,join:theme_id=id" json:"theme,omitempty"`
}

// TemplateRegion describes an individual block/widget surface exposed by a template.
type TemplateRegion struct {
	Name            string   `json:"name"`
	Description     *string  `json:"description,omitempty"`
	AcceptsWidgets  bool     `json:"accepts_widgets"`
	AcceptsBlocks   bool     `json:"accepts_blocks"`
	FallbackRegions []string `json:"fallback_regions,omitempty"`
}

// ThemeConfig records manifest level details parsed from theme descriptors.
type ThemeConfig struct {
	WidgetAreas   []ThemeWidgetArea   `json:"widget_areas,omitempty"`
	MenuLocations []ThemeMenuLocation `json:"menu_locations,omitempty"`
	Assets        *ThemeAssets        `json:"assets,omitempty"`
	Metadata      map[string]any      `json:"metadata,omitempty"`
}

// ThemeWidgetArea declares widget placements injected by a theme.
type ThemeWidgetArea struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Scope       string  `json:"scope,omitempty"` // e.g. "global", "template", "page"
}

// ThemeMenuLocation links menus to theme-defined regions
type ThemeMenuLocation struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

// ThemeAssets references static files associated with the theme
type ThemeAssets struct {
	BasePath *string  `json:"base_path,omitempty"`
	Styles   []string `json:"styles,omitempty"`
	Scripts  []string `json:"scripts,omitempty"`
	Images   []string `json:"images,omitempty"`
}
