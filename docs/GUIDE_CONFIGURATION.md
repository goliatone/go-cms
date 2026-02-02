# Configuration Guide

This guide covers configuring the CMS module and wiring dependencies through the dependency injection container. By the end you will understand every configuration section, how feature flags interact, and how to override default implementations with `di.With*` options.

## Configuration Architecture Overview

`go-cms` uses a single `Config` struct that drives all runtime behaviour. The struct is defined in `internal/runtimeconfig/config.go` and re-exported through the public `config.go` so consumers never import internal packages directly.

The configuration flows through three stages:

```
DefaultConfig()  -->  Host overrides  -->  DI container
     |                     |                    |
  Sensible defaults   Enable features,    Validate, wire repos,
  for quick start     set locales, etc.   create services
```

1. `cms.DefaultConfig()` returns a config with production-safe defaults.
2. The host application overrides specific fields to match its requirements.
3. `cms.New(cfg, opts...)` passes the config to `di.NewContainer`, which validates it, creates repositories, applies option overrides, and initializes services.

All config types are re-exported as type aliases through `config.go`:

```go
type Config            = runtimeconfig.Config
type ContentConfig     = runtimeconfig.ContentConfig
type I18NConfig        = runtimeconfig.I18NConfig
type StorageConfig     = runtimeconfig.StorageConfig
type CacheConfig       = runtimeconfig.CacheConfig
type MenusConfig       = runtimeconfig.MenusConfig
type NavigationConfig  = runtimeconfig.NavigationConfig
type ThemeConfig       = runtimeconfig.ThemeConfig
// ... and so on for every sub-config type
```

This means you always import `github.com/goliatone/go-cms` and access types like `cms.Config`, `cms.Features`, etc.

---

## DefaultConfig Defaults and Structure

`cms.DefaultConfig()` returns an opinionated starting point:

```go
cfg := cms.DefaultConfig()
```

The defaults are:

| Section | Field | Default |
|---------|-------|---------|
| Top-level | `Enabled` | `true` |
| Top-level | `DefaultLocale` | `"en"` |
| Content | `PageHierarchy` | `true` |
| I18N | `Enabled` | `true` |
| I18N | `Locales` | `["en"]` |
| I18N | `RequireTranslations` | `true` |
| I18N | `DefaultLocaleRequired` | `true` |
| Menus | `AllowOutOfOrderUpserts` | `true` |
| Storage | `Provider` | `"bun"` |
| Cache | `Enabled` | `true` |
| Cache | `DefaultTTL` | `1 minute` |
| Themes | `BasePath` | `"themes"` |
| Markdown | `ContentDir` | `"content"` |
| Markdown | `Pattern` | `"*.md"` |
| Markdown | `Recursive` | `true` |
| Generator | `OutputDir` | `"dist"` |
| Generator | `CleanBuild` | `true` |
| Generator | `CopyAssets` | `true` |
| Generator | `GenerateSitemap` | `true` |
| Logging | `Provider` | `"console"` |
| Logging | `Level` | `"info"` |
| Workflow | `Enabled` | `true` |
| Workflow | `Provider` | `"simple"` |
| Activity | `Enabled` | `false` |
| Activity | `Channel` | `"cms"` |
| Features | all flags | `false` |

All feature flags default to `false`. This means that out of the box, only the core content, pages, blocks, and menus subsystems are active. Enable additional features explicitly.

---

## Config Sections

### ContentConfig

Controls core content module behaviour.

```go
type ContentConfig struct {
    PageHierarchy bool  // Enable page hierarchy support (default: true)
}
```

When `PageHierarchy` is true, the page service resolves parent-child relationships and builds routing paths from the page tree.

### I18NConfig

Controls internationalization and translation enforcement.

```go
type I18NConfig struct {
    Enabled               bool      // Enable i18n subsystem (default: true)
    Locales               []string  // Valid locale codes (default: ["en"])
    RequireTranslations   bool      // Enforce at least one translation (default: true)
    DefaultLocaleRequired bool      // Validate default locale exists (default: true)
}
```

**Monolingual setup:** set `RequireTranslations` to `false` and provide a single locale:

