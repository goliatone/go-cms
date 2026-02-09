# Widgets Guide

This guide covers widget definitions, widget instances, translations, area-based placement, visibility rules, and the registry pattern in `go-cms`. By the end you will understand how to define dynamic behavioral components, place them in named areas, manage localized content per widget, and filter visibility based on audience, schedule, segments, and locale.

## Widget Architecture Overview

Widgets in `go-cms` are dynamic behavioral components built on five layers:

- **Widget definitions** describe the shape and validation rules for a category of widget. Each definition carries a JSON schema, optional defaults, a category, and an icon. Think of a definition as a blueprint -- "Promo Banner", "Newsletter Signup", "Cookie Consent".
- **Widget instances** are concrete placements of a definition. Each instance holds its own configuration, visibility rules, optional publish/unpublish schedule, and references a definition by ID.
- **Widget translations** provide localized content for each instance. A translation carries a locale and JSON content. Shortcodes in content are rendered automatically when the shortcode feature is enabled.
- **Area definitions** are named zones where widgets can be placed -- "sidebar.primary", "footer.newsletter", "header.announcements". Areas are scoped globally, per theme, or per template.
- **Area placements** bind widget instances to areas with explicit ordering and optional locale-specific arrangement.

```
Definition (name, schema, defaults)
  └── Instance (configuration, visibilityRules, publishOn/unpublishOn)
        ├── Translation (locale, content)
        ├── Translation (locale, content)
        └── AreaPlacement (areaCode, locale, position, metadata)

AreaDefinition (code, name, scope)
  └── AreaPlacement (instanceID, locale, position)
```

All entities use UUID primary keys and UTC timestamps.

### Accessing the Service

Widget operations are exposed through the `cms.Module` facade:

```go
cfg := cms.DefaultConfig()
cfg.Features.Widgets = true
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es"}

module, err := cms.New(cfg)
if err != nil {
    log.Fatal(err)
}

widgetSvc := module.Widgets()
```

The `widgetSvc` variable satisfies the `widgets.Service` interface. The service delegates to in-memory repositories by default, or SQL-backed repositories when `di.WithBunDB(db)` is provided.

When `cfg.Features.Widgets` is `false` (the default), `module.Widgets()` returns a no-op service that returns `ErrFeatureDisabled` for every operation. No resources are allocated.

---

## Enabling Widgets

Widgets are disabled by default. Enable them via the feature flag:

```go
cfg := cms.DefaultConfig()
cfg.Features.Widgets = true
```

Optionally pre-register definitions through configuration:

```go
cfg.Widgets.Definitions = []cms.WidgetDefinitionConfig{
    {
        Name:        "announcement",
        Description: "Site-wide announcement banner",
        Schema: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "message":    map[string]any{"type": "string"},
                "severity":   map[string]any{"type": "string", "enum": []any{"info", "warning", "error"}},
                "dismissible": map[string]any{"type": "boolean"},
            },
            "required": []string{"message"},
        },
        Defaults: map[string]any{
            "severity":    "info",
            "dismissible": true,
        },
        Category: "notifications",
        Icon:     "bell",
    },
}
```

Config-based definitions are loaded into the widget registry during container initialization and synced to the database when the service starts.

---

## Widget Definitions

### Registering a Definition

A widget definition requires a name and a JSON schema at minimum:

```go
promoDef, err := widgetSvc.RegisterDefinition(ctx, widgets.RegisterDefinitionInput{
    Name:        "Promo Banner",
    Description: stringPtr("Promotional banner with headline and call-to-action"),
    Category:    stringPtr("marketing"),
    Icon:        stringPtr("megaphone"),
    Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "headline":    map[string]any{"type": "string"},
            "cta_text":    map[string]any{"type": "string"},
            "cta_url":     map[string]any{"type": "string", "format": "uri"},
            "background":  map[string]any{"type": "string"},
        },
        "required": []string{"headline"},
    },
    Defaults: map[string]any{
        "cta_text": "Learn more",
    },
})
```

**`RegisterDefinitionInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Name` | `string` | Yes | Unique display name for the widget definition |
| `Description` | `*string` | No | Human-readable description |
| `Schema` | `map[string]any` | Yes | JSON schema defining the widget configuration structure |
| `Defaults` | `map[string]any` | No | Default configuration values applied to new instances |
| `Category` | `*string` | No | Grouping category (e.g. "marketing", "navigation", "social") |
| `Icon` | `*string` | No | Icon identifier for admin UIs |

**What happens:**

1. The name is validated (required, non-empty)
2. The JSON schema is validated for structural correctness
3. Defaults are validated against the schema (if provided)
4. A deterministic UUID is generated from the name via `identity.WidgetDefinitionUUID(name)`
5. The definition is persisted to the repository
6. An activity event is emitted when activity tracking is enabled

### Schema Validation

Schemas follow JSON Schema conventions. The service validates both the schema itself and any configuration or defaults against it:

```go
schema := map[string]any{
    "type": "object",
    "properties": map[string]any{
        "title":       map[string]any{"type": "string"},
        "description": map[string]any{"type": "string"},
        "max_items":   map[string]any{"type": "integer", "minimum": 1, "maximum": 20},
        "style":       map[string]any{"type": "string", "enum": []any{"compact", "full", "minimal"}},
    },
    "required": []string{"title"},
}
```

### Listing Definitions

```go
definitions, err := widgetSvc.ListDefinitions(ctx)
for _, def := range definitions {
    fmt.Printf("  %s (id=%s)\n", def.Name, def.ID)
}
```

