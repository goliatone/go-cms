# Pages Guide

This guide covers page hierarchy, routing paths, translations, page-block relationships, versioning, and scheduling in `go-cms`.

## Status: Legacy Pages Service

Pages and posts are now modeled as **content entries** (content type `page`) with structural fields stored in entry-level `Metadata` (`path`, `template_id`, `parent_id`, `sort_order`). The generator and admin UI derive page views from these content entries. The pages service remains for legacy integrations and compatibility, but new code should prefer content entries (see `GUIDE_CONTENT.md`).

## Page Architecture Overview (Legacy)

Legacy pages wrap content entries with hierarchical structure and routing metadata. While content entries hold the payload (title, body, fields), pages add:

- **Hierarchy** -- parent-child relationships for site structure
- **Routing** -- localized URL paths per translation
- **Templates** -- layout definitions with named regions
- **Blocks** -- reusable content fragments placed in template regions
- **Widgets** -- dynamic behavioral components placed in widget areas

```
Page (slug, status, hierarchy, template)
  ├── ContentID -> Content (payload, schema)
  ├── TemplateID -> Template (layout, regions)
  ├── ParentID -> Page (hierarchy)
  ├── PageTranslation (locale, title, path, summary)
  ├── PageTranslation (locale, title, path, summary)
  ├── PageVersion (snapshot of blocks, widgets, metadata)
  ├── Block Instances (placed in template regions)
  └── Widget Instances (placed in widget areas)
```

All entities use UUID primary keys and UTC timestamps.

### Accessing the Service (Legacy)

Page operations are exposed through the `cms.Module` facade:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es"}

module, err := cms.New(cfg)
if err != nil {
    log.Fatal(err)
}

pageSvc := module.Pages()
```

The `pageSvc` variable satisfies the `pages.Service` interface. This is a legacy surface; new implementations should use content entries with entry metadata. The service delegates to in-memory repositories by default, or SQL-backed repositories when `di.WithBunDB(db)` is provided.

---

## Admin Page Read Model

The admin read model builds a page-shaped view from content entries of type `page`. `Path`, `ParentID`, and `TemplateID` are sourced from entry `Metadata` when present (legacy translation fields are only used as a fallback).

Admin list/detail surfaces should use the admin read model for consistent hydration and locale handling. Access it via the module facade:

```go
adminRead := module.AdminPageRead()

records, total, err := adminRead.List(ctx, cms.AdminPageListOptions{
    Locale:      "en",
    IncludeData: true,
})

record, err := adminRead.Get(ctx, pageID.String(), cms.AdminPageGetOptions{
    Locale:         "en",
    IncludeContent: true,
    IncludeBlocks:  true,
    IncludeData:    true,
})
```

Key contract rules:

- `RequestedLocale` always echoes the caller's `Locale`, even if a translation is missing.
- `ResolvedLocale` is set to the chosen translation locale (requested, fallback, or empty if missing).
- `Translation` and `ContentTranslation` are `TranslationBundle` values; `Meta` includes requested/resolved locales, `MissingRequestedLocale`, `FallbackUsed`, and available locales scoped to the bundle (page vs content).
- `AllowMissingTranslations` controls whether missing requested locales return `ErrTranslationMissing` or a record with empty localized fields plus bundle metadata.
- `IncludeContent`/`IncludeBlocks`/`IncludeData` control heavy fields; omit flags to keep list payloads lean.
- `Blocks` prefers embedded block payloads and falls back to legacy block IDs when embedded data is missing.

## Page CRUD (Legacy)

New implementations should create content entries of type `page` and store routing/hierarchy metadata at the entry level. The CRUD APIs below remain for legacy data models only.

### Creating a Page

Pages are created with `Create`, providing a content ID, a template ID, a slug, and translations:

```go
page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
    ContentID:  articleContent.ID,
    TemplateID: defaultTemplate.ID,
    Slug:       "about-us",
    Status:     "draft",
    CreatedBy:  authorID,
    UpdatedBy:  authorID,
    Translations: []pages.PageTranslationInput{
        {
            Locale: "en",
            Title:  "About Us",
            Path:   "/about-us",
        },
        {
            Locale: "es",
            Title:  "Sobre nosotros",
            Path:   "/es/sobre-nosotros",
        },
    },
})
```

**`CreatePageRequest` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ContentID` | `uuid.UUID` | Yes | References the content entry |
| `TemplateID` | `uuid.UUID` | Yes | References a theme template defining regions |
| `ParentID` | `*uuid.UUID` | No | Parent page for hierarchy; `nil` for root pages |
| `Slug` | `string` | Yes | URL-safe identifier; unique per environment |
| `Status` | `string` | No | Defaults to `"draft"` |
| `EnvironmentKey` | `string` | No | Environment scope; defaults per config |
| `CreatedBy` | `uuid.UUID` | Yes | Actor identifier |
| `UpdatedBy` | `uuid.UUID` | Yes | Actor identifier |
| `Translations` | `[]PageTranslationInput` | Conditional | Required when `RequireTranslations` is `true` |
| `AllowMissingTranslations` | `bool` | No | Override to skip translation requirement |

