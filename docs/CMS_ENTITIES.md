# CMS Components Guide: Pages, Blocks, and Widgets

## Table of Contents
1. [Overview](#overview)
2. [Internationalization Architecture](#internationalization-architecture)
3. [Pages - The Structure](#pages---the-structure)
4. [Blocks - The Content](#blocks---the-content)
5. [Widgets - The Features](#widgets---the-features)
6. [How Components Work Together](#how-components-work-together)
7. [Implementation Examples](#implementation-examples)
8. [Common Patterns and Use Cases](#common-patterns-and-use-cases)
9. [Best Practices](#best-practices)

## Overview

The CMS uses three main components to build dynamic web content:

- **Pages**: Form the site structure and hierarchy
- **Blocks**: Provide composable, reusable content units
- **Widgets**: Add dynamic functionality to specific areas

Think of it as building a house:
- Pages are the rooms and floors (structure)
- Blocks are the furniture and decorations (content)
- Widgets are the utilities like lighting and plumbing (features)

## Internationalization Architecture

### Overview

The CMS includes internationalization (i18n) and localization (l10n) as core features. All user facing content can be translated. The system supports both simple language codes and complex regional locale codes.

**Default Mode**: Basic language codes (`en`, `es`, `fr`, `de`).

**Optional Mode**: Full locale codes (`en-US`, `en-GB`, `fr-CA`, `fr-FR`) for regional variations, fallback chains, and locale specific formatting.

### Locale Complexity Levels

#### Level 1: Simple Language Codes (Default)

**Use Case**: Basic multilingual site with one translation per language.

**Locale Codes**: `en`, `es`, `fr`, `de`, `ja`, `ar`

**Features**:
- Single translation per language
- No fallback chains
- URL patterns: `/en/about`, `/es/acerca`
- No configuration required

**Example**:
```go
cms.AddLocale("en", "English")
cms.AddLocale("es", "Spanish")
cms.AddLocale("fr", "French")
```

#### Level 2: Regional Locales (opt in)

**Use Case**: Regional variations (US English vs UK English, Canadian French vs France French).

**Locale Codes**: `en-US`, `en-GB`, `fr-CA`, `fr-FR`

**Features**:
- Regional locale codes
- Automatic fallback chains (`fr-CA` → `fr` → default)
- Locale specific formatting (dates, numbers, currency)
- Regional URL patterns: `/en-us/about`, `/en-gb/about`

**Example**:
```go
cms.AddLocale("en-US", "English (US)", WithFallback("en"))
cms.AddLocale("en-GB", "English (UK)", WithFallback("en"))
cms.AddLocale("fr-CA", "French (Canada)", WithFallback("fr", "en"))
cms.AddLocale("fr-FR", "French (France)", WithFallback("fr", "en"))
```

#### Level 3: Custom Fallback Chains (Advanced)

**Use Case**: Multi region sites with custom fallback logic.

**Example**:
```go
cms.AddLocaleGroup("french-markets",
    WithPrimary("fr"),
    WithFallbacks("fr-CA", "fr-FR", "fr-BE", "fr-CH"),
    WithDefault("en"),
)
```

### Architecture

**Core Principle**: The locale code is an opaque string. The system accepts `"en"` or `"en-US"` without distinction. Complexity is opt in through configuration.

**Schema Design** (supports both):
```sql
CREATE TABLE locales (
    id UUID PRIMARY KEY,
    code VARCHAR(10) UNIQUE NOT NULL,  -- "en" OR "en-US" (your choice)
    name VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    is_default BOOLEAN DEFAULT false,
    -- OPTIONAL: Only used if you enable fallbacks
    fallback_locale_id UUID REFERENCES locales(id),
    -- OPTIONAL: Only used if you need regional metadata
    metadata JSONB,  -- {"region": "US", "currency": "USD", "direction": "ltr"}
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Key Design Points**:
1. `code` field accepts any string: simple (`en`) or complex (`en-US`)
2. `fallback_locale_id` is **nullable**: only use if you need fallbacks
3. `metadata` is **nullable**: only populate for regional locales
4. The system never assumes complexity: it adapts to your data

### Core Design Principles

#### 1. Separation of Configuration from Content

**Critical Concept**: Configuration settings (layout, styling, behavior) are NOT translated, while content (text, labels, URLs) IS translated.

**Configuration** (Not Translated):
- Block layout settings (alignment, columns, padding)
- Widget behavior settings (count, show/hide flags, sorting)
- Template assignments
- Display preferences
- Numeric thresholds

**Content** (Translated):
- All user visible text
- UI labels and button text
- URLs and slugs
- Meta descriptions and titles
- Alt text for images

**Example**:
```json
{
  "hero_block": {
    "configuration": {
      "layout": "centered",
      "padding": "large",
      "enable_parallax": true
    },
    "translatable_content": {
      "en-US": {"headline": "Welcome", "button_text": "Get Started"},
      "es-ES": {"headline": "Bienvenido", "button_text": "Comenzar"}
    }
  }
}
```

#### 2. Fallback Strategy (Progressive Complexity)

**Simple Mode** (Level 1): No fallbacks
```go
// Missing translation returns default locale
// en → (not found) → en (default)
// es → (not found) → en (default)
```

**Regional Mode** (Level 2): Automatic base language fallback
```go
// System strips region code automatically
// en-US → en → default
// fr-CA → fr → en (default)
```

**Advanced Mode** (Level 3): Custom fallback chains
```
fr-CA → fr-FR → fr → en (default)
```

**Implementation: Fallback Resolution**

The fallback logic checks locales in sequence:

```go
// GetTranslation with automatic fallback
func (s *TranslationService) GetTranslation(contentID, locale string) (*Translation, error) {
    // 1. Try exact locale
    if trans := s.tryLocale(contentID, locale); trans != nil {
        return trans, nil
    }

    // 2. Try configured fallbacks (Level 2 & 3)
    for _, fallback := range s.GetFallbackChain(locale) {
        if trans := s.tryLocale(contentID, fallback); trans != nil {
            return trans, nil
        }
    }

    // 3. Use default locale
    return s.tryLocale(contentID, s.defaultLocale), nil
}

// GetFallbackChain adapts to your setup
func (s *TranslationService) GetFallbackChain(locale string) []string {
    // Check for explicit fallback (Level 2 & 3)
    if fallback := s.getExplicitFallback(locale); fallback != "" {
        return []string{fallback}
    }

    // Auto-generate fallback for regional codes (Level 2)
    if strings.Contains(locale, "-") {
        baseLocale := strings.Split(locale, "-")[0]  // "en-US" → "en"
        return []string{baseLocale}
    }

    // No fallback (Level 1)
    return []string{}
}
```

**Optional: Locale Groups Table** (only for Level 3)
```sql
-- Only create this if you need complex fallback chains
CREATE TABLE locale_groups (
    id UUID PRIMARY KEY,
    name VARCHAR(100),
    primary_locale_id UUID REFERENCES locales(id),
    fallback_order JSONB  -- ["fr-CA", "fr-FR", "fr", "en"]
);
```

**When to Use Each Level**:
- **Level 1**: 90% of projects (one translation per language)
- **Level 2**: Regional sites (UK/US English, Canadian/France French)
- **Level 3**: Complex multi region enterprises

#### 3. Multilingual URL Strategy

**Simple Mode** (Level 1): locale specific paths with simple codes
```
en: /about-us
es: /acerca-de
fr: /a-propos
de: /uber-uns
```

**Regional Mode** (Level 2): Regional paths when needed
```
en-US: /about-us
en-GB: /about-us
fr-CA: /a-propos
fr-FR: /a-propos
```

**Implementation**:
- `pages` table stores page structure (locale independent)
- `page_translations` table stores locale specific paths
- URL router resolves by `locale` + `path`
- Works identically for simple or complex locale codes

**Note**: Same schema for both modes
```sql
-- Simple locales
INSERT INTO page_translations (page_id, locale_id, path)
VALUES (page_id, 'en', '/about-us');

-- Regional locales (identical schema)
INSERT INTO page_translations (page_id, locale_id, path)
VALUES (page_id, 'en-US', '/about-us');
```

**SEO Considerations** (apply to both modes):
- Canonical URLs per locale
- `hreflang` tags for all locale variants
- Locale specific sitemaps
- Proper `301` redirects on locale switching

#### 4. Translation Status Tracking

Track translation completeness across all entities:

```sql
CREATE TABLE translation_status (
    id UUID PRIMARY KEY,
    entity_type VARCHAR(50),
    entity_id UUID,
    locale_id UUID REFERENCES locales(id),
    status VARCHAR(50),
    completeness INTEGER,
    last_updated TIMESTAMP,
    translator_id UUID,
    reviewer_id UUID,
    UNIQUE(entity_type, entity_id, locale_id)
);
```

**Status Values**:
- `missing`: No translation exists
- `draft`: Translation in progress
- `review`: Awaiting review
- `approved`: Ready for production

**Completeness**: Percentage (0-100) of translatable fields completed.

#### 5. Media Locale Variants

Images and media often contain text and need locale specific versions:

**Scenario**: An infographic with embedded text needs different versions per language.

**Solution**: Use `attribute_overrides` in translation tables:

```json
{
  "block_instance": {
    "attributes": {
      "hero_image_id": "default-hero-uuid"
    }
  },
  "translations": {
    "es-ES": {
      "attribute_overrides": {
        "hero_image_id": "spanish-hero-uuid"
      }
    }
  }
}
```

**Fallback for Media**:
- If locale specific media not found, use default
- Configurable per block type
- Automatic CDN path generation per locale

### Performance Optimization

#### Caching Strategies

1. **Translation Cache**: Cache translations by `entity_type:entity_id:locale`
2. **Fallback Cache**: Cache resolved fallback chains
3. **URL Cache**: Cache locale specific URL mappings
4. **Fragment Caching**: Cache rendered blocks with locale in key

**Cache Key Pattern**:
```
block:{block_id}:{locale}:v{version}
page:{page_id}:{locale}:v{version}
widget:{widget_id}:{locale}:v{version}
```

#### Query Optimization

**Problem**: `N+1` queries when loading page with multiple blocks and translations.

**Solution**: Eager loading with joins:
```sql
SELECT
    b.*,
    bt.content,
    bt.attribute_overrides,
    l.code as locale_code
FROM block_instances b
LEFT JOIN block_translations bt ON b.id = bt.block_instance_id
LEFT JOIN locales l ON bt.locale_id = l.id
WHERE b.parent_id = ? AND l.code IN (?, ?, ?)
```

Load all required locales in fallback chain with single query.

#### CDN Distribution

- Serve locale specific assets from regional CDNs
- URL pattern: `cdn.example.com/{locale}/assets/...`
- Automatic cache warming for popular locales
- Locale-based cache TTLs

### URL Redirects and Locale Switching

```sql
CREATE TABLE url_redirects (
    id UUID PRIMARY KEY,
    from_path TEXT NOT NULL,
    to_path TEXT NOT NULL,
    locale_id UUID REFERENCES locales(id),
    redirect_type INTEGER DEFAULT 301,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Use Cases**:
1. User switches locale: Redirect `/about` (en) → `/a-propos` (fr)
2. URL structure changes: Redirect old paths to new
3. Canonical enforcement: Redirect non-canonical to canonical

**Redirect Logic**:
```go
// User clicks language switcher on /about-us (en-US)
// System looks up current page ID
// Finds equivalent path in target locale (fr-FR)
// Redirects to /a-propos with 302 (temporary)
```

### RTL Language Support

For Arabic, Hebrew, and other RTL languages:

**Locale Configuration**:
```json
{
  "code": "ar-SA",
  "direction": "rtl",
  "text_align_default": "right"
}
```

**Template Considerations**:
- CSS direction switching: `dir="rtl"` on `<html>`
- Automatic text alignment adjustments
- Mirrored layout for RTL (sidebar switches sides)
- Icon rotation where needed

**Implementation**:
```html
<html dir="{{ locale.direction }}" lang="{{ locale.code }}">
```

### Formatting Rules Per Locale

Different locales format data differently:

**Date Formatting**:
- en-US: `12/31/2024` (MM/DD/YYYY)
- en-GB: `31/12/2024` (DD/MM/YYYY)
- de-DE: `31.12.2024` (DD.MM.YYYY)
- ja-JP: `2024年12月31日`

**Number Formatting**:
- en-US: `1,234.56` (comma thousands, period decimal)
- de-DE: `1.234,56` (period thousands, comma decimal)
- fr-FR: `1 234,56` (space thousands, comma decimal)

**Currency Display**:
- en-US: `$1,234.56`
- de-DE: `1.234,56 €`
- ja-JP: `¥1,234`

**Implementation**: Use locale aware formatting libraries, don't hardcode formats.

### Configuration & Extension Points

The CMS provides clear extension points to progressively add complexity:

#### 1. Locale Resolution Strategy

**Interface** (implement your own logic):
```go
type LocaleResolver interface {
    // Resolve locale from HTTP request
    ResolveFromRequest(r *http.Request) string

    // Resolve locale from path (/en/about vs /about)
    ResolveFromPath(path string) (locale, cleanPath string)

    // Get default locale
    GetDefault() string
}
```

**Default Implementation** (simple):
```go
type SimpleLocaleResolver struct {
    defaultLocale string
}

func (r *SimpleLocaleResolver) ResolveFromRequest(req *http.Request) string {
    // 1. Check Accept-Language header
    if locale := req.Header.Get("Accept-Language"); locale != "" {
        return parseAcceptLanguage(locale)
    }
    // 2. Return default
    return r.defaultLocale
}
```

**Advanced Implementation** (opt in):
```go
type RegionalLocaleResolver struct {
    SimpleLocaleResolver
    ipGeolocation IPGeolocator  // Inject your IP→locale service
}

func (r *RegionalLocaleResolver) ResolveFromRequest(req *http.Request) string {
    // 1. Check cookie
    if cookie, _ := req.Cookie("locale"); cookie != nil {
        return cookie.Value
    }
    // 2. Check IP geolocation (opt in!)
    if r.ipGeolocation != nil {
        if locale := r.ipGeolocation.GetLocale(req.RemoteAddr); locale != "" {
            return locale
        }
    }
    // 3. Fall back to simple logic
    return r.SimpleLocaleResolver.ResolveFromRequest(req)
}
```

#### 2. Locale Formatter Strategy

**Interface** (progressive enhancement):
```go
type LocaleFormatter interface {
    // Format date according to locale conventions
    FormatDate(t time.Time, locale string) string

    // Format number with locale specific separators
    FormatNumber(n float64, locale string) string

    // Format currency
    FormatCurrency(amount int, currency, locale string) string
}
```

**Default Implementation** (simple - no regional logic):
```go
type SimpleFormatter struct{}

func (f *SimpleFormatter) FormatDate(t time.Time, locale string) string {
    // Use ISO format for all locales (simple!)
    return t.Format("2006-01-02")
}

func (f *SimpleFormatter) FormatNumber(n float64, locale string) string {
    // Use dot decimal for all (simple!)
    return fmt.Sprintf("%.2f", n)
}
```

**Regional Implementation** (opt in via Go's `text` package):
```go
import "golang.org/x/text/language"
import "golang.org/x/text/message"

type RegionalFormatter struct{}

func (f *RegionalFormatter) FormatNumber(n float64, locale string) string {
    tag := language.MustParse(locale)
    p := message.NewPrinter(tag)
    return p.Sprintf("%.2f", n)
}
```

#### 3. URL Pattern Strategy

**Interface**:
```go
type URLPatternStrategy interface {
    // Generate URL for locale and path
    GenerateURL(locale, path string) string

    // Parse URL to extract locale and path
    ParseURL(url string) (locale, path string)
}
```

**Pattern 1: Path Prefix** (default, simple)
```go
// URLs: /en/about, /es/acerca
type PathPrefixStrategy struct{}

func (s *PathPrefixStrategy) GenerateURL(locale, path string) string {
    return fmt.Sprintf("/%s%s", locale, path)
}
```

**Pattern 2: Subdomain** (opt in)
```go
// URLs: en.example.com/about, es.example.com/about
type SubdomainStrategy struct {
    baseDomain string
}

func (s *SubdomainStrategy) GenerateURL(locale, path string) string {
    return fmt.Sprintf("https://%s.%s%s", locale, s.baseDomain, path)
}
```

**Pattern 3: Domain per Locale** (advanced)
```go
// URLs: example.com/about (en), example.es/acerca (es)
type DomainStrategy struct {
    domainMap map[string]string  // {"en": "example.com", "es": "example.es"}
}
```

#### 4. Configuration Example

**Simple Setup**:
```go
package main

func main() {
    cms := NewCMS(&Config{
        DefaultLocale: "en",
        Locales: []Locale{
            {Code: "en", Name: "English"},
            {Code: "es", Name: "Spanish"},
            {Code: "fr", Name: "French"},
        },
    })
}
```

**Advanced Setup**:
```go
package main

func main() {
    cms := NewCMS(&Config{
        DefaultLocale: "en-US",
        Locales: []Locale{
            {Code: "en-US", Name: "English (US)", Fallback: "en"},
            {Code: "en-GB", Name: "English (UK)", Fallback: "en"},
            {Code: "fr-CA", Name: "French (Canada)", Fallback: "fr"},
            {Code: "fr-FR", Name: "French (France)", Fallback: "fr"},
        },

        // Optional: custom implementations
        LocaleResolver: &RegionalLocaleResolver{
            IPGeolocation: myIPService,
        },
        LocaleFormatter: &RegionalFormatter{},
        URLStrategy: &SubdomainStrategy{
            BaseDomain: "example.com",
        },
    })
}
```

### Key Architectural Decisions

1. **Opaque Locale Codes**: System treats locale codes as strings without parsing format assumptions.

2. **Nullable Fields**: Advanced features use nullable foreign keys and JSONB fields. Simple mode leaves these `NULL`.

3. **Interface-Based**: Locale-specific logic is behind interfaces with default implementations.

4. **Default Configuration**: Functions with simple codes without required setup.

5. **opt in Complexity**: Advanced features require explicit configuration.

6. **Unified Schema**: Simple and complex modes use identical database schema with different data.

### Implementation Philosophy: Progressive Complexity

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

**Upgrade Characteristics**: Each level adds configuration without modifying existing code or data.

**Code Example**:
```go
// Initial implementation
cms := NewCMS(&Config{
    DefaultLocale: "en",
    Locales: []Locale{
        {Code: "en", Name: "English"},
        {Code: "es", Name: "Spanish"},
    },
})

// Add regional support
cms := NewCMS(&Config{
    DefaultLocale: "en-US",
    Locales: []Locale{
        {Code: "en-US", Name: "English (US)", Fallback: "en"},
        {Code: "en-GB", Name: "English (UK)", Fallback: "en"},
        {Code: "es-MX", Name: "Spanish (Mexico)", Fallback: "es"},
        {Code: "es-ES", Name: "Spanish (Spain)", Fallback: "es"},
    },
})

// Add custom fallback logic
cms := NewCMS(&Config{
    DefaultLocale: "en-US",
    Locales: []Locale{ /* same as before */ },
    LocaleGroups: []LocaleGroup{
        {
            Name: "north-america",
            Fallbacks: []string{"en-US", "en-CA", "fr-CA", "es-MX"},
        },
    },
    LocaleFormatter: &CustomFormatter{},
})
```

**Note**: Database schema and existing translations remain unchanged across upgrades. Only configuration changes.

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
    content_id UUID,            -- Links to base content table
    parent_id UUID,             -- Parent page (null for top-level)
    template_slug VARCHAR(100), -- Which template to use for rendering
    menu_order INTEGER,         -- Order among siblings
    page_type TEXT,             -- Special page designation
    page_attributes JSONB,      -- Custom attributes
    publish_on TIMESTAMP,       -- Scheduled publishing
    deleted_at TIMESTAMP,       -- Soft delete
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### Page Translations Table

**Critical for i18n**: Each page can have different URLs per locale for SEO and user experience.

```sql
CREATE TABLE page_translations (
    id UUID PRIMARY KEY,
    page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    path TEXT NOT NULL,
    slug VARCHAR(255) NOT NULL,
    meta JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(page_id, locale_id),
    UNIQUE(path, locale_id)
);
```

**Field Details**:

**`path`**: The complete URL path for this locale. Must be unique per locale.
- en-US: `/about-us`
- fr-FR: `/a-propos`
- de-DE: `/uber-uns`

**`slug`**: The path segment for this page (last part of path).
- If parent path is `/company` and slug is `team`, path becomes `/company/team`

**`meta`**: locale specific metadata:
```json
{
  "canonical_url": "https://example.com/about-us",
  "hreflang_alternates": {
    "fr-FR": "https://example.fr/a-propos",
    "de-DE": "https://example.de/uber-uns"
  },
  "og_image": "media-uuid-for-locale",
  "breadcrumb_label": "About Us"
}
```

#### Field Details:

**`content_id`**: References the content table which holds the actual content data. This separation allows pages to inherit all content features (translations, versioning, status).

**`parent_id`**: Creates the hierarchy. When null, the page is at the root level.

**`template_slug`**: Determines which template renders this page. Examples: "default", "full-width", "sidebar-left", "landing-hero".

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

### Page Example with Multilingual URLs

**Simple Locale Example** (most common):
```json
{
  "page": {
    "id": "823e4567-e89b-12d3-a456-426614174000",
    "content_id": "323e4567-e89b-12d3-a456-426614174000",
    "parent_id": null,
    "template_slug": "default",
    "menu_order": 1,
    "page_type": "page",
    "page_attributes": {
      "show_sidebar": true,
      "layout": "container"
    }
  },
  "translations": [
    {
      "locale": "en",
      "path": "/about-us",
      "slug": "about-us"
    },
    {
      "locale": "es",
      "path": "/acerca-de",
      "slug": "acerca-de"
    },
    {
      "locale": "fr",
      "path": "/a-propos",
      "slug": "a-propos"
    }
  ]
}
```

**Regional Locale Example** (when needed):
```json
{
  "page": {
    "id": "823e4567-e89b-12d3-a456-426614174000",
    "content_id": "323e4567-e89b-12d3-a456-426614174000",
    "parent_id": null,
    "template_slug": "default",
    "menu_order": 1,
    "page_type": "page"
  },
  "translations": [
    {
      "locale": "en-US",
      "path": "/about-us",
      "slug": "about-us",
      "meta": {
        "canonical_url": "https://example.com/about-us",
        "hreflang": {
          "en-US": "https://example.com/about-us",
          "en-GB": "https://example.co.uk/about-us",
          "fr-CA": "https://example.ca/a-propos",
          "fr-FR": "https://example.fr/a-propos"
        },
        "og_title": "About Us | Example Company"
      }
    },
    {
      "locale": "en-GB",
      "path": "/about-us",
      "slug": "about-us",
      "meta": {
        "canonical_url": "https://example.co.uk/about-us",
        "og_title": "About Us | Example Company"
      }
    },
    {
      "locale": "fr-CA",
      "path": "/a-propos",
      "slug": "a-propos",
      "meta": {
        "canonical_url": "https://example.ca/a-propos",
        "og_title": "À propos | Example Company"
      }
    }
  ]
}
```

**Note**: Schema is identical for both modes. The `locale` field accepts any string format.

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
    attributes JSONB,          -- Configuration ONLY (not translatable text)
    is_reusable BOOLEAN,       -- Saved for reuse
    name VARCHAR(200),         -- Name if reusable
    publish_on TIMESTAMP,      -- Scheduled publishing
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Critical i18n Note**: The `attributes` field contains ONLY non translatable configuration (layout, styling, behavior). All translatable content goes in `block_translations`.

**Attributes Field Guidelines**:
- **Store in attributes**: layout settings, column counts, padding, alignment, colors, enable/disable flags
- **Do NOT store in attributes**: text content, labels, button text, URLs, alt text
- **Rule**: All user facing text must go in `block_translations.content`

### Block Translations Table

**Enhanced for i18n**: Separates translatable content from configuration and supports locale specific attribute overrides.

```sql
CREATE TABLE block_translations (
    id UUID PRIMARY KEY,
    block_instance_id UUID NOT NULL REFERENCES block_instances(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    content JSONB NOT NULL,              -- All translatable text
    attribute_overrides JSONB,           -- locale specific config overrides (e.g., media_id)
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(block_instance_id, locale_id)
);
```

**Field Details**:

**`content`**: All translatable text for this block:
```json
{
  "headline": "Welcome to Our Platform",
  "subheadline": "We build amazing things",
  "button_text": "Get Started",
  "button_url": "/get-started",
  "alt_text": "Hero image showing our product"
}
```

**`attribute_overrides`**: locale specific configuration overrides (typically for media):
```json
{
  "hero_image_id": "spanish-hero-uuid",
  "background_video_id": "spanish-video-uuid"
}
```

**Rationale**:
- Configuration in `attributes` applies to all locales
- Only text content and locale specific media require translation
- Bulk updates: changing alignment affects all locales
- Prevents layout inconsistencies between languages

### Block Type Translations Table

**New**: Translates block type names and field labels for the admin UI.

```sql
CREATE TABLE block_type_translations (
    id UUID PRIMARY KEY,
    block_type_id UUID NOT NULL REFERENCES block_types(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    field_labels JSONB,              -- Translated field labels for editor
    help_text JSONB,                 -- Translated help text
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(block_type_id, locale_id)
);
```

**Purpose**: When a French editor edits an English page, they see block types and field names in French.

**`field_labels`** example (French):
```json
{
  "headline": "Titre principal",
  "subheadline": "Sous-titre",
  "button_text": "Texte du bouton",
  "button_url": "URL du bouton",
  "alignment": "Alignement"
}
```

**`help_text`** example (French):
```json
{
  "headline": "Le titre principal affiché en haut du bloc",
  "button_url": "L'URL vers laquelle le bouton redirige"
}
```

#### Parent Types:

The `parent_type` field determines what kind of container holds the block. Understanding this is critical for querying and rendering blocks correctly.

##### 1. `parent_type = "content"`

The block belongs directly to a page or post (any content item from the `contents` table).

**Characteristics**:
- Most common case for top-level blocks
- `parent_id` references a `content_id` from the `contents` table
- These blocks appear in the main content area of a page

**Example**: A hero block at the top of an "About Us" page
```json
{
  "id": "block-abc-123",
  "block_type_id": "hero-type-uuid",
  "parent_id": "content-xyz-789",
  "parent_type": "content",
  "order_index": 0
}
```

**Query Pattern**:
```sql
-- Get all blocks for a page's content
SELECT bi.*, bt.content
FROM block_instances bi
JOIN block_translations bt ON bi.id = bt.block_instance_id
WHERE bi.parent_id = ?
  AND bi.parent_type = 'content'
  AND bt.locale_id = ?
ORDER BY bi.order_index;
```

##### 2. `parent_type = "block"`

The block is nested inside another block (composite block pattern).

**Characteristics**:
- Used for blocks that contain other blocks (columns, groups, accordions)
- `parent_id` references another `block_instance_id`
- Creates hierarchical block structures

**Example**: Two column blocks inside a columns container block
```json
{
  "columns_container": {
    "id": "columns-block-456",
    "block_type_id": "columns-type-uuid",
    "parent_id": "content-xyz-789",
    "parent_type": "content",
    "attributes": {
      "columns": 2,
      "gap": "medium"
    }
  },
  "child_blocks": [
    {
      "id": "column-left-789",
      "block_type_id": "column-type-uuid",
      "parent_id": "columns-block-456",
      "parent_type": "block",
      "order_index": 0
    },
    {
      "id": "column-right-012",
      "block_type_id": "column-type-uuid",
      "parent_id": "columns-block-456",
      "parent_type": "block",
      "order_index": 1
    }
  ]
}
```

**Query Pattern** (recursive):
```sql
-- Get block with all nested children
WITH RECURSIVE block_tree AS (
  -- Start with parent block
  SELECT * FROM block_instances WHERE id = ?
  UNION ALL
  -- Recursively get children
  SELECT bi.*
  FROM block_instances bi
  JOIN block_tree bt ON bi.parent_id = bt.id
  WHERE bi.parent_type = 'block'
)
SELECT * FROM block_tree ORDER BY order_index;
```

##### 3. `parent_type = "widget"`

The block is contained within a widget instance.

**Characteristics**:
- Allows widgets to have rich, block-based content
- `parent_id` references a `widget_instance_id` from `widget_instances` table
- Widgets can use blocks for flexible content layouts

**Example**: A call-to-action block inside a promotional widget
```json
{
  "widget_instance": {
    "id": "widget-promo-123",
    "widget_type_id": "promo-widget-type",
    "area": "sidebar"
  },
  "block_in_widget": {
    "id": "block-cta-456",
    "block_type_id": "button-type-uuid",
    "parent_id": "widget-promo-123",
    "parent_type": "widget",
    "order_index": 0
  }
}
```

**Query Pattern**:
```sql
-- Get all blocks for a widget
SELECT bi.*, bt.content
FROM block_instances bi
JOIN block_translations bt ON bi.id = bt.block_instance_id
WHERE bi.parent_id = ?
  AND bi.parent_type = 'widget'
  AND bt.locale_id = ?
ORDER BY bi.order_index;
```

#### Parent Type Summary

| parent_type | parent_id references | Use Case | Query Target |
|-------------|---------------------|----------|--------------|
| `content`   | `contents.id`       | Top-level page/post blocks | Main content area |
| `block`     | `block_instances.id`| Nested blocks | Recursive tree queries |
| `widget`    | `widget_instances.id`| Blocks within widgets | Widget content areas |

### Block Nesting Example with Translations

A columns block containing paragraphs, with proper config/content separation:

```json
{
  "columns_block": {
    "id": "col-block-123",
    "block_type_id": "columns-type",
    "parent_id": "content-page-123",
    "parent_type": "content",
    "order_index": 0,
    "attributes": {
      "columns": 2,
      "gap": "medium",
      "equal_height": true
    }
  },
  "child_blocks": [
    {
      "id": "para-block-1",
      "block_type_id": "paragraph-type",
      "parent_id": "col-block-123",
      "parent_type": "block",
      "order_index": 0,
      "attributes": {
        "alignment": "left",
        "font_size": "medium"
      },
      "translations": [
        {
          "locale": "en-US",
          "content": {
            "text": "Our company was founded in 2010 with a mission to innovate."
          }
        },
        {
          "locale": "fr-FR",
          "content": {
            "text": "Notre entreprise a été fondée en 2010 avec pour mission d'innover."
          }
        }
      ]
    },
    {
      "id": "para-block-2",
      "block_type_id": "paragraph-type",
      "parent_id": "col-block-123",
      "parent_type": "block",
      "order_index": 1,
      "attributes": {
        "alignment": "left",
        "font_size": "medium"
      },
      "translations": [
        {
          "locale": "en-US",
          "content": {
            "text": "Today we serve over 10,000 customers worldwide."
          }
        },
        {
          "locale": "fr-FR",
          "content": {
            "text": "Aujourd'hui, nous servons plus de 10 000 clients dans le monde."
          }
        }
      ]
    }
  ]
}
```

**Structure**:
- Columns block `attributes` (columns, gap) contain configuration only
- Paragraph blocks separate `attributes` (alignment, font_size) from `translations`
- Each child block includes translations
- Layout configuration is locale-independent

### Common Block Types with i18n Examples

#### Hero Block (with Media Locale Variants)

Shows proper separation of config from content and locale specific media:

```json
{
  "block_instance": {
    "id": "hero-123",
    "block_type_id": "hero-block-type",
    "parent_id": "page-content-456",
    "parent_type": "content",
    "attributes": {
      "layout": "centered",
      "padding": "large",
      "background_color": "#f0f0f0",
      "enable_parallax": true,
      "hero_image_id": "default-hero-image-uuid"
    }
  },
  "translations": [
    {
      "locale": "en-US",
      "content": {
        "headline": "Welcome to Our Platform",
        "subheadline": "We build amazing things",
        "button_text": "Get Started",
        "button_url": "/get-started"
      }
    },
    {
      "locale": "es-ES",
      "content": {
        "headline": "Bienvenido a Nuestra Plataforma",
        "subheadline": "Construimos cosas increíbles",
        "button_text": "Comenzar",
        "button_url": "/comenzar"
      },
      "attribute_overrides": {
        "hero_image_id": "spanish-hero-image-uuid"
      }
    },
    {
      "locale": "ja-JP",
      "content": {
        "headline": "私たちのプラットフォームへようこそ",
        "subheadline": "私たちは素晴らしいものを作ります",
        "button_text": "始める",
        "button_url": "/start"
      },
      "attribute_overrides": {
        "hero_image_id": "japanese-hero-image-uuid"
      }
    }
  ]
}
```

**Note**: Spanish and Japanese use different hero images because the original contains embedded English text.

#### Paragraph Block
```json
{
  "slug": "paragraph",
  "category": "text",
  "attributes": {
    "alignment": "left",
    "dropCap": false,
    "font_size": "medium"
  },
  "translations": [
    {
      "locale": "en-US",
      "content": {"text": "Text content here"}
    },
    {
      "locale": "fr-FR",
      "content": {"text": "Contenu textuel ici"}
    }
  ]
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

**Critical for i18n**: Widget `settings` contains ONLY non translatable configuration. Translatable content goes in `widget_translations`.

```sql
CREATE TABLE widget_instances (
    id UUID PRIMARY KEY,
    widget_type_id UUID,       -- Which widget type
    area VARCHAR(100),         -- Widget area placement
    settings JSONB,            -- Configuration ONLY (behavior, not text)
    visibility_rules JSONB,    -- When to show
    order_index INTEGER,       -- Position in area
    is_active BOOLEAN,         -- Enabled/disabled
    publish_on TIMESTAMP,      -- Scheduled activation
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**Settings Field Guidelines**:
- **Store in settings**: behavior flags (show_date, count, sorting), technical config
- **Do NOT store in settings**: titles, labels, help text, content
- **Rule**: All user-visible text goes in `widget_translations`

### Widget Translations Table

**Enhanced for i18n**: Separates widget title, translatable settings (UI labels), and content.

```sql
CREATE TABLE widget_translations (
    id UUID PRIMARY KEY,
    widget_instance_id UUID NOT NULL REFERENCES widget_instances(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id),
    title VARCHAR(200),              -- Widget title
    translatable_settings JSONB,     -- UI labels that need translation
    content JSONB,                   -- Widget-specific content
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(widget_instance_id, locale_id)
);
```

**Field Details**:

**`title`**: The widget title displayed to users:
- en-US: "Recent Posts"
- fr-FR: "Articles Récents"
- de-DE: "Neueste Beiträge"

**`translatable_settings`**: UI labels and text within widget settings:
```json
{
  "date_label": "Published on",
  "read_more_text": "Read more",
  "no_posts_message": "No posts found",
  "category_label": "Category"
}
```

**`content`**: Widget-specific translatable content:
```json
{
  "intro_text": "Check out our latest articles",
  "footer_link_text": "View all posts",
  "footer_link_url": "/blog"
}
```

**Why Three Separate Fields?**
1. `title`: Always translatable, every widget has one
2. `translatable_settings`: Labels shown in the widget UI
3. `content`: Rich content specific to widget type (custom HTML, contact info, etc.)

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

### Common Widget Types with i18n Examples

#### Contact Info Widget

Shows complete separation of configuration from translatable content:

```json
{
  "widget_instance": {
    "id": "contact-widget-789",
    "widget_type_id": "contact-info-type",
    "area": "sidebar",
    "settings": {
      "show_map": true,
      "map_zoom": 15,
      "map_style": "street",
      "layout": "vertical",
      "icon_style": "outlined"
    }
  },
  "translations": [
    {
      "locale": "en-US",
      "title": "Contact Us",
      "translatable_settings": {
        "address_label": "Our Office",
        "phone_label": "Call Us",
        "email_label": "Email",
        "hours_label": "Business Hours"
      },
      "content": {
        "address": "123 Main St, New York, NY 10001",
        "phone": "+1-555-0100",
        "email": "info@example.com",
        "hours": "Mon-Fri 9AM-5PM EST"
      }
    },
    {
      "locale": "fr-FR",
      "title": "Contactez-nous",
      "translatable_settings": {
        "address_label": "Notre Bureau",
        "phone_label": "Appelez-nous",
        "email_label": "Courriel",
        "hours_label": "Heures d'ouverture"
      },
      "content": {
        "address": "45 Rue de la Paix, 75002 Paris",
        "phone": "+33-1-23-45-67-89",
        "email": "info@example.fr",
        "hours": "Lun-Ven 9h-17h CET"
      }
    }
  ]
}
```

**Structure**:
- `settings` contains technical config (map_zoom, icon_style) - not translated
- `translatable_settings` contains UI labels - translated
- `content` contains widget content - translated and locale-specific

#### Recent Posts Widget
```json
{
  "widget_instance": {
    "slug": "recent-posts",
    "settings": {
      "count": 5,
      "category": "news",
      "show_date": true,
      "show_excerpt": true,
      "excerpt_length": 55
    }
  },
  "translations": [
    {
      "locale": "en-US",
      "title": "Latest Articles",
      "translatable_settings": {
        "date_label": "Published",
        "read_more_text": "Read more"
      }
    },
    {
      "locale": "fr-FR",
      "title": "Derniers Articles",
      "translatable_settings": {
        "date_label": "Publié",
        "read_more_text": "Lire la suite"
      }
    }
  ]
}
```

#### Search Widget
```json
{
  "widget_instance": {
    "slug": "search",
    "settings": {
      "search_content_types": ["page", "post"],
      "min_characters": 3,
      "show_suggestions": true
    }
  },
  "translations": [
    {
      "locale": "en-US",
      "title": "Search",
      "translatable_settings": {
        "placeholder": "Search...",
        "button_text": "Go",
        "no_results_text": "No results found"
      }
    },
    {
      "locale": "es-ES",
      "title": "Buscar",
      "translatable_settings": {
        "placeholder": "Buscar...",
        "button_text": "Ir",
        "no_results_text": "No se encontraron resultados"
      }
    }
  ]
}
```

#### Navigation Menu Widget
```json
{
  "widget_instance": {
    "slug": "nav-menu",
    "settings": {
      "menu_id": "footer-menu-uuid",
      "show_hierarchy": true,
      "depth": 2
    }
  },
  "translations": [
    {
      "locale": "en-US",
      "title": "Quick Links"
    },
    {
      "locale": "de-DE",
      "title": "Schnelllinks"
    }
  ]
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

### Translation Service Interface

Core service for handling translations with fallback logic:

```go
package i18n

import (
    "context"
    "fmt"
)

// TranslationService handles all translation operations
type TranslationService interface {
    // GetContentTranslation retrieves translation with automatic fallback
    GetContentTranslation(ctx context.Context, contentID, locale string) (*ContentTranslation, error)

    // GetBlockTranslation retrieves block translation with attribute overrides
    GetBlockTranslation(ctx context.Context, blockID, locale string) (*BlockTranslation, error)

    // GetWidgetTranslation retrieves widget translation
    GetWidgetTranslation(ctx context.Context, widgetID, locale string) (*WidgetTranslation, error)

    // GetFallbackChain returns the fallback locale chain for a locale
    GetFallbackChain(locale string) []string

    // BuildTranslationContext builds complete context for page rendering
    BuildTranslationContext(ctx context.Context, pageID, locale string) (*TranslationContext, error)
}

type translationService struct {
    db    Database
    cache CacheProvider
}

func (s *translationService) GetContentTranslation(
    ctx context.Context,
    contentID, locale string,
) (*ContentTranslation, error) {
    // Try cache first
    cacheKey := fmt.Sprintf("content:%s:locale:%s", contentID, locale)
    if cached, ok := s.cache.Get(cacheKey); ok {
        return cached.(*ContentTranslation), nil
    }

    // Try requested locale
    trans, err := s.db.GetContentTranslation(ctx, contentID, locale)
    if err == nil {
        s.cache.Set(cacheKey, trans, 1*time.Hour)
        return trans, nil
    }

    // Try fallback chain
    for _, fallbackLocale := range s.GetFallbackChain(locale) {
        trans, err = s.db.GetContentTranslation(ctx, contentID, fallbackLocale)
        if err == nil {
            log.Warn("Using fallback locale",
                "content_id", contentID,
                "requested", locale,
                "fallback", fallbackLocale)
            return trans, nil
        }
    }

    return nil, fmt.Errorf("no translation found for content %s", contentID)
}

func (s *translationService) GetFallbackChain(locale string) []string {
    // Load from locale_groups table
    group := s.db.GetLocaleGroup(locale)
    if group != nil {
        return group.FallbackOrder
    }

    // Default fallback: try base language, then default
    if len(locale) > 2 {
        baseLocale := locale[:2] // "en-US" → "en"
        return []string{baseLocale, "en-US"}
    }

    return []string{"en-US"}
}
```

### Multilingual URL Router

Resolves pages by locale specific paths:

```go
package router

import (
    "context"
    "fmt"
)

// MultilingualRouter handles locale specific URL routing
type MultilingualRouter interface {
    // ResolvePage finds page by path and locale
    ResolvePage(ctx context.Context, path, locale string) (*Page, error)

    // GetLocaleURLs returns all locale URLs for a page
    GetLocaleURLs(ctx context.Context, pageID string) map[string]string

    // GenerateURL creates locale specific URL for a page
    GenerateURL(ctx context.Context, pageID, locale string) (string, error)

    // RedirectToLocale finds equivalent page in target locale
    RedirectToLocale(ctx context.Context, currentPath, fromLocale, toLocale string) (string, error)
}

type router struct {
    db    Database
    cache CacheProvider
}

func (r *router) ResolvePage(ctx context.Context, path, locale string) (*Page, error) {
    // Cache key includes both path and locale
    cacheKey := fmt.Sprintf("page:path:%s:locale:%s", path, locale)
    if cached, ok := r.cache.Get(cacheKey); ok {
        return cached.(*Page), nil
    }

    // Query with join to get page via locale specific path
    query := `
        SELECT p.*, pt.path, pt.slug, pt.meta
        FROM pages p
        JOIN page_translations pt ON p.id = pt.page_id
        JOIN locales l ON pt.locale_id = l.id
        WHERE pt.path = ? AND l.code = ? AND p.deleted_at IS NULL
    `

    page, err := r.db.QueryPage(ctx, query, path, locale)
    if err != nil {
        return nil, fmt.Errorf("page not found: %s (%s)", path, locale)
    }

    r.cache.Set(cacheKey, page, 1*time.Hour)
    return page, nil
}

func (r *router) GetLocaleURLs(ctx context.Context, pageID string) map[string]string {
    query := `
        SELECT l.code, pt.path
        FROM page_translations pt
        JOIN locales l ON pt.locale_id = l.id
        WHERE pt.page_id = ?
    `

    urls := make(map[string]string)
    rows, _ := r.db.Query(ctx, query, pageID)
    defer rows.Close()

    for rows.Next() {
        var locale, path string
        rows.Scan(&locale, &path)
        urls[locale] = path
    }

    return urls
}

func (r *router) RedirectToLocale(
    ctx context.Context,
    currentPath, fromLocale, toLocale string,
) (string, error) {
    // Find current page
    page, err := r.ResolvePage(ctx, currentPath, fromLocale)
    if err != nil {
        return "", err
    }

    // Get URL in target locale
    return r.GenerateURL(ctx, page.ID, toLocale)
}
```

### Building Complete Translation Context

Load all translations for a page in a single operation:

```go
package cms

// TranslationContext contains all translations needed to render a page
type TranslationContext struct {
    Page              *Page
    PageTranslation   *PageTranslation
    ContentTranslation *ContentTranslation
    Blocks            []*BlockWithTranslation
    Widgets           []*WidgetWithTranslation
    Locale            *Locale
    FallbackChain     []string
}

func (s *CMSService) BuildPageContext(
    ctx context.Context,
    pageID, locale string,
) (*TranslationContext, error) {
    // Parallel fetching for performance
    var (
        page      *Page
        pageTrans *PageTranslation
        content   *ContentTranslation
        blocks    []*BlockWithTranslation
        widgets   []*WidgetWithTranslation
        err       error
    )

    // Use goroutines to fetch in parallel
    g, gctx := errgroup.WithContext(ctx)

    g.Go(func() error {
        page, err = s.Pages.Get(gctx, pageID)
        return err
    })

    g.Go(func() error {
        pageTrans, err = s.I18n.GetPageTranslation(gctx, pageID, locale)
        return err
    })

    g.Go(func() error {
        // Wait for page first
        <-time.After(10 * time.Millisecond)
        content, err = s.I18n.GetContentTranslation(gctx, page.ContentID, locale)
        return err
    })

    g.Go(func() error {
        blocks, err = s.Blocks.GetForContentWithTranslations(gctx, page.ContentID, locale)
        return err
    })

    g.Go(func() error {
        widgets, err = s.Widgets.GetActiveForPage(gctx, pageID, locale)
        return err
    })

    if err := g.Wait(); err != nil {
        return nil, err
    }

    return &TranslationContext{
        Page:              page,
        PageTranslation:   pageTrans,
        ContentTranslation: content,
        Blocks:            blocks,
        Widgets:           widgets,
        Locale:            s.I18n.GetLocale(locale),
        FallbackChain:     s.I18n.GetFallbackChain(locale),
    }, nil
}
```

### Efficient Batch Loading with Fallbacks

Avoid N+1 queries when loading translations:

```go
// GetBlocksWithTranslations loads blocks with translations in a single query
func (s *BlockService) GetBlocksWithTranslations(
    ctx context.Context,
    parentID, locale string,
) ([]*BlockWithTranslation, error) {
    fallbackChain := s.i18n.GetFallbackChain(locale)

    // Build locale list for IN clause
    locales := append([]string{locale}, fallbackChain...)

    query := `
        WITH block_trans AS (
            SELECT DISTINCT ON (bi.id)
                bi.id as block_id,
                bt.content,
                bt.attribute_overrides,
                l.code as locale,
                CASE
                    WHEN l.code = ? THEN 0
                    ELSE array_position(?, l.code)
                END as priority
            FROM block_instances bi
            LEFT JOIN block_translations bt ON bi.id = bt.block_instance_id
            LEFT JOIN locales l ON bt.locale_id = l.id
            WHERE bi.parent_id = ?
              AND bi.parent_type = 'content'
              AND l.code = ANY(?)
            ORDER BY bi.id, priority
        )
        SELECT
            bi.*,
            bt.content,
            bt.attribute_overrides,
            bt.locale
        FROM block_instances bi
        JOIN block_trans bt ON bi.id = bt.block_id
        WHERE bi.parent_id = ?
        ORDER BY bi.order_index
    `

    rows, err := s.db.Query(ctx, query, locale, locales, parentID, locales, parentID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var blocks []*BlockWithTranslation
    for rows.Next() {
        block := &BlockWithTranslation{}
        err := rows.Scan(
            &block.ID,
            &block.BlockTypeID,
            &block.Attributes,
            &block.Content,
            &block.AttributeOverrides,
            &block.Locale,
        )
        if err != nil {
            return nil, err
        }

        // Merge attribute_overrides into attributes
        if block.AttributeOverrides != nil {
            block.MergedAttributes = mergeAttributes(block.Attributes, block.AttributeOverrides)
        } else {
            block.MergedAttributes = block.Attributes
        }

        blocks = append(blocks, block)
    }

    return blocks, nil
}

func mergeAttributes(base, overrides map[string]any) map[string]any {
    merged := make(map[string]any)

    // Copy base attributes
    for k, v := range base {
        merged[k] = v
    }

    // Apply overrides
    for k, v := range overrides {
        merged[k] = v
    }

    return merged
}
```

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

### Internationalization Best Practices

#### 1. Translation Workflow

**Establish Clear Workflow Stages**:
```
Content Creation → Translation Request → Translation → Review → Approval → Publish
```

**Use Translation Status Tracking**:
```sql
-- Query incomplete translations
SELECT
    ts.entity_type,
    ts.entity_id,
    l.code,
    ts.status,
    ts.completeness
FROM translation_status ts
JOIN locales l ON ts.locale_id = l.id
WHERE ts.status IN ('missing', 'draft')
ORDER BY ts.entity_type, ts.completeness DESC;
```

**Guidelines**:
- Verify all required locale translations reach 100% completeness before publishing
- Use `draft` status for partial translations
- Assign translators and reviewers for accountability tracking
- Configure automated notifications for translations requiring review

#### 2. Configuration vs Content Separation

**Decision Criteria**: Determine if field value varies by language.

**Configuration (Not Translated)**:
```json
{
  "layout": "grid",
  "columns": 3,
  "gap": "medium",
  "enable_animation": true,
  "animation_speed": 300
}
```

**Content (Translated)**:
```json
{
  "en-US": {
    "headline": "Our Products",
    "description": "Browse our selection"
  },
  "ja-JP": {
    "headline": "私たちの製品",
    "description": "選択を閲覧する"
  }
}
```

**Common Errors**:
- Duplicating layout settings across locales
- Varying column counts per language without requirement
- Translating CSS classes or technical identifiers

#### 3. Handling Missing Translations

**Fallback Strategy**:
```go
func GetTranslation(contentID, locale string) *Translation {
    // 1. Try requested locale
    if trans := tryLocale(contentID, locale); trans != nil {
        return trans
    }

    // 2. Try fallback chain
    for _, fallbackLocale := range getFallbackChain(locale) {
        if trans := tryLocale(contentID, fallbackLocale); trans != nil {
            logMissingTranslation(contentID, locale)
            return trans
        }
    }

    // 3. Return default locale
    return tryLocale(contentID, defaultLocale)
}
```

**Display Strategy**:
- Display content in fallback language
- Add visual indicator: `<span class="fallback-locale">Content in English</span>`
- Log missing translations

#### 4. SEO Best Practices

**Hreflang Implementation**:
```html
<link rel="alternate" hreflang="en-US" href="https://example.com/about-us" />
<link rel="alternate" hreflang="fr-FR" href="https://example.fr/a-propos" />
<link rel="alternate" hreflang="de-DE" href="https://example.de/uber-uns" />
<link rel="alternate" hreflang="x-default" href="https://example.com/about-us" />
```

**Canonical URLs**:
```html
<!-- Each locale version declares itself as canonical -->
<link rel="canonical" href="https://example.fr/a-propos" />
```

**Sitemap Generation**:
```xml
<!-- Generate separate sitemaps per locale -->
<sitemap>
  <loc>https://example.com/sitemap-en-US.xml</loc>
</sitemap>
<sitemap>
  <loc>https://example.fr/sitemap-fr-FR.xml</loc>
</sitemap>
```

#### 5. RTL Language Support

**Template Considerations**:
```html
<html dir="{{ if .Locale.IsRTL }}rtl{{ else }}ltr{{ end }}" lang="{{ .Locale.Code }}">
```

**CSS Adjustments**:
```css
/* Use logical properties for RTL compatibility */
.content {
  margin-inline-start: 2rem;  /* Instead of margin-left */
  padding-inline-end: 1rem;   /* Instead of padding-right */
}

/* RTL-specific overrides */
[dir="rtl"] .arrow-icon {
  transform: scaleX(-1);  /* Flip directional icons */
}
```

**Common RTL Issues**:
- Icons pointing wrong direction
- Text alignment defaults
- Sidebar on wrong side
- Form field order

#### 6. locale specific Formatting

**Use Locale-Aware Libraries**:
```go
import "golang.org/x/text/language"
import "golang.org/x/text/message"

// Format numbers per locale
p := message.NewPrinter(language.German)
p.Printf("%d", 1234567)  // "1.234.567"

p = message.NewPrinter(language.English)
p.Printf("%d", 1234567)  // "1,234,567"
```

**Date Formatting**:
```go
// Don't hardcode date formats
// ❌ Bad
fmt.Sprintf("%d/%d/%d", month, day, year)

// ✅ Good
time.Now().Format(getLocaleDateFormat(locale))
```

**Currency Display**:
```go
// Store amounts in cents/smallest unit
// Format according to locale
formatCurrency(1234, "USD", "en-US")  // "$12.34"
formatCurrency(1234, "EUR", "de-DE")  // "12,34 €"
formatCurrency(1234, "JPY", "ja-JP")  // "¥1,234"
```

#### 7. Translation Testing Strategy

**Test Checklist**:
- [ ] All text is translatable (no hardcoded strings)
- [ ] Fallback chain works correctly
- [ ] Layout doesn't break with longer text (German often 30% longer than English)
- [ ] RTL languages display correctly
- [ ] Numbers, dates, currency format correctly
- [ ] locale specific media loads correctly
- [ ] URLs work in all locales
- [ ] Form validation messages are translated
- [ ] SEO tags (meta descriptions, titles) are translated

**Automated Tests**:
```go
func TestTranslationCompleteness(t *testing.T) {
    requiredLocales := []string{"en-US", "fr-FR", "de-DE"}

    for _, page := range getAllPages() {
        for _, locale := range requiredLocales {
            trans := getPageTranslation(page.ID, locale)
            assert.NotNil(t, trans,
                "Missing translation for page %s in locale %s",
                page.ID, locale)
        }
    }
}
```

#### 8. Performance Optimization for Multilingual Sites

**Eager Loading**:
```sql
-- Load page with translations in one query
SELECT
    p.*,
    pt.path,
    pt.meta,
    l.code as locale
FROM pages p
JOIN page_translations pt ON p.id = pt.page_id
JOIN locales l ON pt.locale_id = l.id
WHERE p.id = ? AND l.code IN (?, ?, ?);  -- Include fallback chain
```

**Cache Strategy**:
```go
// Include locale in cache key
cacheKey := fmt.Sprintf("page:%s:locale:%s:v%d", pageID, locale, version)

// Warm cache for popular locales
for _, locale := range []string{"en-US", "es-ES", "fr-FR"} {
    warmCache(pageID, locale)
}
```

**CDN Configuration**:
```
# Vary cache by Accept-Language header
Vary: Accept-Language

# Or use locale specific paths
/en-US/assets/...
/fr-FR/assets/...
```

### Block Design
1. Keep blocks atomic and single-purpose
2. Use nesting for complex layouts
3. Make blocks reusable when patterns emerge
4. Validate block data against schema
5. Provide sensible defaults
6. **Separate configuration from translatable content**
7. **Use `attribute_overrides` for locale specific media**

### Page Organization
1. Use hierarchy to reflect site structure
2. Keep paths short and meaningful
3. Use page types for special behaviors
4. Leverage page attributes for flexibility
5. Plan for URL changes (redirects)
6. **Create locale specific paths in `page_translations`**
7. **Maintain consistent hierarchy across locales**

### Widget Strategy
1. Use visibility rules to reduce clutter
2. Group related widgets in areas
3. Consider performance (cache widget output)
4. Make widgets responsive
5. Test visibility rules thoroughly
6. **Separate `settings` from `translatable_settings`**
7. **Consider locale specific visibility rules**

### Performance Considerations
1. Lazy load blocks below the fold
2. Cache rendered widget output
3. Optimize block queries (avoid N+1)
4. Use CDN for block assets
5. Implement fragment caching for complex blocks
6. **Include locale in all cache keys**
7. **Eager load translations with fallback chain**
8. **Use database indexes on locale_id foreign keys**

### Migration from Existing Systems

**Step 1: Audit Current Content**
```sql
-- Identify hardcoded strings
SELECT * FROM blocks WHERE attributes::text LIKE '%text%';

-- Find locale-independent paths
SELECT * FROM pages WHERE path NOT IN (
    SELECT path FROM page_translations
);
```

**Step 2: Separate Config from Content**
```go
// Transform old format
oldBlock := {
    "attributes": {
        "text": "Welcome",
        "alignment": "center"
    }
}

// To new format
newBlock := {
    "attributes": {
        "alignment": "center"  // Config only
    },
    "translations": {
        "en-US": {
            "content": {"text": "Welcome"}
        }
    }
}
```

**Step 3: Create Translations**
- Export all translatable strings
- Send to translation service
- Import translated content
- Validate completeness

**Step 4: Update Queries**
- Add locale parameter to all content queries
- Implement fallback logic
- Update cache keys

**Step 5: Test Thoroughly**
- Verify all locales display correctly
- Test fallback chains
- Performance test with multiple locales

This guide provides comprehensive understanding of implementing and working with a production-ready, multilingual CMS.
