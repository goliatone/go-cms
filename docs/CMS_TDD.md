# Go CMS Module - Architecture Design Document

## Table of Contents
1. [Overview](#overview)
2. [Design Philosophy](#design-philosophy)
3. [Entity Descriptions](#entity-descriptions)
4. [Core Architecture Components](#core-architecture-components)
5. [Data Model](#data-model)
6. [Go Module Structure](#go-module-structure)
7. [Implementation Roadmap](#implementation-roadmap)

## Overview

This document outlines a self-contained CMS module for Go applications. The module focuses exclusively on content management, providing interfaces for external dependencies while maintaining minimal coupling.

### Module Goals
- Self-contained content management functionality
- Minimal external dependencies
- Interface-based design for pluggable implementations
- Progressive enhancement through vertical slices
- Integration-ready with existing Go ecosystem

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

## Entity Descriptions

### Pages
**Role**: Hierarchical content containers representing website sections. The primary structural element of the site.

**Usage**: Create site structure (homepage, about, contact). Build parent-child relationships for logical content organization. Assign templates for rendering.

**Relations**:
- Extends base content type with hierarchy
- Contains blocks in defined areas (when blocks module is enabled)
- References templates for rendering
- Supports parent-child relationships with other pages

**Big Picture**: Forms the site's information architecture. Provides URL structure and logical content organization.

---

### Blocks
**Role**: Atomic content units that compose pages. The fundamental building block of content.

**Usage**: Create reusable content components (paragraphs, images, galleries). Nest blocks to build complex layouts. Save frequently-used blocks as patterns.

**Relations**:
- Contained within pages and widgets
- Can contain other blocks (nested structure)
- Reference media assets for rich content
- Translated independently per locale

**Big Picture**: Provides content flexibility without code changes. Authors combine blocks to create unique layouts.

---

### Menus
**Role**: Navigation structure that links content and external resources. Organizes site hierarchy for user navigation.

**Usage**: Define navigation bars, footers, sidebars. Create hierarchical menu items linking to pages or custom URLs. Assign menus to locations.

**Relations**:
- Links to pages and external URLs
- Assigned to menu locations
- Menu items support parent-child relationships
- Each item has translations per locale

**Big Picture**: Decouples navigation from content structure. Allows arbitrary organization of site navigation.

---

### Widgets
**Role**: Dynamic content modules displayed in defined areas. Provides contextual functionality across pages.

**Usage**: Add functionality to sidebars, footers (recent posts, search). Configure per-instance settings. Apply visibility rules.

**Relations**:
- Placed in widget areas
- Can contain blocks for rich content
- Visibility controlled by rules
- Settings and content translated per locale

**Big Picture**: Extends pages with reusable functionality. Allows dynamic features without template modifications.

---

### Templates
**Role**: Presentation layer concept defining how content renders. Controls visual structure and layout patterns.

**Usage**: Define page layouts, post formats. Create template hierarchy. Link to theme for organization.

**Relations**:
- Belongs to themes
- Assigned to content types and pages
- Defines widget areas and menu locations
- Specifies block areas

**Big Picture**: Separates presentation from content. Enables design flexibility without content migration.

---

### Themes
**Role**: Collection of templates and assets forming a complete site design. Organizes presentation resources.

**Usage**: Package related templates, styles, and configurations. Switch between designs. Define widget areas and menu locations.

**Relations**:
- Contains templates
- Defines widget areas
- Specifies menu locations
- Provides default configurations

**Big Picture**: Enables complete design changes through theme switching. Encapsulates all presentation logic.

## Core Architecture Components

### Content Module (`content/`)
Core content management functionality:
- Content type definitions
- Version control interfaces
- Draft/publish workflow
- Content validation
- Slug generation

### Blocks Module (`blocks/`)
Block-based content system:
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

### Locales Table
Stores available languages and their configuration for multilingual support.

```sql
CREATE TABLE locales (
    id UUID PRIMARY KEY,
    code VARCHAR(10) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    native_name VARCHAR(100),
    is_active BOOLEAN DEFAULT true,
    is_default BOOLEAN DEFAULT false,
    fallback_locale_id UUID REFERENCES locales(id),
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Example Data:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "code": "en-US",
  "name": "English (United States)",
  "native_name": "English",
  "is_active": true,
  "is_default": true,
  "fallback_locale_id": null,
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Content Types Table
Defines different types of content (pages, posts, custom types) and their capabilities.

```sql
CREATE TABLE content_types (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    type VARCHAR(50) NOT NULL,
    icon VARCHAR(100),
    schema JSONB NOT NULL,
    supports JSONB,
    is_hierarchical BOOLEAN DEFAULT false,
    is_translatable BOOLEAN DEFAULT true,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Example Data:**
```json
{
  "id": "223e4567-e89b-12d3-a456-426614174000",
  "name": "Page",
  "slug": "page",
  "type": "page",
  "icon": "file-text",
  "schema": {
    "properties": {
      "title": {"type": "string", "maxLength": 200},
      "content": {"type": "string"},
      "excerpt": {"type": "string", "maxLength": 500}
    }
  },
  "supports": ["blocks", "revisions", "custom-fields", "page-attributes"],
  "is_hierarchical": true,
  "is_translatable": true,
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Contents Table
Base content storage for all content types.

```sql
CREATE TABLE contents (
    id UUID PRIMARY KEY,
    content_type_id UUID NOT NULL REFERENCES content_types(id),
    slug VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    author_id UUID NOT NULL,
    publish_on TIMESTAMP,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(content_type_id, slug, deleted_at)
);
```

**Example Data:**
```json
{
  "id": "323e4567-e89b-12d3-a456-426614174000",
  "content_type_id": "223e4567-e89b-12d3-a456-426614174000",
  "slug": "about-us",
  "status": "published",
  "author_id": "423e4567-e89b-12d3-a456-426614174000",
  "publish_on": "2024-01-15T09:00:00Z",
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-10T00:00:00Z"
}
```

### Content Translations Table
Stores localized content for each content item.

```sql
CREATE TABLE content_translations (
    id UUID PRIMARY KEY,
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    title VARCHAR(500) NOT NULL,
    data JSONB NOT NULL,
    meta_title VARCHAR(160),
    meta_description TEXT,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(content_id, locale_id, deleted_at)
);
```

**Example Data:**
```json
{
  "id": "523e4567-e89b-12d3-a456-426614174000",
  "content_id": "323e4567-e89b-12d3-a456-426614174000",
  "locale_id": "123e4567-e89b-12d3-a456-426614174000",
  "title": "About Our Company",
  "data": {
    "content": "We are a technology company focused on innovation...",
    "excerpt": "Learn more about our mission and values."
  },
  "meta_title": "About Us | ACME Corp",
  "meta_description": "Learn about ACME Corp's mission, values, and team.",
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-10T00:00:00Z"
}
```

### Block Types Table
Defines available block types and their configuration.

```sql
CREATE TABLE block_types (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    category VARCHAR(50),
    icon VARCHAR(100),
    description TEXT,
    schema JSONB NOT NULL,
    render_callback VARCHAR(200),
    editor_script_url TEXT,
    editor_style_url TEXT,
    frontend_script_url TEXT,
    frontend_style_url TEXT,
    supports JSONB,
    example JSONB,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Field Descriptions:**
- `render_callback`: Function name or template path used to render the block on the frontend. Example: "blocks.RenderParagraph" or "templates/blocks/paragraph.html"
- `editor_script_url`: URL to JavaScript file that implements the block editor interface. Loaded in admin panel for block editing.
- `editor_style_url`: URL to CSS file for block editor styling. Defines how the block looks in the editor.
- `frontend_script_url`: URL to JavaScript file for frontend block functionality. Loaded on public site if block requires interactive features.
- `frontend_style_url`: URL to CSS file for frontend block styling. Defines how the block looks on the public site.
- `supports`: JSON object defining block capabilities like alignment, custom CSS classes, anchors.
- `example`: Sample block data used for preview in block picker.

**Example Data:**
```json
{
  "id": "623e4567-e89b-12d3-a456-426614174000",
  "name": "Paragraph",
  "slug": "paragraph",
  "category": "text",
  "icon": "paragraph",
  "description": "A basic text paragraph block",
  "schema": {
    "properties": {
      "content": {"type": "string"},
      "align": {"type": "string", "enum": ["left", "center", "right"]},
      "dropCap": {"type": "boolean", "default": false}
    }
  },
  "render_callback": "blocks.RenderParagraph",
  "editor_script_url": "/assets/blocks/paragraph/editor.js",
  "editor_style_url": "/assets/blocks/paragraph/editor.css",
  "frontend_script_url": null,
  "frontend_style_url": "/assets/blocks/paragraph/style.css",
  "supports": {
    "align": true,
    "anchor": true,
    "customClassName": true,
    "color": {"background": true, "text": true}
  },
  "example": {
    "attributes": {
      "content": "This is a sample paragraph.",
      "align": "left"
    }
  },
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Block Instances Table
Stores actual block usage instances within content.

```sql
CREATE TABLE block_instances (
    id UUID PRIMARY KEY,
    block_type_id UUID NOT NULL REFERENCES block_types(id),
    parent_id UUID,
    parent_type VARCHAR(50),
    order_index INTEGER NOT NULL DEFAULT 0,
    attributes JSONB,
    is_reusable BOOLEAN DEFAULT false,
    name VARCHAR(200),
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Example Data:**
```json
{
  "id": "723e4567-e89b-12d3-a456-426614174000",
  "block_type_id": "623e4567-e89b-12d3-a456-426614174000",
  "parent_id": "323e4567-e89b-12d3-a456-426614174000",
  "parent_type": "content",
  "order_index": 0,
  "attributes": {
    "content": "Welcome to our about page. Here you'll learn about our company history.",
    "align": "left",
    "dropCap": false
  },
  "is_reusable": false,
  "name": null,
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Pages Table
Extends content for hierarchical page structure.

```sql
CREATE TABLE pages (
    id UUID PRIMARY KEY,
    content_id UUID UNIQUE NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES pages(id),
    template_slug VARCHAR(100),
    path TEXT NOT NULL,
    menu_order INTEGER DEFAULT 0,
    is_front_page BOOLEAN DEFAULT false,
    page_attributes JSONB,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(path, deleted_at)
);
```

**Example Data:**
```json
{
  "id": "823e4567-e89b-12d3-a456-426614174000",
  "content_id": "323e4567-e89b-12d3-a456-426614174000",
  "parent_id": null,
  "template_slug": "default",
  "path": "/about-us",
  "menu_order": 2,
  "is_front_page": false,
  "page_attributes": {
    "show_sidebar": true,
    "layout": "full-width"
  },
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Themes Table
Defines available themes and their configuration.

```sql
CREATE TABLE themes (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    version VARCHAR(20),
    author VARCHAR(200),
    description TEXT,
    config JSONB,
    is_active BOOLEAN DEFAULT false,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Example Data:**
```json
{
  "id": "923e4567-e89b-12d3-a456-426614174000",
  "name": "Corporate Theme",
  "slug": "corporate-theme",
  "version": "1.0.0",
  "author": "ACME Design Team",
  "description": "A clean, professional theme for corporate websites",
  "config": {
    "colors": {
      "primary": "#007bff",
      "secondary": "#6c757d"
    },
    "fonts": {
      "body": "Arial, sans-serif",
      "heading": "Georgia, serif"
    },
    "widget_areas": ["sidebar", "footer-1", "footer-2", "footer-3"],
    "menu_locations": ["primary", "footer"]
  },
  "is_active": true,
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Templates Table
Defines templates available within themes.

```sql
CREATE TABLE templates (
    id UUID PRIMARY KEY,
    theme_id UUID REFERENCES themes(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL,
    description TEXT,
    schema JSONB,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(theme_id, slug, deleted_at)
);
```

**Example Data:**
```json
{
  "id": "a23e4567-e89b-12d3-a456-426614174000",
  "theme_id": "923e4567-e89b-12d3-a456-426614174000",
  "name": "Default Page Template",
  "slug": "default",
  "type": "page",
  "description": "Standard page layout with sidebar",
  "schema": {
    "areas": ["main", "sidebar"],
    "variables": ["title", "content", "featured_image"],
    "widgets": {
      "sidebar": ["search", "recent-posts", "categories"]
    }
  },
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Menus Table
Defines navigation menus.

```sql
CREATE TABLE menus (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    location VARCHAR(100),
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Example Data:**
```json
{
  "id": "b23e4567-e89b-12d3-a456-426614174000",
  "name": "Main Navigation",
  "slug": "main-nav",
  "description": "Primary site navigation",
  "location": "primary",
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Menu Items Table
Defines individual items within menus.

```sql
CREATE TABLE menu_items (
    id UUID PRIMARY KEY,
    menu_id UUID NOT NULL REFERENCES menus(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES menu_items(id),
    type VARCHAR(50) NOT NULL,
    object_id UUID,
    url TEXT,
    target VARCHAR(50),
    css_classes TEXT,
    order_index INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Example Data:**
```json
{
  "id": "c23e4567-e89b-12d3-a456-426614174000",
  "menu_id": "b23e4567-e89b-12d3-a456-426614174000",
  "parent_id": null,
  "type": "page",
  "object_id": "823e4567-e89b-12d3-a456-426614174000",
  "url": null,
  "target": "_self",
  "css_classes": "nav-item",
  "order_index": 1,
  "is_active": true,
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Widget Types Table
Defines available widget types.

```sql
CREATE TABLE widget_types (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    category VARCHAR(50),
    schema JSONB NOT NULL,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Example Data:**
```json
{
  "id": "d23e4567-e89b-12d3-a456-426614174000",
  "name": "Recent Posts",
  "slug": "recent-posts",
  "description": "Display a list of recent posts",
  "category": "posts",
  "schema": {
    "properties": {
      "title": {"type": "string", "default": "Recent Posts"},
      "count": {"type": "integer", "default": 5, "min": 1, "max": 20},
      "show_date": {"type": "boolean", "default": true},
      "show_excerpt": {"type": "boolean", "default": false}
    }
  },
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Widget Instances Table
Stores configured widget instances.

```sql
CREATE TABLE widget_instances (
    id UUID PRIMARY KEY,
    widget_type_id UUID NOT NULL REFERENCES widget_types(id),
    area VARCHAR(100),
    title VARCHAR(200),
    settings JSONB,
    visibility_rules JSONB,
    order_index INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    publish_on TIMESTAMP,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Example Data:**
```json
{
  "id": "e23e4567-e89b-12d3-a456-426614174000",
  "widget_type_id": "d23e4567-e89b-12d3-a456-426614174000",
  "area": "sidebar",
  "title": "Latest News",
  "settings": {
    "count": 5,
    "show_date": true,
    "show_excerpt": true,
    "category": "news"
  },
  "visibility_rules": {
    "show_on_pages": ["home", "blog"],
    "hide_on_pages": ["checkout"],
    "show_if_logged_in": false
  },
  "order_index": 0,
  "is_active": true,
  "publish_on": null,
  "deleted_at": null,
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

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
    Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
    Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
    Transaction(ctx context.Context, fn func(tx Transaction) error) error
}

// CacheProvider defines the interface for caching
type CacheProvider interface {
    Get(ctx context.Context, key string) (interface{}, error)
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
}

// TemplateRenderer defines the interface for template rendering
type TemplateRenderer interface {
    Render(ctx context.Context, template string, data interface{}) (string, error)
    RegisterFunction(name string, fn interface{}) error
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
    Data            map[string]interface{}
    MetaTitle       string
    MetaDescription string
}

type ContentStatus string

const (
    StatusDraft     ContentStatus = "draft"
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
    Delete(ctx context.Context, id string) error
    GetByID(ctx context.Context, id string) (*Content, error)
    GetBySlug(ctx context.Context, slug string) (*Content, error)
    List(ctx context.Context, opts ListOptions) ([]*Content, error)
}

// Service defines business logic for content
type Service interface {
    Create(ctx context.Context, req CreateRequest) (*Content, error)
    Update(ctx context.Context, id string, req UpdateRequest) (*Content, error)
    Publish(ctx context.Context, id string) error
    Schedule(ctx context.Context, id string, publishOn time.Time) error
    Delete(ctx context.Context, id string) error
    Get(ctx context.Context, id string, locale string) (*Content, error)
    Translate(ctx context.Context, id string, locale string, translation ContentTranslation) error
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
    ID            string
    Content       *content.Content
    ParentID      *string
    TemplateSlug  string
    Path          string
    MenuOrder     int
    IsFrontPage   bool
    Attributes    map[string]interface{}
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

---

### Sprint 2: Block System
**Goal**: Composable content with blocks

**Deliverables**:
- Block type registry
- Block instances within pages
- Basic block types (paragraph, heading, image)
- Block rendering interfaces

**Tables**: block_types, block_instances, block_translations, content_blocks

---

### Sprint 3: Menu Management
**Goal**: Dynamic navigation system

**Deliverables**:
- Menu creation and management
- Menu items with hierarchy
- Menu locations
- Menu rendering interfaces

**Tables**: menus, menu_items, menu_item_translations

---

### Sprint 4: Widget System
**Goal**: Dynamic sidebar/area content

**Deliverables**:
- Widget type registry
- Widget areas definition
- Widget visibility rules
- Basic widget types

**Tables**: widget_types, widget_instances, widget_translations

---

### Sprint 5: Themes and Templates
**Goal**: Complete presentation layer

**Deliverables**:
- Theme management
- Template hierarchy
- Template assignment to content
- Widget area definitions per theme

**Tables**: themes, templates

---

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
        Data:            map[string]interface{}{"content": "Our company story..."},
        MetaTitle:       "About | ACME Corp",
        MetaDescription: "Learn about ACME Corp",
    })

    // Add a block to the page (after Sprint 2)
    block, err := cmsInstance.Blocks.AddToContent(ctx, page.Content.ID, blocks.BlockRequest{
        Type:       "paragraph",
        Attributes: map[string]interface{}{"content": "Welcome paragraph"},
        Area:       "main",
    })
}
```

## Key Design Decisions

1. **Interface-Driven**: All external dependencies are interfaces, making the module pluggable into any Go application.

2. **Progressive Enhancement**: Start with pages (Sprint 1), add blocks (Sprint 2), then menus and widgets. Each feature is independent.

3. **Service Layer Architecture**: Business logic resides in services, not in data models or repositories.

4. **Soft Deletes**: All entities support `deleted_at` for data recovery and audit trails.

5. **Scheduled Publishing**: Content and widgets support `publish_on` for future publishing.

6. **Translation-First**: Every user-facing string is translatable from day one.

7. **Minimal Dependencies**: The module itself has minimal external dependencies, relying on interfaces for integration.

8. **Isolated Modules**: Each content type module (pages, blocks, menus, widgets) is independent with no direct dependencies on others.