**Validation on create:**

1. Content entry must exist
2. Slug is normalized and checked for uniqueness within the environment
3. Template must exist when themes are enabled
4. Parent page must exist if `ParentID` is provided
5. Duplicate locale codes in a single request are rejected
6. When `DefaultLocaleRequired` is `true`, at least one translation must use the default locale

### Getting a Page

```go
page, err := pageSvc.Get(ctx, pageID)
```

The returned `Page` struct includes:

- `Translations` -- all locale variants attached to this page
- `Content` -- the content entry record (populated on demand)
- `Blocks` -- block instances assigned to this page's regions
- `Widgets` -- widget instances organized by area
- `EffectiveStatus` -- computed from `Status`, `PublishAt`, `UnpublishAt`, and `PublishedAt`
- `IsVisible` -- `true` when `EffectiveStatus` is `"published"`

Pages go through automatic enrichment when retrieved. The service resolves blocks, widgets, and media bindings, computing the effective status and visibility flag.

### Listing Pages

```go
// List all pages (default environment)
allPages, err := pageSvc.List(ctx)

// List pages in a specific environment
stagingPages, err := pageSvc.List(ctx, "staging")
```

### Updating a Page

```go
updated, err := pageSvc.Update(ctx, pages.UpdatePageRequest{
    ID:         page.ID,
    TemplateID: &newTemplateID,
    Status:     "published",
    UpdatedBy:  authorID,
    Translations: []pages.PageTranslationInput{
        {
            Locale: "en",
            Title:  "About Us (Revised)",
            Path:   "/about-us",
        },
    },
})
```

**`UpdatePageRequest` fields:**

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `uuid.UUID` | Page ID (required) |
| `TemplateID` | `*uuid.UUID` | New template; `nil` keeps current |
| `Status` | `string` | New status value |
| `EnvironmentKey` | `string` | Environment scope |
| `UpdatedBy` | `uuid.UUID` | Actor identifier |
| `Translations` | `[]PageTranslationInput` | Replaces the entire translation set |
| `AllowMissingTranslations` | `bool` | Override translation requirement |

When translations are provided on update, they replace the entire translation set. To mutate a single locale without touching others, use `UpdateTranslation` instead (see the translations section below).

The slug, content ID, and parent ID are immutable through `Update`. Use `Move` to change the parent.

### Deleting a Page

```go
err := pageSvc.Delete(ctx, pages.DeletePageRequest{
    ID:         page.ID,
    DeletedBy:  authorID,
    HardDelete: true,
})
```

`HardDelete` must be `true`. Soft-delete is not currently supported and returns `ErrPageSoftDeleteUnsupported`.

When a page is deleted, any pending scheduler jobs (publish/unpublish) for that page are cancelled automatically.

---

## Page Hierarchy

Pages support parent-child relationships for building site structures like nested navigation trees.

### Creating Pages with Parents

Assign a parent during creation:

