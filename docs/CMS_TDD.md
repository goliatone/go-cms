# Go CMS Module - Architecture Design Document

## Table of Contents
1. [Overview](#overview)
2. [Design Philosophy](#design-philosophy)
3. [Key Architectural Decisions](#key-architectural-decisions)
4. [Entity Descriptions](#entity-descriptions)
5. [Core Architecture Components](#core-architecture-components)
6. [Data Model](#data-model)
7. [Go Module Structure](#go-module-structure)

## Overview

This document outlines a self contained CMS module for Go applications. The module focuses exclusively on content management, providing interfaces for external dependencies while maintaining minimal coupling. All concrete code listings referenced here are collected in [CMS_IMP.md](./CMS_IMP.md).

### Module Goals

- Self contained content management functionality
- Minimal external dependencies
- Interface based design for pluggable implementations
- Progressive enhancement through vertical slices
- Integration ready with existing Go ecosystem

### What This Module Provides

- Content type management (pages, blocks, menus, widgets)
- Template and theme concepts
- Internationalization support (thin wrapper over `github.com/goliatone/go-i18n`)
- Content versioning and scheduling
- Hierarchical content organization

### What This Module Does NOT Provide

- Authentication/Authorization (use external auth module)
- File upload/storage (use external storage module)
- Database implementation (interfaces here are implemented by `github.com/goliatone/go-persistence-bun` + `github.com/goliatone/go-repository-bun`)
- HTTP API/CRUD (use external API layer)
- Caching implementation (interfaces here are satisfied by `github.com/goliatone/go-repository-cache` adapters)
- Template rendering engine (use external template module)

## Design Philosophy

### Vertical Slices Approach

Start with minimal viable functionality and progressively enhance:
1. **Sprint 1**: Pages with basic content
2. **Sprint 2**: Block system for composable content
3. **Sprint 3**: Menu management
4. **Sprint 4**: Widget system
5. **Sprint 5**: Advanced features (versioning, scheduling)

### Module Independence

Each content type is isolated with no direct dependencies on others. Service layers orchestrate interactions between modules.

### Interface-Driven Design

All external dependencies are defined as interfaces, allowing the host application to provide implementations.

## Key Architectural Decisions

1. **Opaque Locale Codes**: System treats locale codes as strings without parsing format assumptions.

2. **Nullable Fields**: Advanced features use nullable foreign keys and JSONB fields. Simple mode leaves these `NULL`.

3. **Interface-Based**: Locale-specific logic is behind interfaces with default implementations.

4. **Default Configuration**: Functions with simple codes without required setup.

5. **Opt-In Complexity**: Advanced features like custom fallback chains or regional formatters are inactive by default and must be enabled via the main `Config` struct.

6. **Unified Schema**: Simple and complex modes use identical database schema with different data.

7. **Progressive Enhancement**: Start with pages (Sprint 1), add blocks (Sprint 2), then menus and widgets. Each feature is independent.

8. **Service Layer Architecture**: Business logic resides in services, not in data models or repositories.

9. **Soft Deletes**: All entities support `deleted_at` for data recovery and audit trails.

10. **Scheduled Publishing**: Content and widgets support `publish_on` for future publishing.
    - Jobs are enqueued through the shared scheduler interface (`pkg/interfaces.Scheduler`) and processed by the `internal/jobs.Worker`, which toggles publish state for content and pages and records audit entries. Hosts wiring their own scheduler must provide unique job keys and idempotent completion semantics so missed jobs can be retried safely.

11. **Translation-First**: Every user-facing string is translatable from day one.

12. **Minimal Dependencies**: The module keeps external dependencies to a minimum; internationalization delegates to `github.com/goliatone/go-i18n` behind a thin wrapper, persistence contracts are satisfied by adapters backed by `github.com/goliatone/go-persistence-bun` and `github.com/goliatone/go-repository-bun`, and caching decorators are provided by `github.com/goliatone/go-repository-cache`, while all other integrations flow through host-provided interfaces.

13. **Pluggable Logging**: The runtime only depends on the leveled logging contracts declared in `pkg/interfaces/logger.go`. A thin console logger is used for tests and bootstrapping, while production deployments can drop in `github.com/goliatone/go-logger` (or any compatible provider) without introducing a mandatory module dependency. Service log entries always include the `operation` name alongside entity identifiers; command handlers reuse the same schema by emitting `module`, `component=command`, `command` (message type), and entity fields (e.g., `content_id`, `page_id`) so telemetry and dashboards line up across services and commands.

14. **Isolated Modules**: Each content type module (pages, blocks, menus, widgets) is independent with no direct dependencies on others.

## Entity Descriptions

The CMS is composed of several key entities that work together to manage and deliver content. Each entity has a distinct role and set of responsibilities.

For a detailed breakdown of each entity, its fields, and database schema, please refer to the [CMS Entities Guide](./CMS_ENTITIES.md).

### Pages
**Role**: Hierarchical content containers representing website sections. The primary structural element of the site.

### Blocks
**Role**: Atomic content units that compose pages. The fundamental building block of content.

### Menus
**Role**: Navigation structure that links content and external resources. Organizes site hierarchy for user navigation.

### Widgets
**Role**: Dynamic content modules displayed in defined areas. Provides contextual functionality across pages.

### Templates
**Role**: Presentation layer concept defining how content renders. Controls visual structure and layout patterns.

### Themes
**Role**: Collection of templates and assets forming a complete site design. Organizes presentation resources.

## Core Architecture Components

### Content Module (`content/`)

Core content management functionality:

- Content type definitions
- Version control interfaces
- Draft/publish workflow
- Content validation
- Slug generation
- Persistence through the `StorageProvider` interface (fulfilled by go-persistence-bun/go-repository-bun adapters via `di.WithBunDB`)
- Repository-level caching backed by `interfaces.CacheProvider` (defaults enabled via `cms.Config.Cache`)
- Version retention limits enforced via `cms.Config.Retention.Content`; non-zero limits emit `ErrContentVersionRetentionExceeded` and log warnings when draft creation exceeds the configured window.

### Blocks Module (`blocks/`)

Block based content system:

- Block type registry
- Block rendering interfaces
- Block validation
- Nested block support
- Reusable block patterns
- Repository integration through go-repository-bun
- Repository-level caching (go-repository-cache) and DI wiring via `BlockService`
- Optional version retention limits controlled by `cms.Config.Retention.Blocks`; when configured the service returns `ErrInstanceVersionRetentionExceeded` once drafts exceed the allowed count.

### Pages Module (`pages/`)

Hierarchical page management:

- Page hierarchy
- Path management
- Template assignment
- Menu order
- Repository backed by go-repository-bun via the shared `StorageProvider`
- Version history retention driven by `cms.Config.Retention.Pages`; when the limit is exceeded the service surfaces `ErrVersionRetentionExceeded` and records a warning so operators can adjust policies or archive older revisions.

### Menus Module (`menus/`)

Navigation management:

- Menu structure
- Menu locations
- Menu item types
- Hierarchical items
- Repository integration through go-repository-bun

### Widgets Module (`widgets/`)

Widget functionality:

- Widget types
- Widget areas
- Visibility rules
- Widget settings
- Repository integration through go-repository-bun

#### Behaviour

- **Definition registration** – widget types require a non-empty name and JSON schema. Defaults are validated against the schema field list to avoid phantom configuration. The service deduplicates registrations and surfaces `ErrDefinitionExists` when IDs collide. Registry entries (built-in or host supplied) are applied automatically at service start.
- **Configuration-driven registry** – `cms.Config.Widgets.Definitions` seeds the registry on startup. The default config still registers the `newsletter_signup` widget so existing deployments keep the prior behaviour. Hosts can clear the slice to opt-out or provide replacement definitions; duplicate names are deduplicated using case-insensitive matching, allowing overrides without touching internal helpers.
- **Instance lifecycle** – instances merge configuration with definition defaults and enforce creator/updater UUIDs. Visibility schedule (`publish_on` / `unpublish_on`) is validated, and visibility rules must contain supported keys (`schedule`, `audience`, `segments`, `locales`). Unknown configuration keys or invalid schedules raise errors.
- **Translations** – each widget instance can store one translation per locale. Duplicate inserts return `ErrTranslationExists`; updates replace the JSON payload in place.
- **Area management** – areas are optional and feature-gated by repository availability. Definitions require canonical codes (`[a-z0-9._-]`), human-readable names, and an optional scope (`global`, `theme`, `template`). Assigning widgets to areas enforces uniqueness per locale/area pair, supports explicit positioning, and persists placement metadata. Reordering requires the full placement list to guarantee deterministic ordering.
- **Visibility evaluation** – `EvaluateVisibility` runs chronological gating followed by audience/segment matching and locale allowlists. Locale mismatches surface `ErrVisibilityLocaleRestricted` so callers can fall back to other placements. `ResolveArea` walks a locale chain (primary → configured fallbacks → default) and only returns visible widgets, preserving placement metadata for rendering.
- **Bootstrapping** – `widgets.Bootstrap` wraps `EnsureDefinitions` / `EnsureAreaDefinitions` to seed built-ins repeatedly without failing when they already exist or when the feature is disabled.

#### Storage

- Memory repositories exist for tests and no-op deployments.
- Bun repositories support optional cache decorators (go-repository-cache) for definitions, instances, and translations.
- Area definitions/placements use Bun transactions to replace locale-specific ordering atomically.
- Integration tests cover sqlite + cache wiring to ensure parity with blocks/pages/menus.
- Workflow fixtures in `internal/workflow/testdata` back integration tests (`internal/workflow/integration_test.go`) that execute multi-step page transitions through the DI container.

### CRUD Usage Examples for go-admin

The Phase 2–3 CRUD work surfaced in [READINESS_TSK.md](../READINESS_TSK.md) is now available through the exported `cms.Module` façade. The following snippets show how go-admin scaffolding should call the new methods when wiring editor forms and drag-and-drop tooling.

#### Blocks

```go
import (
    "context"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/blocks"
    "github.com/google/uuid"
)

func updateBlockDefinition(module *cms.Module, defID, instID, localeID, editorID uuid.UUID) error {
    ctx := context.Background()
    svc := module.Blocks()
    ptr := func[T any](v T) *T { return &v }

    _, err := svc.UpdateDefinition(ctx, blocks.UpdateDefinitionInput{
        ID:   defID,
        Name: ptr("Hero Banner"),
        Schema: map[string]any{
            "type": "object",
            "properties": map[string]any{"headline": map[string]any{"type": "string"}},
        },
        Defaults: map[string]any{"headline": "Launch 2025"},
    })
    if err != nil {
        return err
    }

    if err := svc.DeleteInstance(ctx, blocks.DeleteInstanceRequest{ID: instID}); err != nil {
        return err
    }

    _, err = svc.UpdateTranslation(ctx, blocks.UpdateTranslationInput{
        BlockInstanceID: instID,
        LocaleID:        localeID,
        Content:         map[string]any{"headline": "Launch 2025 (EN)"},
        UpdatedBy:       editorID,
    })
    return err
}
```

*UI hook*: use `UpdateDefinition` to power the schema editor, `UpdateInstance`/`DeleteInstance` for block placements, and `UpdateTranslation` + `DeleteTranslationRequest` for locale tabs. All of these operations continue to emit version snapshots, so go-admin can simply refresh the listing after each mutation.

#### Widgets

```go
import (
    "context"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/widgets"
    "github.com/google/uuid"
)

func purgeWidget(module *cms.Module, defID, instID, localeID uuid.UUID) error {
    ctx := context.Background()
    svc := module.Widgets()

    if err := svc.DeleteDefinition(ctx, widgets.DeleteDefinitionRequest{ID: defID}); err != nil {
        return err
    }
    if err := svc.DeleteInstance(ctx, widgets.DeleteInstanceRequest{InstanceID: instID}); err != nil {
        return err
    }
    return svc.DeleteTranslation(ctx, widgets.DeleteTranslationRequest{
        InstanceID: instID,
        LocaleID:   localeID,
    })
}
```

*UI hook*: bind delete controls in the go-admin widget registry to these service calls so placements are removed via the repository cascade instead of manual SQL or cache flushes. Failures are typed (`ErrInstanceNotFound`, `ErrDefinitionInUse`) and can be shown inline to operators.

#### Menus

```go
import (
    "context"
    "errors"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/menus"
    "github.com/google/uuid"
)

func deleteMenuAndReorder(module *cms.Module, menuID uuid.UUID, actor uuid.UUID, updates []menus.ItemOrder) error {
    ctx := context.Background()
    svc := module.Menus()

    if err := svc.DeleteMenu(ctx, menus.DeleteMenuRequest{
        MenuID:    menuID,
        DeletedBy: actor,
        Force:     false,
    }); err != nil {
        var inUse *menus.MenuInUseError
        if errors.As(err, &inUse) {
            // Surface guard-rail warnings to admins; go-admin can present the bindings in a modal.
            return err
        }
    }

    _, err := svc.BulkReorderMenuItems(ctx, menus.BulkReorderMenuItemsInput{
        MenuID:    menuID,
        Items:     updates,
        UpdatedBy: actor,
    })
    return err
}
```

*UI hook*: go-admin’s drag-and-drop navigation editor should batch the hierarchy into `BulkReorderMenuItemsInput` so reorders stay atomic, and deletions should honor the `MenuUsageResolver` output instead of bypassing guard rails. Translation CRUD (update/delete) remains a follow-up, tracked in Phase 4 to ensure parity once designs land.

### Storage Provider Runtime Configuration

Phase 4 completes the runtime storage story while keeping the module headless:

- `pkg/storage` remains the single contract for adapters; reloadable providers advertise capabilities through the optional mix-ins introduced in Phases 1–3.
- `internal/admin/storage.Service` now surfaces management helpers (`ApplyConfig`, `ListProfiles`, `GetProfile`, `ValidateConfig`, `PreviewProfile`, and `Schemas`) that emit audit events instead of HTTP routes. Hosts plug these methods into their own stacks (`go-router`, `go-command`, gRPC, etc.) without importing `internal/` packages.
- `cms.Module.StorageAdmin()` exposes the service so outer packages can obtain it directly from the public facade.
- Preview helpers call the DI-registered storage factories, performing a dry initialisation and returning provider capabilities/diagnostics without touching the active handle. Failures leave the current profile untouched.
- Observability leans on the existing telemetry surface: audit events (`storage_profile_created/updated/deleted`, `storage_profile_aliases_updated`) plus container logs (`storage.profile_activated`, `storage.profile_activate_failed`, `storage.profile_subscription_failed`) are enough to drive the dashboards referenced in `TODO_TSK.md`. Hosts can forward the preview diagnostics map to their metric system if they need richer charts.
- Integration coverage lives in `internal/integration/storage_admin_test.go`, which rotates profiles while concurrent writes continue to prove the hot-swap path is transparent to callers.

### Workflow Engine (`workflow/`)

Externalises status transitions so host applications can drive lifecycle policies without patching page internals.

- **Interfaces first** – `pkg/interfaces/workflow.go` declares `WorkflowEngine`, `WorkflowDefinition`, and transition DTOs. Services call the engine with a `workflow.PageContext` payload and never inspect raw status strings.
- **Domain enums** – `internal/domain/status.go` centralises canonical states (`Draft`, `Published`, `Scheduled`, `Archived`, etc.) and adapters map legacy string constants to the new enum so existing database rows remain valid during rollout.
- **Default engine** – `internal/workflow/simple` mirrors the historical draft ↔ published behaviour, preserves schedule enforcement, and exposes aggregate `Result.Events` so page and content services can emit audit/telemetry hooks.
- **Dependency injection** – `cms.Config.Workflow` accepts provider functions and optional definitions. `internal/di/container.go` builds the simple engine when nothing is supplied and injects any host-provided implementation into pages/content services alongside the configured definitions.
- **Fixtures & contracts** – JSON fixtures under `internal/workflow/testdata` and golden outputs drive both engine contract tests and DI integration suites, ensuring new workflow definitions behave deterministically across services.

#### Migration & Upgrade Path

- Step 1: Align existing data with enums. The legacy status strings (`"draft"`, `"published"`, `"scheduled"`) continue to load via the enum adapters, so no immediate migrations are required; operators should verify custom statuses map cleanly before enabling custom engines.
- Step 2: Configure definitions. Populate `cms.Config.Workflow.Definitions` (or backing storage) with the desired states and transitions, then inject a custom engine through `di.WithWorkflowEngine`.
- Step 3: Extend consumers. Page/content services already consume `WorkflowResult.Status`, so once the engine returns custom states (for example, `review`, `localised`), the services surface them without further code changes. Downstream integrations (generators, web UIs) must update their allowed status lists to recognise the new states.
- Step 4: Audit hooks & authorisation. Emit additional events via `WorkflowResult.Events` and implement `interfaces.WorkflowAuthorizer` callbacks to gate transitions; TODO entries track deeper permission wiring and UI parity for non-default states.

### i18n Module (`i18n/`)

Internationalization facade:

- Bootstraps `github.com/goliatone/go-i18n` using CMS locale/fallback configuration
- Exposes a CMS-specific service interface for translators, formatters, and culture data
- Adds CMS augmentations (template helper wiring, repository-backed loaders, default fallbacks)
- Provides no-op implementations when the host disables localization

Configuration highlights:

- `cms.Config.DefaultLocale` sets the fallback locale; it must be populated whenever `cms.Config.I18N.DefaultLocaleRequired` or `cms.Config.I18N.RequireTranslations` are true.
- `cms.Config.I18N.RequireTranslations` (defaults to `true`) preserves the legacy requirement that every entity carries at least one translation. Disable it to support staged rollouts or monolingual publishing; see `TRANS_FIX.md` for the rollout rationale.
- `cms.Config.I18N.DefaultLocaleRequired` (defaults to `true`) enforces that a fallback locale exists even when translations are optional, allowing hosts to relax the constraint in tightly scoped deployments.
- When `cms.Config.I18N.Enabled` is `false`, both requirement flags are ignored and services operate in monolingual mode while still accepting explicit translations if provided.
- Operators can toggle translation enforcement at runtime via `module.TranslationAdmin()`:
  - `TranslationsEnabled=false` disables translation mutation APIs (`UpdateTranslation`/`DeleteTranslation`) and skips locale lookups unless explicit translations are provided.
  - `RequireTranslations=false` allows status-only create/update workflows to omit translations while keeping translations enabled for callers that still supply them.

#### Translation Flexibility Migration

1. Keep the defaults (`RequireTranslations = true`, `DefaultLocaleRequired = true`) while upgrading—behaviour remains identical to previous releases.
2. Use the per-request `AllowMissingTranslations` flag on create/update DTOs when a workflow transition or import step needs to bypass enforcement without changing global defaults.
3. For systemic monolingual deployments, flip `RequireTranslations` to `false` (and optionally relax `DefaultLocaleRequired`). Services treat empty translation slices as no-ops so the existing locale data is preserved.
4. When `I18N.Enabled` is disabled entirely, requirement flags are ignored but overrides are still honoured; validate template/rendering fallbacks as outlined in the appendix of `TRANS_FIX.md` before rolling out broadly.

### Markdown Slice – Phase 6 Closeout

The Markdown importer reached functional parity in Phases 3–5; Phase 6 focuses on hardening and documentation:

- **Operational Guides** – `FEAT_MARKDOWN.md` now ships with end-to-end setup instructions (config wiring, CLI usage, adapter behaviour). The README surfaces a quick-start snippet so hosts can enable the slice without spelunking through internal packages.
- **QA Playbook** – A Phase 6 checklist lives alongside the feature document, covering manual import, sync retries, cron dispatch, dry-run previews, and failure handling for missing directories.
- **Telemetry Backlog** – Observability gaps (metrics counters, tracing, tenant tagging) plus automation hooks for cron failures are tracked as follow-ups so Option 6 can evolve into multi-tenant environments without guesswork.
- **Example Alignment** – `examples/web/` documents the delete/recreate adapter strategy demanded by current page APIs; the documentation calls this out so production adopters can swap in richer implementations once incremental page updates land.

Future iterations (post Phase 6) should extend the CLI/command handlers with metrics emitters and pluggable workspace resolvers before the slice is considered production-ready for multi-tenant installations.

### Themes Module (`themes/`)

Theme management:

- Theme registration
- Template organization
- Widget area definitions
- Menu location definitions
- Repository integration through go-repository-bun

### Logging (cross-cutting)

- Runtime diagnostics flow through the `pkg/interfaces.Logger` contract.
- Default development/testing builds rely on a lightweight console logger that satisfies the interface without additional dependencies.
- Production deployments can plug in `github.com/goliatone/go-logger` (or other compatible packages) by wiring a `LoggerProvider` through the DI container.
- `cms.Config.Logging` governs integration: set `Provider` to `console` for the built-in fallback or `gologger` to activate the adapter defined in `internal/logging/gologger`, and use `Level`, `Format`, and `AddSource` to align output with deployment requirements.

## Data Model

The data model is designed to be flexible and support the features outlined in this document, including internationalization, content versioning, and a component-based structure.

All table definitions, field descriptions, and example data are maintained in the [CMS Entities Guide](./CMS_ENTITIES.md). Below is a high-level overview of the tables.

### Locales Table
Stores available languages and their configuration for multilingual support.

*Note: The `locales` table schema in [CMS_ENTITIES.md](./CMS_ENTITIES.md#internationalization-architecture) can be extended with fields like `native_name` for a richer implementation.*

### Content Types Table
Defines different types of content (pages, posts, custom types) and their capabilities.

### Contents Table
Base content storage for all content types.

### Content Translations Table
Stores localized content for each content item.

### Block Types Table
Defines available block types and their configuration.

*Note: The `block_types` table schema in [CMS_ENTITIES.md](./CMS_ENTITIES.md#block-types-table) can be extended with fields like `icon`, `description`, `editor_style_url`, and `frontend_style_url` for a more complete block editor experience.*

### Block Instances Table
Stores actual block usage instances within content.

### Pages Table
Extends content for hierarchical page structure.

### Themes Table
Defines available themes and their configuration.

### Templates Table
Defines templates available within themes.

### Menus Table
Defines navigation menus.

### Menu Items Table
Defines individual items within menus.

### Widget Types Table
Defines available widget types.

*Note: The `widget_types` table schema in [CMS_ENTITIES.md](./CMS_ENTITIES.md#widget-types-table) can be extended with a `description` field.*

### Widget Instances Table
Stores configured widget instances.

## Go Module Structure

### Module Layout

Following ARCH_DESIGN.md principles, the module uses `internal/` for implementation details and `pkg/` for exported packages. The full directory tree and adapter notes live in the [CMS Implementation Reference – Module Layout](./CMS_IMP.md#module-layout).

### External Dependency Interfaces

All infrastructure contracts are centralised under `pkg/interfaces/` so host applications can supply their own implementations. Detailed interface definitions and notes on how the Bun- and cache-backed adapters fulfil them are documented in the [CMS Implementation Reference – External Dependency Interfaces](./CMS_IMP.md#external-dependency-interfaces).

### Configuration Layer

The configuration package separates "what to use" from "how to wire" and encodes feature gating, defaults, and validation. See [CMS Implementation Reference – Configuration Layer](./CMS_IMP.md#configuration-layer) for the full struct definitions and helper methods.

### Public API Layer

Layer 3 exposes a slim façade that forwards calls to the DI container while returning no-op implementations when features are disabled. The full reference implementation lives in the [CMS Implementation Reference – Public API Layer](./CMS_IMP.md#public-api-layer).

### DI Container Implementation

The dependency injection container (Layer 2: Wiring) manages service lifecycle, applies feature gating, and normalises access to external infrastructure. Implementation details, helper functions, and i18n integration notes are consolidated in the [CMS Implementation Reference – Dependency Injection Container](./CMS_IMP.md#dependency-injection-container), including the reminder that the `internal/i18n` wrapper simply configures `go-i18n` on our behalf.

### Content Module Example

The content module models translatable entries, exposes repository/service contracts, and integrates with i18n. Representative type and interface listings are collected in the [CMS Implementation Reference – Content Module](./CMS_IMP.md#content-module).

### Pages Module Example

Page services compose content entities with page-specific metadata and persistence rules. The reference implementation (types and service wiring) is available in the [CMS Implementation Reference – Pages Module](./CMS_IMP.md#pages-module).

## Implementation Approach

### Testing Infrastructure

Following ARCH_DESIGN.md, we rely on fixture-driven tests, golden files, and repository contract suites. Concrete helper functions and example tests are catalogued in the [CMS Implementation Reference – Testing Infrastructure](./CMS_IMP.md#testing-infrastructure).

### Progressive Complexity Phases

**Phase 1: Core**
- Content module (CRUD operations)
- Pages module (hierarchy support)
- i18n wrapper (delegates to go-i18n with simple locale codes)
- Tables: locales, content_types, contents, content_translations, pages

**Phase 2: Blocks**
- Block type registry
- Block instances within pages
- Nested block support
- Tables: block_types, block_instances, block_translations

**Phase 3: Menus**
- Menu management
- Hierarchical menu items
- Tables: menus, menu_items, menu_item_translations

**Phase 4: Widgets**
- Widget type registry
- Widget areas and visibility rules
- Tables: widget_types, widget_instances, widget_translations

**Phase 5: Themes**
- Theme management
- Template hierarchy
- Tables: themes, templates

**Phase 6: Advanced**
- Content versioning
- Scheduled publishing
- Media library integration
- Markdown documentation & QA closeout (operational guides, telemetry backlog)

**Phase 7: Observability**
- Logger interface promotion (`pkg/interfaces/logger.go`)
- Console logger fallback for local/testing builds
- go-logger adapter wiring via DI (`go-logger` alignment and examples)
- Structured audit/worker logs using the new contract
- Documentation for provider wiring and troubleshooting (`docs/LOGGING_GUIDE.md`)

## Static Command Integration

- Static generator handlers (`BuildSiteHandler`, `DiffSiteHandler`, `CleanSiteHandler`, `BuildSitemapHandler`) are built whenever `Config.Generator.Enabled` is true; the CLIs construct them directly from the generator service without a collector/registry path.
- The CLI maps flags to the `staticcmd` message types and attaches a result callback that streams `generator.BuildResult` diagnostics into the shared logging helper. Assets-only and single-page flows emit metadata-only envelopes so callers can record telemetry without bypassing the inline timeout/logging applied by the handlers, and the `sitemap` subcommand triggers the standalone sitemap command for regeneration without a full site build.
- Hosts integrating the handlers programmatically should call the constructors (`staticcmd.NewBuildSiteHandler`, etc.) with a generator service and logger provider, then invoke `Execute` with their own contexts or cron/dispatcher glue. Legacy registry/cron wiring lives in the optional adapter submodule (`github.com/goliatone/go-cms/commands`) for embedders that still need that path.
- The generator writes `sitemap.xml` alongside build artifacts whenever `Config.Generator.GenerateSitemap` is true and exposes a dedicated `BuildSitemap` method for explicit regeneration.
- RSS/Atom feeds are produced per locale under `feeds/<locale>.rss.xml` and `feeds/<locale>.atom.xml` (with default-locale aliases `feed.xml` / `feed.atom.xml`) when `Config.Generator.GenerateFeeds` is enabled; incremental builds continue to refresh these feeds even when page HTML is skipped.

## Usage Examples

End-to-end samples for each sprint (from the minimal setup through the fully featured stack) are maintained in the [CMS Implementation Reference – Usage Examples](./CMS_IMP.md#usage-examples). They demonstrate how configuration flags unlock additional services, how the Bun-based adapters plug in, and how cache decorators shape behaviour across progressive releases.

## Architectural Approach: Progressive Complexity

**Design Constraints**:
- Many i18n libraries require choosing between simple or complex modes at initialization
- Simple implementations cannot handle regional variations
- Complex implementations have higher learning curves
- Switching modes typically requires application rewrites

**Implementation Approach**: The progressive i18n rollout (simple → regional → advanced) is illustrated in the [CMS Implementation Reference – Progressive Complexity Reference](./CMS_IMP.md#progressive-complexity-reference).

**Migration Path**: We still follow the same staged upgrades—start with simple codes, introduce regional variants, then add explicit locale groups—which are documented alongside the implementation diagram in `CMS_IMP.md`.

**Upgrade Path**: Each level of i18n complexity is enabled by providing additional configuration, without requiring modifications to existing application code or database schema. Behind the scenes the wrapper simply feeds these options into `github.com/goliatone/go-i18n`; increasing complexity means enabling more of the external package's capabilities while keeping the CMS-facing API unchanged.