### Getting a Definition

```go
def, err := widgetSvc.GetDefinition(ctx, definitionID)
if err != nil {
    // Returns NotFoundError when definition does not exist
    log.Fatalf("get definition: %v", err)
}
fmt.Printf("Definition: %s\n", def.Name)
```

### Deleting a Definition

Definitions require hard delete. Soft delete is not supported. A definition cannot be deleted if it has active instances.

```go
err := widgetSvc.DeleteDefinition(ctx, widgets.DeleteDefinitionRequest{
    ID:         promoDef.ID,
    HardDelete: true,
})
if err != nil {
    // Returns ErrDefinitionInUse if instances reference this definition
    log.Fatalf("delete definition: %v", err)
}
```

---

## Registry and SyncRegistry

The widget registry enables programmatic registration of widget definitions at application startup. Definitions registered in the registry are synced to the database when `SyncRegistry` is called.

### Creating a Registry

```go
registry := widgets.NewRegistry()

registry.Register(widgets.RegisterDefinitionInput{
    Name: "Promo Banner",
    Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "headline": map[string]any{"type": "string"},
            "cta_text": map[string]any{"type": "string"},
        },
        "required": []string{"headline"},
    },
    Defaults: map[string]any{"cta_text": "Learn more"},
})

registry.Register(widgets.RegisterDefinitionInput{
    Name: "Newsletter Signup",
    Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "heading":     map[string]any{"type": "string"},
            "placeholder": map[string]any{"type": "string"},
            "button_text": map[string]any{"type": "string"},
        },
    },
    Defaults: map[string]any{
        "heading":     "Subscribe to our newsletter",
        "button_text": "Subscribe",
    },
})
```

### Registering a Factory

For dynamic instance configuration, register a factory that generates configuration when instances are created:

```go
registry.RegisterFactory("promo_banner", widgets.Registration{
    Definition: func() widgets.RegisterDefinitionInput {
        return widgets.RegisterDefinitionInput{
            Name: "Promo Banner",
            Schema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "headline": map[string]any{"type": "string"},
                    "cta_text": map[string]any{"type": "string"},
                },
            },
        }
    },
    InstanceFactory: func(ctx context.Context, def *widgets.Definition, input widgets.CreateInstanceInput) (map[string]any, error) {
        // Generate dynamic configuration at instance creation time
        return map[string]any{
            "generated_at": time.Now().UTC().Format(time.RFC3339),
        }, nil
    },
})
```

When `CreateInstance` is called for a definition with an instance factory:
1. The definition defaults are applied first
2. The user-provided configuration is merged on top
3. The instance factory is called and its output is merged last
4. The final merged configuration is validated against the schema

### Syncing to the Database

```go
if err := widgetSvc.SyncRegistry(ctx); err != nil {
    log.Fatalf("sync registry: %v", err)
}
```

`SyncRegistry` iterates over all entries in the registry and persists each definition. Existing definitions (matched by name) are skipped. New definitions are created.

### Config-Based Definitions

Definitions can also be declared in configuration, which is equivalent to programmatic registry registration:

```go
cfg.Widgets.Definitions = []cms.WidgetDefinitionConfig{
    {
        Name:        "Cookie Consent",
        Description: "GDPR-compliant cookie consent banner",
        Schema: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "message":     map[string]any{"type": "string"},
                "accept_text": map[string]any{"type": "string"},
                "reject_text": map[string]any{"type": "string"},
                "policy_url":  map[string]any{"type": "string"},
            },
        },
        Defaults: map[string]any{
            "accept_text": "Accept",
            "reject_text": "Decline",
        },
        Category: "compliance",
        Icon:     "shield",
    },
}
```

The DI container converts these into registry entries during initialization, before the service is created. They are synced automatically alongside any programmatically registered definitions.

---

## Widget Instances

Instances are concrete placements of a widget definition. An instance holds its own configuration, visibility rules, and optional publish schedule.

### Creating an Instance

```go
promoInstance, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
    DefinitionID: promoDef.ID,
    AreaCode:     stringPtr("sidebar.primary"),
    Configuration: map[string]any{
        "headline": "Summer Sale - 50% Off",
        "cta_text": "Shop Now",
        "cta_url":  "/sale",
    },
    VisibilityRules: map[string]any{
        "audience": []any{"guest", "member"},
        "schedule": map[string]any{
            "starts_at": "2025-06-01T00:00:00Z",
            "ends_at":   "2025-08-31T23:59:59Z",
        },
    },
    PublishOn:   timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
    UnpublishOn: timePtr(time.Date(2025, 8, 31, 23, 59, 59, 0, time.UTC)),
    Position:    0,
    CreatedBy:   userID,
    UpdatedBy:   userID,
})
```

**`CreateInstanceInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DefinitionID` | `uuid.UUID` | Yes | Widget definition this instance uses |
| `BlockInstanceID` | `*uuid.UUID` | No | Optional block instance for embedding |
| `AreaCode` | `*string` | No | Area code for placement |
| `Placement` | `map[string]any` | No | Area-specific metadata (layout, theme variant) |
| `Configuration` | `map[string]any` | No | Instance-specific configuration; merged with defaults |
| `VisibilityRules` | `map[string]any` | No | Visibility constraints (audience, schedule, segments, locales) |
| `PublishOn` | `*time.Time` | No | Start of publish window |
| `UnpublishOn` | `*time.Time` | No | End of publish window |
| `Position` | `int` | No | Sort order within area; must be >= 0 |
| `CreatedBy` | `uuid.UUID` | Yes | Actor identifier |
| `UpdatedBy` | `uuid.UUID` | Yes | Actor identifier |