```go
childPage, err := pageSvc.Create(ctx, pages.CreatePageRequest{
    ContentID:  childContent.ID,
    TemplateID: defaultTemplate.ID,
    ParentID:   &parentPage.ID,
    Slug:       "team",
    CreatedBy:  authorID,
    UpdatedBy:  authorID,
    Translations: []pages.PageTranslationInput{
        {Locale: "en", Title: "Our Team", Path: "/about-us/team"},
    },
})
```

### Moving Pages

Use `Move` to reparent a page without modifying its content or translations:

```go
moved, err := pageSvc.Move(ctx, pages.MovePageRequest{
    PageID:      childPage.ID,
    NewParentID: &newParentPage.ID,
    ActorID:     authorID,
})
```

Set `NewParentID` to `nil` to promote a child page to a root page:

```go
promoted, err := pageSvc.Move(ctx, pages.MovePageRequest{
    PageID:      childPage.ID,
    NewParentID: nil, // Promote to root
    ActorID:     authorID,
})
```

**Cycle detection:** The service prevents circular parent assignments. If moving a page would create a cycle (e.g., a page becoming its own descendant), the operation returns `ErrPageParentCycle`.

**`MovePageRequest` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `PageID` | `uuid.UUID` | Yes | Page to move |
| `NewParentID` | `*uuid.UUID` | No | New parent; `nil` promotes to root |
| `ActorID` | `uuid.UUID` | Yes | Actor identifier |

### Duplicating Pages

`Duplicate` clones a page including its translations, creating a new independent copy:

```go
clone, err := pageSvc.Duplicate(ctx, pages.DuplicatePageRequest{
    PageID:    originalPage.ID,
    Slug:      "about-us-copy",
    ParentID:  nil, // Same parent as original if nil
    Status:    "draft",
    CreatedBy: authorID,
    UpdatedBy: authorID,
})
```

**`DuplicatePageRequest` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `PageID` | `uuid.UUID` | Yes | Source page to clone |
| `Slug` | `string` | No | New slug; auto-generated if empty or conflicting |
| `ParentID` | `*uuid.UUID` | No | New parent; defaults to source parent |
| `Status` | `string` | No | Defaults to `"draft"` |
| `CreatedBy` | `uuid.UUID` | Yes | Creator ID |
| `UpdatedBy` | `uuid.UUID` | Yes | Updater ID |

**What happens on duplicate:**

1. The source page is fetched with all its translations
2. A new slug is determined (auto-generated with suffix if conflicting)
3. Translations are cloned with derived unique paths
4. A new page record is created with the cloned data
5. The clone starts in the requested status (default: `"draft"`)

If the service cannot determine a unique slug after retrying, it returns `ErrPageDuplicateSlug`.

### Path Resolution

Each page translation carries a `Path` field that serves as the localized URL for the page. Paths are validated for uniqueness per locale within the environment, ensuring no two pages share the same localized route.

When building hierarchical navigation, paths typically reflect the hierarchy:

```go
// Root page
{Locale: "en", Title: "About", Path: "/about"}

// Child page
{Locale: "en", Title: "Team", Path: "/about/team"}

// Grandchild page
{Locale: "en", Title: "Leadership", Path: "/about/team/leadership"}
```

Path resolution is the responsibility of the caller -- the CMS stores and validates paths but does not automatically derive child paths from parent slugs.

---

## Page Translations

Translations attach localized routing metadata to a page. Each translation carries a locale code, a title, a URL path, and optional summary and media bindings.

### Translation Input

```go
type PageTranslationInput struct {
    Locale        string           // Required; must be in cfg.I18N.Locales
    Title         string           // Required; display title
    Path          string           // Required; localized URL path (unique per locale per env)
    Summary       *string          // Optional; short description
    MediaBindings media.BindingSet // Optional; media slot bindings
}
```

### Providing Translations on Create/Update

Pass translations inline with `Create` or `Update`:

