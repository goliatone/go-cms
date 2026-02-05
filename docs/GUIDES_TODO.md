# Documentation Guides TODO

This document tracks the planned user guides for `go-cms`. Each guide helps users understand and use specific features of the package.

## Guide Status Legend

- `pending` - Not started
- `in-progress` - Currently being written
- `review` - Draft complete, needs review
- `done` - Published and complete

---

## 1. GUIDE_GETTING_STARTED.md

**Status**: `review`

**Purpose**: Get users managing their first content and pages in under 5 minutes.

**Sections**:
- Overview of go-cms (library, not a service)
- Installation (`go get github.com/goliatone/go-cms`)
- Minimal setup with `cms.DefaultConfig()` and in-memory repositories
- Creating a content type, content entry, and page
- Wiring with BunDB for persistent storage
- Feature flags overview
- Next steps / where to go from here

**Primary Audience**: New users
**Complexity**: Beginner

---

## 2. GUIDE_CONTENT.md

**Status**: `review`

**Purpose**: Comprehensive guide to content types, content entries, and translations.

**Sections**:
- Content architecture overview (content types vs content entries)
- Content type lifecycle:
  - `CreateContentType` with JSON schema
  - Schema validation via `jsonschema/v5`
  - Slug rules and uniqueness
- Content CRUD:
  - `Create` with `CreateContentRequest`
  - `Get`, `List`, `Update`, `Delete`
- Content translations:
  - `ContentTranslationInput` on create/update
  - `UpdateTranslation`, `DeleteTranslation`
  - Per-request `AllowMissingTranslations` override
- Content versioning (when `Features.Versioning` is enabled):
  - `CreateDraft`, `PublishDraft`, `PreviewDraft`
  - `ListVersions`, `RestoreVersion`
- Scheduling:
  - `Schedule` with `publishAt`/`unpublishAt` timestamps
- Status transitions and content lifecycle

**Primary Audience**: Backend developers
**Complexity**: Intermediate

---

## 3. GUIDE_PAGES.md

**Status**: `review`

**Purpose**: Managing page hierarchy, routing paths, and page-block relationships.

**Sections**:
- Page architecture overview (pages wrap content with hierarchy and routing)
- Page CRUD:
  - `Create` with `CreatePageRequest` (contentID, templateID, parentID, slug)
  - `Get`, `List`, `Update`, `Delete`
- Page hierarchy:
  - Parent-child relationships
  - `Move` to reparent pages
  - `Duplicate` for page cloning
  - Path resolution from hierarchy
- Page translations:
  - `PageTranslationInput` with locale, title, path, summary
  - `UpdateTranslation`, `DeleteTranslation`
- Page-block integration:
  - Assigning block instances to page regions
  - Listing blocks by page
- Page versioning:
  - `CreateDraft`, `PublishDraft`, `PreviewDraft`
  - `ListVersions`, `RestoreVersion`
- Scheduling pages for publish/unpublish

**Primary Audience**: Backend developers
**Complexity**: Intermediate

---

## 4. GUIDE_BLOCKS.md

**Status**: `review`

**Purpose**: Using reusable content fragments with definitions, instances, and translations.

**Sections**:
- Block architecture overview (definitions, instances, translations)
- Block definitions:
  - `RegisterDefinition` with name, slug, schema, uiSchema, defaults
  - `SyncRegistry` for bulk registration from config
  - `ListDefinitions`, `UpdateDefinition`, `DeleteDefinition`
- Definition versioning:
  - `CreateDefinitionVersion`, `GetDefinitionVersion`, `ListDefinitionVersions`
- Block instances:
  - `CreateInstance` with definitionID, pageID, region, position, configuration
  - `ListPageInstances` and `ListGlobalInstances`
  - `UpdateInstance`, `DeleteInstance`
- Block translations:
  - `AddTranslation` with localeID, content, mediaBindings
  - `UpdateTranslation`, `DeleteTranslation`, `GetTranslation`