```go
cfg.I18N.RequireTranslations = false
cfg.I18N.Locales = []string{"en"}
```

**Multi-locale setup:** list all supported locales:

```go
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es", "fr"}
```

Every `Create*Request` and `Update*Request` accepts an `AllowMissingTranslations` field that bypasses translation enforcement per-request, even when the global requirement is `true`. This is useful for staging workflows where content is published before all translations are ready.

### StorageConfig

Defines the storage provider and optional profiles for runtime switching.

```go
type StorageConfig struct {
    Provider string              // Default storage provider (default: "bun")
    Profiles []storage.Profile   // Named storage profiles
    Aliases  map[string]string   // Maps alias names to profile names
}
```

Without `di.WithBunDB()`, the container uses in-memory repositories regardless of this setting. When BunDB is provided, repositories switch to SQL-backed implementations.

Storage profiles are covered in detail in the **Storage Admin** section below.

### CacheConfig

Controls repository-level caching.

```go
type CacheConfig struct {
    Enabled    bool            // Enable cache layer (default: true)
    DefaultTTL time.Duration   // Cache entry TTL (default: 1 minute)
}
```

When enabled, repositories wrap operations with `go-repository-cache`. Override the cache implementation with `di.WithCache()`.

Note: `Features.AdvancedCache` must also be `true` for repository-level caching to take effect. The `Cache.Enabled` field alone enables the cache infrastructure but the feature flag gates actual use.

### MenusConfig

Controls menu write semantics.

```go
type MenusConfig struct {
    AllowOutOfOrderUpserts bool  // Defer missing parents during bootstrap (default: true)
}
```

When `true`, menu bootstraps can reference parent items that do not yet exist. The container defers unresolved parents and reconciles them after all writes complete. This is essential for declarative menu seeding where item order is not guaranteed.

### NavigationConfig

Configures URL resolution for menus via `go-urlkit`.

```go
type NavigationConfig struct {
    RouteConfig *urlkit.Config         // urlkit route definitions
    URLKit      URLKitResolverConfig   // Resolver behaviour
}

type URLKitResolverConfig struct {
    DefaultGroup  string             // Base route group (e.g., "frontend")
    LocaleGroups  map[string]string  // Locale-to-route-group mapping
    DefaultRoute  string             // Fallback route name for page links
    SlugParam     string
    LocaleParam   string
    LocaleIDParam string
    RouteField    string
    ParamsField   string
    QueryField    string
}
```

Example with multi-locale routing:

```go
cfg.Navigation.RouteConfig = &urlkit.Config{
    Groups: []urlkit.GroupConfig{
        {
            Name:    "frontend",
            BaseURL: "https://example.com",
            Paths: map[string]string{
                "page": "/pages/:slug",
            },
            Groups: []urlkit.GroupConfig{
                {
                    Name: "es",
                    Path: "/es",
                    Paths: map[string]string{
                        "page": "/paginas/:slug",
                    },
                },
            },
        },
    },
}
cfg.Navigation.URLKit.DefaultGroup = "frontend"
cfg.Navigation.URLKit.LocaleGroups = map[string]string{
    "es": "es",
}
cfg.Navigation.URLKit.DefaultRoute = "page"
```

### ThemeConfig

Controls the theme subsystem. Requires `Features.Themes = true`.

```go
type ThemeConfig struct {
    BasePath          string             // Directory containing themes (default: "themes")
    DefaultTheme      string             // Active theme name
    DefaultVariant    string             // Theme variant (e.g., "light", "dark")
    PartialFallbacks  map[string]string  // Fallback partial templates
    CSSVariablePrefix string             // CSS variable namespace
}
```

Setting `DefaultTheme` or `DefaultVariant` without enabling `Features.Themes` causes a validation error.

### WidgetConfig

Bootstraps widget definitions from configuration. Requires `Features.Widgets = true`.

```go
type WidgetConfig struct {
    Definitions []WidgetDefinitionConfig
}

type WidgetDefinitionConfig struct {
    Name        string
    Description string
    Schema      map[string]any  // JSON schema for widget properties
    Defaults    map[string]any  // Default property values
    Category    string
    Icon        string
}
```