```go
page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
    ContentID:  contentEntry.ID,
    TemplateID: templateID,
    Slug:       "contact",
    CreatedBy:  authorID,
    UpdatedBy:  authorID,
    Translations: []pages.PageTranslationInput{
        {Locale: "en", Title: "Contact Us", Path: "/contact"},
        {Locale: "es", Title: "Contacto", Path: "/es/contacto"},
        {Locale: "fr", Title: "Nous contacter", Path: "/fr/nous-contacter"},
    },
})
```

### Updating a Single Translation

To mutate one locale without replacing the entire set:

```go
translation, err := pageSvc.UpdateTranslation(ctx, pages.UpdatePageTranslationRequest{
    PageID:    page.ID,
    Locale:    "es",
    Title:     "Contacto (actualizado)",
    Path:      "/es/contacto",
    Summary:   stringPtr("Informacion de contacto actualizada"),
    UpdatedBy: authorID,
})
```

This finds the existing translation for the `"es"` locale and replaces its fields. If the locale is not found on the page, it returns `ErrPageTranslationNotFound`.

**`UpdatePageTranslationRequest` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `PageID` | `uuid.UUID` | Yes | Page ID |
| `Locale` | `string` | Yes | Locale code to update |
| `Title` | `string` | Yes | New display title |
| `Path` | `string` | Yes | New localized URL path |
| `Summary` | `*string` | No | New summary text |
| `MediaBindings` | `media.BindingSet` | No | New media slot bindings |
| `UpdatedBy` | `uuid.UUID` | Yes | Actor identifier |

### Deleting a Translation

```go
err := pageSvc.DeleteTranslation(ctx, pages.DeletePageTranslationRequest{
    PageID:    page.ID,
    Locale:    "fr",
    DeletedBy: authorID,
})
```

If `RequireTranslations` is `true` and this is the last translation, the operation is rejected with `ErrNoPageTranslations`.

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

Set `AllowMissingTranslations: true` on `CreatePageRequest` or `UpdatePageRequest` to bypass translation requirements for a single operation. This is useful for draft workflows where pages are created incrementally:

```go
draft, err := pageSvc.Create(ctx, pages.CreatePageRequest{
    ContentID:                contentEntry.ID,
    TemplateID:               templateID,
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
| `ErrNoPageTranslations` | No translations provided when `RequireTranslations` is `true` |
| `ErrDefaultLocaleRequired` | Missing default locale when `DefaultLocaleRequired` is `true` |
| `ErrUnknownLocale` | Locale code not in `cfg.I18N.Locales` |
| `ErrDuplicateLocale` | Same locale code appears twice in one request |
| `ErrPathExists` | Translation path already exists for another page in the same locale |
| `ErrPageTranslationNotFound` | Target locale not found on the page |
| `ErrPageTranslationsDisabled` | Translations feature is disabled |

---

## Page-Block Integration

Pages use template regions to organize block instances. Blocks are reusable content fragments placed at specific positions within named regions of a page's template.

### How Blocks Relate to Pages

The relationship flows through templates and regions:

```
Page
  └── Template (defines regions: "header", "main", "sidebar")
        ├── Region "header"
        │     └── Block Instance (hero banner, position=0)
        ├── Region "main"
        │     ├── Block Instance (text block, position=0)
        │     └── Block Instance (image gallery, position=1)
        └── Region "sidebar"
              └── Block Instance (call to action, position=0)
```

### Blocks in Version Snapshots

When creating a page version, block placements are captured in the snapshot:

```go
version, err := pageSvc.CreateDraft(ctx, pages.CreatePageDraftRequest{
    PageID: page.ID,
    Snapshot: pages.PageVersionSnapshot{
        Regions: map[string][]pages.PageBlockPlacement{
            "header": {
                {
                    Region:     "header",
                    Position:   0,
                    BlockID:    heroBannerDef.ID,
                    InstanceID: heroBannerInst.ID,
                },
            },
            "main": {
                {
                    Region:     "main",
                    Position:   0,
                    BlockID:    textBlockDef.ID,
                    InstanceID: textBlockInst.ID,
                },
                {
                    Region:     "main",
                    Position:   1,
                    BlockID:    galleryDef.ID,
                    InstanceID: galleryInst.ID,
                },
            },
        },
    },
    CreatedBy: authorID,
    UpdatedBy: authorID,
})
```

**`PageBlockPlacement` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Region` | `string` | Yes | Template region identifier |
| `Position` | `int` | Yes | Sort order within the region (0-indexed) |
| `BlockID` | `uuid.UUID` | Yes | Block definition ID |
| `InstanceID` | `uuid.UUID` | Yes | Block instance ID |
| `Version` | `*int` | No | Pinned block version (latest if nil) |
| `Snapshot` | `map[string]any` | No | Block-specific snapshot data |

