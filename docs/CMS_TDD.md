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

### i18n Module (`i18n/`)

Internationalization facade:

- Bootstraps `github.com/goliatone/go-i18n` using CMS locale/fallback configuration
- Exposes a CMS-specific service interface for translators, formatters, and culture data
- Adds CMS augmentations (template helper wiring, repository-backed loaders, default fallbacks)
- Provides no-op implementations when the host disables localization

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

- Static generator handlers (`BuildSiteHandler`, `DiffSiteHandler`, `CleanSiteHandler`, `BuildSitemapHandler`) are registered by the DI container whenever `Config.Generator.Enabled` and `Config.Commands.Enabled` are both true.
- The CLI bootstrap (`cmd/static/internal/bootstrap`) enables commands, applies optional overrides for logger providers or generator storage, and returns a collector so the CLI (or host applications) can retrieve the instantiated handlers.
- CLI execution (`cmd/static/main.go`) now maps flags to the `staticcmd` message types and attaches a result callback that streams `generator.BuildResult` diagnostics into the shared logging helper. Assets-only and single-page flows emit metadata-only envelopes so callers can record telemetry without bypassing the handler middleware, and the `sitemap` subcommand triggers the standalone sitemap command for regeneration without a full site build.
- Hosts integrating the handlers directly should enable commands, request the collector or call `Module.CommandHandlers()`, and execute handlers with their own callbacks. If handlers are missing, the CLI surfaces a clear configuration error instead of silently falling back to service calls.
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