`DefaultConfig()` ships with a `newsletter_signup` widget definition. Add your own definitions to the list or replace it entirely.

### RetentionConfig

Sets per-module version retention limits when `Features.Versioning` is enabled.

```go
type RetentionConfig struct {
    Content int  // Max versions for content entries (0 = unlimited)
    Pages   int  // Max versions for pages
    Blocks  int  // Max versions for blocks
}
```

### ShortcodeConfig

Controls shortcode processing. Requires `Features.Shortcodes = true`.

```go
type ShortcodeConfig struct {
    Enabled               bool
    EnableWordPressSyntax bool
    BuiltIns              []string  // Default: ["youtube", "alert", "gallery", "figure", "code"]
    CustomDefinitions     []ShortcodeDefinitionConfig
    Security              ShortcodeSecurityConfig
    Cache                 ShortcodeCacheConfig
}
```

Security defaults: max nesting depth 5, max execution time 5s, output sanitization on.

Cache defaults: disabled, 1 hour TTL when enabled. Per-shortcode TTL overrides are supported via `Cache.PerShortcode`.

### MarkdownConfig

Controls markdown import and sync. Requires `Features.Markdown = true`.

```go
type MarkdownConfig struct {
    Enabled           bool
    ContentDir        string             // Base directory (default: "content")
    Pattern           string             // File glob (default: "*.md")
    Recursive         bool               // Recurse into subdirectories (default: true)
    LocalePatterns    map[string]string  // Per-locale patterns
    DefaultLocale     string
    Locales           []string
    Parser            MarkdownParserConfig
    ProcessShortcodes bool               // Render shortcodes in markdown
}
```

Setting `Enabled` to `true` without `Features.Markdown` causes a validation error. The `ContentDir` must be non-empty when enabled.

### GeneratorConfig

Controls the static site generator.

```go
type GeneratorConfig struct {
    Enabled          bool
    OutputDir        string         // Output directory (default: "dist")
    BaseURL          string         // Site base URL for absolute links
    CleanBuild       bool           // Remove output before build (default: true)
    Incremental      bool           // Only regenerate changed entities
    CopyAssets       bool           // Copy theme/media assets (default: true)
    GenerateSitemap  bool           // Generate sitemap.xml (default: true)
    GenerateRobots   bool           // Generate robots.txt
    GenerateFeeds    bool           // Generate RSS/Atom feeds
    Workers          int            // Parallel render workers (0 = auto)
    Menus            map[string]string  // Menus to include (code -> location)
    RenderTimeout    time.Duration  // Timeout per page render
    AssetCopyTimeout time.Duration  // Timeout for asset copy
}
```

### LoggingConfig

Controls structured logging. Requires `Features.Logger = true`.

```go
type LoggingConfig struct {
    Provider  string    // "console" (default) or "gologger"
    Level     string    // "trace", "debug", "info", "warn", "error", "fatal"
    Format    string    // "json", "console", "pretty" (gologger only)
    AddSource bool      // Include file/line in log output
    Focus     []string  // Filter logs to specific module names
}
```

### WorkflowConfig

Controls the content lifecycle state machine.

```go
type WorkflowConfig struct {
    Enabled     bool    // Enable workflows (default: true)
    Provider    string  // "simple" (default), "custom", or ""
    Definitions []WorkflowDefinitionConfig
}
```

The `"simple"` provider uses the built-in state machine. Set `"custom"` and provide `di.WithWorkflowEngine()` to use an external engine.

### ActivityConfig

Controls activity event emission. Requires `Features.Activity = true`.

```go
type ActivityConfig struct {
    Enabled bool    // Enable emissions (default: false)
    Channel string  // Default channel tag (default: "cms")
}
```

Both `Features.Activity` and `Activity.Enabled` must be `true` for events to fire.

### EnvironmentsConfig

Controls multi-environment content scoping. Requires `Features.Environments = true`.