### Automatic Block Enrichment

When retrieving a page, the service automatically resolves block instances and populates the `Blocks` field:

```go
page, _ := pageSvc.Get(ctx, pageID)
for _, block := range page.Blocks {
    fmt.Printf("Block: %s in region (instance=%s)\n",
        block.DefinitionID, block.ID)
}
```

If the block service is unavailable, the service falls back to an embedded block bridge for basic resolution.

### Widget Placement in Snapshots

Widgets can also be captured in version snapshots, organized by area:

```go
snapshot := pages.PageVersionSnapshot{
    Widgets: map[string][]pages.WidgetPlacementSnapshot{
        "sidebar": {
            {
                Area:       "sidebar",
                WidgetID:   promotionDef.ID,
                InstanceID: promotionInst.ID,
                Configuration: map[string]any{
                    "theme": "highlight",
                },
            },
        },
    },
}
```

**`WidgetPlacementSnapshot` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Area` | `string` | Yes | Widget area identifier |
| `WidgetID` | `uuid.UUID` | Yes | Widget definition ID |
| `InstanceID` | `uuid.UUID` | Yes | Widget instance ID |
| `Configuration` | `map[string]any` | No | Widget-specific configuration |

---

## Page Versioning

Versioning captures immutable snapshots of page layout (blocks, widgets, metadata), enabling draft/publish workflows and rollback.

### Enabling Versioning

```go
cfg.Features.Versioning = true
```

All versioning methods return `ErrVersioningDisabled` when this flag is `false`.

### Creating a Draft

```go
version, err := pageSvc.CreateDraft(ctx, pages.CreatePageDraftRequest{
    PageID: page.ID,
    Snapshot: pages.PageVersionSnapshot{
        Regions: map[string][]pages.PageBlockPlacement{
            "main": {
                {Region: "main", Position: 0, BlockID: blockDefID, InstanceID: blockInstID},
            },
        },
        Metadata: map[string]any{"layout": "two-column"},
    },
    CreatedBy:   authorID,
    UpdatedBy:   authorID,
    BaseVersion: intPtr(1), // Optimistic concurrency check (optional)
})
```

**What happens:**

1. If `BaseVersion` is set, it must match `CurrentVersion` (otherwise `ErrVersionConflict`)
2. If `VersionRetentionLimit` is set and reached, `ErrVersionRetentionExceeded` is returned
3. The version number is auto-incremented
4. `Page.CurrentVersion` is updated to the new version number
5. The version is stored with status `"draft"`

**`CreatePageDraftRequest` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `PageID` | `uuid.UUID` | Yes | Page to version |
| `Snapshot` | `PageVersionSnapshot` | Yes | Layout snapshot (blocks, widgets, metadata) |
| `CreatedBy` | `uuid.UUID` | Yes | Creator ID |
| `UpdatedBy` | `uuid.UUID` | Yes | Updater ID |
| `BaseVersion` | `*int` | No | Conflict detection; must match current version |

### Publishing a Draft

```go
published, err := pageSvc.PublishDraft(ctx, pages.PublishPagePublishRequest{
    PageID:      page.ID,
    Version:     2,
    PublishedBy: authorID,
    PublishedAt: nil, // Defaults to now
})
```

**What happens:**

1. The version status changes from `"draft"` to `"published"`
2. Any previously published version is archived (status set to `"archived"`)
3. `Page.PublishedVersion`, `Page.PublishedAt`, and `Page.Status` are updated
4. Returns `ErrVersionAlreadyPublished` if the version is already published

### Previewing a Draft

Preview renders a draft snapshot without persisting any changes:

```go
preview, err := pageSvc.PreviewDraft(ctx, pages.PreviewPageDraftRequest{
    PageID:  page.ID,
    Version: 2,
})

