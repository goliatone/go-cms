# Content Guide

This guide covers content types, content entries, translations, versioning, and scheduling in `go-cms`. By the end you will understand how to define schemas, create and manage content, work with multiple locales, and use the draft/publish workflow.

## Content Architecture Overview

Content in `go-cms` is split into two distinct entities:

- **Content types** define the structure and validation rules for a category of content. Each content type carries a JSON schema, a slug, and a status lifecycle. Think of a content type as a blueprint -- "Article", "Product", "FAQ".
- **Content entries** are individual records that belong to a content type. Each entry has a slug, a status, optional entry-level `Metadata`, and one or more translations. An entry references its content type by ID and is validated against the type's schema on every write.

```
ContentType (schema, slug, status)
  └── Content (slug, status, translations)
        ├── ContentTranslation (locale, title, content)
        ├── ContentTranslation (locale, title, content)
        └── ContentVersion (snapshot, status)
```

All entities use UUID primary keys and UTC timestamps.

### Accessing the Service

Content operations are exposed through the `cms.Module` facade:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es"}

module, err := cms.New(cfg)
if err != nil {
    log.Fatal(err)
}

contentSvc := module.Content()
contentTypeSvc := module.Container().ContentTypeService()
```

The `contentSvc` variable satisfies the `content.Service` interface for content entries. Content types are managed by `content.ContentTypeService`, available via `module.Container().ContentTypeService()`. Both services delegate to in-memory repositories by default, or SQL-backed repositories when `di.WithBunDB(db)` is provided.

---

## Content Type Lifecycle

### Creating a Content Type

A content type requires a name and a JSON schema at minimum. The slug is auto-generated from the name if omitted.

```go
articleType, err := contentTypeSvc.Create(ctx, content.CreateContentTypeRequest{
    Name: "Article",
    Slug: "article",
    Description: stringPtr("Long-form articles"),
    Schema: map[string]any{
        "fields": []map[string]any{
            {"name": "title", "type": "string", "required": true},
            {"name": "body", "type": "text", "required": true},
            {"name": "hero_image", "type": "string"},
        },
    },
    Status:    "draft",
    CreatedBy: authorID,
    UpdatedBy: authorID,
})
```

**`CreateContentTypeRequest` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Name` | `string` | Yes | Display name for the content type |
| `Slug` | `string` | No | URL-safe identifier; auto-generated from name if empty |
| `Description` | `*string` | No | Human-readable description |
| `Schema` | `map[string]any` | Yes | JSON schema defining the content structure |
| `UISchema` | `map[string]any` | No | UI rendering hints for admin interfaces |
| `Capabilities` | `map[string]any` | No | Feature toggles per content type |
| `Icon` | `*string` | No | Icon identifier for admin UIs |
| `Status` | `string` | No | Initial status; defaults to `"draft"` |
| `EnvironmentKey` | `string` | No | Environment scope; defaults per config |
| `CreatedBy` | `uuid.UUID` | Yes | Actor identifier |
| `UpdatedBy` | `uuid.UUID` | Yes | Actor identifier |

### Schema Validation

Schemas are validated on create and update. The service uses `schema.EnsureSchemaVersion` to assign a semantic version to each schema. Version history is tracked in the `SchemaHistory` field as immutable snapshots.

When updating a content type with `Status: "active"`, the service checks schema compatibility. Breaking changes (removed required fields, type changes) require explicit opt-in:

```go
updated, err := contentTypeSvc.Update(ctx, content.UpdateContentTypeRequest{
    ID:     articleType.ID,
    Schema: newSchema,
    Status: stringPtr("active"),
    AllowBreakingChanges: true, // Required for breaking changes on active types
    UpdatedBy: authorID,
})
```

### Slug Rules and Uniqueness

Slugs are normalized via `go-slug`:

- Input is trimmed and lowercased
- Special characters are replaced with hyphens
- Validated with `slug.IsValid()` after normalization
- Uniqueness is enforced per environment