```go
type EnvironmentsConfig struct {
    DefaultKey         string  // Default environment key
    RequireExplicit    bool    // Require environment in requests
    RequireActive      bool    // Only allow active environments
    EnforceDefault     bool    // Auto-create default environment
    PermissionScoped   bool    // Enable permission scoping
    PermissionStrategy string  // "env_first", "global_first", "custom"
    Definitions        []EnvironmentConfig
}

type EnvironmentConfig struct {
    Key         string
    Name        string
    Description string
    Default     bool
    Disabled    bool
}
```

Environment keys must match `^[a-z0-9_-]+$`. When multiple definitions are provided, exactly one must be marked as the default.

---

## Feature Flags and Their Interdependencies

All feature flags live in `cfg.Features` and default to `false`:

```go
type Features struct {
    Widgets       bool  // Widget subsystem
    Themes        bool  // Theme management
    Versioning    bool  // Content/page/block versioning
    Scheduling    bool  // Publish scheduling
    MediaLibrary  bool  // Media library features
    AdvancedCache bool  // Repository-level caching
    Markdown      bool  // Markdown import/sync
    Logger        bool  // Structured logging
    Shortcodes    bool  // Shortcode processing
    Activity      bool  // Activity event emission
    Environments  bool  // Environment configuration
}
```

### Interdependency Rules

The container validates these rules at initialization:

| Flag | Requires | Error |
|------|----------|-------|
| `Scheduling` | `Versioning` | `ErrSchedulingFeatureRequiresVersioning` |
| `AdvancedCache` | `Cache.Enabled` | `ErrAdvancedCacheRequiresEnabledCache` |

### Configuration Guard Rules

Setting configuration values for a disabled feature causes a validation error:

| Config Section | Requires Feature | Error |
|----------------|-----------------|-------|
| `Themes.DefaultTheme` (non-empty) | `Features.Themes` | `ErrThemesFeatureRequired` |
| `Shortcodes.Enabled` | `Features.Shortcodes` | `ErrShortcodesFeatureRequired` |
| `Markdown.Enabled` | `Features.Markdown` | `ErrMarkdownFeatureRequired` |
| `Activity.Enabled` | `Features.Activity` | `ErrActivityFeatureRequired` |
| `Environments.Definitions` (non-empty) | `Features.Environments` | `ErrEnvironmentsFeatureRequired` |

### No-op Behaviour

When a feature is disabled, its service returns a no-op implementation. Calling methods on a no-op service is safe -- it will return empty results or nil errors. Do not rely on disabled features producing real data.

### Common Feature Combinations

**Minimal CMS** (content and pages only):

```go
cfg := cms.DefaultConfig()
// All features default to false -- nothing else needed
```

**Full-featured site**:

```go
cfg := cms.DefaultConfig()
cfg.Features.Widgets = true
cfg.Features.Themes = true
cfg.Features.Versioning = true
cfg.Features.Scheduling = true   // Requires Versioning
cfg.Features.MediaLibrary = true
cfg.Features.AdvancedCache = true // Requires Cache.Enabled (already true by default)
cfg.Features.Markdown = true
cfg.Features.Logger = true
cfg.Features.Shortcodes = true
cfg.Features.Activity = true
cfg.Features.Environments = true
```

**Static site generator**:

```go
cfg := cms.DefaultConfig()
cfg.Features.Themes = true
cfg.Features.Markdown = true
cfg.Themes.DefaultTheme = "aurora"
cfg.Generator.Enabled = true
cfg.Generator.OutputDir = "./dist"
cfg.Generator.BaseURL = "https://example.com"
cfg.Markdown.Enabled = true
cfg.Markdown.ContentDir = "./content"
```

---

## DI Container

The `di.Container` wires all dependencies based on the provided config and options. It is created internally by `cms.New()` or directly via `di.NewContainer()`.

### Container Lifecycle

```go
func NewContainer(cfg Config, opts ...Option) (*Container, error)
```

The container goes through these stages:

1. **Validate** -- calls `cfg.Validate()` to catch configuration errors early.
2. **Create memory repositories** -- initializes in-memory implementations for every module.
3. **Wrap in proxies** -- wraps each repository in a thread-safe proxy that allows runtime swapping.
4. **Apply options** -- runs each `di.With*` option to inject overrides.
5. **Initialize subsystems** -- configures translation settings, logger, activity emitter, storage backend, environments, locales, navigation, and scheduler.
6. **Create services** -- builds service instances from configured repositories and settings.

