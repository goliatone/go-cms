# CMS Implementation Reference

This document consolidates the code-level examples and wiring details that support the architectural plan described in `CMS_TDD.md`. Each section mirrors a conceptual area in the TDD and provides the corresponding implementation snippets.

## Module Layout

```
cms/
├── go.mod
├── go.sum
├── cms.go                      # Public API wrapper
├── config.go                   # Configuration types
│
├── cmd/
│   └── example/
│       └── main.go             # Example CLI with DI wiring
│
├── internal/
│   ├── di/
│   │   └── container.go        # Dependency injection container
│   │
│   ├── domain/
│   │   └── types.go            # Core domain types (Status, etc.)
│   │
│   ├── content/
│   │   ├── types.go            # Content-specific types
│   │   ├── service.go          # Interface + implementation
│   │   ├── repository.go       # Storage interface
│   │   └── testdata/           # Fixtures for contract tests
│   │       ├── basic_content.json
│   │       └── basic_content_output.json
│   │
│   ├── pages/
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── testdata/
│   │       ├── hierarchical_pages.json
│   │       └── hierarchical_pages_output.json
│   │
│   ├── blocks/
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── registry.go
│   │   ├── repository.go
│   │   └── testdata/
│   │       ├── nested_blocks.json
│   │       └── nested_blocks_output.json
│   │
│   ├── menus/
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── testdata/
│   │
│   ├── widgets/
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── registry.go
│   │   ├── repository.go
│   │   └── testdata/
│   │
│   ├── themes/
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── testdata/
│   │
│   └── i18n/
│       ├── service.go          # Wraps go-i18n translator/culture services
│       ├── config.go           # Maps CMS config to go-i18n options
│       ├── loader.go           # Optional DB-backed loader plugged into go-i18n
│       ├── template_helpers.go # Augments go-i18n template helpers
│       └── testdata/
│           ├── fallback_chain.json
│           └── fallback_chain_output.json
│
└── pkg/
    ├── interfaces/             # External dependency interfaces
    │   ├── storage.go
    │   ├── cache.go
    │   ├── template.go
    │   ├── media.go
    │   └── auth.go
    │
    └── testsupport/            # Shared test utilities
        ├── fixtures.go         # LoadFixture, LoadGolden, WriteGolden
        └── dbtest.go           # Test database setup helpers
```

> Storage adapters provided with the CMS wrap `github.com/goliatone/go-persistence-bun` for connection/migrations and `github.com/goliatone/go-repository-bun` for repositories so they fulfil the `StorageProvider` interface without coupling domain services to a specific ORM.
>
> Cache decorators provided with the CMS wrap `github.com/goliatone/go-repository-cache`, giving any registered repository transparent read caching while still honouring the `CacheProvider` interface defined here.

## External Dependency Interfaces

### Storage Provider

```go
// pkg/interfaces/storage.go
package interfaces

import "context"

// StorageProvider defines the interface for data persistence
type StorageProvider interface {
    Query(ctx context.Context, query string, args ...any) (Rows, error)
    Exec(ctx context.Context, query string, args ...any) (Result, error)
    Transaction(ctx context.Context, fn func(tx Transaction) error) error
}

// Rows represents query result rows
type Rows interface {
    Next() bool
    Scan(dest ...any) error
    Close() error
}

// Result represents execution result
type Result interface {
    RowsAffected() (int64, error)
    LastInsertId() (int64, error)
}

// Transaction represents a database transaction
type Transaction interface {
    StorageProvider
    Commit() error
    Rollback() error
}
```

Implementations based on `github.com/goliatone/go-persistence-bun` and `github.com/goliatone/go-repository-bun` satisfy `StorageProvider` out of the box; the adapters in `adapters/storage` simply expose those packages through this interface so domain services can remain persistence-agnostic.

### Cache Provider

```go
// pkg/interfaces/cache.go
package interfaces

import (
    "context"
    "time"
)

// CacheProvider defines the interface for caching
type CacheProvider interface {
    Get(ctx context.Context, key string) (any, error)
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
}
```

`github.com/goliatone/go-repository-cache` provides ready-made decorators that implement this interface and can be plugged into the CMS container, ensuring cached repositories stay stampede-safe without leaking implementation details into services.

### Template Renderer

```go
// pkg/interfaces/template.go
package interfaces

import "context"

// TemplateRenderer defines the interface for template rendering
type TemplateRenderer interface {
    Render(ctx context.Context, template string, data any) (string, error)
    RegisterFunction(name string, fn any) error
}
```

### Media Provider

```go
// pkg/interfaces/media.go
package interfaces

import "context"

// MediaProvider defines the interface for media/asset handling
type MediaProvider interface {
    GetURL(ctx context.Context, path string) (string, error)
    GetMetadata(ctx context.Context, id string) (MediaMetadata, error)
}

// MediaMetadata represents media file metadata
type MediaMetadata struct {
    ID       string
    MimeType string
    Size     int64
    Width    int
    Height   int
}
```

### Auth Provider

```go
// pkg/interfaces/auth.go
package interfaces

import "context"

// AuthProvider defines the interface for authentication
type AuthProvider interface {
    GetCurrentUser(ctx context.Context) (User, error)
    HasPermission(ctx context.Context, user User, permission string) bool
}

// User represents an authenticated user
type User struct {
    ID    string
    Email string
    Roles []string
}
```

