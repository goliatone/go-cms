# Blocks Guide

This guide covers block definitions, block instances, translations, versioning, and the registry pattern in `go-cms`. By the end you will understand how to define reusable content fragments, place them on pages or globally, manage localized content per block, and use the draft/publish workflow for block instances.

## Block Architecture Overview

Blocks in `go-cms` are reusable content fragments built on three layers:

- **Block definitions** describe the shape and validation rules for a category of block. Each definition carries a JSON schema, a slug, optional defaults, and a UI schema. Think of a definition as a blueprint -- "Hero Banner", "Call to Action", "Feature Grid".
- **Block instances** are concrete placements of a definition on a page region or as a global block. Each instance holds its own configuration and references a definition by ID.
- **Block translations** provide localized content for each instance. A translation carries a locale, JSON content, optional attribute overrides, and media bindings.

```
Definition (schema, slug, defaults)
  ├── DefinitionVersion (schema revision snapshot)
  └── Instance (page, region, position, configuration)
        ├── Translation (locale, content, media bindings)
        ├── Translation (locale, content, media bindings)
        └── InstanceVersion (snapshot, status)
```

All entities use UUID primary keys and UTC timestamps.

### Accessing the Service

Block operations are exposed through the `cms.Module` facade:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es"}

module, err := cms.New(cfg)
if err != nil {
    log.Fatal(err)
}

blocksSvc := module.Blocks()
```

The `blocksSvc` variable satisfies the `blocks.Service` interface. The service delegates to in-memory repositories by default, or SQL-backed repositories when `di.WithBunDB(db)` is provided.

---

## Block Definitions

### Registering a Definition

A block definition requires a name and a JSON schema at minimum. The slug is auto-generated from the name if omitted.

```go
heroDef, err := blocksSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
    Name: "Hero Banner",
    Slug: "hero-banner",
    Description: stringPtr("Full-width hero section with heading and CTA"),
    Icon:     stringPtr("hero"),
    Category: stringPtr("layout"),
    Status:   "active",
    Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "heading":    map[string]any{"type": "string"},
            "subheading": map[string]any{"type": "string"},
            "cta_text":   map[string]any{"type": "string"},
            "cta_url":    map[string]any{"type": "string"},
            "image_url":  map[string]any{"type": "string"},
        },
        "required": []string{"heading"},
    },
    Defaults: map[string]any{
        "cta_text": "Learn More",
    },
})
```

**`RegisterDefinitionInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Name` | `string` | Yes | Display name for the block definition |
| `Slug` | `string` | No | URL-safe identifier; auto-generated from name if empty |
| `Description` | `*string` | No | Human-readable description |
| `Icon` | `*string` | No | Icon identifier for admin UIs |
| `Category` | `*string` | No | Grouping category (e.g. "layout", "content", "media") |
| `Status` | `string` | No | Initial status; defaults to `"draft"` |
| `Schema` | `map[string]any` | Yes | JSON schema defining the block structure |
| `UISchema` | `map[string]any` | No | UI rendering hints for admin interfaces |
| `Defaults` | `map[string]any` | No | Default configuration values for new instances |
| `EditorStyleURL` | `*string` | No | URL to editor-specific stylesheet |
| `FrontendStyleURL` | `*string` | No | URL to frontend stylesheet |
| `EnvironmentKey` | `string` | No | Environment scope; defaults per config |

**What happens:**

1. The name and slug are validated (slug is normalized via `go-slug`)
2. The JSON schema is validated and assigned a semantic version
3. A deterministic UUID is generated from the slug and environment
4. The definition is persisted to the repository
5. An activity event is emitted when activity tracking is enabled

### Schema Validation

Schemas are validated on register and update. The service uses `schema.EnsureSchemaVersion` to assign a semantic version to each schema. When updating a definition, schema changes produce a new version automatically.

Schemas follow JSON Schema conventions:

```go
schema := map[string]any{
    "type": "object",
    "properties": map[string]any{
        "title":       map[string]any{"type": "string"},
        "description": map[string]any{"type": "string"},
        "columns":     map[string]any{"type": "integer", "minimum": 1, "maximum": 12},
    },
    "required": []string{"title"},
}
```

### Listing Definitions

```go
// List all definitions
definitions, err := blocksSvc.ListDefinitions(ctx)