- Instance versioning:
  - `CreateDraft`, `PublishDraft`, `ListVersions`, `RestoreVersion`
- Embedded blocks and block-in-block patterns
- Block admin service (`module.BlockAdmin()`)

**Primary Audience**: Backend developers
**Complexity**: Intermediate

---

## 5. GUIDE_WIDGETS.md

**Status**: `review`

**Purpose**: Dynamic behavioral components with area-based placement and visibility rules.

**Sections**:
- Widget architecture overview (definitions, instances, areas, visibility)
- Enabling widgets: `cfg.Features.Widgets = true`
- Widget definitions:
  - `RegisterDefinition` with name, schema, defaults
  - `SyncRegistry` for config-driven registration
  - Config-based definitions via `cfg.Widgets.Definitions`
- Widget instances:
  - `CreateInstance` with visibilityRules, publishOn, unpublishOn
  - `ListInstancesByDefinition`, `ListInstancesByArea`, `ListAllInstances`
  - `UpdateInstance`, `DeleteInstance`
- Widget translations:
  - `AddTranslation`, `UpdateTranslation`, `DeleteTranslation`
- Area management:
  - `RegisterAreaDefinition` for named widget zones
  - `AssignWidgetToArea`, `RemoveWidgetFromArea`, `ReorderAreaWidgets`
  - `ResolveArea` for rendering
- Visibility rules:
  - `EvaluateVisibility` with `VisibilityContext`
  - Time-based, locale-based, audience-based, and custom rules
- Common patterns (sidebars, footers, promotions)

**Primary Audience**: Backend developers
**Complexity**: Intermediate

---

## 6. GUIDE_MENUS.md

**Status**: `review`

**Purpose**: Navigation structures with URL resolution and i18n support.

**Sections**:
- Menu architecture overview (menus, items, translations, navigation resolution)
- Menu CRUD:
  - `CreateMenu`, `GetOrCreateMenu`, `UpsertMenu`
  - `GetMenu`, `GetMenuByCode`, `GetMenuByLocation`
  - `DeleteMenu`, `ResetMenuByCode`
- Menu items:
  - `AddMenuItem` with externalCode, position, type, target, translations
  - `UpsertMenuItem` for idempotent bootstraps
  - `UpdateMenuItem`, `DeleteMenuItem`
  - `BulkReorderMenuItems` for drag-and-drop UIs
  - `ReconcileMenu` for deferred parent resolution
- Menu item translations:
  - `AddMenuItemTranslation`, `UpsertMenuItemTranslation`
- Out-of-order upserts: `cfg.Menus.AllowOutOfOrderUpserts`
- Navigation resolution:
  - `ResolveNavigation` by menu code and locale
  - `ResolveNavigationByLocation` for theme-bound menus
  - `NavigationNode` tree structure
- URL resolution with go-urlkit:
  - `cfg.Navigation.RouteConfig` setup
  - Locale groups and default routes
  - `URLKit.DefaultGroup` and `URLKit.LocaleGroups`
- Cache invalidation: `InvalidateCache`

**Primary Audience**: Backend developers
**Complexity**: Intermediate

---

## 7. GUIDE_I18N.md

**Status**: `review`

**Purpose**: Internationalization, locale management, and translation workflows.

**Sections**:
- I18N architecture overview
- Configuration:
  - `cfg.I18N.Enabled`, `cfg.I18N.Locales`
  - `cfg.DefaultLocale`
  - `cfg.I18N.RequireTranslations` and `cfg.I18N.DefaultLocaleRequired`
- Translation model across entities:
  - Content translations
  - Page translations
  - Block translations
  - Widget translations
  - Menu item translations
- Per-request flexibility:
  - `AllowMissingTranslations` on create/update requests