## Configuration Layer

```go
// config.go
package cms

import (
    "fmt"
    "time"

    "github.com/goliatone/cms/pkg/interfaces"
)

// Config defines what the CMS should use (Layer 1: Configuration)
type Config struct {
    // Required infrastructure
    Storage interfaces.StorageProvider

    // Optional infrastructure (can be nil)
    Cache    interfaces.CacheProvider
    Template interfaces.TemplateRenderer
    Media    interfaces.MediaProvider
    Auth     interfaces.AuthProvider

    // Locale configuration
    DefaultLocale string
    Locales       []Locale

    // Feature flags (progressive complexity)
    Features Features

    // Performance tuning
    CacheTTL      time.Duration
    MaxPageDepth  int
    MaxBlockDepth int
}

// Locale represents a language/region configuration
type Locale struct {
    Code             string
    Name             string
    IsDefault        bool
    FallbackLocaleID string // Optional: for regional fallback
}

// Features controls which CMS features are enabled
type Features struct {
    // Sprint 1 (always enabled)
    BasicContent bool // Always true
    BasicPages   bool // Always true

    // Sprint 2
    Blocks       bool
    NestedBlocks bool

    // Sprint 3
    Menus             bool
    HierarchicalMenus bool

    // Sprint 4
    Widgets bool

    // Sprint 5
    Themes    bool
    Templates bool

    // Sprint 6
    Versioning    bool
    Scheduling    bool
    MediaLibrary  bool
    AdvancedCache bool
}

// Validate ensures config is usable
func (c *Config) Validate() error {
    if c.Storage == nil {
        return fmt.Errorf("storage provider is required")
    }

    if c.DefaultLocale == "" {
        c.DefaultLocale = "en"
    }

    if len(c.Locales) == 0 {
        c.Locales = []Locale{
            {Code: c.DefaultLocale, Name: "English", IsDefault: true},
        }
    }

    if c.CacheTTL == 0 {
        c.CacheTTL = 5 * time.Minute
    }

    if c.MaxPageDepth == 0 {
        c.MaxPageDepth = 5
    }

    if c.MaxBlockDepth == 0 {
        c.MaxBlockDepth = 10
    }

    // Validate feature dependencies
    if c.Features.NestedBlocks && !c.Features.Blocks {
        return fmt.Errorf("NestedBlocks requires Blocks feature")
    }

    if c.Features.HierarchicalMenus && !c.Features.Menus {
        return fmt.Errorf("HierarchicalMenus requires Menus feature")
    }

    if c.Features.Templates && !c.Features.Themes {
        return fmt.Errorf("Templates requires Themes feature")
    }

    return nil
}

// ApplyDefaults sets reasonable defaults for all features
func (c *Config) ApplyDefaults() {
    if c.Features == (Features{}) {
        c.Features = Features{
            BasicContent: true,
            BasicPages:   true,
            // Rest are false by default (progressive complexity)
        }
    }
}
```

## Public API Layer

```go
// cms.go
package cms

import (
    "context"
    "fmt"

    "github.com/goliatone/cms/internal/di"
    "github.com/goliatone/cms/internal/content"
    "github.com/goliatone/cms/internal/pages"
    "github.com/goliatone/cms/internal/blocks"
    "github.com/goliatone/cms/internal/menus"
    "github.com/goliatone/cms/internal/widgets"
    "github.com/goliatone/cms/internal/themes"
)

// CMS is the main entry point for the CMS module
type CMS struct {
    container *di.Container
}

// New creates a new CMS instance
func New(config Config) (*CMS, error) {
    container, err := di.NewContainer(&config)
    if err != nil {
        return nil, fmt.Errorf("failed to create CMS: %w", err)
    }

    return &CMS{
        container: container,
    }, nil
}

// Content returns the content service
func (c *CMS) Content() content.Service {
    return c.container.ContentService()
}

// Pages returns the page service
func (c *CMS) Pages() pages.Service {
    return c.container.PageService()
}

// Blocks returns the block service
// Returns no-op service if blocks feature not enabled
func (c *CMS) Blocks() blocks.Service {
    return c.container.BlockService()
}

// Menus returns the menu service
// Returns no-op service if menus feature not enabled
func (c *CMS) Menus() menus.Service {
    return c.container.MenuService()
}

// Widgets returns the widget service
// Returns no-op service if widgets feature not enabled
func (c *CMS) Widgets() widgets.Service {
    return c.container.WidgetService()
}

// Themes returns the theme service
// Returns no-op service if themes feature not enabled
func (c *CMS) Themes() themes.Service {
    return c.container.ThemeService()
}

// Close cleans up CMS resources
func (c *CMS) Close() error {
    return c.container.Close()
}

// HealthCheck verifies CMS is healthy
func (c *CMS) HealthCheck(ctx context.Context) error {
    // Check storage connectivity
    if err := c.container.Storage().Ping(ctx); err != nil {
        return fmt.Errorf("storage unhealthy: %w", err)
    }

    // Check if any services failed to initialize
    if errors := c.container.GetErrors(); len(errors) > 0 {
        return fmt.Errorf("service initialization errors: %v", errors)
    }

    return nil
}
```