### Repository Proxies

Each repository is wrapped in a proxy that enables runtime storage switching:

```
Container creates:
  MemoryContentRepository
        |
        v
  contentRepositoryProxy  <-- services read/write through this
        |
        v
  (swappable: memory or Bun-backed repo)
```

When `di.WithBunDB(db)` is provided, the container swaps the memory repository for a Bun-backed implementation through the proxy. All existing service references continue to work without re-initialization.

### Accessing the Container

Most users interact through the `Module` facade (see below). For advanced use:

```go
container := module.Container()
contentSvc := container.ContentService()
```

---

## DI Options (di.With*)

All options follow the functional option pattern:

```go
type Option func(*Container)
```

Pass them to `cms.New()` or `di.NewContainer()`:

```go
module, err := cms.New(cfg,
    di.WithBunDB(db),
    di.WithLoggerProvider(logger),
    di.WithActivityHooks(hooks),
)
```

### Storage and Database

```go
di.WithBunDB(db *bun.DB)
```
Provide a pre-configured Bun database connection. This switches all repositories from in-memory to SQL-backed. **Call this first** -- other options may depend on having a database available.

```go
di.WithStorage(sp interfaces.StorageProvider)
```
Override the file/object storage provider.

```go
di.WithStorageRepository(repo storageconfig.Repository)
```
Override the storage profile repository.

```go
di.WithStorageFactory(kind string, factory StorageFactory)
```
Register a storage provider factory for a specific backend kind (e.g., `"s3"`).

### Cache

```go
di.WithCache(service repocache.CacheService, serializer repocache.KeySerializer)
```
Override the cache service and key serializer used by repository caching.

```go
di.WithCacheProvider(provider interfaces.CacheProvider)
```
Override the cache provider interface.

### Service Overrides

Replace any service with a custom implementation:

```go
di.WithContentService(svc content.Service)
di.WithPageService(svc pages.Service)
di.WithBlockService(svc blocks.Service)
di.WithMenuService(svc menus.Service)
di.WithWidgetService(svc widgets.Service)
di.WithThemeService(svc themes.Service)
di.WithI18nService(svc i18n.Service)
di.WithEnvironmentService(svc environments.Service)
di.WithContentTypeService(svc content.ContentTypeService)
```

### Template and Media

```go
di.WithTemplate(tr interfaces.TemplateRenderer)
```
Override the template renderer used by the generator.

```go
di.WithMedia(mp interfaces.MediaProvider)
```
Override the media file provider.

### Workflow and Logging

```go
di.WithWorkflowEngine(engine interfaces.WorkflowEngine)
```
Override the workflow state machine. Use with `cfg.Workflow.Provider = "custom"`.

```go
di.WithWorkflowDefinitionStore(store interfaces.WorkflowDefinitionStore)
```
Register an external workflow definition source.

```go
di.WithLoggerProvider(provider interfaces.LoggerProvider)
```
Override the logger provider. Requires `Features.Logger = true`.

### Generator

```go
di.WithGeneratorOutput(output string)
di.WithGeneratorStorage(sp interfaces.StorageProvider)
di.WithGeneratorAssetResolver(resolver generator.AssetResolver)
di.WithGeneratorHooks(hooks generator.Hooks)
```

### Activity and Shortcodes

```go
di.WithActivityHooks(hooks activity.Hooks)
```
Register activity emission hooks. Requires `Features.Activity = true` and `Activity.Enabled = true`.

```go
di.WithActivitySink(sink interfaces.ActivitySink)
```
Use a `go-users` sink for activity emission.

```go
di.WithShortcodeCacheProvider(name string, provider interfaces.CacheProvider)
```
Register a named cache provider for shortcode output. Pass `name=""` for the default provider.

```go
di.WithShortcodeMetrics(metrics interfaces.ShortcodeMetrics)
```
Register a metrics recorder for shortcode render telemetry.

### Miscellaneous

