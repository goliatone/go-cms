package widgets

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Definition captures a widget type, its configuration schema, and default values.
type Definition struct {
	bun.BaseModel `bun:"table:widget_definitions,alias:wd"`

	ID          uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	Name        string         `bun:"name,notnull,unique" json:"name"`
	Description *string        `bun:"description" json:"description,omitempty"`
	Schema      map[string]any `bun:"schema,type:jsonb,notnull" json:"schema"`
	Defaults    map[string]any `bun:"defaults,type:jsonb" json:"defaults,omitempty"`
	Category    *string        `bun:"category" json:"category,omitempty"`
	Icon        *string        `bun:"icon" json:"icon,omitempty"`
	DeletedAt   *time.Time     `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt   time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt   time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	// Instances is populated when loading widget definitions with eager relations.
	Instances []*Instance `bun:"rel:has-many,join:id=definition_id" json:"instances,omitempty"`
}

// Instance represents a concrete placement of a widget definition.
type Instance struct {
	bun.BaseModel `bun:"table:widget_instances,alias:wi"`

	ID              uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	DefinitionID    uuid.UUID      `bun:"definition_id,notnull,type:uuid" json:"definition_id"`
	BlockInstanceID *uuid.UUID     `bun:"block_instance_id,type:uuid" json:"block_instance_id,omitempty"`
	AreaCode        *string        `bun:"area_code" json:"area_code,omitempty"`
	Placement       map[string]any `bun:"placement_metadata,type:jsonb" json:"placement,omitempty"`
	Configuration   map[string]any `bun:"configuration,type:jsonb,notnull,default:'{}'::jsonb" json:"configuration"`
	VisibilityRules map[string]any `bun:"visibility_rules,type:jsonb" json:"visibility_rules,omitempty"`
	PublishOn       *time.Time     `bun:"publish_on" json:"publish_on,omitempty"`
	UnpublishOn     *time.Time     `bun:"unpublish_on" json:"unpublish_on,omitempty"`
	Position        int            `bun:"position,notnull,default:0" json:"position"`
	CreatedBy       uuid.UUID      `bun:"created_by,notnull,type:uuid" json:"created_by"`
	UpdatedBy       uuid.UUID      `bun:"updated_by,notnull,type:uuid" json:"updated_by"`
	DeletedAt       *time.Time     `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt       time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt       time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Definition   *Definition    `bun:"rel:belongs-to,join:definition_id=id" json:"definition,omitempty"`
	Translations []*Translation `bun:"rel:has-many,join:id=widget_instance_id" json:"translations,omitempty"`
}

// Translation stores localized data for a widget instance.
type Translation struct {
	bun.BaseModel `bun:"table:widget_translations,alias:wt"`

	ID               uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	WidgetInstanceID uuid.UUID      `bun:"widget_instance_id,notnull,type:uuid" json:"widget_instance_id"`
	LocaleID         uuid.UUID      `bun:"locale_id,notnull,type:uuid" json:"locale_id"`
	Content          map[string]any `bun:"content,type:jsonb,notnull" json:"content"`
	DeletedAt        *time.Time     `bun:"deleted_at,nullzero" json:"deleted_at,omitempty"`
	CreatedAt        time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt        time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Instance *Instance `bun:"rel:belongs-to,join:widget_instance_id=id" json:"instance,omitempty"`
}

// AreaScope identifies how broadly an area definition applies.
type AreaScope string

const (
	// AreaScopeGlobal registers an area that applies across the entire site.
	AreaScopeGlobal AreaScope = "global"
	// AreaScopeTheme registers an area that is scoped to a specific theme.
	AreaScopeTheme AreaScope = "theme"
	// AreaScopeTemplate registers an area scoped to a specific template.
	AreaScopeTemplate AreaScope = "template"
)

// AreaDefinition documents a named region where widgets can be rendered.
type AreaDefinition struct {
	bun.BaseModel `bun:"table:widget_area_definitions,alias:wad"`

	ID          uuid.UUID  `bun:",pk,type:uuid" json:"id"`
	Code        string     `bun:"code,notnull,unique" json:"code"`
	Name        string     `bun:"name,notnull" json:"name"`
	Description *string    `bun:"description" json:"description,omitempty"`
	Scope       AreaScope  `bun:"scope,notnull,default:'global'" json:"scope"`
	ThemeID     *uuid.UUID `bun:"theme_id,type:uuid" json:"theme_id,omitempty"`
	TemplateID  *uuid.UUID `bun:"template_id,type:uuid" json:"template_id,omitempty"`
	CreatedAt   time.Time  `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt   time.Time  `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`
}

// AreaPlacement binds a widget instance to an area with optional locale-specific ordering.
type AreaPlacement struct {
	bun.BaseModel `bun:"table:widget_area_placements,alias:wap"`

	ID         uuid.UUID      `bun:",pk,type:uuid" json:"id"`
	AreaCode   string         `bun:"area_code,notnull" json:"area_code"`
	LocaleID   *uuid.UUID     `bun:"locale_id,type:uuid" json:"locale_id,omitempty"`
	InstanceID uuid.UUID      `bun:"instance_id,notnull,type:uuid" json:"instance_id"`
	Position   int            `bun:"position,notnull,default:0" json:"position"`
	Metadata   map[string]any `bun:"metadata,type:jsonb" json:"metadata,omitempty"`
	CreatedAt  time.Time      `bun:"created_at,nullzero,default:current_timestamp" json:"created_at"`
	UpdatedAt  time.Time      `bun:"updated_at,nullzero,default:current_timestamp" json:"updated_at"`

	Instance *Instance `bun:"rel:belongs-to,join:instance_id=id" json:"instance,omitempty"`
}

// translationKey formats a composite cache key for widget instance translations.
func translationKey(instanceID uuid.UUID, localeID uuid.UUID) string {
	return instanceID.String() + ":" + localeID.String()
}

// areaLocaleKey generates a stable key for area + locale combinations.
func areaLocaleKey(areaCode string, localeID *uuid.UUID) string {
	if localeID == nil {
		return areaCode + ":default"
	}
	return areaCode + ":" + localeID.String()
}