## Dependency Injection Container

```go
// internal/di/container.go
package di

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/internal/content"
    "github.com/goliatone/cms/internal/pages"
    "github.com/goliatone/cms/internal/blocks"
    "github.com/goliatone/cms/internal/menus"
    "github.com/goliatone/cms/internal/widgets"
    "github.com/goliatone/cms/internal/themes"
    "github.com/goliatone/cms/internal/i18n"
    exti18n "github.com/goliatone/go-i18n"
    "github.com/goliatone/cms/pkg/interfaces"
)

// Container manages service lifecycle and dependencies
type Container struct {
    config *cms.Config

    // Infrastructure (provided by user)
    storage  interfaces.StorageProvider
    cache    interfaces.CacheProvider
    template interfaces.TemplateRenderer
    media    interfaces.MediaProvider
    auth     interfaces.AuthProvider

    // Services (lazy-initialized, singleton pattern)
    i18nService    i18n.Service
    contentService content.Service
    pageService    pages.Service
    blockService   blocks.Service
    menuService    menus.Service
    widgetService  widgets.Service
    themeService   themes.Service

    // Synchronization for lazy initialization
    once struct {
        i18n    sync.Once
        content sync.Once
        page    sync.Once
        block   sync.Once
        menu    sync.Once
        widget  sync.Once
        theme   sync.Once
    }

    // Error tracking for initialization failures
    initErrors map[string]error
    mu         sync.RWMutex
}

// NewContainer creates a new DI container
func NewContainer(config *cms.Config) (*Container, error) {
    // Config validation happens here
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }

    config.ApplyDefaults()

    // Extract infrastructure
    storage := config.Storage // typically provided by go-persistence-bun/go-repository-bun adapter
    cache := config.Cache
    template := config.Template
    media := config.Media
    auth := config.Auth

    // Provide no-op implementations if not provided
    if cache == nil {
        cache = newNoOpCache()
    }

    return &Container{
        config:     config,
        storage:    storage,
        cache:      cache,
        template:   template,
        media:      media,
        auth:       auth,
        initErrors: make(map[string]error),
    }, nil
}

// I18nService returns the i18n service (lazy init, singleton)
func (c *Container) I18nService() i18n.Service {
    c.once.i18n.Do(func() {
        cfg, err := exti18n.NewConfig(
            exti18n.WithDefaultLocale(c.config.DefaultLocale),
            exti18n.WithLocales(c.config.LocaleCodes()...), // helper returns []string of locale codes
            exti18n.WithFallbackResolver(i18n.BuildFallbackResolver(c.config.Locales)),
            exti18n.WithLoader(i18n.NewLoader(c.storage)), // optional DB loader
            exti18n.WithCultureData(c.config.CultureDataPath),
        )
        if err != nil {
            c.recordError("i18n", fmt.Errorf("build i18n config: %w", err))
            c.i18nService = i18n.NewNoOpService()
            return
        }

        translator, err := cfg.BuildTranslator()
        if err != nil {
            c.recordError("i18n", fmt.Errorf("build translator: %w", err))
            c.i18nService = i18n.NewNoOpService()
            return
        }

        helpers := cfg.TemplateHelpers(translator, exti18n.HelperConfig{
            TemplateHelperKey: c.config.TemplateLocaleKey,
        })

        c.i18nService = i18n.NewService(i18n.ServiceOptions{
            Translator:    translator,
            Culture:       cfg.CultureService(),
            Helpers:       helpers,
            DefaultLocale: c.config.DefaultLocale,
        })
    })

    return c.i18nService
}

// ContentService returns the content service (lazy init, singleton)
func (c *Container) ContentService() content.Service {
    c.once.content.Do(func() {
        repo := content.NewRepository(c.storage) // adapter uses go-repository-bun under the hood

        c.contentService = content.NewService(
            repo,
            c.cache,
            c.I18nService(), // Dependency: i18n
            content.Options{
                CacheTTL: c.config.CacheTTL,
            },
        )
    })

    return c.contentService
}

// PageService returns the page service (lazy init, singleton)
func (c *Container) PageService() pages.Service {
    c.once.page.Do(func() {
        repo := pages.NewRepository(c.storage)

        c.pageService = pages.NewService(
            repo,
            c.cache,
            c.I18nService(),    // Dependency: i18n
            c.ContentService(), // Dependency: content
            pages.Options{
                MaxDepth: c.config.MaxPageDepth,
                CacheTTL: c.config.CacheTTL,
            },
        )
    })

    return c.pageService
}

// BlockService returns the block service (lazy init, singleton)
func (c *Container) BlockService() blocks.Service {
    if !c.config.Features.Blocks {
        c.recordError("block", fmt.Errorf("blocks feature not enabled"))
        return blocks.NewNoOpService()
    }

    c.once.block.Do(func() {
        repo := blocks.NewRepository(c.storage)
        registry := blocks.NewRegistry()

        // Register built-in block types
        registerBuiltInBlocks(registry)

        opts := blocks.Options{
            MaxDepth:     c.config.MaxBlockDepth,
            AllowNesting: c.config.Features.NestedBlocks,
            CacheTTL:     c.config.CacheTTL,
        }

        c.blockService = blocks.NewService(
            repo,
            registry,
            c.cache,
            c.I18nService(),
            opts,
        )
    })

    return c.blockService
}

// MenuService returns the menu service (lazy init, singleton)
func (c *Container) MenuService() menus.Service {
    if !c.config.Features.Menus {
        c.recordError("menu", fmt.Errorf("menus feature not enabled"))
        return menus.NewNoOpService()
    }

    c.once.menu.Do(func() {
        repo := menus.NewRepository(c.storage)

        opts := menus.Options{
            AllowHierarchical: c.config.Features.HierarchicalMenus,
            CacheTTL:          c.config.CacheTTL,
        }

        c.menuService = menus.NewService(
            repo,
            c.cache,
            c.I18nService(),
            c.PageService(), // Dependency: pages (for menu item validation)
            opts,
        )
    })

    return c.menuService
}

// WidgetService returns the widget service (lazy init, singleton)
func (c *Container) WidgetService() widgets.Service {
    if !c.config.Features.Widgets {
        c.recordError("widget", fmt.Errorf("widgets feature not enabled"))
        return widgets.NewNoOpService()
    }

    c.once.widget.Do(func() {
        repo := widgets.NewRepository(c.storage)
        registry := widgets.NewRegistry()

        // Register built-in widgets
        registerBuiltInWidgets(registry)

        c.widgetService = widgets.NewService(
            repo,
            registry,
            c.cache,
            c.I18nService(),
            c.BlockService(), // Dependency: blocks (widgets can contain blocks)
            widgets.Options{
                CacheTTL: c.config.CacheTTL,
            },
        )
    })

    return c.widgetService
}

// ThemeService returns the theme service (lazy init, singleton)
func (c *Container) ThemeService() themes.Service {
    if !c.config.Features.Themes {
        c.recordError("theme", fmt.Errorf("themes feature not enabled"))
        return themes.NewNoOpService()
    }

    if c.template == nil {
        c.recordError("theme", fmt.Errorf("themes feature requires template renderer"))
        return themes.NewNoOpService()
    }

    c.once.theme.Do(func() {
        repo := themes.NewRepository(c.storage)

        c.themeService = themes.NewService(
            repo,
            c.template,
            c.WidgetService(), // Dependency: widgets
            c.I18nService(),
            themes.Options{
                CacheTTL: c.config.CacheTTL,
            },
        )
    })

    return c.themeService
}

// Storage returns the storage provider
func (c *Container) Storage() interfaces.StorageProvider {
    return c.storage
}

// Close cleans up resources
func (c *Container) Close() error {
    var errs []error

    // Close services in reverse dependency order
    if c.themeService != nil {
        if closer, ok := c.themeService.(interface{ Close() error }); ok {
            if err := closer.Close(); err != nil {
                errs = append(errs, fmt.Errorf("theme service: %w", err))
            }
        }
    }

    // Close infrastructure
    if closer, ok := c.storage.(interface{ Close() error }); ok {
        if err := closer.Close(); err != nil {
            errs = append(errs, fmt.Errorf("storage: %w", err))
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("errors during cleanup: %v", errs)
    }

    return nil
}

// GetErrors returns all initialization errors
func (c *Container) GetErrors() map[string]error {
    c.mu.RLock()
    defer c.mu.RUnlock()

    errors := make(map[string]error)
    for k, v := range c.initErrors {
        errors[k] = v
    }
    return errors
}

// recordError stores initialization errors
func (c *Container) recordError(service string, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.initErrors[service] = err
}

// Helper functions (shared by the i18n wrapper + domain services)

func hasRegionalLocales(locales []cms.Locale) bool {
    for _, loc := range locales {
        if len(loc.Code) > 2 && loc.Code[2] == '-' {
            return true
        }
    }
    return false
}

func registerBuiltInBlocks(registry *blocks.Registry) {
    // Register core block types
    registry.Register("text", blocks.TextBlockType)
    registry.Register("image", blocks.ImageBlockType)
    registry.Register("video", blocks.VideoBlockType)
}

func registerBuiltInWidgets(registry *widgets.Registry) {
    // Register core widgets
    registry.Register("recent_posts", widgets.RecentPostsWidget)
    registry.Register("tag_cloud", widgets.TagCloudWidget)
}

type noOpCache struct{}

func newNoOpCache() interfaces.CacheProvider {
    return &noOpCache{}
}

func (n *noOpCache) Get(ctx context.Context, key string) (interface{}, error) {
    return nil, fmt.Errorf("not found")
}

func (n *noOpCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    return nil // No-op
}

func (n *noOpCache) Delete(ctx context.Context, key string) error {
    return nil // No-op
}

func (n *noOpCache) Clear(ctx context.Context) error {
    return nil // No-op
}
```

