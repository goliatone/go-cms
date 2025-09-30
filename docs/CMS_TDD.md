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

This document outlines a self contained CMS module for Go applications. The module focuses exclusively on content management, providing interfaces for external dependencies while maintaining minimal coupling.

### Module Goals

- Self contained content management functionality
- Minimal external dependencies
- Interface based design for pluggable implementations
- Progressive enhancement through vertical slices
- Integration ready with existing Go ecosystem

### What This Module Provides

- Content type management (pages, blocks, menus, widgets)
- Template and theme concepts
- Internationalization support
- Content versioning and scheduling
- Hierarchical content organization

### What This Module Does NOT Provide

- Authentication/Authorization (use external auth module)
- File upload/storage (use external storage module)
- Database implementation (use external persistence layer)
- HTTP API/CRUD (use external API layer)
- Caching implementation (use external cache module)
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

11. **Translation-First**: Every user-facing string is translatable from day one.

12. **Minimal Dependencies**: The module itself has minimal external dependencies, relying on interfaces for integration.

13. **Isolated Modules**: Each content type module (pages, blocks, menus, widgets) is independent with no direct dependencies on others.

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

### Blocks Module (`blocks/`)

Block based content system:

- Block type registry
- Block rendering interfaces
- Block validation
- Nested block support
- Reusable block patterns

### Pages Module (`pages/`)

Hierarchical page management:

- Page hierarchy
- Path management
- Template assignment
- Menu order

### Menus Module (`menus/`)

Navigation management:

- Menu structure
- Menu locations
- Menu item types
- Hierarchical items

### Widgets Module (`widgets/`)

Widget functionality:

- Widget types
- Widget areas
- Visibility rules
- Widget settings

### i18n Module (`i18n/`)

Internationalization support:

- Locale management
- Translation interfaces
- Fallback chains
- Content negotiation

### Themes Module (`themes/`)

Theme management:

- Theme registration
- Template organization
- Widget area definitions
- Menu location definitions

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

Following ARCH_DESIGN.md principles, the module uses `internal/` for implementation details and `pkg/` for exported packages:

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
│       ├── types.go
│       ├── service.go
│       ├── resolver.go         # Locale resolution strategy
│       ├── formatter.go        # Locale formatting strategy
│       ├── repository.go
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

### External Dependency Interfaces

Located in `pkg/interfaces/` to allow external implementations:

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

### Configuration Layer

Separates "what to use" from "how to wire" following ARCH_DESIGN.md principles:

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

### Public API Layer

Clean interface that delegates to DI container (Layer 3: Public API):

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

### DI Container Implementation

The dependency injection container (Layer 2: Wiring) manages service lifecycle and lazy initialization:

```go
// internal/di/container.go
package di

import (
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
    storage := config.Storage
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
        // Create locale resolver based on config
        var resolver i18n.LocaleResolver
        if hasRegionalLocales(c.config.Locales) {
            resolver = i18n.NewRegionalResolver(c.config.Locales)
        } else {
            resolver = i18n.NewSimpleResolver(c.config.Locales)
        }

        // Create formatter
        formatter := i18n.NewFormatter(c.config.DefaultLocale)

        c.i18nService = i18n.NewService(
            c.storage,
            resolver,
            formatter,
            c.config.DefaultLocale,
        )
    })

    return c.i18nService
}

// ContentService returns the content service (lazy init, singleton)
func (c *Container) ContentService() content.Service {
    c.once.content.Do(func() {
        repo := content.NewRepository(c.storage)

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

// Helper functions

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

### Content Module Example

```go
// content/types.go
package content

import (
    "time"
)

type Content struct {
    ID           string
    ContentType  string
    Slug         string
    Status       ContentStatus
    AuthorID     string
    PublishOn    *time.Time
    DeletedAt    *time.Time
    CreatedAt    time.Time
    UpdatedAt    time.Time
    Translations map[string]*ContentTranslation
}

type ContentTranslation struct {
    ID              string
    ContentID       string
    LocaleCode      string
    Title           string
    Data            map[string]any
    MetaTitle       string
    MetaDescription string
}

type ContentStatus string

const (
    StatusDraft     ContentStatus = "draft"
    StatusInReview  ContentStatus = "review"
    StatusPublished ContentStatus = "published"
    StatusScheduled ContentStatus = "scheduled"
    StatusArchived  ContentStatus = "archived"
)
```

```go
// content/interfaces.go
package content

import (
    "context"
)