**What happens:**

1. The definition ID, creator, and updater are validated
2. Position must be non-negative
3. If both `PublishOn` and `UnpublishOn` are set, `PublishOn` must be before `UnpublishOn`
4. Configuration is merged: definition defaults + user configuration + factory output (if registered)
5. The merged configuration is validated against the definition schema
6. Visibility rules are validated (only allowed keys: `audience`, `schedule`, `segments`, `locales`, `conditions`)
7. The instance is persisted with a random UUID
8. An activity event is emitted

### Getting an Instance

```go
instance, err := widgetSvc.GetInstance(ctx, instanceID)
if err != nil {
    log.Fatalf("get instance: %v", err)
}
// Instance is returned with Translations hydrated and shortcodes rendered
for _, t := range instance.Translations {
    fmt.Printf("  Locale %s: %v\n", t.LocaleID, t.Content)
}
```

### Listing Instances

```go
// List all instances for a specific definition
instances, err := widgetSvc.ListInstancesByDefinition(ctx, promoDef.ID)

// List all instances in a specific area
sidebarWidgets, err := widgetSvc.ListInstancesByArea(ctx, "sidebar.primary")

// List all instances across all definitions and areas
allWidgets, err := widgetSvc.ListAllInstances(ctx)
```

All list operations return instances with translations hydrated and shortcodes rendered.

### Updating an Instance

```go
updated, err := widgetSvc.UpdateInstance(ctx, widgets.UpdateInstanceInput{
    InstanceID: promoInstance.ID,
    Configuration: map[string]any{
        "headline": "Extended Sale - 60% Off",
        "cta_text": "Shop Now",
        "cta_url":  "/extended-sale",
    },
    VisibilityRules: map[string]any{
        "audience": []any{"guest", "member", "vip"},
    },
    UnpublishOn: timePtr(time.Date(2025, 9, 30, 23, 59, 59, 0, time.UTC)),
    UpdatedBy:   userID,
})
```

**`UpdateInstanceInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `InstanceID` | `uuid.UUID` | Yes | Instance identifier |
| `Configuration` | `map[string]any` | No | Overwrites configuration |
| `VisibilityRules` | `map[string]any` | No | Overwrites visibility rules |
| `Placement` | `map[string]any` | No | Overwrites area metadata |
| `PublishOn` | `*time.Time` | No | Overwrites publish start |
| `UnpublishOn` | `*time.Time` | No | Overwrites publish end |
| `Position` | `*int` | No | Overwrites sort position |
| `AreaCode` | `*string` | No | Reassign area (empty string clears area) |
| `UpdatedBy` | `uuid.UUID` | Yes | Actor identifier |

### Deleting an Instance

Instance deletion cascades to area placements and translations:

```go
err := widgetSvc.DeleteInstance(ctx, widgets.DeleteInstanceRequest{
    InstanceID: promoInstance.ID,
    DeletedBy:  userID,
    HardDelete: true,
})
```

Only hard delete is supported. The cascade order is:
1. All area placements referencing the instance are removed
2. All translations for the instance are deleted
3. The instance itself is deleted

---

## Widget Translations

Translations provide localized content for each widget instance. Each translation is scoped to a locale and carries JSON content.

### Adding a Translation

```go
enTranslation, err := widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
    InstanceID: promoInstance.ID,
    LocaleID:   enLocaleID,
    Content: map[string]any{
        "headline": "Summer Sale - 50% Off",
        "cta_text": "Shop Now",
    },
})
```

```go
esTranslation, err := widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
    InstanceID: promoInstance.ID,
    LocaleID:   esLocaleID,
    Content: map[string]any{
        "headline": "Rebajas de verano - 50% de descuento",
        "cta_text": "Comprar ahora",
    },
})
```

**`AddTranslationInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `InstanceID` | `uuid.UUID` | Yes | Parent widget instance |
| `LocaleID` | `uuid.UUID` | Yes | Locale identifier |
| `Content` | `map[string]any` | Yes | Localized content payload (must be non-nil) |

**What happens:**

1. The instance and locale IDs are validated
2. Content must be non-nil
3. A duplicate translation for the same instance and locale returns `ErrTranslationExists`
4. The translation is persisted with a new UUID
5. If shortcodes are enabled, content is rendered through the shortcode service
6. An activity event is emitted

### Getting a Translation

```go
translation, err := widgetSvc.GetTranslation(ctx, promoInstance.ID, enLocaleID)
fmt.Printf("Content: %v\n", translation.Content)
```

The returned translation has shortcodes rendered if the shortcode feature is enabled.

### Updating a Translation

```go
updated, err := widgetSvc.UpdateTranslation(ctx, widgets.UpdateTranslationInput{
    InstanceID: promoInstance.ID,
    LocaleID:   enLocaleID,
    Content: map[string]any{
        "headline": "Extended Summer Sale - 60% Off",
        "cta_text": "Shop the Sale",
    },
})
```