If a slug conflicts with an existing content type in the same environment, the service returns `ErrContentTypeSlugExists`.

### Content Type Status

Content types follow a three-state lifecycle:

| Status | Description |
|--------|-------------|
| `draft` | Under development; default for new types |
| `active` | Available for content creation; schema changes require compatibility checks |
| `deprecated` | Retired; existing content preserved but new entries discouraged |

Valid transitions:

- `draft` -> `active` or `deprecated`
- `active` -> `deprecated`
- `deprecated` -> `active` (reactivation)

### Pages/Posts UI Sync (Admin Panels)

Admin UI panels for Pages and Posts are driven by **content entries**, not bespoke page/post services. Dynamic panels are created only for content types that are **active**, so the seeded types must be activated to make Pages/Posts available in the admin. If the seeded types remain in `draft`, the Pages/Posts panels do not render and the admin will fall back to legacy routes.

**Required activation (examples):**

- `page` content type: `status = "active"` with `capabilities` including `tree: true`, `seo: true`, `blocks: true`, `workflow: "pages"`, `permissions: "admin.pages"`.
- `page` content type should also set `structural_fields: true` and `policy_entity: "pages"` to explicitly opt into entry-level structural metadata.
- `blog_post` content type: `status = "active"` with `capabilities` including `seo: true`, `blocks: true`, `workflow: "posts"`, `permissions: "admin.posts"`, `panel_slug: "posts"`, `policy_entity: "posts"`.

Pages and Posts are always stored as content entries. The admin UI reads and writes them through the content entry APIs once the content types are active.

**Admin panel routing:**

- Content entry panels are mounted at `/admin/content/:panel_slug`.
- If `panel_slug` is not set, the content type slug is used.
- Alias routes keep legacy URLs working:
  - `/admin/pages` -> `/admin/content/pages`
  - `/admin/posts` -> `/admin/content/posts`

This keeps Pages/Posts UI aligned with content entries as the single source of truth while preserving existing URLs. The `panel_slug` capability is the mapping mechanism that lets `blog_post` render as "Posts" without renaming the content type, so `/admin/content/posts` can target either a `post` or `blog_post` content type depending on the app seed data.

### Querying Content Types

```go
// Get by ID
ct, err := contentTypeSvc.Get(ctx, typeID)

// Get by slug
ct, err := contentTypeSvc.GetBySlug(ctx, "article")

// List all content types
types, err := contentTypeSvc.List(ctx)

// Search by name or slug
results, err := contentTypeSvc.Search(ctx, "article")
```

All query methods accept optional environment keys as variadic parameters for environment-scoped lookups.

### Promotion Across Environments

When environments are enabled, content types and entries can be promoted from one environment to another via the admin API or the promotion service. Key behaviours:

- **Content types** preserve the source `schema_version` exactly; schema history is appended with promotion metadata.
- **Block definitions** referenced by a content type are promoted first so the target schema can resolve block references.
- **Content entries** can promote the latest published snapshot (default) or drafts, with `strict` vs `upsert` modes and optional schema migrations.
- **Dry-run** support is available for promotion flows to preview changes without persistence.
- **Bulk content slug promotions** must include a content type filter (`content_entry_type_id` or `content_entry_type_slug`) to avoid slug ambiguity across content types.

---

## Content CRUD

### Creating Content

Content entries are created with `Create`, providing a content type ID, a slug, and translations:

```go
article, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: articleType.ID,
    Slug:          "hello-world",
    Status:        "published",
    CreatedBy:     authorID,
    UpdatedBy:     authorID,
    Translations: []content.ContentTranslationInput{
        {
            Locale:  "en",
            Title:   "Hello World",
            Summary: stringPtr("An introductory article"),
            Content: map[string]any{
                "body": "Welcome to go-cms.",
            },
        },
    },
})
```