// Repository defines data access for content
type Repository interface {
    Create(ctx context.Context, content *Content) error
    Update(ctx context.Context, content *Content) error
    Delete(ctx context.Context, id *uuid.UUID) error
    GetByID(ctx context.Context, id *uuid.UUID) (*Content, error)
    GetBySlug(ctx context.Context, slug string) (*Content, error)
    List(ctx context.Context, opts ListOptions) ([]*Content, error)
}

// Service defines business logic for content
type Service interface {
    Create(ctx context.Context, req CreateRequest) (*Content, error)
    Update(ctx context.Context, id *uuid.UUID, req UpdateRequest) (*Content, error)
    Publish(ctx context.Context, id *uuid.UUID) error
    Schedule(ctx context.Context, id *uuid.UUID, publishOn time.Time) error
    Delete(ctx context.Context, id *uuid.UUID) error
    Get(ctx context.Context, id *uuid.UUID, locale string) (*Content, error)
    Translate(ctx context.Context, id *uuid.UUID, locale string, translation ContentTranslation) error
}
```

### Pages Module Example

```go
// pages/types.go
package pages

import (
    "time"
    "github.com/goliatone/cms/content"
)

type Page struct {
    ID            *uuid.UUID
    Content       *content.Content
    ParentID      *uuid.UUID
    TemplateSlug  string
    Path          string
    MenuOrder     int
    PageType      string
    Attributes    map[string]any
    Children      []*Page
    DeletedAt     *time.Time
}
```

```go
// pages/service.go
package pages

import (
    "context"
    "fmt"
    "github.com/goliatone/cms/content"
)

type service struct {
    contentService content.Service
    storage        StorageProvider
}

func NewService(contentService content.Service, storage StorageProvider) Service {
    return &service{
        contentService: contentService,
        storage:        storage,
    }
}

func (s *service) Create(ctx context.Context, req CreateRequest) (*Page, error) {
    // First create the content
    contentReq := content.CreateRequest{
        ContentType: "page",
        Slug:        req.Slug,
        Status:      req.Status,
        AuthorID:    req.AuthorID,
    }

    c, err := s.contentService.Create(ctx, contentReq)
    if err != nil {
        return nil, fmt.Errorf("failed to create content: %w", err)
    }

    // Then create the page-specific data
    page := &Page{
        Content:      c,
        ParentID:     req.ParentID,
        TemplateSlug: req.TemplateSlug,
        Path:         s.generatePath(req.ParentID, req.Slug),
        MenuOrder:    req.MenuOrder,
    }

    // Save page data
    err = s.storage.Exec(ctx, `
        INSERT INTO pages (id, content_id, parent_id, template_slug, path, menu_order)
        VALUES (?, ?, ?, ?, ?, ?)
    `, page.ID, c.ID, page.ParentID, page.TemplateSlug, page.Path, page.MenuOrder)

    if err != nil {
        return nil, fmt.Errorf("failed to save page: %w", err)
    }

    return page, nil
}

func (s *service) generatePath(parentID *string, slug string) string {
    if parentID == nil {
        return "/" + slug
    }
    // Get parent path and append slug
    // Implementation details...
    return ""
}
```

## Implementation Approach

### Testing Infrastructure

Following ARCH_DESIGN.md, establish fixture-driven testing infrastructure:

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

### Progressive Complexity Phases

**Phase 1: Core**
- Content module (CRUD operations)
- Pages module (hierarchy support)
- i18n (simple locale codes)
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

## Usage Examples

### Sprint 1: Minimal Setup

Basic content and pages with minimal configuration:

```go
package main

import (
    "context"
    "log"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/adapters/storage/postgres"
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

Enable blocks and nested blocks with caching:

```go
package main

import (
    "context"
    "log"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/adapters/storage/postgres"
    "github.com/goliatone/cms/adapters/cache/redis"
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

Enable menu system with hierarchical support:

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/adapters/storage/postgres"
    "github.com/goliatone/cms/adapters/cache/redis"
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

Complete CMS with all features enabled:

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/adapters/storage/postgres"
    "github.com/goliatone/cms/adapters/cache/redis"
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

## Architectural Approach: Progressive Complexity

**Design Constraints**:
- Many i18n libraries require choosing between simple or complex modes at initialization
- Simple implementations cannot handle regional variations
- Complex implementations have higher learning curves
- Switching modes typically requires application rewrites

**Implementation Approach**:
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

**Upgrade Path**: Each level of i18n complexity is enabled by providing additional configuration, without requiring modifications to existing application code or database schema.