**`UpdateTranslationInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `InstanceID` | `uuid.UUID` | Yes | Parent widget instance |
| `LocaleID` | `uuid.UUID` | Yes | Locale identifier |
| `Content` | `map[string]any` | Yes | Updated localized content |

### Deleting a Translation

```go
err := widgetSvc.DeleteTranslation(ctx, widgets.DeleteTranslationRequest{
    InstanceID: promoInstance.ID,
    LocaleID:   esLocaleID,
})
```

---

## Area Management

Areas are named zones where widgets are placed. Each area has a code, a human-readable name, and a scope that determines where it applies.

### Registering an Area Definition

```go
sidebarArea, err := widgetSvc.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
    Code:        "sidebar.primary",
    Name:        "Primary Sidebar",
    Description: stringPtr("Main sidebar widget area"),
    Scope:       widgets.AreaScopeGlobal,
})
```

**`RegisterAreaDefinitionInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Code` | `string` | Yes | Unique identifier (alphanumeric, dashes, underscores, dots) |
| `Name` | `string` | Yes | Human-readable area name |
| `Description` | `*string` | No | Description of the area's purpose |
| `Scope` | `AreaScope` | No | Defaults to `AreaScopeGlobal` |
| `ThemeID` | `*uuid.UUID` | No | Required when scope is `AreaScopeTheme` |
| `TemplateID` | `*uuid.UUID` | No | Required when scope is `AreaScopeTemplate` |

**Area scopes:**

| Scope | Constant | Description |
|-------|----------|-------------|
| Global | `widgets.AreaScopeGlobal` | Available everywhere |
| Theme | `widgets.AreaScopeTheme` | Scoped to a specific theme |
| Template | `widgets.AreaScopeTemplate` | Scoped to a specific template |

The area ID is deterministic, computed from the code via `identity.WidgetAreaDefinitionUUID(code)`.

**Code format:** alphanumeric characters, dashes (`-`), underscores (`_`), and dots (`.`) are allowed. The dot separator is conventionally used for namespace hierarchy (e.g., `sidebar.primary`, `footer.newsletter`).

### Listing Area Definitions

```go
areas, err := widgetSvc.ListAreaDefinitions(ctx)
for _, area := range areas {
    fmt.Printf("  %s: %s (scope=%s)\n", area.Code, area.Name, area.Scope)
}
```

Results are sorted by code.

### Assigning a Widget to an Area

```go
placements, err := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
    AreaCode:   "sidebar.primary",
    InstanceID: promoInstance.ID,
    Position:   intPtr(0),          // Insert at position 0
    Metadata: map[string]any{
        "layout": "compact",
    },
})
```

**`AssignWidgetToAreaInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `AreaCode` | `string` | Yes | Target area code |
| `LocaleID` | `*uuid.UUID` | No | Nil for global placement; non-nil for locale-specific ordering |
| `InstanceID` | `uuid.UUID` | Yes | Widget instance to place |
| `Position` | `*int` | No | Insert position; appends if omitted |
| `Metadata` | `map[string]any` | No | Placement-level metadata |

**What happens:**

1. The area definition must exist
2. The widget instance must exist
3. Duplicate placements (same area + locale + instance) return `ErrAreaPlacementExists`
4. Existing placements are loaded for the area and locale
5. The new placement is inserted at the specified position (shifting others down)
6. All positions are renumbered (0, 1, 2, ...) and atomically replaced

### Locale-Specific Placement

Widgets can have different ordering per locale. Pass a `LocaleID` to create locale-specific arrangements:

```go
// English ordering: promo first, then newsletter
widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
    AreaCode:   "sidebar.primary",
    LocaleID:   &enLocaleID,
    InstanceID: promoInstance.ID,
    Position:   intPtr(0),
})

widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
    AreaCode:   "sidebar.primary",
    LocaleID:   &enLocaleID,
    InstanceID: newsletterInstance.ID,
    Position:   intPtr(1),
})

// Spanish ordering: newsletter first, then promo
widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
    AreaCode:   "sidebar.primary",
    LocaleID:   &esLocaleID,
    InstanceID: newsletterInstance.ID,
    Position:   intPtr(0),
})

widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
    AreaCode:   "sidebar.primary",
    LocaleID:   &esLocaleID,
    InstanceID: promoInstance.ID,
    Position:   intPtr(1),
})
```

When `LocaleID` is nil, the placement applies to the default (global) locale.

### Removing a Widget from an Area

```go
err := widgetSvc.RemoveWidgetFromArea(ctx, widgets.RemoveWidgetFromAreaInput{
    AreaCode:   "sidebar.primary",
    InstanceID: promoInstance.ID,
    // LocaleID: nil for global placement
})
```

After removal, remaining placements are automatically renumbered to maintain a contiguous sequence.

### Reordering Area Widgets

Reorder supports drag-and-drop UIs by accepting a complete ordering of all placements in an area:

```go
placements, err := widgetSvc.ReorderAreaWidgets(ctx, widgets.ReorderAreaWidgetsInput{
    AreaCode: "sidebar.primary",
    // LocaleID: nil for global reordering
    Items: []widgets.AreaWidgetOrder{
        {PlacementID: newsletterPlacement.ID, Position: 0},
        {PlacementID: promoPlacement.ID, Position: 1},
        {PlacementID: socialPlacement.ID, Position: 2},
    },
})
```

**`ReorderAreaWidgetsInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `AreaCode` | `string` | Yes | Target area code |
| `LocaleID` | `*uuid.UUID` | No | Locale scope for reordering |
| `Items` | `[]AreaWidgetOrder` | Yes | Complete ordering of all placements |

The `Items` slice must include every existing placement in the area exactly once. Missing or extra placements return `ErrAreaWidgetOrderMismatch`. Positions are renumbered based on the sort order of the items.

---

## Resolving Areas for Rendering

`ResolveArea` is the primary method for rendering widgets. It loads placements for an area, hydrates instances with translations, evaluates visibility rules, and returns an ordered list of resolved widgets.

```go
resolved, err := widgetSvc.ResolveArea(ctx, widgets.ResolveAreaInput{
    AreaCode:          "sidebar.primary",
    LocaleID:          &enLocaleID,
    FallbackLocaleIDs: []uuid.UUID{defaultLocaleID},
    Audience:          []string{"guest"},
    Segments:          []string{"new-user"},
    Now:               time.Now().UTC(),
})
```

**`ResolveAreaInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `AreaCode` | `string` | Yes | Area to resolve |
| `LocaleID` | `*uuid.UUID` | No | Primary locale for placement lookup |
| `FallbackLocaleIDs` | `[]uuid.UUID` | No | Fallback locale chain |
| `Audience` | `[]string` | No | Current visitor audience tags |
| `Segments` | `[]string` | No | Current visitor segment tags |
| `Now` | `time.Time` | No | Evaluation time; defaults to current time |

**Resolution process:**

1. A locale chain is built: `[LocaleID, FallbackLocaleIDs..., nil]`
2. For each locale in the chain, placements are fetched; the first non-empty result is used
3. For each placement:
   - The instance is loaded with translations hydrated and shortcodes rendered
   - Visibility is evaluated against the provided context
   - Widgets that fail visibility checks are silently excluded
   - Widgets restricted by locale are skipped without error
4. A `[]*ResolvedWidget` is returned, ordered by placement position

**Using the result:**

```go
for _, rw := range resolved {
    fmt.Printf("Widget %s at position %d\n", rw.Instance.ID, rw.Placement.Position)
    fmt.Printf("  Configuration: %v\n", rw.Instance.Configuration)
    for _, t := range rw.Instance.Translations {
        fmt.Printf("  Translation (locale=%s): %v\n", t.LocaleID, t.Content)
    }
}
```

The `ResolvedWidget` type pairs the hydrated instance with its area placement:

```go
type ResolvedWidget struct {
    Instance  *Instance      // Hydrated with translations, shortcodes rendered
    Placement *AreaPlacement // Area binding with position and metadata
}
```

---

## Visibility Rules

Visibility rules control when and to whom a widget is displayed. Rules are stored on the widget instance in the `VisibilityRules` map and evaluated at resolve time.

### Evaluating Visibility

```go
visible, err := widgetSvc.EvaluateVisibility(ctx, instance, widgets.VisibilityContext{
    Now:      time.Now().UTC(),
    LocaleID: &enLocaleID,
    Audience: []string{"member"},
    Segments: []string{"returning-user"},
})
if visible {
    // render the widget
}
```

**`VisibilityContext` fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Now` | `time.Time` | Evaluation timestamp; defaults to current time |
| `LocaleID` | `*uuid.UUID` | Current locale for locale-based filtering |
| `Audience` | `[]string` | Visitor audience tags (e.g. "guest", "member", "admin") |
| `Segments` | `[]string` | Visitor segment tags (e.g. "new-user", "returning-user") |
| `CustomRules` | `map[string]any` | Additional context for custom rule evaluation |

### Rule Types

All rules are evaluated in sequence. Every rule must pass for the widget to be visible.

#### Time-Based Rules

Instance-level publish window:

```go
instance := widgets.CreateInstanceInput{
    // ...
    PublishOn:   timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
    UnpublishOn: timePtr(time.Date(2025, 8, 31, 23, 59, 59, 0, time.UTC)),
}
```

Visibility-rules-level schedule (finer control):

```go
VisibilityRules: map[string]any{
    "schedule": map[string]any{
        "starts_at": "2025-06-01T00:00:00Z",
        "ends_at":   "2025-08-31T23:59:59Z",
    },
},
```

Both mechanisms are evaluated independently. The instance-level `PublishOn`/`UnpublishOn` is checked first, followed by the schedule in `VisibilityRules`. Both must pass.

Schedule timestamps use RFC 3339 format or `time.Time` values.

#### Audience-Based Rules

Target specific visitor groups:

```go
VisibilityRules: map[string]any{
    "audience": []any{"guest", "member"},
},
```

The widget is visible when at least one audience tag from the context matches. Matching is case-insensitive. When no audience rule is specified, the widget is visible to all audiences.

#### Segment-Based Rules

Target specific visitor segments:

```go
VisibilityRules: map[string]any{
    "segments": []any{"high-value", "returning-user"},
},
```

The widget is visible when at least one segment tag from the context matches. Matching is case-insensitive. When no segment rule is specified, the widget is visible to all segments.

#### Locale-Based Rules

Restrict to specific locales:

```go
VisibilityRules: map[string]any{
    "locales": []any{"en-locale-uuid", "fr-locale-uuid"},
},
```

When specified, the widget is only visible if the current `LocaleID` matches one of the listed locale IDs. If the locale does not match, `ErrVisibilityLocaleRestricted` is returned. In `ResolveArea`, this error causes the widget to be silently excluded rather than failing the entire resolution.

#### Combined Rules

Rules compose naturally. All specified rules must pass:

```go
VisibilityRules: map[string]any{
    "audience": []any{"member", "vip"},
    "segments": []any{"returning-user"},
    "schedule": map[string]any{
        "starts_at": "2025-01-01T00:00:00Z",
        "ends_at":   "2025-12-31T23:59:59Z",
    },
    "locales": []any{"en-locale-uuid"},
},
```

This widget is visible only when:
- The visitor audience is "member" or "vip", AND
- The visitor segment is "returning-user", AND
- The current time is within 2025, AND
- The current locale is the English locale

### Allowed Visibility Keys

Only these keys are allowed in `VisibilityRules`:

| Key | Value Type | Description |
|-----|-----------|-------------|
| `audience` | `[]any` (strings) | Audience tags that can see the widget |
| `segments` | `[]any` (strings) | Segment tags that can see the widget |
| `schedule` | `map[string]any` | Time window with `starts_at` and/or `ends_at` |
| `locales` | `[]any` (strings) | Locale IDs allowed to see the widget |
| `conditions` | `map[string]any` | Reserved for custom condition evaluation |

Unknown keys in `VisibilityRules` cause `ErrVisibilityRulesInvalid` at instance creation/update time.

---

## Bootstrap Helpers

The `widgets` package provides bootstrap helpers for idempotent setup during application startup:

```go
err := widgets.Bootstrap(ctx, widgetSvc, widgets.BootstrapConfig{
    Definitions: []widgets.RegisterDefinitionInput{
        {
            Name: "Promo Banner",
            Schema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "headline": map[string]any{"type": "string"},
                    "cta_text": map[string]any{"type": "string"},
                },
                "required": []string{"headline"},
            },
            Defaults: map[string]any{"cta_text": "Learn more"},
        },
        {
            Name: "Newsletter Signup",
            Schema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "heading":     map[string]any{"type": "string"},
                    "button_text": map[string]any{"type": "string"},
                },
            },
        },
    },
    Areas: []widgets.RegisterAreaDefinitionInput{
        {
            Code:  "sidebar.primary",
            Name:  "Primary Sidebar",
            Scope: widgets.AreaScopeGlobal,
        },
        {
            Code:  "footer.newsletter",
            Name:  "Footer Newsletter Area",
            Scope: widgets.AreaScopeGlobal,
        },
    },
})
if err != nil && !errors.Is(err, widgets.ErrFeatureDisabled) {
    log.Fatalf("bootstrap widgets: %v", err)
}
```

`Bootstrap` calls `EnsureDefinitions` and `EnsureAreaDefinitions` in sequence. Both are idempotent -- duplicate definitions and areas are silently skipped. When widgets are disabled, `ErrFeatureDisabled` is returned and can be safely ignored.

Individual helpers are also available:

```go
// Register only definitions
err := widgets.EnsureDefinitions(ctx, widgetSvc, definitions)