> The `internal/i18n` package shown above is intentionally lightweight: it builds a `go-i18n` configuration, wires CMS-specific fallbacks/loaders, and then exposes the resulting translator/culture services through our own interfaces. All locale resolution, formatting, and culture data rules live in the external `github.com/goliatone/go-i18n` module.

## Domain Module Examples

### Data Models (Bun ORM)

The Bun models below mirror the latest schema described in `CMS_ENTITIES.md`, including the new tables (`content_types`, `themes`, `templates`) and the additional audit/soft-delete fields introduced across the existing entities.

```go
// internal/models/models.go
package models

import (
    "time"

    "github.com/google/uuid"
    "github.com/uptrace/bun"
)

type ContentType struct {
    bun.BaseModel `bun:"table:content_types,alias:ct"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    Name          string         `bun:"name,notnull"`
    Description   *string        `bun:"description"`
    Schema        map[string]any `bun:"schema,type:jsonb,notnull"`
    Capabilities  map[string]any `bun:"capabilities,type:jsonb"`
    Icon          *string        `bun:"icon"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`
}

type Page struct {
    bun.BaseModel `bun:"table:pages,alias:p"`
    ID            uuid.UUID   `bun:",pk,type:uuid"`
    ParentID      *uuid.UUID  `bun:"parent_id"`
    Slug          string      `bun:"slug,notnull"`
    TemplateID    uuid.UUID   `bun:"template_id,notnull"`
    Status        string      `bun:"status,notnull"`
    PublishAt     *time.Time  `bun:"publish_at"`
    UnpublishAt   *time.Time  `bun:"unpublish_at"`
    CreatedBy     uuid.UUID   `bun:"created_by,notnull"`
    UpdatedBy     uuid.UUID   `bun:"updated_by,notnull"`
    DeletedAt     *time.Time  `bun:"deleted_at"`
    CreatedAt     time.Time   `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time   `bun:"updated_at,nullzero,default:now()"`

    Template     *Template        `bun:"rel:belongs-to,join:template_id=id"`
    Versions     []*PageVersion   `bun:"rel:has-many,join:id=page_id"`
    Translations []*PageTranslation `bun:"rel:has-many,join:id=page_id"`
    Blocks       []*BlockInstance `bun:"rel:has-many,join:id=page_id"`
}

type PageVersion struct {
    bun.BaseModel `bun:"table:page_versions,alias:pv"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    PageID        uuid.UUID      `bun:"page_id,notnull"`
    Version       int            `bun:"version,notnull"`
    Snapshot      map[string]any `bun:"snapshot,type:jsonb,notnull"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedBy     uuid.UUID      `bun:"created_by,notnull"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`

    Page *Page `bun:"rel:belongs-to,join:page_id=id"`
}

type PageTranslation struct {
    bun.BaseModel `bun:"table:page_translations,alias:pt"`
    ID            uuid.UUID  `bun:",pk,type:uuid"`
    PageID        uuid.UUID  `bun:"page_id,notnull"`
    LocaleID      uuid.UUID  `bun:"locale_id,notnull"`
    Title         string     `bun:"title,notnull"`
    Path          string     `bun:"path,notnull"`
    SEOTitle      *string    `bun:"seo_title"`
    SEODescription *string   `bun:"seo_description"`
    Summary       *string    `bun:"summary"`
    DeletedAt     *time.Time `bun:"deleted_at"`
    CreatedAt     time.Time  `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time  `bun:"updated_at,nullzero,default:now()"`

    Page   *Page   `bun:"rel:belongs-to,join:page_id=id"`
    Locale *Locale `bun:"rel:belongs-to,join:locale_id=id"`
}