// List definitions scoped to an environment
definitions, err := blocksSvc.ListDefinitions(ctx, "production")
```

### Getting a Definition

```go
def, err := blocksSvc.GetDefinition(ctx, definitionID)
if err != nil {
    // Returns NotFoundError when definition does not exist
    log.Fatalf("get definition: %v", err)
}
fmt.Printf("Definition: %s (slug=%s)\n", def.Name, def.Slug)
```

### Updating a Definition

Update uses pointer fields so only specified fields are modified:

```go
updated, err := blocksSvc.UpdateDefinition(ctx, blocks.UpdateDefinitionInput{
    ID:          heroDef.ID,
    Description: stringPtr("Updated hero section with video support"),
    Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "heading":    map[string]any{"type": "string"},
            "subheading": map[string]any{"type": "string"},
            "cta_text":   map[string]any{"type": "string"},
            "cta_url":    map[string]any{"type": "string"},
            "image_url":  map[string]any{"type": "string"},
            "video_url":  map[string]any{"type": "string"},
        },
        "required": []string{"heading"},
    },
})
```

**`UpdateDefinitionInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ID` | `uuid.UUID` | Yes | Definition identifier |
| `Name` | `*string` | No | Updated display name |
| `Slug` | `*string` | No | Updated slug |
| `Description` | `*string` | No | Updated description |
| `Icon` | `*string` | No | Updated icon |
| `Category` | `*string` | No | Updated category |
| `Status` | `*string` | No | Updated status |
| `Schema` | `map[string]any` | No | Updated JSON schema |
| `UISchema` | `map[string]any` | No | Updated UI schema |
| `Defaults` | `map[string]any` | No | Updated default values |
| `EditorStyleURL` | `*string` | No | Updated editor stylesheet URL |
| `FrontendStyleURL` | `*string` | No | Updated frontend stylesheet URL |
| `EnvironmentKey` | `*string` | No | Updated environment scope |

### Deleting a Definition

Definitions require hard delete. Soft delete is not supported. A definition cannot be deleted if it has active instances.

```go
err := blocksSvc.DeleteDefinition(ctx, blocks.DeleteDefinitionRequest{
    ID:         heroDef.ID,
    HardDelete: true,
})
if err != nil {
    // Returns ErrDefinitionInUse if instances reference this definition
    log.Fatalf("delete definition: %v", err)
}
```

---

## Definition Versioning

Definition versions track schema revisions independently of definition updates. Each version captures a snapshot of the schema and defaults at a point in time.

### Creating a Definition Version

```go
defVersion, err := blocksSvc.CreateDefinitionVersion(ctx, blocks.CreateDefinitionVersionInput{
    DefinitionID: heroDef.ID,
    Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "heading":    map[string]any{"type": "string"},
            "subheading": map[string]any{"type": "string"},
            "cta_text":   map[string]any{"type": "string"},
            "cta_url":    map[string]any{"type": "string"},
            "image_url":  map[string]any{"type": "string"},
            "video_url":  map[string]any{"type": "string"},
            "overlay":    map[string]any{"type": "boolean"},
        },
        "required": []string{"heading"},
    },
    Defaults: map[string]any{
        "cta_text": "Learn More",
        "overlay":  false,
    },
})
fmt.Printf("Version: %s (id=%s)\n", defVersion.SchemaVersion, defVersion.ID)
```

**`CreateDefinitionVersionInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DefinitionID` | `uuid.UUID` | Yes | Parent definition identifier |
| `Schema` | `map[string]any` | Yes | JSON schema for this version |
| `Defaults` | `map[string]any` | No | Default values for this version |

### Querying Definition Versions

```go
// Get a specific version
version, err := blocksSvc.GetDefinitionVersion(ctx, heroDef.ID, "1.0.0")