// Register only area definitions
err := widgets.EnsureAreaDefinitions(ctx, widgetSvc, areas)
```

---

## Common Patterns

### Sidebar with Promotions

A sidebar area that shows promotional widgets based on visitor audience:

```go
// Setup
widgetSvc.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
    Code: "sidebar.promotions",
    Name: "Sidebar Promotions",
    Scope: widgets.AreaScopeGlobal,
})

// Create a guest-only promo
guestPromo, _ := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
    DefinitionID: promoDef.ID,
    Configuration: map[string]any{
        "headline": "Sign up for 10% off!",
        "cta_text": "Create Account",
        "cta_url":  "/signup",
    },
    VisibilityRules: map[string]any{
        "audience": []any{"guest"},
    },
    Position:  0,
    CreatedBy: userID,
    UpdatedBy: userID,
})

// Create a member-only promo
memberPromo, _ := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
    DefinitionID: promoDef.ID,
    Configuration: map[string]any{
        "headline": "Exclusive member deals",
        "cta_text": "View Deals",
        "cta_url":  "/deals",
    },
    VisibilityRules: map[string]any{
        "audience": []any{"member", "vip"},
    },
    Position:  0,
    CreatedBy: userID,
    UpdatedBy: userID,
})

// Assign both to the area
widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
    AreaCode:   "sidebar.promotions",
    InstanceID: guestPromo.ID,
    Position:   intPtr(0),
})

widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
    AreaCode:   "sidebar.promotions",
    InstanceID: memberPromo.ID,
    Position:   intPtr(1),
})

// Resolve for a guest visitor -- only sees the guest promo
resolved, _ := widgetSvc.ResolveArea(ctx, widgets.ResolveAreaInput{
    AreaCode: "sidebar.promotions",
    Audience: []string{"guest"},
})
// len(resolved) == 1, showing "Sign up for 10% off!"
```

### Footer Newsletter Signup

A globally-visible newsletter widget with locale-aware content:

```go
// Define the area
widgetSvc.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
    Code:  "footer.newsletter",
    Name:  "Footer Newsletter",
    Scope: widgets.AreaScopeGlobal,
})

// Create the widget instance (no visibility restrictions -- always visible)
newsletter, _ := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
    DefinitionID: newsletterDef.ID,
    Configuration: map[string]any{
        "heading":     "Stay in the loop",
        "button_text": "Subscribe",
    },
    Position:  0,
    CreatedBy: userID,
    UpdatedBy: userID,
})

// Add translations
widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
    InstanceID: newsletter.ID,
    LocaleID:   enLocaleID,
    Content: map[string]any{
        "heading":     "Stay in the loop",
        "placeholder": "Enter your email",
        "button_text": "Subscribe",
    },
})

widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
    InstanceID: newsletter.ID,
    LocaleID:   esLocaleID,
    Content: map[string]any{
        "heading":     "Mantente informado",
        "placeholder": "Introduce tu email",
        "button_text": "Suscribirse",
    },
})

// Assign to area
widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
    AreaCode:   "footer.newsletter",
    InstanceID: newsletter.ID,
    Position:   intPtr(0),
})