type BlockDefinition struct {
    bun.BaseModel `bun:"table:block_definitions,alias:bd"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    Name          string         `bun:"name,notnull"`
    Description   *string        `bun:"description"`
    Icon          *string        `bun:"icon"`
    Schema        map[string]any `bun:"schema,type:jsonb,notnull"`
    Defaults      map[string]any `bun:"defaults,type:jsonb"`
    EditorStyleURL   *string     `bun:"editor_style_url"`
    FrontendStyleURL *string     `bun:"frontend_style_url"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`

    Instances []*BlockInstance `bun:"rel:has-many,join:id=definition_id"`
}

type BlockInstance struct {
    bun.BaseModel `bun:"table:block_instances,alias:bi"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    PageID        *uuid.UUID     `bun:"page_id"`
    Region        string         `bun:"region,notnull"`
    Position      int            `bun:"position,notnull"`
    DefinitionID  uuid.UUID      `bun:"definition_id,notnull"`
    Configuration map[string]any `bun:"configuration,type:jsonb,notnull"`
    IsGlobal      bool           `bun:"is_global,notnull"`
    CreatedBy     uuid.UUID      `bun:"created_by,notnull"`
    UpdatedBy     uuid.UUID      `bun:"updated_by,notnull"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Definition   *BlockDefinition     `bun:"rel:belongs-to,join:definition_id=id"`
    Page         *Page                `bun:"rel:belongs-to,join:page_id=id"`
    Translations []*BlockTranslation  `bun:"rel:has-many,join:id=block_instance_id"`
    Widgets      []*WidgetInstance    `bun:"rel:has-many,join:id=block_instance_id"`
}

type BlockTranslation struct {
    bun.BaseModel `bun:"table:block_translations,alias:bt"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    BlockInstanceID uuid.UUID    `bun:"block_instance_id,notnull"`
    LocaleID      uuid.UUID      `bun:"locale_id,notnull"`
    Content       map[string]any `bun:"content,type:jsonb,notnull"`
    AttributeOverrides map[string]any `bun:"attribute_overrides,type:jsonb"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Instance *BlockInstance `bun:"rel:belongs-to,join:block_instance_id=id"`
    Locale   *Locale        `bun:"rel:belongs-to,join:locale_id=id"`
}

type WidgetDefinition struct {
    bun.BaseModel `bun:"table:widget_definitions,alias:wd"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    Name          string         `bun:"name,notnull"`
    Description   *string        `bun:"description"`
    Schema        map[string]any `bun:"schema,type:jsonb,notnull"`
    Defaults      map[string]any `bun:"defaults,type:jsonb"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`

    Instances []*WidgetInstance `bun:"rel:has-many,join:id=definition_id"`
}

