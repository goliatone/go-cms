# CMS Components Guide: Pages, Blocks, and Widgets

## Table of Contents
1. [Overview](#overview)
2. [Pages - The Structure](#pages---the-structure)
3. [Blocks - The Content](#blocks---the-content)
4. [Widgets - The Features](#widgets---the-features)
5. [How Components Work Together](#how-components-work-together)
6. [Implementation Examples](#implementation-examples)
7. [Common Patterns and Use Cases](#common-patterns-and-use-cases)

## Overview

The CMS uses three main components to build dynamic web content:

- **Pages**: Form the site structure and hierarchy
- **Blocks**: Provide composable, reusable content units
- **Widgets**: Add dynamic functionality to specific areas

Think of it as building a house:
- Pages are the rooms and floors (structure)
- Blocks are the furniture and decorations (content)
- Widgets are the utilities like lighting and plumbing (features)

## Pages - The Structure

### What is a Page?

A page is a hierarchical content container that represents a distinct section of your website. Pages form the backbone of your site's information architecture.

### Key Concepts

#### Page Hierarchy

Pages can have parent child relationships, creating a tree structure:

```
Home (/)
├── About (/about)
│   ├── Team (/about/team)
│   └── History (/about/history)
├── Products (/products)
│   ├── Software (/products/software)
│   └── Hardware (/products/hardware)
└── Contact (/contact)
```

#### Page Types
The `page_type` field categorizes pages for special treatment:

- `page`: Standard content page
- `front_page`: Homepage of the site
- `posts_page`: Blog listing page
- `archive`: Archive page for posts
- `landing`: Landing page for campaigns

### Page Fields Explained

```sql
CREATE TABLE pages (
    id UUID PRIMARY KEY,
    content_id UUID,           -- Links to base content table
    parent_id UUID,            -- Parent page (null for top-level)
    template_slug VARCHAR(100), -- Which template to use for rendering
    path TEXT,                 -- Full URL path (/about/team)
    menu_order INTEGER,        -- Order among siblings
    page_type TEXT,           -- Special page designation
    page_attributes JSONB,     -- Custom attributes
    publish_on TIMESTAMP,      -- Scheduled publishing
    deleted_at TIMESTAMP,       -- Soft delete
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

#### Field Details:

**`content_id`**: References the content table which holds the actual content data. This separation allows pages to inherit all content features (translations, versioning, status).

**`parent_id`**: Creates the hierarchy. When null, the page is at the root level.

**`template_slug`**: Determines which template renders this page. Examples: "default", "full-width", "sidebar-left", "landing-hero".

**`path`**: The complete URL path. Automatically generated from parent paths + slug. Must be unique among active pages.

**`menu_order`**: Determines display order among sibling pages. Lower numbers appear first.

**`page_type`**: Special types get special treatment:
```json
{
  "page_type": "front_page",  // This page loads at domain root
  "page_type": "posts_page",  // This page shows blog posts
  "page_type": "page"         // Standard page
}
```

**`page_attributes`**: Flexible storage for page-specific settings:
```json
{
  "show_sidebar": true,
  "sidebar_position": "right",
  "hero_image": "uuid-of-media",
  "custom_css_class": "dark-theme",
  "breadcrumbs": false,
  "comments_enabled": true
}
```

**`publish_on`**: Schedule future publishing. Page remains in draft until this timestamp.

### Page Example

```json
{
  "id": "823e4567-e89b-12d3-a456-426614174000",
  "content_id": "323e4567-e89b-12d3-a456-426614174000",
  "parent_id": null,
  "template_slug": "default",
  "path": "/about",
  "menu_order": 1,
  "page_type": "page",
  "page_attributes": {
    "show_sidebar": true,
    "hero_image": "media-uuid-123",
    "layout": "container"
  },
  "publish_on": null,
  "deleted_at": null
}
```

## Blocks - The Content

### What is a Block?

A block is an atomic unit of content that can be combined to create rich layouts. Blocks are the LEGO pieces of your content.

### Block Architecture

The block system has two main tables:

1. **block_types**: Defines what kinds of blocks are available
2. **block_instances**: Actual blocks used in content

### Block Types Table

Defines the blueprint for each block type:

```sql
CREATE TABLE block_types (
    id UUID PRIMARY KEY,
    name VARCHAR(100),         -- Display name
    slug VARCHAR(100),         -- Unique identifier
    category VARCHAR(50),      -- Grouping for UI
    schema JSONB,              -- Validation schema
    render_callback VARCHAR,   -- How to render
    editor_script_url TEXT,    -- Editor JS
    frontend_script_url TEXT,  -- Frontend JS
    supports JSONB,            -- Capabilities
    example JSONB              -- Preview data
);
```

#### Key Fields Explained:

**`schema`**: JSON Schema defining the block's data structure:
```json
{
  "type": "object",
  "properties": {
    "content": {
      "type": "string",
      "description": "The paragraph text"
    },
    "alignment": {
      "type": "string",
      "enum": ["left", "center", "right", "justify"],
      "default": "left"
    },
    "dropCap": {
      "type": "boolean",
      "default": false
    }
  },
  "required": ["content"]
}
```

**`render_callback`**: Specifies how to render the block:
- Function reference: `"blocks.RenderParagraph"`
- Template path: `"templates/blocks/paragraph.html"`
- Service method: `"ParagraphBlockService.Render"`

**`supports`**: Defines what features this block supports:
```json
{
  "align": true,              // Can be aligned
  "anchor": true,             // Can have HTML anchor
  "customClassName": true,    // Can add CSS classes
  "multiple": true,           // Can be used multiple times
  "reusable": true,          // Can be saved as reusable
  "html": false,             // Doesn't allow raw HTML editing
  "color": {
    "background": true,
    "text": true,
    "gradient": true
  }
}
```

**`example`**: Preview data for block picker:
```json
{
  "attributes": {
    "content": "This is what the paragraph block looks like.",
    "alignment": "left",
    "dropCap": true
  },
  "innerBlocks": []
}
```

### Block Instances Table

Stores actual block usage:

```sql
CREATE TABLE block_instances (
    id UUID PRIMARY KEY,
    block_type_id UUID,        -- Which type of block
    parent_id UUID,            -- Container (page/block/widget)
    parent_type VARCHAR(50),   -- Type of container
    order_index INTEGER,       -- Position in container
    attributes JSONB,          -- Block data
    is_reusable BOOLEAN,       -- Saved for reuse
    name VARCHAR(200),         -- Name if reusable
    publish_on TIMESTAMP       -- Scheduled publishing
);
```

#### Parent Types:

The `parent_type` field tells us what kind of container holds the block:

1. "content" - The block belongs to a page or post (any content item from the contents table)
    - This is the most common case
    - The `parent_id` references a content ID
    - Example: A paragraph block on the "About Us" page
2. "block" - The block is nested inside another block
    - Used for composite blocks (columns, groups, etc.)
    - The `parent_id` references another block instance ID
    - Example: A paragraph block inside a column block
3. "widget" - The block is contained within a widget
    - Allows widgets to have rich content via blocks
    - The `parent_id` references a widget instance ID
    - Example: A paragraph block inside a custom HTML widget

- `"content"`: Block belongs to a page/post
- `"block"`: Nested inside another block
- `"widget"`: Block inside a widget

### Block Nesting Example

A columns block containing paragraphs:

```json
{
  "id": "col-block-123",
  "block_type_id": "columns-type",
  "parent_id": "page-123",
  "parent_type": "content",
  "attributes": {
    "columns": 2,
    "gap": "medium"
  },
  "children": [
    {
      "id": "para-block-1",
      "block_type_id": "paragraph-type",
      "parent_id": "col-block-123",
      "parent_type": "block",
      "attributes": {
        "content": "Left column content"
      }
    },
    {
      "id": "para-block-2",
      "block_type_id": "paragraph-type",
      "parent_id": "col-block-123",
      "parent_type": "block",
      "attributes": {
        "content": "Right column content"
      }
    }
  ]
}
```

### Common Block Types

#### Paragraph Block
```json
{
  "slug": "paragraph",
  "category": "text",
  "attributes": {
    "content": "Text content here",
    "alignment": "left",
    "dropCap": false
  }
}
```

#### Image Block
```json
{
  "slug": "image",
  "category": "media",
  "attributes": {
    "media_id": "uuid-of-media",
    "alt": "Description of image",
    "caption": "Optional caption",
    "size": "large",
    "link": "https://example.com"
  }
}
```

#### Gallery Block
```json
{
  "slug": "gallery",
  "category": "media",
  "attributes": {
    "images": ["media-1", "media-2", "media-3"],
    "columns": 3,
    "imageCrop": true,
    "linkTo": "media"
  }
}
```

#### Button Block
```json
{
  "slug": "button",
  "category": "design",
  "attributes": {
    "text": "Click Me",
    "url": "/contact",
    "style": "primary",
    "size": "large",
    "align": "center",
    "openInNewTab": false
  }
}
```

## Widgets - The Features

### What is a Widget?

A widget is a self contained piece of functionality that can be placed in predefined areas of your site (sidebar, footer, header). Unlike blocks which are content, widgets are features.

### Widget Architecture

Widgets have three main components:

1. **widget_types**: Available widget blueprints
2. **widget_instances**: Configured widget instances
3. **widget areas**: Where widgets can be placed (defined by themes)

### Widget Types Table

```sql
CREATE TABLE widget_types (
    id UUID PRIMARY KEY,
    name VARCHAR(100),         -- Display name
    slug VARCHAR(100),         -- Unique identifier
    category VARCHAR(50),      -- Grouping
    schema JSONB              -- Configuration schema
);
```

**`schema`** defines widget settings:
```json
{
  "type": "object",
  "properties": {
    "title": {
      "type": "string",
      "default": "Recent Posts"
    },
    "count": {
      "type": "integer",
      "minimum": 1,
      "maximum": 10,
      "default": 5
    },
    "category": {
      "type": "string",
      "description": "Filter by category"
    },
    "show_date": {
      "type": "boolean",
      "default": true
    }
  }
}
```

### Widget Instances Table

```sql
CREATE TABLE widget_instances (
    id UUID PRIMARY KEY,
    widget_type_id UUID,       -- Which widget type
    area VARCHAR(100),         -- Widget area placement
    title VARCHAR(200),        -- Instance title
    settings JSONB,            -- Configuration
    visibility_rules JSONB,    -- When to show
    order_index INTEGER,       -- Position in area
    is_active BOOLEAN,         -- Enabled/disabled
    publish_on TIMESTAMP       -- Scheduled activation
);
```

#### Visibility Rules

Control when widgets appear:

```json
{
  "show_on_pages": ["home", "about", "contact"],
  "hide_on_pages": ["checkout", "cart"],
  "show_on_page_types": ["posts_page", "archive"],
  "show_if_logged_in": true,
  "show_if_logged_out": false,
  "show_on_devices": ["desktop", "tablet"],
  "hide_on_devices": ["mobile"],
  "custom_rules": {
    "show_after_date": "2024-12-01",
    "show_before_date": "2024-12-31",
    "show_to_roles": ["subscriber", "member"]
  }
}
```

### Common Widget Types

#### Recent Posts Widget
```json
{
  "slug": "recent-posts",
  "settings": {
    "title": "Latest Articles",
    "count": 5,
    "category": "news",
    "show_date": true,
    "show_excerpt": true,
    "excerpt_length": 55
  }
}
```

#### Search Widget
```json
{
  "slug": "search",
  "settings": {
    "title": "Search",
    "placeholder": "Search...",
    "button_text": "Go",
    "search_content_types": ["page", "post"]
  }
}
```

#### Navigation Menu Widget
```json
{
  "slug": "nav-menu",
  "settings": {
    "title": "Quick Links",
    "menu_id": "footer-menu-uuid",
    "show_hierarchy": true,
    "depth": 2
  }
}
```

#### Custom HTML Widget
```json
{
  "slug": "custom-html",
  "settings": {
    "title": "Newsletter Signup",
    "content": "<form>...</form>",
    "wrap_in_container": true
  }
}
```

## How Components Work Together

### The Rendering Pipeline

1. **Page Request**: User visits `/about/team`
2. **Page Lookup**: System finds page with path `/about/team`
3. **Content Load**: Load page content and translations
4. **Block Assembly**: Load and render blocks assigned to page
5. **Widget Processing**: Evaluate widget visibility rules and render active widgets
6. **Template Application**: Apply page template with blocks and widgets
7. **Final Output**: Return rendered HTML

### Component Relationships

```
Theme
├── Templates
│   ├── Defines widget areas (sidebar, footer)
│   ├── Defines block areas (main, hero)
│   └── Renders pages
├── Widget Areas
│   └── Contains widget instances
└── Menu Locations
    └── Contains menus

Page
├── Uses template
├── Contains blocks (in areas)
├── Has content (translatable)
└── Can have child pages

Block
├── Has a type (from block_types)
├── Can contain other blocks
├── Can be reusable
└── Can be in pages or widgets

Widget
├── Has a type (from widget_types)
├── Placed in widget areas
├── Can contain blocks
└── Has visibility rules
```

### Content Areas and Placement

Pages and templates define "areas" where content can be placed:

```json
{
  "template": {
    "block_areas": ["hero", "main", "aside"],
    "widget_areas": ["sidebar", "footer-1", "footer-2", "footer-3"]
  }
}
```

Blocks are assigned to block areas:
```json
{
  "page_blocks": {
    "hero": ["hero-image-block", "hero-text-block"],
    "main": ["paragraph-1", "gallery-1", "paragraph-2"],
    "aside": ["quote-block"]
  }
}
```

Widgets are assigned to widget areas:
```json
{
  "active_widgets": {
    "sidebar": ["search-widget", "recent-posts-widget"],
    "footer-1": ["about-widget"],
    "footer-2": ["nav-menu-widget"],
    "footer-3": ["social-links-widget"]
  }
}
```

## Implementation Examples

### Creating a Page with Blocks

```go
// 1. Create the page
page := &Page{
    ContentID:    contentUUID,
    ParentID:     nil,
    TemplateSlug: "full-width",
    Path:         "/services",
    MenuOrder:    3,
    PageType:     "page",
    PageAttributes: map[string]any{
        "hero_enabled": true,
        "show_breadcrumbs": true,
    },
}

// 2. Add a hero block
heroBlock := &BlockInstance{
    BlockTypeID: heroBlockTypeID,
    ParentID:    page.ID,
    ParentType:  "content",
    OrderIndex:  0,
    Attributes: map[string]any{
        "title": "Our Services",
        "subtitle": "What we do best",
        "background_image": "media-uuid",
        "overlay_opacity": 0.6,
    },
}

// 3. Add content blocks
paragraphBlock := &BlockInstance{
    BlockTypeID: paragraphBlockTypeID,
    ParentID:    page.ID,
    ParentType:  "content",
    OrderIndex:  1,
    Attributes: map[string]any{
        "content": "We provide comprehensive solutions...",
        "alignment": "center",
    },
}

// 4. Add a columns block with nested content
columnsBlock := &BlockInstance{
    BlockTypeID: columnsBlockTypeID,
    ParentID:    page.ID,
    ParentType:  "content",
    OrderIndex:  2,
    Attributes: map[string]any{
        "columns": 3,
        "gap": "large",
    },
}

// 5. Add blocks inside columns
for i, service := range services {
    serviceBlock := &BlockInstance{
        BlockTypeID: cardBlockTypeID,
        ParentID:    columnsBlock.ID,
        ParentType:  "block",
        OrderIndex:  i,
        Attributes: map[string]any{
            "title": service.Name,
            "description": service.Description,
            "icon": service.Icon,
            "link": service.URL,
        },
    }
}
```

### Creating a Reusable Block Pattern

```go
// Create a reusable "call to action" block
ctaPattern := &BlockInstance{
    BlockTypeID: groupBlockTypeID,
    ParentID:    nil,  // Not attached to content
    ParentType:  "",
    IsReusable:  true,
    Name:        "Standard CTA",
    Attributes: map[string]any{
        "background": "primary",
        "padding": "large",
    },
}

// Add child blocks to the pattern
ctaHeading := &BlockInstance{
    BlockTypeID: headingBlockTypeID,
    ParentID:    ctaPattern.ID,
    ParentType:  "block",
    OrderIndex:  0,
    Attributes: map[string]any{
        "content": "Ready to get started?",
        "level": 2,
        "alignment": "center",
    },
}

ctaButton := &BlockInstance{
    BlockTypeID: buttonBlockTypeID,
    ParentID:    ctaPattern.ID,
    ParentType:  "block",
    OrderIndex:  1,
    Attributes: map[string]any{
        "text": "Contact Us",
        "url": "/contact",
        "style": "secondary",
        "size": "large",
        "alignment": "center",
    },
}

// Use the pattern in multiple pages
pageService.AddReusableBlock(pageID, ctaPattern.ID, "main", 5)
```

### Configuring Widget Visibility

```go
// Create a promotional widget that only shows during December
promoWidget := &WidgetInstance{
    WidgetTypeID: customHTMLWidgetType,
    Area:        "header-banner",
    Title:       "Holiday Sale",
    Settings: map[string]any{
        "content": "<div class='promo'>50% off everything!</div>",
    },
    VisibilityRules: map[string]any{
        "show_after_date": "2024-12-01",
        "show_before_date": "2024-12-31",
        "hide_on_pages": []string{"checkout", "cart"},
        "show_on_devices": []string{"desktop", "tablet", "mobile"},
    },
    IsActive: true,
}

// Create a members-only widget
membersWidget := &WidgetInstance{
    WidgetTypeID: recentPostsWidgetType,
    Area:        "sidebar",
    Title:       "Member Content",
    Settings: map[string]any{
        "count": 5,
        "category": "members-only",
    },
    VisibilityRules: map[string]any{
        "show_if_logged_in": true,
        "show_to_roles": []string{"member", "premium"},
    },
}
```

## Common Patterns and Use Cases

### Landing Pages

Landing pages typically use:
- Custom template without header/footer
- Hero block at top
- Multiple sections with alternating layouts
- CTA blocks
- Minimal widgets (focus on conversion)

```go
landingPage := &Page{
    PageType:     "landing",
    TemplateSlug: "blank-canvas",
    PageAttributes: map[string]any{
        "hide_header": true,
        "hide_footer": true,
        "conversion_tracking": true,
    },
}
```

### Blog Posts

Blog posts use:
- Post content type (not page)
- Standard blocks for content
- Sidebar widgets (categories, recent posts, tags)
- Comments widget at bottom

### Dynamic Sidebars

Different sidebar content per section:

```go
// Homepage sidebar
homepageSidebar := []WidgetInstance{
    searchWidget,
    featuredPostsWidget,
    newsletterWidget,
}

// Blog sidebar
blogSidebar := []WidgetInstance{
    searchWidget,
    categoriesWidget,
    recentPostsWidget,
    tagCloudWidget,
}

// Shop sidebar
shopSidebar := []WidgetInstance{
    productSearchWidget,
    categoriesWidget,
    priceFilterWidget,
    recentlyViewedWidget,
}
```

### Scheduled Content

Schedule blocks and widgets for campaigns:

```go
// Black Friday banner - auto-publish
blackFridayBlock := &BlockInstance{
    BlockTypeID: bannerBlockTypeID,
    PublishOn:   time.Date(2024, 11, 29, 0, 0, 0, 0, time.UTC),
    Attributes: map[string]any{
        "message": "Black Friday Sale - Up to 70% off!",
        "link": "/black-friday",
        "style": "urgent",
    },
}

// Auto-remove after sale
blackFridayBlock.DeletedAt = time.Date(2024, 12, 2, 0, 0, 0, 0, time.UTC)
```

### Multi-language Blocks

Blocks support translations:

```go
// Create block
block := &BlockInstance{
    BlockTypeID: paragraphBlockTypeID,
    // ... other fields
}

// Add translations
translations := []BlockTranslation{
    {
        BlockInstanceID: block.ID,
        LocaleID:       enUSLocale,
        Content: map[string]any{
            "text": "Welcome to our website",
        },
    },
    {
        BlockInstanceID: block.ID,
        LocaleID:       frFRLocale,
        Content: map[string]any{
            "text": "Bienvenue sur notre site",
        },
    },
}
```

## Best Practices

### Block Design
1. Keep blocks atomic and single-purpose
2. Use nesting for complex layouts
3. Make blocks reusable when patterns emerge
4. Validate block data against schema
5. Provide sensible defaults

### Page Organization
1. Use hierarchy to reflect site structure
2. Keep paths short and meaningful
3. Use page types for special behaviors
4. Leverage page attributes for flexibility
5. Plan for URL changes (redirects)

### Widget Strategy
1. Use visibility rules to reduce clutter
2. Group related widgets in areas
3. Consider performance (cache widget output)
4. Make widgets responsive
5. Test visibility rules thoroughly

### Performance Considerations
1. Lazy load blocks below the fold
2. Cache rendered widget output
3. Optimize block queries (avoid N+1)
4. Use CDN for block assets
5. Implement fragment caching for complex blocks

This guide provides the foundational understanding needed to implement and work with the CMS component system effectively.