- Global opt-out for monolingual sites
- Translation grouping
- i18n.Service and template helpers
- Translation admin service (`module.TranslationAdmin()`)
- Common patterns (monolingual, bilingual, full multi-locale)

**Primary Audience**: Backend developers
**Complexity**: Intermediate

---

## 8. GUIDE_THEMES.md

**Status**: `review`

**Purpose**: Theme management, template registration, and asset resolution.

**Sections**:
- Theme architecture overview (go-theme integration)
- Enabling themes: `cfg.Features.Themes = true`
- Configuration: `cfg.Themes.BasePath`, `cfg.Themes.DefaultTheme`, `cfg.Themes.DefaultVariant`
- Theme lifecycle:
  - `RegisterTheme` with name, version, themePath, config
  - `ActivateTheme`, `DeactivateTheme`
  - `ListThemes`, `ListActiveThemes`, `ListActiveSummaries`
- Theme manifest (`theme.json`):
  - Asset declarations (scripts, styles)
  - Version and metadata
- Template management:
  - `RegisterTemplate` with slug, name, templatePath, regions
  - `UpdateTemplate`, `DeleteTemplate`
  - `ListTemplates` per theme
- Regions:
  - `TemplateRegions` for a single template
  - `ThemeRegionIndex` for all templates in a theme
- Template helpers in rendering:
  - `.Theme.AssetURL(path)`
  - `.Theme.Partials`
  - `.Theme.CSSVars`
  - `.Helpers.WithBaseURL(url)`

**Primary Audience**: Frontend/full-stack developers
**Complexity**: Intermediate

---

## 9. GUIDE_STATIC_GENERATION.md

**Status**: `review`

**Purpose**: Building static sites with the locale-aware generator.

**Sections**:
- Generator architecture overview
- Configuration: `cfg.Generator.OutputDir`, locales, workers, timeouts
- Generator dependencies (pages, content, blocks, widgets, menus, themes, renderer, storage)
- Build operations:
  - `Build` with `BuildOptions` (locales, pageIDs, dryRun, force, assetsOnly)
  - `BuildPage` for single-page regeneration
  - `BuildAssets` for asset-only copy
  - `BuildSitemap` for sitemap generation
  - `Clean` for output cleanup
- `BuildResult` structure: pages built, diagnostics, metrics
- Template context variables:
  - `{{ site }}`, `{{ page_info }}`, `{{ content }}`, `{{ data }}`
  - `{{ meta }}`, `{{ assets }}`, `{{ helpers }}`, `{{ locale }}`, `{{ theme }}`
- COLABS site module as reference implementation
- CLI usage: `go run cmd/static/main.go build --output ./dist`
- Integration with ci/cd pipelines

**Primary Audience**: Full-stack developers
**Complexity**: Intermediate

---

## 10. GUIDE_MARKDOWN.md

**Status**: `review`

**Purpose**: Importing and syncing markdown content with frontmatter support.

**Sections**:
- Markdown architecture overview
- Enabling markdown: `cfg.Features.Markdown = true`
- Configuration: `cfg.Markdown.BasePath`, locales, pattern, recursive, processShortcodes
- Loading documents:
  - `Load` a single file with `LoadOptions`
  - `LoadDirectory` for batch loading
  - YAML frontmatter parsing
- Rendering:
  - `Render` raw markdown bytes
  - `RenderDocument` from parsed document
  - Goldmark pipeline configuration
- Import workflow:
  - `Import` a single document with `ImportOptions`
  - `ImportDirectory` for batch import
  - `ImportResult` structure
- Sync workflow:
  - `Sync` a directory with `SyncOptions`
  - `SyncResult` with created/updated/skipped counts
- Shortcode integration when `cfg.Markdown.ProcessShortcodes = true`
- CLI usage:
  - `go run cmd/markdown/import/main.go --path ./content/en/about.md`
  - `go run cmd/markdown/sync/main.go --dir ./content`

**Primary Audience**: Content authors / backend developers
**Complexity**: Beginner