type WidgetInstance struct {
    bun.BaseModel `bun:"table:widget_instances,alias:wi"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    BlockInstanceID *uuid.UUID   `bun:"block_instance_id"`
    DefinitionID  uuid.UUID      `bun:"definition_id,notnull"`
    Configuration map[string]any `bun:"configuration,type:jsonb,notnull"`
    CreatedBy     uuid.UUID      `bun:"created_by,notnull"`
    UpdatedBy     uuid.UUID      `bun:"updated_by,notnull"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Definition   *WidgetDefinition    `bun:"rel:belongs-to,join:definition_id=id"`
    Block        *BlockInstance       `bun:"rel:belongs-to,join:block_instance_id=id"`
    Translations []*WidgetTranslation `bun:"rel:has-many,join:id=widget_instance_id"`
}

type WidgetTranslation struct {
    bun.BaseModel `bun:"table:widget_translations,alias:wt"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    WidgetInstanceID uuid.UUID   `bun:"widget_instance_id,notnull"`
    LocaleID      uuid.UUID      `bun:"locale_id,notnull"`
    Content       map[string]any `bun:"content,type:jsonb,notnull"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Instance *WidgetInstance `bun:"rel:belongs-to,join:widget_instance_id=id"`
    Locale   *Locale         `bun:"rel:belongs-to,join:locale_id=id"`
}

type Menu struct {
    bun.BaseModel `bun:"table:menus,alias:m"`
    ID            uuid.UUID   `bun:",pk,type:uuid"`
    Code          string      `bun:"code,notnull"`
    Description   *string     `bun:"description"`
    CreatedBy     uuid.UUID   `bun:"created_by,notnull"`
    UpdatedBy     uuid.UUID   `bun:"updated_by,notnull"`
    CreatedAt     time.Time   `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time   `bun:"updated_at,nullzero,default:now()"`

    Items []*MenuItem `bun:"rel:has-many,join:id=menu_id"`
}

type MenuItem struct {
    bun.BaseModel `bun:"table:menu_items,alias:mi"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    MenuID        uuid.UUID      `bun:"menu_id,notnull"`
    ParentID      *uuid.UUID     `bun:"parent_id"`
    Position      int            `bun:"position,notnull"`
    Target        map[string]any `bun:"target,type:jsonb,notnull"`
    CreatedBy     uuid.UUID      `bun:"created_by,notnull"`
    UpdatedBy     uuid.UUID      `bun:"updated_by,notnull"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Menu         *Menu                `bun:"rel:belongs-to,join:menu_id=id"`
    Parent       *MenuItem            `bun:"rel:belongs-to,join:parent_id=id"`
    Children     []*MenuItem          `bun:"rel:has-many,join:id=parent_id"`
    Translations []*MenuItemTranslation `bun:"rel:has-many,join:id=menu_item_id"`
}

type MenuItemTranslation struct {
    bun.BaseModel `bun:"table:menu_item_translations,alias:mit"`
    ID            uuid.UUID  `bun:",pk,type:uuid"`
    MenuItemID    uuid.UUID  `bun:"menu_item_id,notnull"`
    LocaleID      uuid.UUID  `bun:"locale_id,notnull"`
    Label         string     `bun:"label,notnull"`
    URLOverride   *string    `bun:"url_override"`
    DeletedAt     *time.Time `bun:"deleted_at"`
    CreatedAt     time.Time  `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time  `bun:"updated_at,nullzero,default:now()"`

    Item   *MenuItem `bun:"rel:belongs-to,join:menu_item_id=id"`
    Locale *Locale   `bun:"rel:belongs-to,join:locale_id=id"`
}