**`CreateContentRequest` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ContentTypeID` | `uuid.UUID` | Yes | References the content type |
| `Slug` | `string` | Yes | URL-safe identifier; unique per content type per environment |
| `Status` | `string` | No | Defaults to `"draft"` |
| `EnvironmentKey` | `string` | No | Environment scope |
| `CreatedBy` | `uuid.UUID` | Yes | Actor identifier |
| `UpdatedBy` | `uuid.UUID` | Yes | Actor identifier |
| `Metadata` | `map[string]any` | No | Entry-level metadata (non-localized structural fields) |
| `Translations` | `[]ContentTranslationInput` | Conditional | Required when `RequireTranslations` is `true` |
| `AllowMissingTranslations` | `bool` | No | Override to skip translation requirement |

**Validation on create:**

1. Content type must exist
2. Slug is normalized and checked for uniqueness within the content type and environment
3. Translation payloads are validated against the content type's JSON schema
4. Duplicate locale codes in a single request are rejected
5. When `DefaultLocaleRequired` is `true`, at least one translation must use the default locale

### Getting Content

```go
article, err := contentSvc.Get(ctx, articleID)
```

The returned `Content` struct includes:

- `Translations` -- all locale variants attached to this entry
- `Type` -- the content type record (populated on demand)
- `EffectiveStatus` -- computed from `Status`, `PublishAt`, `UnpublishAt`, and `PublishedAt`
- `IsVisible` -- `true` when `EffectiveStatus` is `"published"`

### Listing Content

```go
// List all content entries (default environment)
entries, err := contentSvc.List(ctx)

