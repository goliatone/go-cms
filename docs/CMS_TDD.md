# Go CMS Module - Architecture Design Document

## Table of Contents
1. [Overview](#overview)
2. [Design Philosophy](#design-philosophy)
3. [Key Architectural Decisions](#key-architectural-decisions)
4. [Entity Descriptions](#entity-descriptions)
5. [Core Architecture Components](#core-architecture-components)
6. [Data Model](#data-model)
7. [Go Module Structure](#go-module-structure)
8. [Implementation Roadmap](#implementation-roadmap)

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

```
cms/
├── go.mod
├── go.sum
├── cms.go           # Main module interface
├── interfaces.go    # External dependency interfaces
├── content/
│   ├── types.go
│   ├── service.go
│   └── interfaces.go
├── blocks/
│   ├── types.go
│   ├── registry.go
│   ├── service.go
│   └── interfaces.go
├── pages/
│   ├── types.go
│   ├── service.go
│   └── interfaces.go
├── menus/
│   ├── types.go
│   ├── service.go
│   └── interfaces.go
├── widgets/
│   ├── types.go
│   ├── service.go
│   └── interfaces.go
├── themes/
│   ├── types.go
│   ├── service.go
│   └── interfaces.go
├── i18n/
│   ├── types.go
│   ├── service.go
│   └── interfaces.go
└── examples/
    └── basic/
        └── main.go
```

### External Dependency Interfaces

```go
// interfaces.go
package cms

import (
    "context"
    "time"
)

// StorageProvider defines the interface for data persistence
type StorageProvider interface {
    Query(ctx context.Context, query string, args ...any) (Rows, error)
    Exec(ctx context.Context, query string, args ...any) (Result, error)
    Transaction(ctx context.Context, fn func(tx Transaction) error) error
}

// CacheProvider defines the interface for caching
type CacheProvider interface {
    Get(ctx context.Context, key string) (any, error)
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
}

// TemplateRenderer defines the interface for template rendering
type TemplateRenderer interface {
    Render(ctx context.Context, template string, data any) (string, error)
    RegisterFunction(name string, fn any) error
}

// MediaProvider defines the interface for media/asset handling
type MediaProvider interface {
    GetURL(ctx context.Context, path string) (string, error)
    GetMetadata(ctx context.Context, id string) (MediaMetadata, error)
}

// AuthProvider defines the interface for authentication
type AuthProvider interface {
    GetCurrentUser(ctx context.Context) (User, error)
    HasPermission(ctx context.Context, user User, permission string) bool
}
```

### Core Module Implementation

```go
// cms.go
package cms

import (
    "context"
    "github.com/yourdomain/cms/content"
    "github.com/yourdomain/cms/pages"
    "github.com/yourdomain/cms/blocks"
    "github.com/yourdomain/cms/menus"
    "github.com/yourdomain/cms/widgets"
    "github.com/yourdomain/cms/themes"
    "github.com/yourdomain/cms/i18n"
)

// CMS is the main entry point for the CMS module
type CMS struct {
    storage  StorageProvider
    cache    CacheProvider
    template TemplateRenderer
    media    MediaProvider
    auth     AuthProvider

    // Services
    Content  content.Service
    Pages    pages.Service
    Blocks   blocks.Service
    Menus    menus.Service
    Widgets  widgets.Service
    Themes   themes.Service
    I18n     i18n.Service
}

// Config holds CMS configuration
type Config struct {
    Storage  StorageProvider
    Cache    CacheProvider
    Template TemplateRenderer
    Media    MediaProvider
    Auth     AuthProvider
}

// New creates a new CMS instance
func New(config Config) *CMS {
    cms := &CMS{
        storage:  config.Storage,
        cache:    config.Cache,
        template: config.Template,
        media:    config.Media,
        auth:     config.Auth,
    }

    // Initialize services
    cms.I18n = i18n.NewService(config.Storage)
    cms.Content = content.NewService(config.Storage, cms.I18n)
    cms.Themes = themes.NewService(config.Storage)
    cms.Pages = pages.NewService(cms.Content, config.Storage)
    cms.Blocks = blocks.NewService(config.Storage, cms.I18n)
    cms.Menus = menus.NewService(config.Storage, cms.I18n)
    cms.Widgets = widgets.NewService(config.Storage, cms.I18n)

    return cms
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
    "github.com/yourdomain/cms/content"
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
    "github.com/yourdomain/cms/content"
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

## Implementation Roadmap

### Sprint 1: Core Foundation
**Goal**: Basic page management with content

**Deliverables**:
- Content module with basic CRUD
- Pages module with hierarchy
- Simple template assignment
- i18n structure (locale management)

**Tables**: locales, content_types, contents, content_translations, pages

### Sprint 2: Block System
**Goal**: Composable content with blocks

**Deliverables**:
- Block type registry
- Block instances within pages
- Basic block types (paragraph, heading, image)
- Block rendering interfaces

**Tables**: block_types, block_instances, block_translations, content_blocks

### Sprint 3: Menu Management
**Goal**: Dynamic navigation system

**Deliverables**:
- Menu creation and management
- Menu items with hierarchy
- Menu locations
- Menu rendering interfaces

**Tables**: menus, menu_items, menu_item_translations

### Sprint 4: Widget System
**Goal**: Dynamic sidebar/area content

**Deliverables**:
- Widget type registry
- Widget areas definition
- Widget visibility rules
- Basic widget types

**Tables**: widget_types, widget_instances, widget_translations

### Sprint 5: Themes and Templates
**Goal**: Complete presentation layer

**Deliverables**:
- Theme management
- Template hierarchy
- Template assignment to content
- Widget area definitions per theme

**Tables**: themes, templates

### Sprint 6: Advanced Features
**Goal**: Production-ready features

**Deliverables**:
- Content versioning
- Scheduled publishing
- Soft deletes
- Media references
- Advanced block patterns

## Usage Example

```go
package main

import (
    "context"
    "github.com/yourdomain/cms"
    "github.com/yourdomain/storage"
    "github.com/yourdomain/cache"
    "github.com/yourdomain/templates"
)

func main() {
    // Initialize external dependencies
    db := storage.New(storageConfig)
    cacheProvider := cache.New(cacheConfig)
    templateEngine := templates.New(templateConfig)

    // Initialize CMS
    cmsInstance := cms.New(cms.Config{
        Storage:  db,
        Cache:    cacheProvider,
        Template: templateEngine,
        Media:    mediaProvider,
        Auth:     authProvider,
    })

    ctx := context.Background()

    // Create a page
    page, err := cmsInstance.Pages.Create(ctx, pages.CreateRequest{
        Slug:         "about",
        TemplateSlug: "default",
        Status:       "published",
        AuthorID:     "user-123",
    })

    // Add content translation
    err = cmsInstance.Content.Translate(ctx, page.Content.ID, "en-US", content.ContentTranslation{
        Title:           "About Us",
        Data:            map[string]any{"content": "Our company story..."},
        MetaTitle:       "About | ACME Corp",
        MetaDescription: "Learn about ACME Corp",
    })

    // Add a block to the page (after Sprint 2)
    block, err := cmsInstance.Blocks.AddToContent(ctx, page.Content.ID, blocks.BlockRequest{
        Type:       "paragraph",
        Attributes: map[string]any{"content": "Welcome paragraph"},
        Area:       "main",
    })
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