// preview.Page    -- Page record with preview state applied
// preview.Version -- The version being previewed
```

The `ContentSnapshot` field allows overlaying a content version for combined page+content preview:

```go
preview, err := pageSvc.PreviewDraft(ctx, pages.PreviewPageDraftRequest{
    PageID:  page.ID,
    Version: 2,
    ContentSnapshot: &content.ContentVersionSnapshot{
        Translations: []content.ContentVersionTranslationSnapshot{
            {Locale: "en", Title: "Preview Title", Content: map[string]any{"body": "Draft body"}},
        },
    },
})
```

### Listing Versions

```go
versions, err := pageSvc.ListVersions(ctx, page.ID)
for _, v := range versions {
    fmt.Printf("Version %d: status=%s created=%s\n", v.Version, v.Status, v.CreatedAt)
}
```

### Restoring a Version

Restore creates a new draft from a previous version's snapshot:

```go
restored, err := pageSvc.RestoreVersion(ctx, pages.RestorePageVersionRequest{
    PageID:     page.ID,
    Version:    1,
    RestoredBy: authorID,
})
// restored is a new PageVersion with the next version number
```

This does not overwrite the existing version -- it creates a new draft containing the old snapshot.

### Version Status Lifecycle

| Status | Description |
|--------|-------------|
| `draft` | Created by `CreateDraft` or `RestoreVersion` |
| `published` | Set by `PublishDraft`; only one version can be published at a time |
| `archived` | Previous published version; set automatically when a new version is published |

### Version Retention

Configure the maximum number of versions per page:

```go
// Via config
cfg.Retention.Pages = 20 // 0 = unlimited
```

When the limit is reached, `CreateDraft` returns `ErrVersionRetentionExceeded`.

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
scheduled, err := pageSvc.Schedule(ctx, pages.SchedulePageRequest{
    PageID:      page.ID,
    PublishAt:   timePtr(time.Now().Add(24 * time.Hour)),
    UnpublishAt: timePtr(time.Now().Add(30 * 24 * time.Hour)),
    ScheduledBy: authorID,
})
```

**`SchedulePageRequest` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `PageID` | `uuid.UUID` | Yes | Page to schedule |
| `PublishAt` | `*time.Time` | No | Schedule publish time (UTC) |
| `UnpublishAt` | `*time.Time` | No | Schedule unpublish time (UTC) |
| `ScheduledBy` | `uuid.UUID` | Yes | Actor identifier |

**Validation:**

- `PublishAt` must be before `UnpublishAt` (if both are set)
- Neither timestamp can be zero-valued
- Returns `ErrSchedulingDisabled` when the feature is off
- Returns `ErrScheduleWindowInvalid` when `UnpublishAt` is before `PublishAt`

**What happens:**

1. `Page.PublishAt` and `Page.UnpublishAt` are set
2. Status is automatically adjusted:
   - `"scheduled"` if `PublishAt` is in the future
   - `"published"` if `PublishAt` has passed or no schedule is set with a published version
   - `"draft"` otherwise