// List all versions for a definition
versions, err := blocksSvc.ListDefinitionVersions(ctx, heroDef.ID)
for _, v := range versions {
    fmt.Printf("  %s: created=%s\n", v.SchemaVersion, v.CreatedAt)
}
```

---

## Block Instances

Instances are concrete placements of a block definition. An instance can be assigned to a specific page region or marked as global (shared across pages).

### Creating an Instance

```go
heroInstance, err := blocksSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
    DefinitionID: heroDef.ID,
    PageID:       &pageID,        // nil for global blocks
    Region:       "header",
    Position:     0,
    Configuration: map[string]any{
        "heading": "Welcome to Our Site",
        "cta_url": "/getting-started",
    },
    IsGlobal:  false,
    CreatedBy: authorID,
    UpdatedBy: authorID,
})
```

**`CreateInstanceInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `DefinitionID` | `uuid.UUID` | Yes | Block definition this instance uses |
| `PageID` | `*uuid.UUID` | No | Page this block belongs to; nil for global blocks |
| `Region` | `string` | Yes | Template region name (e.g. "header", "sidebar", "footer") |
| `Position` | `int` | No | Sort order within the region; must be >= 0 |
| `Configuration` | `map[string]any` | No | Instance-specific configuration data |
| `IsGlobal` | `bool` | No | Whether this block appears on all pages |
| `CreatedBy` | `uuid.UUID` | Yes | Actor identifier |
| `UpdatedBy` | `uuid.UUID` | Yes | Actor identifier |

**What happens:**

1. The definition ID is validated (must exist)
2. The region name is required and validated
3. Position must be non-negative
4. Configuration is stored as JSON
5. The instance is persisted with a new UUID

### Listing Instances

```go
// List all block instances for a page
pageBlocks, err := blocksSvc.ListPageInstances(ctx, pageID)
for _, b := range pageBlocks {
    fmt.Printf("  %s in %s (pos=%d)\n", b.DefinitionID, b.Region, b.Position)
}

// List all global block instances
globalBlocks, err := blocksSvc.ListGlobalInstances(ctx)
for _, b := range globalBlocks {
    fmt.Printf("  Global: %s in %s\n", b.DefinitionID, b.Region)
}
```

### Updating an Instance

Update uses pointer fields so only specified fields are modified:

```go
updated, err := blocksSvc.UpdateInstance(ctx, blocks.UpdateInstanceInput{
    InstanceID: heroInstance.ID,
    Position:   intPtr(1),
    Configuration: map[string]any{
        "heading": "Welcome Back!",
        "cta_url": "/dashboard",
    },
    UpdatedBy: authorID,
})
```