type Theme struct {
    bun.BaseModel `bun:"table:themes,alias:th"`
    ID            uuid.UUID   `bun:",pk,type:uuid"`
    Name          string      `bun:"name,notnull"`
    Description   *string     `bun:"description"`
    Version       string      `bun:"version,notnull"`
    Author        *string     `bun:"author"`
    IsActive      bool        `bun:"is_active"`
    ThemePath     string      `bun:"theme_path,notnull"`
    Config        map[string]any `bun:"config,type:jsonb"`
    CreatedAt     time.Time   `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time   `bun:"updated_at,nullzero,default:now()"`

    Templates []*Template `bun:"rel:has-many,join:id=theme_id"`
}

type Template struct {
    bun.BaseModel `bun:"table:templates,alias:tp"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    ThemeID       uuid.UUID      `bun:"theme_id,notnull"`
    Name          string         `bun:"name,notnull"`
    Slug          string         `bun:"slug,notnull"`
    Description   *string        `bun:"description"`
    TemplatePath  string         `bun:"template_path,notnull"`
    Regions       map[string]any `bun:"regions,type:jsonb,notnull"`
    Metadata      map[string]any `bun:"metadata,type:jsonb"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Theme *Theme `bun:"rel:belongs-to,join:theme_id=id"`
}

type TranslationStatus struct {
    bun.BaseModel `bun:"table:translation_status,alias:ts"`
    ID            uuid.UUID  `bun:",pk,type:uuid"`
    EntityType    string     `bun:"entity_type,notnull"`
    EntityID      uuid.UUID  `bun:"entity_id,notnull"`
    LocaleID      uuid.UUID  `bun:"locale_id,notnull"`
    Status        string     `bun:"status,notnull"`
    Completeness  int        `bun:"completeness,notnull"`
    LastUpdated   time.Time  `bun:"last_updated,nullzero,default:now()"`
    TranslatorID  *uuid.UUID `bun:"translator_id"`
    ReviewerID    *uuid.UUID `bun:"reviewer_id"`

    Locale *Locale `bun:"rel:belongs-to,join:locale_id=id"`
}

type Locale struct {
    bun.BaseModel `bun:"table:locales,alias:l"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    Code          string         `bun:"code,notnull"`
    DisplayName   string         `bun:"display_name,notnull"`
    NativeName    *string        `bun:"native_name"`
    IsActive      bool           `bun:"is_active"`
    IsDefault     bool           `bun:"is_default"`
    Metadata      map[string]any `bun:"metadata,type:jsonb"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
}
```

These models provide a one-to-one mapping with the SQL DDL in `CMS_ENTITIES.md`, ensuring the Go layer captures newly added audit fields (`created_by`, `updated_by`, `deleted_at`), JSON-backed configuration columns, and the new top-level tables for content types, themes, and templates.

## Testing Infrastructure

```go
// pkg/testsupport/fixtures.go
func LoadFixture(t *testing.T, path string) []byte
func LoadGolden(t *testing.T, path string, v any) error
func WriteGolden(t *testing.T, path string, data any) error

// pkg/testsupport/dbtest.go
func NewMemoryStorage(t *testing.T) interfaces.StorageProvider
func NewPostgresForTest(t *testing.T) interfaces.StorageProvider
```

**Contract Test Pattern**:

```go
// internal/content/service_test.go
func TestContentService_Create_BasicContent(t *testing.T) {
    input := testsupport.LoadFixture(t, "testdata/basic_content.json")
    storage := testdb.NewMemoryStorage(t)
    svc := content.NewService(storage, nil, nil, content.Options{})

    got, err := svc.Create(context.Background(), input)
    if err != nil {
        t.Fatalf("Create: %v", err)
    }

    want := testsupport.LoadGolden(t, "testdata/basic_content_output.json")
    if diff := cmp.Diff(want, got); diff != "" {
        t.Fatalf("mismatch (-want +got):\n%s", diff)
    }
}
```

## Usage Examples

### Sprint 1: Minimal Setup

```go
package main

import (
    "context"
    "log"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/adapters/storage/postgres" // wraps go-persistence-bun/go-repository-bun
)

func main() {
    // Setup storage
    storage, err := postgres.New("postgres://localhost/cms")
    if err != nil {
        log.Fatal(err)
    }
    defer storage.Close()

    // Create CMS with minimal config
    app, err := cms.New(cms.Config{
        Storage: storage,
        // Everything else uses defaults
        // Only content and pages are enabled by default
    })
    if err != nil {
        log.Fatal(err)
    }
    defer app.Close()

    // Use services
    ctx := context.Background()

    // Content service is ready
    content, err := app.Content().Create(ctx, cms.CreateContentRequest{
        Slug:   "hello-world",
        Title:  "Hello World",
        Body:   "Welcome to our CMS",
        Status: cms.StatusPublished,
        Locale: "en",
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Created content: %s", content.ID)

    // Pages service is ready
    page, err := app.Pages().Create(ctx, cms.CreatePageRequest{
        Path:      "/hello",
        ContentID: content.ID,
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Created page: %s", page.ID)

    // Blocks service returns no-op (feature not enabled)
    // This is safe, won't panic
    blocks := app.Blocks()
    _, err = blocks.Create(ctx, cms.CreateBlockRequest{})
    // err will be "blocks feature not enabled"
}
```

### Sprint 2: Add Blocks

```go
package main

import (
    "context"
    "log"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/adapters/storage/postgres" // wraps go-persistence-bun/go-repository-bun
    "github.com/goliatone/cms/adapters/cache/redis" // built on go-repository-cache decorators
)

func main() {
    storage, _ := postgres.New("postgres://localhost/cms")
    defer storage.Close()

    cache, _ := redis.New("redis://localhost:6379")
    defer cache.Close()

    // Enable blocks feature
    app, err := cms.New(cms.Config{
        Storage: storage,
        Cache:   cache,
        Features: cms.Features{
            BasicContent: true,
            BasicPages:   true,
            Blocks:       true,        // NEW
            NestedBlocks: true,        // NEW
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    defer app.Close()

    ctx := context.Background()

    // Now blocks work
    block, err := app.Blocks().Create(ctx, cms.CreateBlockRequest{
        TypeID: "text",
        Attributes: map[string]any{
            "text": "Hello from a block",
        },
        Locale: "en",
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Created block: %s", block.ID)
}
```

### Sprint 3: Add Menus

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/adapters/storage/postgres" // wraps go-persistence-bun/go-repository-bun
    "github.com/goliatone/cms/adapters/cache/redis" // built on go-repository-cache decorators
)