---

## 11. GUIDE_WORKFLOW.md

**Status**: `review`

**Purpose**: Content lifecycle orchestration with state machines.

**Sections**:
- Workflow architecture overview
- Configuration:
  - `cfg.Workflow.Provider` (simple, custom)
  - `cfg.Workflow.Definitions` with states and transitions
- Workflow states and transitions:
  - `WorkflowStateConfig` and `WorkflowTransitionConfig`
  - Default lifecycle: draft, review, published, archived
- WorkflowEngine interface:
  - Overriding with `di.WithWorkflowEngine(engine)`
  - Custom engine integration via adapter module (`internal/workflow/adapter/`)
- Running workflow tests:
  - `./taskfile workflow:test`
  - Custom engine: `CMS_WORKFLOW_PROVIDER=custom CMS_WORKFLOW_ENGINE_ADDR=http://localhost:8080`
- Integration with content/page status transitions

**Primary Audience**: Backend developers
**Complexity**: Advanced

---

## 12. GUIDE_SHORTCODES.md

**Status**: `review`

**Purpose**: Registering and processing shortcodes in markdown and templates.

**Sections**:
- Shortcode architecture overview
- Enabling shortcodes: `cfg.Features.Shortcodes = true` and `cfg.Shortcodes.Enabled = true`
- Configuration:
  - Built-in shortcodes: youtube, alert, gallery, figure, code
  - WordPress syntax toggle: `cfg.Shortcodes.EnableWordPressSyntax`
  - Security settings: maxNestingDepth, maxExecutionTime, sanitizeOutput
- Service API:
  - `Process` content string with `ShortcodeProcessOptions`
  - `Render` a single shortcode with params and inner content
  - `Registry()` for shortcode registration
- Shortcode caching:
  - `cfg.Shortcodes.Cache.Enabled`, provider, TTL
  - `di.WithShortcodeCacheProvider` for named providers
  - Per-shortcode TTL overrides
- Metrics: `di.WithShortcodeMetrics` for render telemetry
- Integration with markdown processing: `cfg.Markdown.ProcessShortcodes = true`
- Writing custom shortcodes

**Primary Audience**: Backend developers
**Complexity**: Intermediate

---

## 13. GUIDE_ACTIVITY.md

**Status**: `review`

**Purpose**: Activity event emission, audit logging, and hook integration.

**Sections**:
- Activity architecture overview
- Enabling activity: `cfg.Features.Activity = true` and `cfg.Activity.Enabled = true`
- Configuration: `cfg.Activity.Channel` for event filtering
- Activity event structure: verb, actor IDs, object type/ID, channel, module metadata
- Module-specific metadata: slug, status, locale, path, menu code
- Hook injection:
  - `di.WithActivityHooks` for custom hooks
  - `di.WithActivitySink` for go-users integration
- No-op behavior when hooks are not provided
- Testing activities:
  - `activity.CaptureHook` with `activity.NewEmitter`
  - Asserting events without persistence
- Reference: `docs/ACTIVITY_TDD.md` for technical design

**Primary Audience**: Backend developers
**Complexity**: Intermediate

---

## 14. GUIDE_CONFIGURATION.md

**Status**: `review`

**Purpose**: Configuring the CMS and wiring dependencies via the DI container.

**Sections**:
- Configuration architecture overview
- `cms.DefaultConfig()` defaults and structure
- Config sections:
  - `ContentConfig`, `I18NConfig`, `StorageConfig`, `CacheConfig`
  - `MenusConfig`, `NavigationConfig`, `ThemeConfig`, `WidgetConfig`
  - `ShortcodeConfig`, `MarkdownConfig`, `GeneratorConfig`
  - `LoggingConfig`, `WorkflowConfig`, `ActivityConfig`, `EnvironmentsConfig`