**`UpdateInstanceInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `InstanceID` | `uuid.UUID` | Yes | Instance identifier |
| `PageID` | `*uuid.UUID` | No | Move to a different page |
| `Region` | `*string` | No | Move to a different region |
| `Position` | `*int` | No | Updated sort order |
| `Configuration` | `map[string]any` | No | Updated configuration |
| `IsGlobal` | `*bool` | No | Toggle global visibility |
| `UpdatedBy` | `uuid.UUID` | Yes | Actor identifier |

### Deleting an Instance

Instances require hard delete. Soft delete is not supported.

```go
err := blocksSvc.DeleteInstance(ctx, blocks.DeleteInstanceRequest{
    ID:         heroInstance.ID,
    DeletedBy:  authorID,
    HardDelete: true,
})
```

---

## Block Translations

Translations provide localized content for each block instance. Each translation is scoped to a locale and carries JSON content, optional attribute overrides, and media bindings.

### Adding a Translation

```go
enTranslation, err := blocksSvc.AddTranslation(ctx, blocks.AddTranslationInput{
    BlockInstanceID: heroInstance.ID,
    LocaleID:        enLocaleID,
    Content: map[string]any{
        "heading":    "Welcome to Our Site",
        "subheading": "Discover what we have to offer",
        "cta_text":   "Get Started",
    },
    AttributeOverrides: map[string]any{
        "aria-label": "Hero banner section",
    },
})
```

```go
esTranslation, err := blocksSvc.AddTranslation(ctx, blocks.AddTranslationInput{
    BlockInstanceID: heroInstance.ID,
    LocaleID:        esLocaleID,
    Content: map[string]any{
        "heading":    "Bienvenido a nuestro sitio",
        "subheading": "Descubre lo que tenemos para ofrecer",
        "cta_text":   "Comenzar",
    },
})
```

**`AddTranslationInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BlockInstanceID` | `uuid.UUID` | Yes | Parent block instance |
| `LocaleID` | `uuid.UUID` | Yes | Locale identifier |
| `Content` | `map[string]any` | Yes | Localized content payload |
| `AttributeOverrides` | `map[string]any` | No | Locale-specific attribute overrides |
| `MediaBindings` | `media.BindingSet` | No | Media attachment bindings |

**What happens:**

1. The instance and locale IDs are validated
2. Content is required and validated against the definition schema (when schema validation is enabled)
3. A duplicate translation for the same instance and locale returns `ErrTranslationExists`
4. The translation is persisted with a new UUID

### Getting a Translation

```go
translation, err := blocksSvc.GetTranslation(ctx, heroInstance.ID, enLocaleID)
fmt.Printf("Content: %v\n", translation.Content)
```

### Updating a Translation

```go
updated, err := blocksSvc.UpdateTranslation(ctx, blocks.UpdateTranslationInput{
    BlockInstanceID: heroInstance.ID,
    LocaleID:        enLocaleID,
    Content: map[string]any{
        "heading":    "Welcome Back!",
        "subheading": "New features are waiting for you",
        "cta_text":   "Explore Now",
    },
    UpdatedBy: authorID,
})
```

**`UpdateTranslationInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BlockInstanceID` | `uuid.UUID` | Yes | Parent block instance |
| `LocaleID` | `uuid.UUID` | Yes | Locale identifier |
| `Content` | `map[string]any` | No | Updated localized content |
| `AttributeOverrides` | `map[string]any` | No | Updated attribute overrides |
| `MediaBindings` | `media.BindingSet` | No | Updated media bindings |
| `UpdatedBy` | `uuid.UUID` | Yes | Actor identifier |

### Deleting a Translation

```go
err := blocksSvc.DeleteTranslation(ctx, blocks.DeleteTranslationRequest{
    BlockInstanceID:          heroInstance.ID,
    LocaleID:                 esLocaleID,
    DeletedBy:                authorID,
    AllowMissingTranslations: false, // Set true to bypass minimum translation enforcement
})
```

When `cfg.I18N.RequireTranslations` is `true` (the default), deleting the last translation returns `ErrTranslationMinimum`. Set `AllowMissingTranslations` to `true` on the request to bypass this check for staging or workflow transitions.

---

## Instance Versioning

When `cfg.Features.Versioning` is enabled, block instances support a draft/publish workflow. Versions capture snapshots of the instance configuration and all translations at a point in time.

### Enabling Versioning

```go
cfg := cms.DefaultConfig()
cfg.Features.Versioning = true