// List content in a specific environment
entries, err := contentSvc.List(ctx, "staging")
```

### Updating Content

```go
updated, err := contentSvc.Update(ctx, content.UpdateContentRequest{
    ID:        article.ID,
    Status:    "draft",
    UpdatedBy: authorID,
    Translations: []content.ContentTranslationInput{
        {
            Locale:  "en",
            Title:   "Hello World (Revised)",
            Content: map[string]any{"body": "Updated content."},
        },
    },
})
```

**`UpdateContentRequest` fields:**

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `uuid.UUID` | Content entry ID (required) |
| `Status` | `string` | New status value |
| `EnvironmentKey` | `string` | Environment scope |
| `UpdatedBy` | `uuid.UUID` | Actor identifier |
| `Translations` | `[]ContentTranslationInput` | Replaces the entire translation set |
| `Metadata` | `map[string]any` | Entry-level metadata (replaces stored metadata when provided) |
| `AllowMissingTranslations` | `bool` | Override translation requirement |

When translations are provided on update, they replace the entire translation set. To mutate a single locale without touching others, use `UpdateTranslation` instead (see the translations section below).

The slug and content type are immutable after creation.

### Entry Metadata (Structural Fields)

Content entries include a non-localized `Metadata` map for structural fields used by pages and tree-aware content types. Common keys:

- `parent_id` -- parent content entry ID (UUID)
- `template_id` -- template ID (UUID)
- `path` -- canonical URL path
- `sort_order` -- sibling ordering integer (`order` is an alias)

Metadata is normalized on write:

- UUIDs are stored as strings
- `path` is trimmed and cannot be empty
- `order` is normalized to `sort_order`
- `sort_order` must be an integer

When `Metadata` is provided on update, it replaces the stored map. Clients should send the full metadata map each time; send `null` for a key to remove it. Translation-level path overrides are not supported (legacy path fields are only used as a fallback when `metadata.path` is empty).

### Deleting Content

```go
err := contentSvc.Delete(ctx, content.DeleteContentRequest{
    ID:         article.ID,
    DeletedBy:  authorID,
    HardDelete: true,
})
```

`HardDelete` must be `true`. Soft-delete is not currently supported and returns `ErrContentSoftDeleteUnsupported`.

When a content entry is deleted, any pending scheduler jobs (publish/unpublish) for that entry are cancelled automatically.

---

## Content Translations

Translations attach localized variants to a content entry. Each translation carries a locale code, a title, an optional summary, and a content payload validated against the content type schema.

### Translation Input

```go
type ContentTranslationInput struct {
    Locale  string         // Required; must be in cfg.I18N.Locales
    Title   string         // Required
    Summary *string        // Optional
    Content map[string]any // Validated against content type schema
    Blocks  []map[string]any // Merged into Content["blocks"]
}
```

### Providing Translations on Create/Update

Pass translations inline with `Create` or `Update`:

```go
article, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: articleType.ID,
    Slug:          "about-us",
    Status:        "published",
    CreatedBy:     authorID,
    UpdatedBy:     authorID,
    Translations: []content.ContentTranslationInput{
        {Locale: "en", Title: "About Us", Content: map[string]any{"body": "..."}},
        {Locale: "es", Title: "Sobre nosotros", Content: map[string]any{"body": "..."}},
    },
})
```

### Updating a Single Translation

To mutate one locale without replacing the entire set:

```go
translation, err := contentSvc.UpdateTranslation(ctx, content.UpdateContentTranslationRequest{
    ContentID: article.ID,
    Locale:    "es",
    Title:     "Sobre nosotros (actualizado)",
    Content:   map[string]any{"body": "Contenido actualizado."},
    UpdatedBy: authorID,
})
```

This finds the existing translation for the `"es"` locale and replaces its fields. If the locale is not found on the content entry, it returns `ErrContentTranslationNotFound`.

### Deleting a Translation

```go
err := contentSvc.DeleteTranslation(ctx, content.DeleteContentTranslationRequest{
    ContentID: article.ID,
    Locale:    "es",
    DeletedBy: authorID,
})
```

If `RequireTranslations` is `true` and this is the last translation, the operation is rejected with `ErrNoTranslations`.

### Translation Configuration

Translation behavior is controlled via `cfg.I18N`:

```go
cfg.I18N.Enabled = true              // Enable translation handling
cfg.I18N.Locales = []string{"en", "es", "fr"}
cfg.I18N.RequireTranslations = true  // At least one translation required (default)
cfg.I18N.DefaultLocaleRequired = true // Default locale translation required (default)
cfg.DefaultLocale = "en"
```

**Per-request override:**

Set `AllowMissingTranslations: true` on `CreateContentRequest` or `UpdateContentRequest` to bypass translation requirements for a single operation. This is useful for draft workflows where content is created incrementally:

```go
draft, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID:            articleType.ID,
    Slug:                     "work-in-progress",
    Status:                   "draft",
    CreatedBy:                authorID,
    UpdatedBy:                authorID,
    AllowMissingTranslations: true, // No translations needed yet
})
```

### Translation Errors

| Error | Cause |
|-------|-------|
| `ErrNoTranslations` | No translations provided when `RequireTranslations` is `true` |
| `ErrDefaultLocaleRequired` | Missing default locale when `DefaultLocaleRequired` is `true` |
| `ErrUnknownLocale` | Locale code not in `cfg.I18N.Locales` |
| `ErrDuplicateLocale` | Same locale code appears twice in one request |
| `ErrContentTranslationNotFound` | Target locale not found on the content entry |
| `ErrContentTranslationsDisabled` | Translations feature is disabled |

---

## Content Versioning

Versioning captures immutable snapshots of content payloads, enabling draft/publish workflows and rollback.

### Enabling Versioning

```go
cfg.Features.Versioning = true
```

All versioning methods return `ErrVersioningDisabled` when this flag is `false`.

### Creating a Draft

```go
version, err := contentSvc.CreateDraft(ctx, content.CreateContentDraftRequest{
    ContentID: article.ID,
    Snapshot: content.ContentVersionSnapshot{
        Fields: map[string]any{"category": "tech"},
        Translations: []content.ContentVersionTranslationSnapshot{
            {
                Locale:  "en",
                Title:   "Hello World v2",
                Content: map[string]any{"body": "Revised content."},
            },
        },
        Metadata: map[string]any{"editor": "admin"},
    },
    CreatedBy:   authorID,
    UpdatedBy:   authorID,
    BaseVersion: intPtr(1), // Optimistic concurrency check (optional)
})
```

**What happens:**

1. The snapshot is validated against the content type schema
2. If `BaseVersion` is set, it must match `CurrentVersion - 1` (otherwise `ErrContentVersionConflict`)
3. If `VersionRetentionLimit` is set and reached, `ErrContentVersionRetentionExceeded` is returned
4. The version number is auto-incremented
5. `Content.CurrentVersion` is updated to the new version number
6. The version is stored with status `"draft"`

### Publishing a Draft

```go
published, err := contentSvc.PublishDraft(ctx, content.PublishContentDraftRequest{
    ContentID:   article.ID,
    Version:     2,
    PublishedBy: authorID,
    PublishedAt: nil, // Defaults to now
})
```

**What happens:**

1. The draft snapshot is migrated to the current content type schema version (strict validation)
2. The version status changes from `"draft"` to `"published"`
3. Any previously published version is archived (status set to `"archived"`)
4. `Content.PublishedVersion`, `Content.PublishedAt`, and `Content.Status` are updated
5. Returns `ErrContentVersionAlreadyPublished` if the version is already published

### Previewing a Draft

Preview renders a draft snapshot without persisting any changes:

```go
preview, err := contentSvc.PreviewDraft(ctx, content.PreviewContentDraftRequest{
    ContentID: article.ID,
    Version:   2,
})