- Feature flags: `cfg.Features.*` and their interdependencies
- DI container (`di.NewContainer`):
  - `di.WithBunDB()` for persistent storage
  - `di.WithCache()`, `di.WithStorage()`, `di.WithTemplateRenderer()`
  - `di.WithWorkflowEngine()`, `di.WithLoggerProvider()`
  - `di.WithActivityHooks()`, `di.WithActivitySink()`
  - `di.WithShortcodeCacheProvider()`, `di.WithShortcodeMetrics()`
- Override order: `WithBunDB()` first, then other overrides
- Lazy initialization and proxy pattern
- Module facade: `cms.New(cfg, opts...)` and accessor methods
- Storage admin: runtime profile switching via `module.StorageAdmin()`
- Environment configuration when `cfg.Features.Environments = true`

**Primary Audience**: Backend developers
**Complexity**: Intermediate

---

## 15. GUIDE_REPOSITORIES.md

**Status**: `review`

**Purpose**: Understanding and customizing the repository layer and storage backends.

**Sections**:
- Repository architecture overview
- Repository interfaces per module:
  - Content, Pages, Blocks, Widgets, Menus, Themes, Media repositories
- Built-in implementations:
  - Memory repositories (in-memory maps, no DB)
  - Bun repositories (uptrace/bun ORM, SQL-backed)
- Storage proxies (`internal/di/storage_proxies.go`):
  - Runtime backend switching
  - Automatic delegation to active backend
- Storage profiles:
  - `cfg.Storage.Profiles` and `cfg.Storage.Aliases`
  - `StorageAdmin.ListProfiles`, `PreviewProfile`, `ApplyConfig`
- Transaction strategy:
  - Repository-level `db.RunInTx()` for complex operations
  - `Create()` uses `CreateTx()` to fail fast on duplicates
- Error handling: domain-specific error types (`NotFoundError`)
- Repository caching when `cfg.Cache.Enabled = true`:
  - go-repository-cache integration
  - TTL configuration via `cfg.Cache.DefaultTTL`

**Primary Audience**: Platform engineers
**Complexity**: Advanced

---

## 16. GUIDE_MIGRATIONS.md

**Status**: `review`

**Purpose**: Managing database migrations for go-cms.

**Sections**:
- Migration architecture overview
- Migration files in `data/sql/migrations/`
- Dual dialect support: PostgreSQL (`*_pg.up.sql`) and SQLite (`*_sqlite.up.sql`)
- Embedded migrations via `cms.GetMigrationsFS()`
- Registering migrations with Bun
- Schema overview:
  - Content and content types tables
  - Pages and page hierarchy
  - Blocks: definitions, instances, translations
  - Widgets: definitions, instances, areas
  - Menus: menus, items, translations, locations
  - I18N settings and translation grouping
  - Environments
- Adding custom migrations
- Migration ordering and naming conventions
- Testing with in-memory SQLite (`file::memory:?cache=shared`)

**Primary Audience**: DevOps / DBAs
**Complexity**: Intermediate

---

## 17. GUIDE_TESTING.md

**Status**: `review`

**Purpose**: Testing strategies for applications using go-cms.

**Sections**:
- Testing architecture overview
- Test types and naming conventions:
  - `*_test.go` - Unit tests with memory repositories
  - `*_contract_test.go` - Contract tests for both memory and Bun implementations
  - `*_integration_test.go` - Integration tests with BunDB
  - `*_storage_integration_test.go` - Storage profile switching tests
- Test utilities:
  - `pkg/testsupport` fixtures and database setup helpers
  - `internal/di/testing` for generator-specific helpers
- Unit testing with memory repositories (fast, no DB)
- Contract testing pattern: verifying interface compliance across implementations
- Integration testing with SQLite:
  - `t.TempDir()` for test databases
  - In-memory SQLite: `file::memory:?cache=shared`
  - `t.Cleanup()` for teardown