module, err := cms.New(cfg)
```

### Creating a Draft

A draft captures a snapshot of the block instance's current configuration and translations:

```go
draft, err := blocksSvc.CreateDraft(ctx, blocks.CreateInstanceDraftRequest{
    InstanceID: heroInstance.ID,
    Snapshot: blocks.BlockVersionSnapshot{
        Configuration: map[string]any{
            "heading": "Welcome Back!",
            "cta_url": "/dashboard",
        },
        Translations: []blocks.BlockVersionTranslationSnapshot{
            {
                Locale:  "en",
                Content: map[string]any{"heading": "Welcome Back!", "cta_text": "Explore"},
            },
            {
                Locale:  "es",
                Content: map[string]any{"heading": "Bienvenido!", "cta_text": "Explorar"},
            },
        },
    },
    CreatedBy: authorID,
    UpdatedBy: authorID,
})
fmt.Printf("Draft: version=%d status=%s\n", draft.Version, draft.Status)
```

**`CreateInstanceDraftRequest` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `InstanceID` | `uuid.UUID` | Yes | Block instance identifier |
| `Snapshot` | `BlockVersionSnapshot` | Yes | Configuration and translation snapshot |
| `CreatedBy` | `uuid.UUID` | Yes | Actor identifier |
| `UpdatedBy` | `uuid.UUID` | Yes | Actor identifier |
| `BaseVersion` | `*int` | No | Expected current version for optimistic concurrency |

**`BlockVersionSnapshot` fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Configuration` | `map[string]any` | Instance configuration at this version |
| `Translations` | `[]BlockVersionTranslationSnapshot` | Localized content snapshots |
| `Metadata` | `map[string]any` | Arbitrary metadata |
| `Media` | `media.BindingSet` | Media bindings at this version |

When `BaseVersion` is provided, the service checks for conflicts. If the instance's current version does not match, `ErrInstanceVersionConflict` is returned.

### Publishing a Draft

```go
published, err := blocksSvc.PublishDraft(ctx, blocks.PublishInstanceDraftRequest{
    InstanceID:  heroInstance.ID,
    Version:     draft.Version,
    PublishedBy: authorID,
    PublishedAt: timePtr(time.Now()),
})
fmt.Printf("Published: version=%d\n", published.Version)
```

**`PublishInstanceDraftRequest` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `InstanceID` | `uuid.UUID` | Yes | Block instance identifier |
| `Version` | `int` | Yes | Draft version number to publish |
| `PublishedBy` | `uuid.UUID` | Yes | Actor identifier |
| `PublishedAt` | `*time.Time` | No | Publish timestamp; defaults to now |

**What happens:**

1. The draft version is validated (must exist, must not already be published)
2. The version status transitions from `draft` to `published`
3. The instance's `PublishedVersion`, `PublishedAt`, and `PublishedBy` fields are updated
4. Previously published versions are archived

### Listing Versions

```go
versions, err := blocksSvc.ListVersions(ctx, heroInstance.ID)
for _, v := range versions {
    fmt.Printf("  Version %d: status=%s created=%s\n", v.Version, v.Status, v.CreatedAt)
}
```

### Restoring a Version

Restore creates a new draft from a previously recorded version snapshot:

```go
restored, err := blocksSvc.RestoreVersion(ctx, blocks.RestoreInstanceVersionRequest{
    InstanceID: heroInstance.ID,
    Version:    1,
    RestoredBy: authorID,
})
fmt.Printf("Restored as version=%d\n", restored.Version)
```

**Version retention:** The maximum number of versions per instance is controlled by `cfg.Retention.Blocks`. When the limit is reached, `ErrInstanceVersionRetentionExceeded` is returned.

---

## Registry and SyncRegistry

The block registry enables programmatic registration of block definitions at application startup. Definitions registered in the registry are synced to the database when `SyncRegistry` is called.

### Creating a Registry

```go
registry := blocks.NewRegistry()

registry.Register(blocks.RegisterDefinitionInput{
    Name: "Hero Banner",
    Slug: "hero-banner",
    Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "heading":    map[string]any{"type": "string"},
            "subheading": map[string]any{"type": "string"},
            "cta_text":   map[string]any{"type": "string"},
        },
        "required": []string{"heading"},
    },
    Defaults: map[string]any{"cta_text": "Learn More"},
    Status:   "active",
})

registry.Register(blocks.RegisterDefinitionInput{
    Name: "Feature Grid",
    Slug: "feature-grid",
    Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "columns": map[string]any{"type": "integer", "minimum": 1, "maximum": 4},
            "items":   map[string]any{"type": "array"},
        },
    },
    Defaults: map[string]any{"columns": 3},
    Status:   "active",
})
```