func main() {
    storage, _ := postgres.New("postgres://localhost/cms")
    defer storage.Close()

    cache, _ := redis.New("redis://localhost:6379")
    defer cache.Close()

    // Enable menus feature
    app, err := cms.New(cms.Config{
        Storage: storage,
        Cache:   cache,
        Features: cms.Features{
            BasicContent:      true,
            BasicPages:        true,
            Blocks:            true,
            NestedBlocks:      true,
            Menus:             true,  // NEW
            HierarchicalMenus: true,  // NEW
        },
        CacheTTL: 10 * time.Minute,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer app.Close()

    ctx := context.Background()

    // Create menu
    menu, err := app.Menus().Create(ctx, cms.CreateMenuRequest{
        Name:     "Main Navigation",
        Location: "header",
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Created menu: %s", menu.ID)
}
```

### Sprint 6: Full Features

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/adapters/storage/postgres" // wraps go-persistence-bun/go-repository-bun
    "github.com/goliatone/cms/adapters/cache/redis" // built on go-repository-cache decorators
    "github.com/goliatone/cms/adapters/template/handlebars"
    "github.com/goliatone/cms/adapters/media/s3"
    "github.com/goliatone/cms/adapters/auth/jwt"
)

func main() {
    storage, _ := postgres.New("postgres://localhost/cms")
    defer storage.Close()

    cache, _ := redis.New("redis://localhost:6379")
    defer cache.Close()

    template, _ := handlebars.New(handlebars.Config{
        TemplateDir: "./themes",
    })

    media, _ := s3.New(s3.Config{
        Bucket: "my-cms-media",
        Region: "us-east-1",
    })

    auth, _ := jwt.New(jwt.Config{
        Secret: "my-secret",
    })

    // Full feature set
    app, err := cms.New(cms.Config{
        Storage:  storage,
        Cache:    cache,
        Template: template,
        Media:    media,
        Auth:     auth,

        DefaultLocale: "en",
        Locales: []cms.Locale{
            {Code: "en-US", Name: "English (US)", IsDefault: true},
            {Code: "en-GB", Name: "English (UK)", FallbackLocaleID: "en-US"},
            {Code: "es-ES", Name: "Spanish (Spain)"},
            {Code: "es-MX", Name: "Spanish (Mexico)", FallbackLocaleID: "es-ES"},
        },

        Features: cms.Features{
            BasicContent:      true,
            BasicPages:        true,
            Blocks:            true,
            NestedBlocks:      true,
            Menus:             true,
            HierarchicalMenus: true,
            Widgets:           true,
            Themes:            true,
            Templates:         true,
            Versioning:        true,
            Scheduling:        true,
            MediaLibrary:      true,
            AdvancedCache:     true,
        },

        CacheTTL:      10 * time.Minute,
        MaxPageDepth:  10,
        MaxBlockDepth: 15,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer app.Close()

    ctx := context.Background()

    // All services available
    content := app.Content()
    pages := app.Pages()
    blocks := app.Blocks()
    menus := app.Menus()
    widgets := app.Widgets()
    themes := app.Themes()

    // Use advanced features
    scheduled, err := content.Schedule(ctx, "content-123", time.Now().Add(24*time.Hour))
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Scheduled content for: %s", scheduled.PublishAt)

    // Health check
    if err := app.HealthCheck(ctx); err != nil {
        log.Printf("Health check failed: %v", err)
    } else {
        log.Println("CMS is healthy")
    }
}
```

## Progressive Complexity Reference

```
Level 1 (Simple)     Level 2 (Regional)      Level 3 (Advanced)
     |                      |                       |
     v                      v                       v
   "en"  ───────────────> "en-US" ──────────────> Custom
   "es"                   "en-GB"                  Fallback
   "fr"                   "fr-CA"                  Chains
                          "fr-FR"
     |                      |                       |
  No config       Auto fallback "en-US"→"en"   Locale groups
  No fallbacks    Automatic parsing            Explicit config
```

**Migration Path**:
1. Start with simple codes: `"en"`, `"es"`, `"fr"`
2. Add regions if needed: change `"en"` to `"en-US"` (automatic fallback to base code)
3. Add locale groups for custom fallback logic

**Upgrade Path**: Each level of i18n complexity is enabled by providing additional configuration, without requiring modifications to existing application code or database schema. Behind the scenes the wrapper simply feeds these options into `github.com/goliatone/go-i18n`; increasing complexity means enabling more of the external package's capabilities while keeping the CMS-facing API unchanged.