// preview.Content -- Content record with preview status applied
// preview.Version -- The version being previewed
```

The snapshot is migrated and validated, but no database writes occur.

### Listing Versions

```go
versions, err := contentSvc.ListVersions(ctx, article.ID)
for _, v := range versions {
    fmt.Printf("Version %d: status=%s created=%s\n", v.Version, v.Status, v.CreatedAt)
}
```

### Restoring a Version

Restore creates a new draft from a previous version's snapshot:

```go
restored, err := contentSvc.RestoreVersion(ctx, content.RestoreContentVersionRequest{
    ContentID:  article.ID,
    Version:    1,
    RestoredBy: authorID,
})
// restored is a new ContentVersion with the next version number
```

This does not overwrite the existing version -- it creates a new draft containing the old snapshot.

### Version Status Lifecycle

| Status | Description |
|--------|-------------|
| `draft` | Created by `CreateDraft` or `RestoreVersion` |
| `published` | Set by `PublishDraft`; only one version can be published at a time |
| `archived` | Previous published version; set automatically when a new version is published |

### Version Retention

Configure the maximum number of versions per content entry:

```go
// Via service option (set by DI container)
content.WithVersionRetentionLimit(20)
```

When the limit is reached, `CreateDraft` returns `ErrContentVersionRetentionExceeded`.

---

## Scheduling

Scheduling automates publish and unpublish events at specified times.

### Enabling Scheduling

```go
cfg.Features.Versioning = true  // Required dependency
cfg.Features.Scheduling = true
```

### Scheduling Publish/Unpublish

```go
scheduled, err := contentSvc.Schedule(ctx, content.ScheduleContentRequest{
    ContentID:   article.ID,
    PublishAt:    timePtr(time.Now().Add(24 * time.Hour)),
    UnpublishAt:  timePtr(time.Now().Add(30 * 24 * time.Hour)),
    ScheduledBy: authorID,
})
```

**Validation:**

- `PublishAt` must be before `UnpublishAt` (if both are set)
- Neither timestamp can be zero-valued
- Returns `ErrSchedulingDisabled` when the feature is off
- Returns `ErrScheduleWindowInvalid` when `UnpublishAt` is before `PublishAt`

**What happens:**

1. `Content.PublishAt` and `Content.UnpublishAt` are set
2. Status is automatically adjusted:
   - `"scheduled"` if `PublishAt` is in the future
   - `"published"` if `PublishAt` has passed or no schedule is set with a published version
   - `"draft"` otherwise
3. Scheduler jobs are enqueued for the publish and unpublish times
4. Previous jobs for the same content entry are cancelled and replaced

### Effective Status

The effective status is computed at retrieval time based on the current clock:

```
if UnpublishAt <= now     -> "archived"
if PublishAt > now        -> "scheduled"
if PublishAt <= now       -> "published"
if PublishedAt <= now     -> "published"
otherwise                 -> record.Status (typically "draft")
```

Access computed status via:

```go
article, _ := contentSvc.Get(ctx, articleID)
fmt.Println(article.EffectiveStatus) // "published", "scheduled", "archived", or "draft"
fmt.Println(article.IsVisible)       // true only when EffectiveStatus is "published"
```

---

## Status Transitions and Content Lifecycle

### Content Entry Status

| Status | Description |
|--------|-------------|
| `draft` | Default for new entries; under preparation |
| `published` | Visible to consumers |
| `scheduled` | Has a future `PublishAt` time |
| `archived` | Retained for history but not publicly visible |

Content entry status is a mutable string field. It can be set directly on `Create` and `Update`, or modified automatically by `PublishDraft` and `Schedule`.

The `EffectiveStatus` and `IsVisible` fields are computed at retrieval time and account for scheduling timestamps. Always prefer `EffectiveStatus` over `Status` when determining visibility.

### Content Type Status

| Status | Description |
|--------|-------------|
| `draft` | Under development; default for new types |
| `active` | Available for content creation; breaking schema changes require opt-in |
| `deprecated` | Retired; delete sets this status on soft-delete |

---

## Error Reference

| Error | Cause |
|-------|-------|
| `ErrContentTypeRequired` | Content type ID missing or content type not found |
| `ErrSlugRequired` | Empty slug on content entry |
| `ErrSlugInvalid` | Slug contains invalid characters |
| `ErrSlugExists` | Slug already taken in the same content type and environment |
| `ErrContentIDRequired` | Missing content entry ID |
| `ErrContentSchemaInvalid` | Content payload failed schema validation |
| `ErrContentSoftDeleteUnsupported` | `HardDelete: false` is not supported |
| `ErrVersioningDisabled` | Versioning method called without `Features.Versioning` |
| `ErrContentVersionRequired` | Version number missing or invalid |
| `ErrContentVersionConflict` | `BaseVersion` does not match expected value |
| `ErrContentVersionAlreadyPublished` | Attempting to publish an already-published version |
| `ErrContentVersionRetentionExceeded` | Maximum version count reached |
| `ErrSchedulingDisabled` | Scheduling method called without `Features.Scheduling` |
| `ErrScheduleWindowInvalid` | `PublishAt` is after `UnpublishAt` |
| `ErrScheduleTimestampInvalid` | Zero-valued timestamp |

---

## Complete Example

This example demonstrates the full lifecycle: creating a content type, creating content with translations, versioning with draft/publish, and scheduling.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/content"
    "github.com/google/uuid"
)

func main() {
    ctx := context.Background()

    // Configure with versioning and scheduling
    cfg := cms.DefaultConfig()
    cfg.DefaultLocale = "en"
    cfg.I18N.Locales = []string{"en", "es"}
    cfg.Features.Versioning = true
    cfg.Features.Scheduling = true

    module, err := cms.New(cfg)
    if err != nil {
        log.Fatal(err)
    }

    contentSvc := module.Content()
    contentTypeSvc := module.Container().ContentTypeService()
    authorID := uuid.New()

    // 1. Create a content type
    articleType, err := contentTypeSvc.Create(ctx, content.CreateContentTypeRequest{
        Name: "Article",
        Slug: "article",
        Schema: map[string]any{
            "fields": []map[string]any{
                {"name": "title", "type": "string", "required": true},
                {"name": "body", "type": "text", "required": true},
            },
        },
        CreatedBy: authorID,
        UpdatedBy: authorID,
    })
    if err != nil {
        log.Fatalf("create content type: %v", err)
    }
    fmt.Printf("Content type: %s (id=%s)\n", articleType.Name, articleType.ID)

    // 2. Create content with multi-locale translations
    article, err := contentSvc.Create(ctx, content.CreateContentRequest{
        ContentTypeID: articleType.ID,
        Slug:          "getting-started",
        Status:        "draft",
        CreatedBy:     authorID,
        UpdatedBy:     authorID,
        Translations: []content.ContentTranslationInput{
            {
                Locale:  "en",
                Title:   "Getting Started",
                Content: map[string]any{"body": "Welcome to our platform."},
            },
            {
                Locale:  "es",
                Title:   "Primeros pasos",
                Content: map[string]any{"body": "Bienvenido a nuestra plataforma."},
            },
        },
    })
    if err != nil {
        log.Fatalf("create content: %v", err)
    }
    fmt.Printf("Content: %s (status=%s)\n", article.Slug, article.Status)

    // 3. Create a draft version
    draft, err := contentSvc.CreateDraft(ctx, content.CreateContentDraftRequest{
        ContentID: article.ID,
        Snapshot: content.ContentVersionSnapshot{
            Translations: []content.ContentVersionTranslationSnapshot{
                {
                    Locale:  "en",
                    Title:   "Getting Started (v2)",
                    Content: map[string]any{"body": "Revised welcome content."},
                },
                {
                    Locale:  "es",
                    Title:   "Primeros pasos (v2)",
                    Content: map[string]any{"body": "Contenido de bienvenida revisado."},
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

    // 4. Publish the draft
    published, err := contentSvc.PublishDraft(ctx, content.PublishContentDraftRequest{
        ContentID:   article.ID,
        Version:     draft.Version,
        PublishedBy: authorID,
    })
    if err != nil {
        log.Fatalf("publish draft: %v", err)
    }
    fmt.Printf("Published: version=%d\n", published.Version)

    // 5. Schedule an unpublish window
    scheduled, err := contentSvc.Schedule(ctx, content.ScheduleContentRequest{
        ContentID:   article.ID,
        UnpublishAt: timePtr(time.Now().Add(90 * 24 * time.Hour)),
        ScheduledBy: authorID,
    })
    if err != nil {
        log.Fatalf("schedule: %v", err)
    }
    fmt.Printf("Scheduled: effective_status=%s unpublish_at=%s\n",
        scheduled.EffectiveStatus, scheduled.UnpublishAt)

    // 6. Update a single translation
    _, err = contentSvc.UpdateTranslation(ctx, content.UpdateContentTranslationRequest{
        ContentID: article.ID,
        Locale:    "es",
        Title:     "Primeros pasos (actualizado)",
        Content:   map[string]any{"body": "Contenido actualizado."},
        UpdatedBy: authorID,
    })
    if err != nil {
        log.Fatalf("update translation: %v", err)
    }
    fmt.Println("Spanish translation updated")

    // 7. List all versions
    versions, err := contentSvc.ListVersions(ctx, article.ID)
    if err != nil {
        log.Fatalf("list versions: %v", err)
    }
    for _, v := range versions {
        fmt.Printf("  Version %d: status=%s\n", v.Version, v.Status)
    }
}

func timePtr(t time.Time) *time.Time { return &t }
func stringPtr(s string) *string     { return &s }
func intPtr(i int) *int              { return &i }
```

---

## Next Steps

- **GUIDE_PAGES.md** -- page hierarchy, routing paths, and page-block relationships
- **GUIDE_BLOCKS.md** -- reusable content fragments with definitions and instances
- **GUIDE_I18N.md** -- internationalization, locale management, and translation workflows
- **GUIDE_CONFIGURATION.md** -- full config reference and DI container wiring
- **GUIDE_WORKFLOW.md** -- content lifecycle orchestration with state machines