### Syncing to the Database

Once the registry is populated and attached to the service, call `SyncRegistry` to persist all definitions:

```go
cfg := cms.DefaultConfig()
module, err := cms.New(cfg, di.WithBlockRegistry(registry))
if err != nil {
    log.Fatal(err)
}

blocksSvc := module.Blocks()
if err := blocksSvc.SyncRegistry(ctx); err != nil {
    log.Fatalf("sync registry: %v", err)
}
```

`SyncRegistry` iterates over all entries in the registry and calls `RegisterDefinition` for each. Existing definitions (matched by slug) are updated; new definitions are created.

### Schema Version Tracking

The registry automatically tracks schema versions. When you register multiple versions of the same block name, the registry keeps the latest version as the primary entry:

```go
// Register v1
registry.Register(blocks.RegisterDefinitionInput{
    Name: "Hero Banner",
    Schema: map[string]any{
        "type":       "object",
        "properties": map[string]any{"heading": map[string]any{"type": "string"}},
    },
})

// Register v2 with additional fields
registry.Register(blocks.RegisterDefinitionInput{
    Name: "Hero Banner",
    Schema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "heading":    map[string]any{"type": "string"},
            "subheading": map[string]any{"type": "string"},
        },
    },
})

// List returns only the latest version per name
entries := registry.List() // len(entries) == 1, contains v2 schema

// ListVersions returns all registered versions for a name
allVersions := registry.ListVersions("Hero Banner") // returns v1 and v2
```

### Schema Migrations

The registry supports migrations between schema versions. Register migration functions that transform data payloads from one schema version to another:

```go
registry.RegisterMigration("hero-banner", "1.0.0", "2.0.0", func(data map[string]any) (map[string]any, error) {
    // Migrate "title" field to "heading"
    if title, ok := data["title"]; ok {
        data["heading"] = title
        delete(data, "title")
    }
    return data, nil
})
```

Migrations are applied automatically when publishing drafts if the instance's schema version differs from the definition's current version.

---

## Embedded Blocks

The embedded blocks bridge coordinates block data that lives inline within content entries alongside traditional block instances. This pattern supports content types that embed block configurations directly in their JSON payload.

The bridge (`EmbeddedBlockBridge`) provides:

- **`SyncEmbeddedBlocks`** -- Projects embedded block payloads from content into legacy block instances (dual-write pattern)
- **`MergeLegacyBlocks`** -- Populates embedded block payloads in content when they are missing
- **`MigrateEmbeddedBlocks`** -- Upgrades embedded blocks to the latest definition schema
- **`ValidateEmbeddedBlocks`** -- Validates embedded block payloads against their definition schemas
- **`ValidateBlockAvailability`** -- Enforces content-type block availability constraints
- **`InstancesFromEmbeddedContent`** -- Builds in-memory block instances from embedded payloads without persistence
- **`BackfillFromLegacy`** -- Writes embedded blocks into stored content translations from legacy instances
- **`ListConflicts`** -- Reports discrepancies between embedded and legacy block data

This bridge is primarily used internally by the content service during create and update operations. Direct use is needed only for migration or admin scenarios.

---

## Block Admin Service

The block admin service exposes administrative operations for managing embedded blocks:

```go
blockAdmin := module.BlockAdmin()

// List conflicts between embedded and legacy block data
conflicts, err := blockAdmin.ListConflicts(ctx, opts)

// Backfill embedded blocks from legacy instances
report, err := blockAdmin.BackfillEmbeddedBlocks(ctx, opts)
```

This service is useful during migrations from the legacy block instance model to the embedded block model.

---

## Error Reference