- Testing activity emission: `activity.CaptureHook` + `activity.NewEmitter`
- Running tests:
  - `go test ./...` (all tests)
  - `go test ./internal/content/... -run TestServiceCreate` (single test)
  - `./taskfile dev:test race` (with race detection)
  - `./taskfile dev:cover` (with coverage)
- Generator regression tests: `./taskfile dev:test:regression`
- COLABS site regression tests: `./taskfile colabs:verify:regressions`

**Primary Audience**: Developers
**Complexity**: Intermediate

---

## Summary

| Guide | Audience | Complexity | Status |
|-------|----------|------------|--------|
| GUIDE_GETTING_STARTED | New users | Beginner | `review` |
| GUIDE_CONTENT | Backend developers | Intermediate | `review` |
| GUIDE_PAGES | Backend developers | Intermediate | `review` |
| GUIDE_BLOCKS | Backend developers | Intermediate | `review` |
| GUIDE_WIDGETS | Backend developers | Intermediate | `review` |
| GUIDE_MENUS | Backend developers | Intermediate | `review` |
| GUIDE_I18N | Backend developers | Intermediate | `review` |
| GUIDE_THEMES | Frontend/full-stack developers | Intermediate | `review` |
| GUIDE_STATIC_GENERATION | Full-stack developers | Intermediate | `review` |
| GUIDE_MARKDOWN | Content authors / backend devs | Beginner | `review` |
| GUIDE_WORKFLOW | Backend developers | Advanced | `review` |
| GUIDE_SHORTCODES | Backend developers | Intermediate | `review` |
| GUIDE_ACTIVITY | Backend developers | Intermediate | `review` |
| GUIDE_CONFIGURATION | Backend developers | Intermediate | `review` |
| GUIDE_REPOSITORIES | Platform engineers | Advanced | `review` |
| GUIDE_MIGRATIONS | DevOps / DBAs | Intermediate | `review` |
| GUIDE_TESTING | Developers | Intermediate | `review` |

---

## Suggested Priority Order

1. **GUIDE_GETTING_STARTED** - Essential for onboarding new users
2. **GUIDE_CONTENT** - Core content management, most users need this first
3. **GUIDE_PAGES** - Second most common use case after content
4. **GUIDE_CONFIGURATION** - Required to wire anything beyond defaults
5. **GUIDE_I18N** - Locale-first design means translations come up early
6. **GUIDE_BLOCKS** - Reusable content is a key differentiator
7. **GUIDE_MENUS** - Navigation is needed for most sites
8. **GUIDE_THEMES** - Required for rendering output
9. **GUIDE_STATIC_GENERATION** - Primary output mechanism
10. **GUIDE_MARKDOWN** - Common authoring workflow
11. **GUIDE_WIDGETS** - Optional but powerful feature
12. **GUIDE_WORKFLOW** - Advanced content lifecycle
13. **GUIDE_SHORTCODES** - Markdown enhancement feature
14. **GUIDE_ACTIVITY** - Audit and observability
15. **GUIDE_REPOSITORIES** - Advanced customization
16. **GUIDE_MIGRATIONS** - Database setup reference
17. **GUIDE_TESTING** - Quality assurance patterns

---

## Notes

- Guides follow the structure established in `go-users/docs/GUIDE_*.md` and `go-formgen/docs/GUIDE_*.md`
- Each guide should include runnable code examples
- Reference existing internal docs (`docs/CMS_TDD.md`, `docs/BLOCK_TDD.md`, `docs/MENU_TDD.md`, `docs/I18N_TDD.md`, `docs/ACTIVITY_TDD.md`) for technical design details
- Prioritize GUIDE_GETTING_STARTED first to help new users onboard quickly
- Code examples should use in-memory repositories where appropriate (no DB setup required)
- See `cmd/example/main.go` for a comprehensive usage example that can inform guide content
- The COLABS site module (`site/`) serves as a real-world integration reference for static generation guides