// Resolve for Spanish locale with English fallback
resolved, _ := widgetSvc.ResolveArea(ctx, widgets.ResolveAreaInput{
    AreaCode:          "footer.newsletter",
    LocaleID:          &esLocaleID,
    FallbackLocaleIDs: []uuid.UUID{enLocaleID},
})
```

### Time-Limited Announcement Banner

A site-wide announcement that appears during a specific date range:

```go
widgetSvc.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
    Code:  "header.announcements",
    Name:  "Header Announcements",
    Scope: widgets.AreaScopeGlobal,
})

announcement, _ := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
    DefinitionID: announcementDef.ID,
    Configuration: map[string]any{
        "message":     "We'll be performing maintenance this Saturday",
        "severity":    "warning",
        "dismissible": true,
    },
    PublishOn:   timePtr(time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)),
    UnpublishOn: timePtr(time.Date(2025, 3, 15, 23, 59, 59, 0, time.UTC)),
    Position:    0,
    CreatedBy:   userID,
    UpdatedBy:   userID,
})

widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
    AreaCode:   "header.announcements",
    InstanceID: announcement.ID,
    Position:   intPtr(0),
})

// Only resolves to a non-empty list between March 10-15
resolved, _ := widgetSvc.ResolveArea(ctx, widgets.ResolveAreaInput{
    AreaCode: "header.announcements",
    Now:      time.Now().UTC(),
})
```

---

## Error Reference

| Error | Cause |
|-------|-------|
| `ErrFeatureDisabled` | Operation called but `cfg.Features.Widgets` is `false` |
| `ErrDefinitionNameRequired` | `RegisterDefinition` called without a name |
| `ErrDefinitionSchemaRequired` | No schema provided on registration |
| `ErrDefinitionSchemaInvalid` | Schema fails structural validation |
| `ErrDefinitionDefaultsInvalid` | Defaults do not match the definition schema |
| `ErrDefinitionExists` | A definition with the same name already exists |
| `ErrDefinitionInUse` | Delete attempted on definition with active instances |
| `ErrDefinitionSoftDeleteUnsupported` | Soft delete attempted; only hard delete is supported |
| `ErrInstanceDefinitionRequired` | `CreateInstance` called without definition ID |
| `ErrInstanceCreatorRequired` | `CreatedBy` not provided on create |
| `ErrInstanceUpdaterRequired` | `UpdatedBy` not provided on mutation |
| `ErrInstanceIDRequired` | Update or delete called without instance ID |
| `ErrInstancePositionInvalid` | Position is negative |
| `ErrInstanceConfigurationInvalid` | Configuration does not match definition schema |
| `ErrInstanceScheduleInvalid` | `PublishOn` is after `UnpublishOn` |
| `ErrInstanceSoftDeleteUnsupported` | Soft delete attempted; only hard delete is supported |
| `ErrVisibilityRulesInvalid` | Unknown keys in visibility rules |
| `ErrVisibilityScheduleInvalid` | Schedule timestamp cannot be parsed |
| `ErrVisibilityLocaleRestricted` | Widget restricted to locales that exclude the current locale |
| `ErrTranslationContentRequired` | Translation content is nil |
| `ErrTranslationLocaleRequired` | Locale ID not provided |
| `ErrTranslationExists` | Translation already exists for this instance and locale |
| `ErrTranslationNotFound` | No translation exists for the requested instance and locale |
| `ErrAreaCodeRequired` | Area code not provided |
| `ErrAreaCodeInvalid` | Area code contains invalid characters |
| `ErrAreaNameRequired` | Area name not provided |
| `ErrAreaDefinitionExists` | Area with the same code already exists |
| `ErrAreaDefinitionNotFound` | Area with the specified code does not exist |
| `ErrAreaFeatureDisabled` | Area repositories not initialized |
| `ErrAreaInstanceRequired` | Instance ID not provided for area operation |
| `ErrAreaPlacementExists` | Duplicate placement for (area, locale, instance) |
| `ErrAreaPlacementPosition` | Placement position is negative |
| `ErrAreaPlacementNotFound` | Placement not found for removal |
| `ErrAreaWidgetOrderMismatch` | Reorder items do not match existing placements exactly |

---

## Complete Example

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"
    "time"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/widgets"
    "github.com/google/uuid"
)

func main() {
    ctx := context.Background()

    // 1. Configure with widgets enabled
    cfg := cms.DefaultConfig()
    cfg.Features.Widgets = true
    cfg.DefaultLocale = "en"
    cfg.I18N.Locales = []string{"en", "es"}

    module, err := cms.New(cfg)
    if err != nil {
        log.Fatal(err)
    }

    widgetSvc := module.Widgets()
    userID := uuid.New()
    enLocaleID := uuid.New()
    esLocaleID := uuid.New()

    // 2. Bootstrap definitions and areas
    err = widgets.Bootstrap(ctx, widgetSvc, widgets.BootstrapConfig{
        Definitions: []widgets.RegisterDefinitionInput{
            {
                Name: "Promo Banner",
                Schema: map[string]any{
                    "type": "object",
                    "properties": map[string]any{
                        "headline": map[string]any{"type": "string"},
                        "cta_text": map[string]any{"type": "string"},
                        "cta_url":  map[string]any{"type": "string"},
                    },
                    "required": []string{"headline"},
                },
                Defaults: map[string]any{"cta_text": "Learn more"},
            },
            {
                Name: "Newsletter Signup",
                Schema: map[string]any{
                    "type": "object",
                    "properties": map[string]any{
                        "heading":     map[string]any{"type": "string"},
                        "button_text": map[string]any{"type": "string"},
                    },
                },
                Defaults: map[string]any{"button_text": "Subscribe"},
            },
        },
        Areas: []widgets.RegisterAreaDefinitionInput{
            {Code: "sidebar.primary", Name: "Primary Sidebar", Scope: widgets.AreaScopeGlobal},
            {Code: "footer.newsletter", Name: "Footer Newsletter", Scope: widgets.AreaScopeGlobal},
        },
    })
    if err != nil && !errors.Is(err, widgets.ErrFeatureDisabled) {
        log.Fatalf("bootstrap: %v", err)
    }
    fmt.Println("Definitions and areas bootstrapped")

    // 3. Look up definitions
    definitions, _ := widgetSvc.ListDefinitions(ctx)
    var promoDef, newsletterDef *widgets.Definition
    for _, d := range definitions {
        switch d.Name {
        case "Promo Banner":
            promoDef = d
        case "Newsletter Signup":
            newsletterDef = d
        }
    }

    // 4. Create a promo widget instance with visibility rules
    promo, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
        DefinitionID: promoDef.ID,
        Configuration: map[string]any{
            "headline": "Summer Sale - 50% Off",
            "cta_text": "Shop Now",
            "cta_url":  "/sale",
        },
        VisibilityRules: map[string]any{
            "audience": []any{"guest"},
            "schedule": map[string]any{
                "starts_at": "2025-06-01T00:00:00Z",
                "ends_at":   "2025-08-31T23:59:59Z",
            },
        },
        Position:  0,
        CreatedBy: userID,
        UpdatedBy: userID,
    })
    if err != nil {
        log.Fatalf("create promo: %v", err)
    }
    fmt.Printf("Promo instance: %s\n", promo.ID)

    // 5. Create a newsletter widget instance (always visible)
    newsletter, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
        DefinitionID: newsletterDef.ID,
        Configuration: map[string]any{
            "heading":     "Stay updated",
            "button_text": "Subscribe",
        },
        Position:  0,
        CreatedBy: userID,
        UpdatedBy: userID,
    })
    if err != nil {
        log.Fatalf("create newsletter: %v", err)
    }
    fmt.Printf("Newsletter instance: %s\n", newsletter.ID)

    // 6. Add translations
    widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
        InstanceID: promo.ID,
        LocaleID:   enLocaleID,
        Content:    map[string]any{"headline": "Summer Sale - 50% Off", "cta_text": "Shop Now"},
    })
    widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
        InstanceID: promo.ID,
        LocaleID:   esLocaleID,
        Content:    map[string]any{"headline": "Rebajas de verano - 50%", "cta_text": "Comprar"},
    })
    widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
        InstanceID: newsletter.ID,
        LocaleID:   enLocaleID,
        Content:    map[string]any{"heading": "Stay updated", "button_text": "Subscribe"},
    })
    widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
        InstanceID: newsletter.ID,
        LocaleID:   esLocaleID,
        Content:    map[string]any{"heading": "Mantente informado", "button_text": "Suscribirse"},
    })
    fmt.Println("Translations added")

    // 7. Assign widgets to areas
    widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
        AreaCode:   "sidebar.primary",
        InstanceID: promo.ID,
        Position:   intPtr(0),
    })
    widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
        AreaCode:   "sidebar.primary",
        InstanceID: newsletter.ID,
        Position:   intPtr(1),
    })
    widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
        AreaCode:   "footer.newsletter",
        InstanceID: newsletter.ID,
        Position:   intPtr(0),
    })
    fmt.Println("Widgets assigned to areas")

    // 8. Resolve the sidebar for a guest visitor
    now := time.Date(2025, 7, 15, 12, 0, 0, 0, time.UTC) // mid-summer
    resolved, err := widgetSvc.ResolveArea(ctx, widgets.ResolveAreaInput{
        AreaCode:          "sidebar.primary",
        LocaleID:          &enLocaleID,
        FallbackLocaleIDs: []uuid.UUID{},
        Audience:          []string{"guest"},
        Now:               now,
    })
    if err != nil {
        log.Fatalf("resolve area: %v", err)
    }

    fmt.Printf("\nSidebar widgets for guest (%d):\n", len(resolved))
    for _, rw := range resolved {
        fmt.Printf("  Position %d: %v\n", rw.Placement.Position, rw.Instance.Configuration)
    }

    // 9. Resolve for a member visitor (promo not visible to members)
    memberResolved, _ := widgetSvc.ResolveArea(ctx, widgets.ResolveAreaInput{
        AreaCode: "sidebar.primary",
        LocaleID: &enLocaleID,
        Audience: []string{"member"},
        Now:      now,
    })
    fmt.Printf("\nSidebar widgets for member (%d):\n", len(memberResolved))
    for _, rw := range memberResolved {
        fmt.Printf("  Position %d: %v\n", rw.Placement.Position, rw.Instance.Configuration)
    }
}

func timePtr(t time.Time) *time.Time { return &t }
func stringPtr(s string) *string     { return &s }
func intPtr(i int) *int              { return &i }
```

---

## Next Steps

- [GUIDE_BLOCKS.md](GUIDE_BLOCKS.md) -- reusable content fragments with definitions, instances, and translations
- [GUIDE_MENUS.md](GUIDE_MENUS.md) -- navigation structures with URL resolution and i18n support
- [GUIDE_I18N.md](GUIDE_I18N.md) -- internationalization, locale management, and translation workflows
- [GUIDE_THEMES.md](GUIDE_THEMES.md) -- theme management, template registration, and region definitions
- [GUIDE_CONFIGURATION.md](GUIDE_CONFIGURATION.md) -- full config reference and DI container wiring