| Error | Cause |
|-------|-------|
| `ErrDefinitionNameRequired` | `RegisterDefinition` called without a name |
| `ErrDefinitionSlugRequired` | Slug is empty and cannot be generated from name |
| `ErrDefinitionSlugInvalid` | Slug contains invalid characters |
| `ErrDefinitionSlugExists` | Another definition already uses this slug |
| `ErrDefinitionSchemaRequired` | No schema provided on registration |
| `ErrDefinitionSchemaInvalid` | Schema fails JSON schema validation |
| `ErrDefinitionSchemaVersionInvalid` | Schema version string cannot be parsed |
| `ErrDefinitionIDRequired` | Update or delete called without definition ID |
| `ErrDefinitionInUse` | Delete attempted on definition with active instances |
| `ErrDefinitionSoftDeleteUnsupported` | Soft delete attempted; only hard delete is supported |
| `ErrDefinitionVersionRequired` | Version identifier missing on version query |
| `ErrDefinitionVersionExists` | Schema version already recorded for this definition |
| `ErrInstanceDefinitionRequired` | `CreateInstance` called without definition ID |
| `ErrInstanceRegionRequired` | `CreateInstance` called without region name |
| `ErrInstancePositionInvalid` | Position is negative |
| `ErrInstanceIDRequired` | Update or delete called without instance ID |
| `ErrInstanceUpdaterRequired` | `UpdatedBy` not provided on mutation |
| `ErrInstanceSoftDeleteUnsupported` | Soft delete attempted; only hard delete is supported |
| `ErrTranslationContentRequired` | Translation content is nil or empty |
| `ErrTranslationExists` | Translation already exists for this instance and locale |
| `ErrTranslationLocaleRequired` | Locale ID not provided |
| `ErrTranslationSchemaInvalid` | Translation content fails schema validation |
| `ErrTranslationNotFound` | No translation exists for the requested instance and locale |
| `ErrTranslationMinimum` | Deleting would leave zero translations when translations are required |
| `ErrVersioningDisabled` | Versioning operation called but `cfg.Features.Versioning` is false |
| `ErrInstanceVersionRequired` | Version number not provided |
| `ErrInstanceVersionConflict` | Base version mismatch during draft creation |
| `ErrInstanceVersionAlreadyPublished` | Publish attempted on already-published version |
| `ErrInstanceVersionRetentionExceeded` | Version count exceeds `cfg.Retention.Blocks` limit |

---

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/blocks"
    "github.com/google/uuid"
)

