# Go CMS Architecture Design Document

## Table of Contents
1. [Overview](#overview)
2. [Core Architecture Components](#core-architecture-components)
3. [Data Model](#data-model)
4. [Go Module Structure](#go-module-structure)
5. [Integration Examples](#integration-examples)
6. [Key Design Principles](#key-design-principles)

## Overview

This document outlines a modular CMS architecture in Go with built-in internationalization (i18n) and localization (l10n) support. The system is designed with a block-based content model similar to WordPress, supporting pages, menus, widgets, and templates.

### Technology Stack
- **Language**: Go
- **Database**: PostgreSQL / SQLite (transparent support)
- **SQL Libraries**: sqlx, golang-migrate, squirrel
- **Architecture Pattern**: Domain-Driven Design with Repository Pattern

## CMS Entity Descriptions

### Blocks
**Role**: Atomic content units that compose pages and posts. The fundamental building block of content.

**Usage**: Create reusable content components (paragraphs, images, galleries, quotes, embeds). Nest blocks to build complex layouts. Save frequently-used blocks as reusable patterns.

**Relations**:
- Contained within pages, posts, and widgets
- Can contain other blocks (nested structure)
- Reference media assets for rich content
- Translated independently per locale

**Big Picture**: Provides content flexibility without requiring developer intervention. Authors combine blocks to create unique layouts while maintaining consistent styling.

### Menus
**Role**: Navigation structure that links content and external resources. Organizes site hierarchy for user navigation.

**Usage**: Define navigation bars, footers, sidebars. Create hierarchical menu items linking to pages, posts, categories, or custom URLs. Assign menus to theme-defined locations.

**Relations**:
- Links to pages, posts, and taxonomy terms
- Assigned to menu locations defined by templates
- Menu items support parent-child relationships
- Each item has translations per locale

**Big Picture**: Decouples navigation from content structure. Allows arbitrary organization of site navigation independent of page hierarchy.

### Pages
**Role**: Hierarchical content containers representing static website sections. The primary structural element of the site.

**Usage**: Create site structure (homepage, about, contact). Build parent-child relationships for logical content organization. Assign custom templates for unique layouts.

**Relations**:
- Extends base content type with hierarchy
- Contains blocks in defined areas
- References specific templates for rendering
- Can be menu items and widget visibility conditions
- Supports parent-child relationships with other pages

**Big Picture**: Forms the site's information architecture. Provides URL structure and logical content organization that users and search engines understand.

### Widgets
**Role**: Dynamic content modules displayed in template-defined areas. Provides contextual functionality across multiple pages.

**Usage**: Add functionality to sidebars, footers, headers (recent posts, search, categories). Configure per-instance settings. Apply visibility rules for conditional display.

**Relations**:
- Placed in widget areas defined by templates
- Can contain blocks for rich content
- Visibility controlled by page context, user state, device type
- Settings and content translated per locale

**Big Picture**: Extends pages with reusable functionality. Allows non-technical users to add dynamic features without modifying templates.

### Templates
**Role**: Presentation layer defining how content renders. Controls visual structure and layout patterns.

**Usage**: Define page layouts, post formats, archive pages. Create template hierarchy (generic → specific). Override parent theme templates in child themes.

**Relations**:
- Belongs to themes
- Assigned to content types and individual pages
- Defines widget areas and menu locations
- Specifies which block areas are available
- Renders blocks, widgets, and menus in designated zones

**Big Picture**: Separates presentation from content. Enables design changes without content migration and supports multiple designs for different content types.

## Core Architecture Components

### Content Module (`content/`)
Core CMS content management:
- **Content types manager** - Define and manage different content structures
- **Version control** - Track content changes and revisions
- **Draft/publish workflow** - Content state management
- **Content validation** - Ensure data integrity
- **Slug generation and management**

### i18n Module (`i18n/`)
Internationalization and localization:
- **Locale management** - Handle language codes, regions
- **Translation service** - Store and retrieve translations
- **Fallback chain** - Handle missing translations gracefully
- **Content negotiation** - Detect user language preferences
- **Pluralization rules** - Language-specific plural forms

### Templates Module (`templates/`)
Template and theme management:
- **Template engine integration** - Support for Go templates, custom engines
- **Theme management** - Handle multiple themes
- **Layout system** - Master layouts and partials
- **Template inheritance** - Override and extend base templates
- **Asset pipeline** - CSS/JS bundling per template

### Blocks Module (`blocks/`)
Block-based content system:
- **Block registry** - Register and manage block types
- **Block renderer** - Render blocks with their specific logic
- **Block validation** - Ensure block data integrity
- **Custom block API** - Allow plugins to register new blocks
- **Block transformations** - Convert between block types

### Menus Module (`menus/`)
Navigation management:
- **Menu builder** - Drag-and-drop menu creation
- **Menu locations** - Define where menus appear
- **Menu item types** - Pages, custom links, categories
- **Nested menu support** - Multi-level navigation

### Widgets Module (`widgets/`)
Widget system for dynamic content areas:
- **Widget areas/zones** - Define widget placement areas
- **Widget registry** - Available widget types
- **Widget instances** - Configured widget implementations
- **Widget visibility rules** - Conditional display logic

## Data Model

### Core Tables

#### Locales
```sql
CREATE TABLE locales (
    id UUID PRIMARY KEY,
    code VARCHAR(10) UNIQUE NOT NULL, -- e.g., 'en-US', 'fr-FR'
    name VARCHAR(100) NOT NULL,
    native_name VARCHAR(100),
    is_active BOOLEAN DEFAULT true,
    is_default BOOLEAN DEFAULT false,
    fallback_locale_id UUID REFERENCES locales(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Content Types
```sql
CREATE TABLE content_types (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    type VARCHAR(50) NOT NULL, -- 'page', 'post', 'block', 'widget', 'custom'
    icon VARCHAR(100), -- UI icon identifier
    schema JSONB NOT NULL, -- JSON schema for validation
    default_template_id UUID REFERENCES templates(id),
    supports JSONB, -- features like 'blocks', 'comments', 'revisions'
    is_hierarchical BOOLEAN DEFAULT false,
    is_translatable BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Contents
```sql
CREATE TABLE contents (
    id UUID PRIMARY KEY,
    content_type_id UUID NOT NULL REFERENCES content_types(id),
    slug VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    author_id UUID NOT NULL REFERENCES users(id),
    published_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(content_type_id, slug)
);
```

#### Content Translations
```sql
CREATE TABLE content_translations (
    id UUID PRIMARY KEY,
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    title VARCHAR(500) NOT NULL,
    data JSONB NOT NULL, -- Flexible field storage
    meta_title VARCHAR(160),
    meta_description TEXT,
    meta_keywords TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(content_id, locale_id)
);
```

#### Content Versions
```sql
CREATE TABLE content_versions (
    id UUID PRIMARY KEY,
    content_translation_id UUID NOT NULL REFERENCES content_translations(id),
    version_number INTEGER NOT NULL,
    title VARCHAR(500) NOT NULL,
    data JSONB NOT NULL,
    change_summary TEXT,
    author_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(content_translation_id, version_number)
);
```

### Template System Tables

#### Themes
```sql
CREATE TABLE themes (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    version VARCHAR(20),
    author VARCHAR(200),
    description TEXT,
    screenshot_url TEXT,
    config JSONB,
    is_active BOOLEAN DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Templates
```sql
CREATE TABLE templates (
    id UUID PRIMARY KEY,
    theme_id UUID REFERENCES themes(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL, -- 'page', 'post', 'archive', 'single', 'partial'
    description TEXT,
    template_path TEXT,
    schema JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(theme_id, slug)
);
```

### Block System Tables

#### Block Types
```sql
CREATE TABLE block_types (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    category VARCHAR(50), -- 'text', 'media', 'layout', 'widgets', 'embeds'
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
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Block Instances
```sql
CREATE TABLE block_instances (
    id UUID PRIMARY KEY,
    block_type_id UUID NOT NULL REFERENCES block_types(id),
    parent_id UUID, -- for nested blocks
    parent_type VARCHAR(50), -- 'content', 'widget', 'block'
    order_index INTEGER NOT NULL DEFAULT 0,
    attributes JSONB,
    is_reusable BOOLEAN DEFAULT false,
    name VARCHAR(200), -- for reusable blocks
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Block Translations
```sql
CREATE TABLE block_translations (
    id UUID PRIMARY KEY,
    block_instance_id UUID NOT NULL REFERENCES block_instances(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    content JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(block_instance_id, locale_id)
);
```

#### Content Blocks Relationship
```sql
CREATE TABLE content_blocks (
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    block_instance_id UUID NOT NULL REFERENCES block_instances(id) ON DELETE CASCADE,
    area VARCHAR(100) DEFAULT 'main',
    order_index INTEGER NOT NULL,
    PRIMARY KEY (content_id, block_instance_id)
);
```

### Page System Tables

#### Pages
```sql
CREATE TABLE pages (
    id UUID PRIMARY KEY,
    content_id UUID UNIQUE NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES pages(id),
    template_id UUID REFERENCES templates(id),
    path TEXT NOT NULL,
    menu_order INTEGER DEFAULT 0,
    is_front_page BOOLEAN DEFAULT false,
    is_posts_page BOOLEAN DEFAULT false,
    page_attributes JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(path)
);
```

### Menu System Tables

#### Menu Locations
```sql
CREATE TABLE menu_locations (
    id UUID PRIMARY KEY,
    theme_id UUID NOT NULL REFERENCES themes(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(theme_id, slug)
);
```

#### Menus
```sql
CREATE TABLE menus (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Menu Items
```sql
CREATE TABLE menu_items (
    id UUID PRIMARY KEY,
    menu_id UUID NOT NULL REFERENCES menus(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES menu_items(id),
    type VARCHAR(50) NOT NULL, -- 'page', 'post', 'custom', 'category', 'tag'
    object_id UUID,
    url TEXT,
    target VARCHAR(50),
    css_classes TEXT,
    order_index INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Menu Item Translations
```sql
CREATE TABLE menu_item_translations (
    id UUID PRIMARY KEY,
    menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    title VARCHAR(200) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(menu_item_id, locale_id)
);
```

### Widget System Tables

#### Widget Areas
```sql
CREATE TABLE widget_areas (
    id UUID PRIMARY KEY,
    theme_id UUID NOT NULL REFERENCES themes(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    before_widget TEXT,
    after_widget TEXT,
    before_title TEXT,
    after_title TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(theme_id, slug)
);
```

#### Widget Types
```sql
CREATE TABLE widget_types (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    category VARCHAR(50),
    schema JSONB NOT NULL,
    render_callback VARCHAR(200),
    form_callback VARCHAR(200),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Widget Instances
```sql
CREATE TABLE widget_instances (
    id UUID PRIMARY KEY,
    widget_type_id UUID NOT NULL REFERENCES widget_types(id),
    widget_area_id UUID REFERENCES widget_areas(id),
    title VARCHAR(200),
    settings JSONB,
    visibility_rules JSONB,
    css_classes TEXT,
    order_index INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Widget Translations
```sql
CREATE TABLE widget_translations (
    id UUID PRIMARY KEY,
    widget_instance_id UUID NOT NULL REFERENCES widget_instances(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    title VARCHAR(200),
    content JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(widget_instance_id, locale_id)
);
```

### User and Permission Tables

#### Users
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255),
    preferred_locale_id UUID REFERENCES locales(id),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Roles
```sql
CREATE TABLE roles (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Permissions
```sql
CREATE TABLE permissions (
    id UUID PRIMARY KEY,
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    scope VARCHAR(50),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(resource, action, scope)
);
```

#### User Roles
```sql
CREATE TABLE user_roles (
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);
```

#### Role Permissions
```sql
CREATE TABLE role_permissions (
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);
```

### Media Tables

#### Media
```sql
CREATE TABLE media (
    id UUID PRIMARY KEY,
    filename VARCHAR(500) NOT NULL,
    original_name VARCHAR(500) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    metadata JSONB,
    uploader_id UUID REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Media Translations
```sql
CREATE TABLE media_translations (
    id UUID PRIMARY KEY,
    media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    alt_text TEXT,
    caption TEXT,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(media_id, locale_id)
);
```

#### Content Media
```sql
CREATE TABLE content_media (
    content_id UUID REFERENCES contents(id) ON DELETE CASCADE,
    media_id UUID REFERENCES media(id) ON DELETE CASCADE,
    field_name VARCHAR(100),
    sort_order INTEGER,
    PRIMARY KEY (content_id, media_id, field_name)
);
```

### Taxonomy Tables

#### Taxonomies
```sql
CREATE TABLE taxonomies (
    id UUID PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    type VARCHAR(50) NOT NULL,
    is_hierarchical BOOLEAN DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Taxonomy Terms
```sql
CREATE TABLE taxonomy_terms (
    id UUID PRIMARY KEY,
    taxonomy_id UUID NOT NULL REFERENCES taxonomies(id),
    parent_id UUID REFERENCES taxonomy_terms(id),
    slug VARCHAR(100) NOT NULL,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(taxonomy_id, slug)
);
```

#### Taxonomy Term Translations
```sql
CREATE TABLE taxonomy_term_translations (
    id UUID PRIMARY KEY,
    term_id UUID NOT NULL REFERENCES taxonomy_terms(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    name VARCHAR(200) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(term_id, locale_id)
);
```

#### Content Taxonomy Terms
```sql
CREATE TABLE content_taxonomy_terms (
    content_id UUID REFERENCES contents(id) ON DELETE CASCADE,
    term_id UUID REFERENCES taxonomy_terms(id) ON DELETE CASCADE,
    PRIMARY KEY (content_id, term_id)
);
```

### Performance Indexes

```sql
CREATE INDEX idx_block_instances_parent ON block_instances(parent_id, parent_type);
CREATE INDEX idx_content_blocks_order ON content_blocks(content_id, area, order_index);
CREATE INDEX idx_menu_items_menu_order ON menu_items(menu_id, order_index);
CREATE INDEX idx_pages_path ON pages(path);
CREATE INDEX idx_pages_hierarchy ON pages(parent_id, menu_order);
CREATE INDEX idx_widget_instances_area ON widget_instances(widget_area_id, order_index);
```

## Go Module Structure

### Project Layout

```
cms/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── internal/
│   ├── core/
│   │   ├── config/
│   │   ├── database/
│   │   ├── events/
│   │   └── errors/
│   ├── storage/
│   │   ├── postgres/
│   │   ├── sqlite/
│   │   └── repository.go
│   ├── content/
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── types.go
│   ├── blocks/
│   │   ├── registry.go
│   │   ├── renderer.go
│   │   └── types.go
│   ├── pages/
│   │   ├── service.go
│   │   ├── builder.go
│   │   └── types.go
│   ├── menus/
│   │   ├── service.go
│   │   ├── renderer.go
│   │   └── types.go
│   ├── widgets/
│   │   ├── service.go
│   │   ├── registry.go
│   │   └── types.go
│   ├── templates/
│   │   ├── engine.go
│   │   ├── service.go
│   │   └── types.go
│   ├── i18n/
│   │   ├── translator.go
│   │   ├── locale.go
│   │   └── types.go
│   ├── media/
│   │   ├── service.go
│   │   ├── storage.go
│   │   └── processor.go
│   ├── auth/
│   │   ├── service.go
│   │   ├── jwt.go
│   │   └── rbac.go
│   ├── api/
│   │   ├── rest/
│   │   ├── graphql/
│   │   └── middleware/
│   └── cache/
│       ├── memory.go
│       ├── redis.go
│       └── interface.go
├── pkg/
│   ├── validator/
│   ├── slugify/
│   └── utils/
├── migrations/
├── templates/
├── static/
├── go.mod
└── go.sum
```

### Core Interfaces

#### Content Module

```go
// content/types.go
package content

import (
    "context"
    "time"
)

// Content represents a piece of content
type Content struct {
    ID           string
    ContentType  *ContentType
    Slug         string
    Status       ContentStatus
    AuthorID     string
    PublishedAt  *time.Time
    Translations map[string]*ContentTranslation
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// ContentTranslation represents localized content
type ContentTranslation struct {
    ID               string
    ContentID        string
    LocaleCode       string
    Title            string
    Data             map[string]interface{}
    MetaTitle        string
    MetaDescription  string
    MetaKeywords     []string
}

// Repository interface for data access
type Repository interface {
    Create(ctx context.Context, content *Content) error
    Update(ctx context.Context, content *Content) error
    Delete(ctx context.Context, id string) error
    FindByID(ctx context.Context, id string, locale string) (*Content, error)
    FindBySlug(ctx context.Context, slug string, locale string) (*Content, error)
    List(ctx context.Context, filters ListFilters) ([]*Content, error)
}

// Service provides business logic
type Service interface {
    CreateContent(ctx context.Context, req CreateContentRequest) (*Content, error)
    UpdateContent(ctx context.Context, id string, req UpdateContentRequest) (*Content, error)
    PublishContent(ctx context.Context, id string) error
    GetContent(ctx context.Context, id string, locale string) (*Content, error)
    TranslateContent(ctx context.Context, id string, locale string, translation ContentTranslation) error
}
```

#### Block Module

```go
// blocks/types.go
package blocks

import (
    "context"
    "encoding/json"
)

// BlockType defines a reusable block structure
type BlockType struct {
    ID          string
    Name        string
    Slug        string
    Category    BlockCategory
    Icon        string
    Description string
    Schema      json.RawMessage
    Supports    BlockSupports
    Example     json.RawMessage
}

// BlockInstance represents an instance of a block
type BlockInstance struct {
    ID         string
    BlockType  *BlockType
    ParentID   *string
    ParentType string
    OrderIndex int
    Attributes map[string]interface{}
    IsReusable bool
    Name       string
    Children   []*BlockInstance
}

// BlockRenderer handles block rendering
type BlockRenderer interface {
    Render(ctx context.Context, block *BlockInstance, locale string) (string, error)
    RenderEditor(ctx context.Context, block *BlockInstance) (string, error)
}

// BlockRegistry manages block types
type BlockRegistry interface {
    Register(blockType *BlockType, renderer BlockRenderer) error
    Get(slug string) (*BlockType, BlockRenderer, error)
    List(category BlockCategory) ([]*BlockType, error)
}
```

#### Page Module

```go
// pages/types.go
package pages

import (
    "context"
    "github.com/yourdomain/cms/content"
    "github.com/yourdomain/cms/templates"
    "github.com/yourdomain/cms/blocks"
)

// Page represents a hierarchical content page
type Page struct {
    ID           string
    Content      *content.Content
    ParentID     *string
    Template     *templates.Template
    Path         string
    MenuOrder    int
    IsFrontPage  bool
    IsPostsPage  bool
    Attributes   map[string]interface{}
    Children     []*Page
    Blocks       []*blocks.BlockInstance
}

// PageService handles page operations
type PageService interface {
    Create(ctx context.Context, req CreatePageRequest) (*Page, error)
    Update(ctx context.Context, id string, req UpdatePageRequest) (*Page, error)
    Delete(ctx context.Context, id string) error
    GetByPath(ctx context.Context, path string, locale string) (*Page, error)
    GetHierarchy(ctx context.Context, parentID *string) ([]*Page, error)
    AssignTemplate(ctx context.Context, pageID string, templateID string) error
    AddBlock(ctx context.Context, pageID string, block *blocks.BlockInstance, area string) error
}

// PageBuilder constructs page output
type PageBuilder interface {
    Build(ctx context.Context, page *Page, locale string) (*PageOutput, error)
}

type PageOutput struct {
    HTML       string
    Head       HeadElements
    Assets     []Asset
    Blocks     map[string]string
    Widgets    map[string]string
}
```

#### Menu Module

```go
// menus/types.go
package menus

import (
    "context"
)

// Menu represents a navigation menu
type Menu struct {
    ID          string
    Name        string
    Slug        string
    Description string
    Items       []*MenuItem
    Locations   []string
}

// MenuItem represents an item in a menu
type MenuItem struct {
    ID         string
    MenuID     string
    ParentID   *string
    Type       MenuItemType
    ObjectID   *string
    URL        string
    Target     string
    CSSClasses string
    OrderIndex int
    IsActive   bool
    Children   []*MenuItem
    Title      map[string]string
}

type MenuItemType string

const (
    MenuItemPage     MenuItemType = "page"
    MenuItemPost     MenuItemType = "post"
    MenuItemCustom   MenuItemType = "custom"
    MenuItemCategory MenuItemType = "category"
    MenuItemTag      MenuItemType = "tag"
)

// MenuService handles menu operations
type MenuService interface {
    CreateMenu(ctx context.Context, menu *Menu) error
    GetMenu(ctx context.Context, slug string, locale string) (*Menu, error)
    GetMenuByLocation(ctx context.Context, location string, locale string) (*Menu, error)
    AddMenuItem(ctx context.Context, menuID string, item *MenuItem) error
    UpdateMenuItem(ctx context.Context, itemID string, item *MenuItem) error
    ReorderItems(ctx context.Context, menuID string, itemOrders []ItemOrder) error
    AssignToLocation(ctx context.Context, menuID string, location string) error
}

// MenuRenderer renders menus for display
type MenuRenderer interface {
    Render(ctx context.Context, menu *Menu, opts RenderOptions) (string, error)
}
```

#### Widget Module

```go
// widgets/types.go
package widgets

import (
    "context"
    "encoding/json"
)

// WidgetType defines a reusable widget
type WidgetType struct {
    ID          string
    Name        string
    Slug        string
    Description string
    Category    string
    Schema      json.RawMessage
}

// WidgetInstance represents a configured widget
type WidgetInstance struct {
    ID               string
    WidgetType       *WidgetType
    WidgetAreaID     *string
    Title            string
    Settings         map[string]interface{}
    VisibilityRules  *VisibilityRules
    CSSClasses       string
    OrderIndex       int
    IsActive         bool
}

// VisibilityRules defines when a widget should be displayed
type VisibilityRules struct {
    ShowOn          []string
    HideOn          []string
    ShowIfLoggedIn  bool
    ShowIfLoggedOut bool
    ShowOnDevices   []string
    CustomRules     json.RawMessage
}

// WidgetArea represents a widget placement zone
type WidgetArea struct {
    ID           string
    ThemeID      string
    Name         string
    Slug         string
    Description  string
    BeforeWidget string
    AfterWidget  string
    BeforeTitle  string
    AfterTitle   string
}

// WidgetService handles widget operations
type WidgetService interface {
    RegisterWidget(widgetType *WidgetType, handler WidgetHandler) error
    CreateInstance(ctx context.Context, req CreateWidgetRequest) (*WidgetInstance, error)
    AssignToArea(ctx context.Context, widgetID string, areaID string) error
    GetAreaWidgets(ctx context.Context, areaSlug string, locale string) ([]*WidgetInstance, error)
    EvaluateVisibility(ctx context.Context, widget *WidgetInstance, context PageContext) bool
}

// WidgetHandler processes widget rendering and form handling
type WidgetHandler interface {
    Render(ctx context.Context, instance *WidgetInstance, locale string) (string, error)
    RenderForm(ctx context.Context, instance *WidgetInstance) (string, error)
    Validate(settings map[string]interface{}) error
}
```

#### Template Module

```go
// templates/types.go
package templates

import (
    "context"
    "html/template"
    "encoding/json"
)

// Theme represents a collection of templates
type Theme struct {
    ID          string
    Name        string
    Slug        string
    Version     string
    Author      string
    Description string
    Config      map[string]interface{}
    IsActive    bool
}

// Template represents a page template
type Template struct {
    ID           string
    ThemeID      string
    Name         string
    Slug         string
    Type         TemplateType
    Description  string
    TemplatePath string
    Schema       json.RawMessage
}

type TemplateType string

const (
    TemplateTypePage    TemplateType = "page"
    TemplateTypePost    TemplateType = "post"
    TemplateTypeArchive TemplateType = "archive"
    TemplateTypeSingle  TemplateType = "single"
    TemplateTypePartial TemplateType = "partial"
)

// TemplateEngine handles template rendering
type TemplateEngine interface {
    LoadTheme(ctx context.Context, theme *Theme) error
    Render(ctx context.Context, template *Template, data TemplateData) (string, error)
    RenderString(ctx context.Context, tmpl string, data TemplateData) (string, error)
}

// TemplateData holds data passed to templates
type TemplateData struct {
    Page        interface{}
    Content     interface{}
    Site        SiteData
    User        interface{}
    Locale      string
    Blocks      map[string]template.HTML
    Widgets     map[string]template.HTML
    Menus       map[string]interface{}
    Assets      []Asset
    Custom      map[string]interface{}
}

// TemplateService manages templates and themes
type TemplateService interface {
    InstallTheme(ctx context.Context, themePath string) (*Theme, error)
    ActivateTheme(ctx context.Context, themeID string) error
    GetActiveTheme(ctx context.Context) (*Theme, error)
    GetTemplate(ctx context.Context, slug string) (*Template, error)
    GetTemplatesForType(ctx context.Context, templateType TemplateType) ([]*Template, error)
    CreateCustomTemplate(ctx context.Context, req CreateTemplateRequest) (*Template, error)
}
```

### Database Connection

```go
// storage/db.go
package storage

import (
    "database/sql"
    "fmt"

    "github.com/jmoiron/sqlx"
    _ "github.com/lib/pq"
    _ "github.com/mattn/go-sqlite3"
)

type DBConfig struct {
    Driver string // "postgres" or "sqlite3"
    DSN    string
}

func NewDB(config DBConfig) (*sqlx.DB, error) {
    db, err := sqlx.Connect(config.Driver, config.DSN)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }

    // Configure connection pool based on driver
    if config.Driver == "postgres" {
        db.SetMaxOpenConns(25)
        db.SetMaxIdleConns(5)
    } else if config.Driver == "sqlite3" {
        db.SetMaxOpenConns(1) // SQLite doesn't handle concurrency well
    }

    return db, nil
}
```

## Integration Examples

### Page Rendering Example

```go
// Example of how all components work together
package main

import (
    "context"
    "github.com/yourdomain/cms/blocks"
    "github.com/yourdomain/cms/pages"
    "github.com/yourdomain/cms/templates"
    "github.com/yourdomain/cms/widgets"
    "github.com/yourdomain/cms/menus"
)

type PageRenderer struct {
    pageService     pages.PageService
    blockRegistry   blocks.BlockRegistry
    widgetService   widgets.WidgetService
    menuService     menus.MenuService
    templateEngine  templates.TemplateEngine
}

func (r *PageRenderer) RenderPage(ctx context.Context, path string, locale string) (string, error) {
    // 1. Get the page
    page, err := r.pageService.GetByPath(ctx, path, locale)
    if err != nil {
        return "", err
    }

    // 2. Render blocks
    renderedBlocks := make(map[string]string)
    for area, blocks := range page.GetBlocksByArea() {
        var areaHTML string
        for _, block := range blocks {
            blockType, renderer, _ := r.blockRegistry.Get(block.BlockType.Slug)
            html, _ := renderer.Render(ctx, block, locale)
            areaHTML += html
        }
        renderedBlocks[area] = areaHTML
    }

    // 3. Get and render widgets for this page
    renderedWidgets := make(map[string]string)
    theme, _ := r.templateEngine.GetActiveTheme(ctx)
    for _, area := range theme.WidgetAreas {
        widgets, _ := r.widgetService.GetAreaWidgets(ctx, area.Slug, locale)
        var areaHTML string
        for _, widget := range widgets {
            if r.widgetService.EvaluateVisibility(ctx, widget, pageContext) {
                html, _ := widget.Render(ctx, locale)
                areaHTML += area.BeforeWidget + html + area.AfterWidget
            }
        }
        renderedWidgets[area.Slug] = areaHTML
    }

    // 4. Get menus
    menus := make(map[string]*menus.Menu)
    for _, location := range theme.MenuLocations {
        menu, _ := r.menuService.GetMenuByLocation(ctx, location.Slug, locale)
        if menu != nil {
            menus[location.Slug] = menu
        }
    }

    // 5. Prepare template data
    templateData := templates.TemplateData{
        Page:    page,
        Content: page.Content,
        Locale:  locale,
        Blocks:  renderedBlocks,
        Widgets: renderedWidgets,
        Menus:   menus,
    }

    // 6. Render with template
    return r.templateEngine.Render(ctx, page.Template, templateData)
}
```

### Block Implementation Example

```go
// Example custom block implementation
package blocks

import (
    "context"
    "fmt"
)

// ParagraphBlock - a simple text paragraph block
type ParagraphBlock struct{}

func (p *ParagraphBlock) Render(ctx context.Context, block *BlockInstance, locale string) (string, error) {
    content := block.Attributes["content"].(string)
    align := block.Attributes["align"].(string)
    return fmt.Sprintf(`<p class="align-%s">%s</p>`, align, content), nil
}

func (p *ParagraphBlock) RenderEditor(ctx context.Context, block *BlockInstance) (string, error) {
    // Return editor HTML for block editing
    return `<div class="paragraph-block-editor">...</div>`, nil
}

// ImageBlock - an image block with caption
type ImageBlock struct {
    mediaService media.Service
}

func (i *ImageBlock) Render(ctx context.Context, block *BlockInstance, locale string) (string, error) {
    imageID := block.Attributes["image_id"].(string)
    caption := block.Attributes["caption"].(string)
    alignment := block.Attributes["alignment"].(string)

    image, err := i.mediaService.Get(ctx, imageID)
    if err != nil {
        return "", err
    }

    html := fmt.Sprintf(`
        <figure class="wp-block-image align%s">
            <img src="%s" alt="%s" />
            <figcaption>%s</figcaption>
        </figure>
    `, alignment, image.URL, image.AltText, caption)

    return html, nil
}
```

### Widget Implementation Example

```go
// Example widget implementations
package widgets

import (
    "context"
    "fmt"
)

// RecentPostsWidget displays recent posts
type RecentPostsWidget struct {
    contentService content.Service
}

func (w *RecentPostsWidget) Render(ctx context.Context, instance *WidgetInstance, locale string) (string, error) {
    count := instance.Settings["count"].(int)
    showDate := instance.Settings["show_date"].(bool)

    posts, err := w.contentService.GetRecentPosts(ctx, count, locale)
    if err != nil {
        return "", err
    }

    html := `<div class="widget widget-recent-posts">`
    html += fmt.Sprintf(`<h3 class="widget-title">%s</h3>`, instance.Title)
    html += `<ul>`

    for _, post := range posts {
        html += `<li>`
        html += fmt.Sprintf(`<a href="%s">%s</a>`, post.URL, post.Title)
        if showDate {
            html += fmt.Sprintf(`<span class="post-date">%s</span>`, post.Date)
        }
        html += `</li>`
    }

    html += `</ul></div>`
    return html, nil
}

func (w *RecentPostsWidget) RenderForm(ctx context.Context, instance *WidgetInstance) (string, error) {
    // Return form HTML for widget configuration
    return `
        <div class="widget-form">
            <label>Number of posts: <input type="number" name="count" /></label>
            <label>Show date: <input type="checkbox" name="show_date" /></label>
        </div>
    `, nil
}

func (w *RecentPostsWidget) Validate(settings map[string]interface{}) error {
    count, ok := settings["count"].(int)
    if !ok || count < 1 || count > 20 {
        return fmt.Errorf("count must be between 1 and 20")
    }
    return nil
}
```

## Key Design Principles

### 1. Separation of Concerns
Each module has a single, well-defined responsibility. This makes the codebase easier to understand, test, and maintain.

### 2. Dependency Injection
Use interfaces throughout to allow for easy testing and flexibility. Dependencies are injected rather than created internally.

### 3. Context-First Design
All operations accept a context.Context as their first parameter for proper cancellation, timeout, and request-scoped value propagation.

### 4. Translation by Design
Every user-facing string is translatable from the start. The i18n/l10n support is built into the data model, avoiding costly refactoring later.

### 5. Extensibility
Plugin patterns are used where appropriate (storage backends, auth providers, block types, widgets) allowing third-party extensions without modifying core code.

### 6. Event-Driven Architecture
Modules communicate through an event bus to stay loosely coupled. This allows for features like cache invalidation, webhooks, and audit logging without tight coupling.

### 7. Composable Content
Content is built from composable blocks that can be nested and reused. Pages can contain blocks, blocks can contain other blocks, and widgets can contain blocks.

### 8. Template Flexibility
Templates exist at multiple levels (theme, content type, individual page) with proper inheritance and override mechanisms.

### 9. Database Agnostic
The storage layer abstracts database differences, allowing transparent use of PostgreSQL or SQLite based on deployment needs.

### 10. Performance Considerations
- Proper indexing on frequently queried columns
- Multi-level caching support
- Lazy loading for related data
- Connection pooling configured per database type
- Query optimization through careful N+1 query prevention

## Database Library Recommendations

### Primary Libraries

1. **sqlx** - Extends database/sql with better scanning and struct mapping
   - Provides named parameters
   - Automatic struct scanning
   - Compatible with database/sql

2. **golang-migrate/migrate** - Database migration management
   - Supports both PostgreSQL and SQLite
   - Version control for schema changes
   - Rollback capabilities

3. **squirrel** - Fluent SQL query builder
   - Helps abstract SQL differences between databases
   - Type-safe query construction
   - Supports complex queries

### Alternative Options

1. **GORM** - Full-featured ORM (if you prefer ORM over raw SQL)
2. **sqlc** - Compile-time checked SQL queries
3. **Ent** - Entity framework for Go

## Deployment Considerations

### Development Environment
- Use SQLite for rapid local development
- Docker Compose for full stack with PostgreSQL
- Hot reload support for template development

### Production Environment
- PostgreSQL for production deployments
- Redis for caching layer
- S3 or compatible object storage for media
- CDN for static asset delivery
- Horizontal scaling through stateless design

### Configuration Management
- Environment variables for sensitive data
- Configuration files for complex settings
- Support for multiple environments (dev, staging, production)
- Feature flags for gradual rollout

## Security Considerations

1. **SQL Injection Prevention** - Use parameterized queries exclusively
2. **XSS Protection** - Automatic HTML escaping in templates
3. **CSRF Protection** - Token-based CSRF protection for forms
4. **Authentication** - JWT with refresh tokens or secure sessions
5. **Authorization** - Fine-grained RBAC with resource-level permissions
6. **Input Validation** - Comprehensive validation at API and service layers
7. **Rate Limiting** - API rate limiting to prevent abuse
8. **Audit Logging** - Track all content and configuration changes

## Performance Optimization Strategies

1. **Query Optimization**
   - Use prepared statements
   - Implement query result caching
   - Batch operations where possible
   - Optimize N+1 queries with eager loading

2. **Caching Strategy**
   - Page-level caching for anonymous users
   - Fragment caching for expensive components
   - Query result caching with smart invalidation
   - CDN integration for static assets

3. **Asset Optimization**
   - Image lazy loading
   - Responsive image generation
   - CSS/JS minification and bundling
   - HTTP/2 push for critical resources

## Conclusion

This architecture provides a solid foundation for a modern, scalable CMS with WordPress-like flexibility while maintaining clean Go code structure. The modular design allows teams to work independently on different components, and the comprehensive i18n/l10n support ensures global readiness from day one.

The system is designed to be:
- **Maintainable** through clear separation of concerns
- **Scalable** through stateless design and proper caching
- **Extensible** through plugin interfaces and event-driven architecture
- **Testable** through dependency injection and interface-based design
- **Performant** through optimized queries and multi-level caching
- **Secure** through proper validation, authentication, and authorization