```go
di.WithAuth(ap interfaces.AuthService)
di.WithScheduler(s interfaces.Scheduler)
di.WithMarkdownService(svc interfaces.MarkdownService)
di.WithSlugNormalizer(normalizer slug.Normalizer)
di.WithAuditRecorder(recorder jobs.AuditRecorder)
di.WithEnvironmentPermissionStrategy(strategy permissions.Strategy)
```

### Override Order

Some options have ordering requirements:

1. **`di.WithBunDB()`** should come first. It initializes the database layer that other options may depend on.
2. **Service overrides** (e.g., `di.WithContentService()`) can come in any order after the database option.
3. **Feature-gated options** (e.g., `di.WithActivityHooks()`) require the corresponding feature flag to be enabled in the config.

Example with proper ordering:

```go
module, err := cms.New(cfg,
    // 1. Database first
    di.WithBunDB(db),
    // 2. Infrastructure overrides
    di.WithCache(cacheService, keySerializer),
    di.WithStorage(s3Provider),
    di.WithLoggerProvider(zapLogger),
    // 3. Feature-specific overrides
    di.WithActivityHooks(activity.Hooks{auditHook}),
    di.WithWorkflowEngine(customEngine),
    di.WithTemplate(goTemplateRenderer),
)
```

---

## Lazy Initialization and Proxy Pattern

The container uses two patterns to manage complexity:

### Lazy Initialization

Some services are created on first access rather than at container construction time. This avoids circular dependencies and defers work for features that may never be used:

```go
// Shortcode service is initialized on first call
shortcodeSvc := module.Shortcodes()
```

If you never call `module.Shortcodes()`, the shortcode service is never created.

### Repository Proxies

Every repository is fronted by a thread-safe proxy:

```go
type contentRepositoryProxy struct {
    mu   sync.RWMutex
    repo content.ContentRepository
}

func (p *contentRepositoryProxy) swap(repo content.ContentRepository) {
    p.mu.Lock()
    defer p.mu.Unlock()
    if repo != nil {
        p.repo = repo
    }
}
```

All service methods read through the proxy using a read lock. The proxy's `swap()` method takes a write lock to atomically replace the underlying repository. This enables:

- **Zero-downtime storage switching** -- swap the Bun database without restarting.
- **In-memory to SQL migration** -- start with memory repos, add BunDB later.
- **Storage profile changes** -- switch between database backends at runtime.

---

## Module Facade

The `cms.Module` struct wraps the DI container and provides a clean public API:

```go
module, err := cms.New(cfg, opts...)
if err != nil {
    log.Fatal(err)
}
```

### Service Accessors

```go
contentSvc   := module.Content()         // Content and content types
pageSvc      := module.Pages()           // Page hierarchy and routing
blockSvc     := module.Blocks()          // Block definitions and instances
menuSvc      := module.Menus()           // Menus and navigation resolution
widgetSvc    := module.Widgets()         // Widget registry and areas
themeSvc     := module.Themes()          // Theme management
mediaSvc     := module.Media()           // Media file handling
generatorSvc := module.Generator()       // Static site generation
markdownSvc  := module.Markdown()        // Markdown import/sync
shortcodeSvc := module.Shortcodes()      // Shortcode processing
scheduler    := module.Scheduler()       // Job scheduling
workflowEng  := module.WorkflowEngine()  // Workflow state machine
```

### Admin Services

```go
storageAdmin  := module.StorageAdmin()       // Runtime storage profile management
translationAd := module.TranslationAdmin()   // Translation settings admin
blockAdmin    := module.BlocksAdmin()        // Embedded blocks admin
```

### Utility Methods

```go
module.TranslationsEnabled()   // Whether i18n is globally enabled
module.TranslationsRequired()  // Whether translations are enforced
module.Container()             // Access the underlying DI container (advanced)
```

The module re-exports all service types so consumers never import `internal/` packages:

```go
type ContentService         = content.Service
type PageService            = pages.Service
type BlockService           = blocks.Service
type StorageAdminService    = *adminstorage.Service
type TranslationAdminService = *admintranslations.Service
type BlockAdminService      = *adminblocks.Service
```

---

## Storage Admin: Runtime Profile Switching