func main() {
    ctx := context.Background()

    // Configure with versioning enabled
    cfg := cms.DefaultConfig()
    cfg.DefaultLocale = "en"
    cfg.I18N.Locales = []string{"en", "es"}
    cfg.Features.Versioning = true

    module, err := cms.New(cfg)
    if err != nil {
        log.Fatal(err)
    }

    blocksSvc := module.Blocks()
    authorID := uuid.New()
    enLocaleID := uuid.New()
    esLocaleID := uuid.New()
    pageID := uuid.New()

    // 1. Register a block definition
    heroDef, err := blocksSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
        Name:     "Hero Banner",
        Slug:     "hero-banner",
        Category: stringPtr("layout"),
        Status:   "active",
        Schema: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "heading":    map[string]any{"type": "string"},
                "subheading": map[string]any{"type": "string"},
                "cta_text":   map[string]any{"type": "string"},
                "cta_url":    map[string]any{"type": "string"},
            },
            "required": []string{"heading"},
        },
        Defaults: map[string]any{"cta_text": "Learn More"},
    })
    if err != nil {
        log.Fatalf("register definition: %v", err)
    }
    fmt.Printf("Definition: %s (slug=%s)\n", heroDef.Name, heroDef.Slug)

    // 2. Create a block instance on a page
    heroInstance, err := blocksSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
        DefinitionID: heroDef.ID,
        PageID:       &pageID,
        Region:       "header",
        Position:     0,
        Configuration: map[string]any{
            "heading": "Welcome to Our Site",
            "cta_url": "/getting-started",
        },
        CreatedBy: authorID,
        UpdatedBy: authorID,
    })
    if err != nil {
        log.Fatalf("create instance: %v", err)
    }
    fmt.Printf("Instance: %s in region=%s\n", heroInstance.ID, heroInstance.Region)

    // 3. Add translations for English and Spanish
    _, err = blocksSvc.AddTranslation(ctx, blocks.AddTranslationInput{
        BlockInstanceID: heroInstance.ID,
        LocaleID:        enLocaleID,
        Content: map[string]any{
            "heading":    "Welcome to Our Site",
            "subheading": "Discover what we offer",
            "cta_text":   "Get Started",
        },
    })
    if err != nil {
        log.Fatalf("add EN translation: %v", err)
    }

    _, err = blocksSvc.AddTranslation(ctx, blocks.AddTranslationInput{
        BlockInstanceID: heroInstance.ID,
        LocaleID:        esLocaleID,
        Content: map[string]any{
            "heading":    "Bienvenido a nuestro sitio",
            "subheading": "Descubre lo que ofrecemos",
            "cta_text":   "Comenzar",
        },
    })
    if err != nil {
        log.Fatalf("add ES translation: %v", err)
    }
    fmt.Println("Translations added for EN and ES")

    // 4. Create a draft version
    draft, err := blocksSvc.CreateDraft(ctx, blocks.CreateInstanceDraftRequest{
        InstanceID: heroInstance.ID,
        Snapshot: blocks.BlockVersionSnapshot{
            Configuration: map[string]any{
                "heading": "Welcome Back!",
                "cta_url": "/dashboard",
            },
            Translations: []blocks.BlockVersionTranslationSnapshot{
                {
                    Locale:  "en",
                    Content: map[string]any{"heading": "Welcome Back!", "cta_text": "Explore"},
                },
                {
                    Locale:  "es",
                    Content: map[string]any{"heading": "Bienvenido de nuevo!", "cta_text": "Explorar"},
                },
            },
        },
        CreatedBy: authorID,
        UpdatedBy: authorID,
    })
    if err != nil {
        log.Fatalf("create draft: %v", err)
    }
    fmt.Printf("Draft: version=%d status=%s\n", draft.Version, draft.Status)

    // 5. Publish the draft
    published, err := blocksSvc.PublishDraft(ctx, blocks.PublishInstanceDraftRequest{
        InstanceID:  heroInstance.ID,
        Version:     draft.Version,
        PublishedBy: authorID,
        PublishedAt: timePtr(time.Now()),
    })
    if err != nil {
        log.Fatalf("publish draft: %v", err)
    }
    fmt.Printf("Published: version=%d\n", published.Version)

    // 6. List all versions
    versions, err := blocksSvc.ListVersions(ctx, heroInstance.ID)
    if err != nil {
        log.Fatalf("list versions: %v", err)
    }
    for _, v := range versions {
        fmt.Printf("  Version %d: status=%s\n", v.Version, v.Status)
    }

    // 7. List page blocks
    pageBlocks, err := blocksSvc.ListPageInstances(ctx, pageID)
    if err != nil {
        log.Fatalf("list page blocks: %v", err)
    }
    fmt.Printf("Page has %d block(s)\n", len(pageBlocks))
}

func timePtr(t time.Time) *time.Time { return &t }
func stringPtr(s string) *string     { return &s }
func intPtr(i int) *int              { return &i }
```

---

## Next Steps

- [GUIDE_WIDGETS.md](GUIDE_WIDGETS.md) -- dynamic behavioral components with area-based placement and visibility rules
- [GUIDE_PAGES.md](GUIDE_PAGES.md) -- page hierarchy, routing paths, and page-block integration
- [GUIDE_I18N.md](GUIDE_I18N.md) -- internationalization, locale management, and translation workflows
- [GUIDE_CONFIGURATION.md](GUIDE_CONFIGURATION.md) -- full config reference and DI container wiring
- [GUIDE_THEMES.md](GUIDE_THEMES.md) -- theme management, template registration, and region definitions