3. Scheduler jobs are enqueued for the publish and unpublish times
4. Previous jobs for the same page are cancelled and replaced

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
page, _ := pageSvc.Get(ctx, pageID)
fmt.Println(page.EffectiveStatus) // "published", "scheduled", "archived", or "draft"
fmt.Println(page.IsVisible)       // true only when EffectiveStatus is "published"
```

---

## Error Reference

| Error | Cause |
|-------|-------|
| `ErrContentRequired` | Content ID missing or content entry not found |
| `ErrTemplateRequired` | Template ID is required |
| `ErrTemplateUnknown` | Template not found |
| `ErrSlugRequired` | Empty slug |
| `ErrSlugInvalid` | Slug contains invalid characters |
| `ErrSlugExists` | Slug already taken in the same environment |
| `ErrPathExists` | Translation path already exists for another page |
| `ErrUnknownLocale` | Locale code not in `cfg.I18N.Locales` |
| `ErrDuplicateLocale` | Same locale code appears twice in one request |
| `ErrParentNotFound` | Parent page not found |
| `ErrPageParentCycle` | Parent assignment creates a hierarchy cycle |
| `ErrNoPageTranslations` | At least one translation required |
| `ErrDefaultLocaleRequired` | Default locale translation required |
| `ErrPageRequired` | Page ID missing |
| `ErrPageSoftDeleteUnsupported` | `HardDelete: false` is not supported |
| `ErrPageTranslationNotFound` | Target locale not found on the page |
| `ErrPageTranslationsDisabled` | Translations feature is disabled |
| `ErrPageDuplicateSlug` | Unable to determine unique slug for duplicate |
| `ErrVersioningDisabled` | Versioning method called without `Features.Versioning` |
| `ErrPageVersionRequired` | Version number missing or invalid |
| `ErrVersionAlreadyPublished` | Attempting to publish an already-published version |
| `ErrVersionRetentionExceeded` | Maximum version count reached |
| `ErrVersionConflict` | `BaseVersion` does not match expected value |
| `ErrSchedulingDisabled` | Scheduling method called without `Features.Scheduling` |
| `ErrScheduleWindowInvalid` | `PublishAt` is after `UnpublishAt` |
| `ErrScheduleTimestampInvalid` | Zero-valued timestamp |
| `ErrPageMediaReferenceRequired` | Media reference requires id or path |

---

## Complete Example

This example demonstrates the full lifecycle: creating a page with hierarchy, translations, block placements, versioning with draft/publish, and scheduling.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/content"
    "github.com/goliatone/go-cms/internal/pages"
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
    pageSvc := module.Pages()
    authorID := uuid.New()

    // 1. Create a content type and content entry
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

    articleContent, err := contentSvc.Create(ctx, content.CreateContentRequest{
        ContentTypeID: articleType.ID,
        Slug:          "about-us-content",
        Status:        "published",
        CreatedBy:     authorID,
        UpdatedBy:     authorID,
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "About Us", Content: map[string]any{"body": "Welcome."}},
            {Locale: "es", Title: "Sobre nosotros", Content: map[string]any{"body": "Bienvenido."}},
        },
    })
    if err != nil {
        log.Fatalf("create content: %v", err)
    }

    // 2. Create a root page
    templateID := uuid.New() // Assumes a registered template
    aboutPage, err := pageSvc.Create(ctx, pages.CreatePageRequest{
        ContentID:  articleContent.ID,
        TemplateID: templateID,
        Slug:       "about",
        Status:     "draft",
        CreatedBy:  authorID,
        UpdatedBy:  authorID,
        Translations: []pages.PageTranslationInput{
            {Locale: "en", Title: "About Us", Path: "/about"},
            {Locale: "es", Title: "Sobre nosotros", Path: "/es/sobre-nosotros"},
        },
    })
    if err != nil {
        log.Fatalf("create page: %v", err)
    }
    fmt.Printf("Page: %s (id=%s, status=%s)\n", aboutPage.Slug, aboutPage.ID, aboutPage.Status)

    // 3. Create a child page
    teamContent, _ := contentSvc.Create(ctx, content.CreateContentRequest{
        ContentTypeID: articleType.ID,
        Slug:          "team-content",
        Status:        "published",
        CreatedBy:     authorID,
        UpdatedBy:     authorID,
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "Our Team", Content: map[string]any{"body": "Meet the team."}},
        },
    })

    teamPage, err := pageSvc.Create(ctx, pages.CreatePageRequest{
        ContentID:  teamContent.ID,
        TemplateID: templateID,
        ParentID:   &aboutPage.ID, // Child of About page
        Slug:       "team",
        Status:     "draft",
        CreatedBy:  authorID,
        UpdatedBy:  authorID,
        Translations: []pages.PageTranslationInput{
            {Locale: "en", Title: "Our Team", Path: "/about/team"},
        },
    })
    if err != nil {
        log.Fatalf("create child page: %v", err)
    }
    fmt.Printf("Child page: %s (parent=%v)\n", teamPage.Slug, teamPage.ParentID)

    // 4. Create a versioned draft with block placements
    blockDefID := uuid.New()  // Assumes registered block definition
    blockInstID := uuid.New() // Assumes created block instance
    draft, err := pageSvc.CreateDraft(ctx, pages.CreatePageDraftRequest{
        PageID: aboutPage.ID,
        Snapshot: pages.PageVersionSnapshot{
            Regions: map[string][]pages.PageBlockPlacement{
                "main": {
                    {Region: "main", Position: 0, BlockID: blockDefID, InstanceID: blockInstID},
                },
            },
            Metadata: map[string]any{"layout": "single-column"},
        },
        CreatedBy: authorID,
        UpdatedBy: authorID,
    })
    if err != nil {
        log.Fatalf("create draft: %v", err)
    }
    fmt.Printf("Draft: version=%d status=%s\n", draft.Version, draft.Status)

    // 5. Publish the draft
    published, err := pageSvc.PublishDraft(ctx, pages.PublishPagePublishRequest{
        PageID:      aboutPage.ID,
        Version:     draft.Version,
        PublishedBy: authorID,
    })
    if err != nil {
        log.Fatalf("publish draft: %v", err)
    }
    fmt.Printf("Published: version=%d\n", published.Version)

    // 6. Schedule an unpublish window
    scheduled, err := pageSvc.Schedule(ctx, pages.SchedulePageRequest{
        PageID:      aboutPage.ID,
        UnpublishAt: timePtr(time.Now().Add(90 * 24 * time.Hour)),
        ScheduledBy: authorID,
    })
    if err != nil {
        log.Fatalf("schedule: %v", err)
    }
    fmt.Printf("Scheduled: effective_status=%s\n", scheduled.EffectiveStatus)

    // 7. Duplicate the page
    clone, err := pageSvc.Duplicate(ctx, pages.DuplicatePageRequest{
        PageID:    aboutPage.ID,
        Slug:      "about-v2",
        Status:    "draft",
        CreatedBy: authorID,
        UpdatedBy: authorID,
    })
    if err != nil {
        log.Fatalf("duplicate: %v", err)
    }
    fmt.Printf("Duplicate: %s (id=%s)\n", clone.Slug, clone.ID)

    // 8. Move the child page to the clone
    moved, err := pageSvc.Move(ctx, pages.MovePageRequest{
        PageID:      teamPage.ID,
        NewParentID: &clone.ID,
        ActorID:     authorID,
    })
    if err != nil {
        log.Fatalf("move: %v", err)
    }
    fmt.Printf("Moved: %s now under parent=%v\n", moved.Slug, moved.ParentID)

    // 9. Update a single translation
    _, err = pageSvc.UpdateTranslation(ctx, pages.UpdatePageTranslationRequest{
        PageID:    aboutPage.ID,
        Locale:    "es",
        Title:     "Sobre nosotros (actualizado)",
        Path:      "/es/sobre-nosotros",
        UpdatedBy: authorID,
    })
    if err != nil {
        log.Fatalf("update translation: %v", err)
    }
    fmt.Println("Spanish translation updated")

    // 10. List all versions
    versions, err := pageSvc.ListVersions(ctx, aboutPage.ID)
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

- **GUIDE_BLOCKS.md** -- reusable content fragments with definitions, instances, and translations
- **GUIDE_WIDGETS.md** -- dynamic behavioral components with area-based placement
- **GUIDE_MENUS.md** -- navigation structures with URL resolution and i18n
- **GUIDE_I18N.md** -- internationalization, locale management, and translation workflows
- **GUIDE_THEMES.md** -- theme management, template registration, and asset resolution
- **GUIDE_CONFIGURATION.md** -- full config reference and DI container wiring