The storage admin service enables zero-downtime database migrations and multi-backend configurations.

### Configuring Profiles

Define storage profiles in the config:

```go
cfg.Storage.Profiles = []storage.Profile{
    {
        Name:     "primary",
        Provider: "bun",
        Default:  true,
        Config: storage.ProfileConfig{
            Name:   "primary",
            Driver: "postgres",
            DSN:    "postgres://user:pass@localhost:5432/cms",
        },
    },
    {
        Name:     "replica",
        Provider: "bun",
        Config: storage.ProfileConfig{
            Name:   "replica",
            Driver: "postgres",
            DSN:    "postgres://user:pass@replica:5432/cms",
        },
        Fallbacks: []string{"primary"},
    },
}

cfg.Storage.Aliases = map[string]string{
    "read-only": "replica",
}
```

Profile names must match `^[a-z0-9_-]+$`. Only one profile can be marked as default. Fallback chains cannot reference themselves.

### Using the Admin Service

```go
storageAdmin := module.StorageAdmin()

// List all configured profiles
profiles, err := storageAdmin.ListProfiles(ctx)

// Preview a profile change without applying
preview, err := storageAdmin.PreviewProfile(ctx, targetProfile)

// Switch the active storage backend
err = storageAdmin.ApplyConfig(ctx, newConfig)
```

When you call `ApplyConfig`, the container:

1. Validates the new profile configuration.
2. Creates a new Bun database connection using the registered factory.
3. Creates new Bun-backed repositories.
4. Swaps all repository proxies atomically.
5. Updates the active profile reference.

All in-flight requests complete against the old backend. New requests use the new backend.

---

## Environment Configuration

When `Features.Environments = true`, the CMS supports multi-environment content scoping.

### Setup

```go
cfg.Features.Environments = true
cfg.Environments = cms.EnvironmentsConfig{
    DefaultKey:      "production",
    RequireExplicit: false,
    RequireActive:   true,
    EnforceDefault:  true,
    Definitions: []cms.EnvironmentConfig{
        {
            Key:         "production",
            Name:        "Production",
            Description: "Live production environment",
            Default:     true,
        },
        {
            Key:         "staging",
            Name:        "Staging",
            Description: "Pre-production testing",
        },
    },
}
```

### Permission Strategies

When `PermissionScoped` is `true`, environment-level permissions are enforced:

```go
cfg.Environments.PermissionScoped = true
cfg.Environments.PermissionStrategy = "env_first"
```

| Strategy | Behaviour |
|----------|-----------|
| `"env_first"` | Check environment-scoped permissions first, fall back to global |
| `"global_first"` | Check global permissions first, fall back to environment-scoped |
| `"custom"` | Use custom resolver via `di.WithEnvironmentPermissionStrategy()` |

---

## Complete Example

This example configures a multi-locale CMS with versioning, themes, widgets, and a PostgreSQL backend:

```go
package main

import (
    "context"
    "database/sql"
    "log"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/di"
    urlkit "github.com/goliatone/go-urlkit"
    "github.com/uptrace/bun"
    "github.com/uptrace/bun/dialect/pgdialect"
    "github.com/uptrace/bun/driver/pgdriver"
)

func main() {
    ctx := context.Background()

    // --- Step 1: Configure the CMS
    cfg := cms.DefaultConfig()
    cfg.DefaultLocale = "en"
    cfg.I18N.Locales = []string{"en", "es"}

    // Enable features
    cfg.Features.Widgets = true
    cfg.Features.Themes = true
    cfg.Features.Versioning = true
    cfg.Features.Scheduling = true
    cfg.Features.Activity = true
    cfg.Features.Logger = true

    // Theme settings
    cfg.Themes.DefaultTheme = "aurora"
    cfg.Themes.DefaultVariant = "light"

    // Activity emission
    cfg.Activity.Enabled = true
    cfg.Activity.Channel = "cms"

    // Navigation routes
    cfg.Navigation.RouteConfig = &urlkit.Config{
        Groups: []urlkit.GroupConfig{
            {
                Name:    "frontend",
                BaseURL: "https://example.com",
                Paths: map[string]string{
                    "page": "/pages/:slug",
                },
                Groups: []urlkit.GroupConfig{
                    {
                        Name: "es",
                        Path: "/es",
                        Paths: map[string]string{
                            "page": "/paginas/:slug",
                        },
                    },
                },
            },
        },
    }
    cfg.Navigation.URLKit.DefaultGroup = "frontend"
    cfg.Navigation.URLKit.LocaleGroups = map[string]string{"es": "es"}

    // --- Step 2: Set up the database
    sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN("postgres://user:pass@localhost:5432/cms?sslmode=disable")))
    db := bun.NewDB(sqldb, pgdialect.New())

    // --- Step 3: Create the module
    module, err := cms.New(cfg,
        di.WithBunDB(db),
    )
    if err != nil {
        log.Fatal(err)
    }

    // --- Step 4: Access services
    contentSvc := module.Content()
    pageSvc := module.Pages()
    menuSvc := module.Menus()
    widgetSvc := module.Widgets()

    log.Println("CMS module ready")
    _ = ctx
    _ = contentSvc
    _ = pageSvc
    _ = menuSvc
    _ = widgetSvc
}
```

---

## Validation Reference

The container validates the config at initialization. These are the validation errors you may encounter:

| Error | Cause |
|-------|-------|
| `ErrDefaultLocaleRequired` | `I18N.RequireTranslations` is true but `DefaultLocale` is empty |
| `ErrThemesFeatureRequired` | Theme config set without `Features.Themes = true` |
| `ErrSchedulingFeatureRequiresVersioning` | `Features.Scheduling` without `Features.Versioning` |
| `ErrAdvancedCacheRequiresEnabledCache` | `Features.AdvancedCache` without `Cache.Enabled` |
| `ErrShortcodesFeatureRequired` | Shortcode config set without `Features.Shortcodes = true` |
| `ErrMarkdownFeatureRequired` | `Markdown.Enabled` without `Features.Markdown = true` |
| `ErrMarkdownContentDirRequired` | `Markdown.Enabled` with empty `ContentDir` |
| `ErrGeneratorOutputDirRequired` | `Generator.Enabled` with empty `OutputDir` |
| `ErrActivityFeatureRequired` | `Activity.Enabled` without `Features.Activity = true` |
| `ErrLoggingProviderRequired` | `Features.Logger` with empty `Logging.Provider` |
| `ErrLoggingProviderUnknown` | Unrecognized logging provider |
| `ErrLoggingLevelInvalid` | Unrecognized logging level |
| `ErrWorkflowProviderUnknown` | Unrecognized workflow provider |
| `ErrWorkflowProviderConfiguredWhenDisabled` | Non-simple workflow provider with `Workflow.Enabled = false` |
| `ErrEnvironmentsFeatureRequired` | Environments config set without `Features.Environments = true` |
| `ErrEnvironmentKeyRequired` | Environment definition with empty key |
| `ErrEnvironmentKeyInvalid` | Environment key not matching `^[a-z0-9_-]+$` |
| `ErrEnvironmentKeyDuplicate` | Duplicate environment key |
| `ErrEnvironmentDefaultRequired` | Multiple definitions without a default |
| `ErrEnvironmentDefaultMultiple` | More than one default environment |
| `ErrStorageProfileNameRequired` | Storage profile with empty name |
| `ErrStorageProfileDuplicateName` | Duplicate storage profile name |
| `ErrStorageProfileMultipleDefaults` | More than one default storage profile |
| `ErrStorageProfileFallbackSelf` | Profile fallback referencing itself |
| `ErrStorageProfileFallbackUnknown` | Fallback referencing non-existent profile |

---

## Next Steps

- **GUIDE_REPOSITORIES.md** -- Understand and customize the repository layer and storage backends.
- **GUIDE_GETTING_STARTED.md** -- If you have not yet built your first module, start here.
- **GUIDE_I18N.md** -- Deep dive into translation workflows and locale management.
- **GUIDE_CONTENT.md** -- Content types, entries, and versioning.
- **GUIDE_THEMES.md** -- Theme management, template registration, and asset resolution.
- See `cmd/example/main.go` for a comprehensive usage example covering all features.
