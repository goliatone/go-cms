# CMS Internationalization (i18n) and Localization (l10n) Package Guide

## Table of Contents
1. [Overview](#overview)
2. [Design Philosophy](#design-philosophy)
3. [Locale Architecture](#locale-architecture)
4. [Core Components](#core-components)
5. [Database Schema](#database-schema)
6. [Translation Service](#translation-service)
7. [Locale Resolution](#locale-resolution)
8. [Locale Formatting](#locale-formatting)
9. [URL Routing](#url-routing)
10. [Fallback Strategy](#fallback-strategy)
11. [Performance Optimization](#performance-optimization)
12. [Implementation Roadmap](#implementation-roadmap)
13. [Usage Examples](#usage-examples)
14. [Best Practices](#best-practices)

## Overview

The CMS includes internationalization (i18n) and localization (l10n) as core features. All user-facing content can be translated. The system supports both simple language codes and complex regional locale codes with progressive complexity.

### Key Features

- **Progressive Complexity**: Start simple (`en`, `es`) and add regional variations (`en-US`, `en-GB`) as needed
- **Opaque Locale Codes**: System treats locale codes as strings without parsing assumptions
- **Flexible Fallback Chains**: Support for automatic and custom fallback logic
- **Translation-First**: Every user-facing string is translatable from day one
- **Separation of Concerns**: Configuration (layout, behavior) is NOT translated; content (text, URLs) IS translated
- **Performance Focused**: Built-in caching strategies and query optimization

## Design Philosophy

### Core Principles

1. **Opaque Locale Codes**: The `locale` code is treated as an opaque string. The system does not automatically parse `"en-US"` to infer a relationship with `"en"`. Regional fallbacks are enabled through explicit configuration.

2. **Nullable Fields**: Advanced features use nullable foreign keys and JSONB fields. Simple mode leaves these `NULL`.

3. **Interface-Based**: Locale-specific logic is behind interfaces with default implementations.

4. **Default Configuration**: Functions with simple codes without required setup.

5. **Opt-In Complexity**: Advanced features are inactive by default and enabled via configuration.

6. **Unified Schema**: Simple and complex modes use identical database schema with different data.

7. **Progressive Enhancement**: Start with Level 1, upgrade to Level 2 or 3 as needed without schema changes.

### Progressive Complexity Levels

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

## Locale Architecture

### Level 1: Simple Language Codes (Default)

**Use Case**: Basic multilingual site with one translation per language.

**Locale Codes**: `en`, `es`, `fr`, `de`, `ja`, `ar`

**Features**:
- Single translation per language
- No fallback chains
- URL patterns: `/en/about`, `/es/acerca`
- No configuration required

**Example Configuration**:
```go
cms.AddLocale("en", "English")
cms.AddLocale("es", "Spanish")
cms.AddLocale("fr", "French")
```

### Level 2: Regional Locales (opt-in)

**Use Case**: Regional variations (US English vs UK English, Canadian French vs France French).

**Locale Codes**: `en-US`, `en-GB`, `fr-CA`, `fr-FR`

**Features**:
- Regional locale codes
- Automatic fallback chains (`fr-CA` → `fr` → default)
- Locale-specific formatting (dates, numbers, currency)
- Regional URL patterns: `/en-us/about`, `/en-gb/about`

**Example Configuration**:
```go
cms.AddLocale("en-US", "English (US)", WithFallback("en"))
cms.AddLocale("en-GB", "English (UK)", WithFallback("en"))
cms.AddLocale("fr-CA", "French (Canada)", WithFallback("fr", "en"))
cms.AddLocale("fr-FR", "French (France)", WithFallback("fr", "en"))
```

### Level 3: Custom Fallback Chains (Advanced)

**Use Case**: Multi-region sites with custom fallback logic.

**Example Configuration**:
```go
cms := NewCMS(&Config{
    DefaultLocale: "en-US",
    Locales: []Locale{
        {Code: "en-US", Name: "English (US)"},
        {Code: "fr-CA", Name: "French (Canada)"},
        {Code: "fr-FR", Name: "French (France)"},
        {Code: "fr-BE", Name: "French (Belgium)"},
        {Code: "fr-CH", Name: "French (Switzerland)"},
    },
    Locale: &LocaleConfig{
        Groups: []LocaleGroup{
            {
                Name: "french-markets",
                Fallbacks: []string{"fr-CA", "fr-FR", "fr-BE", "fr-CH", "en-US"},
            },
        },
    },
})
```

## Core Components

### Module Structure

```
internal/i18n/
├── types.go            # Core types (Locale, Translation, etc.)
├── service.go          # Interface + implementation
├── resolver.go         # Locale resolution strategy
├── formatter.go        # Locale formatting strategy
├── repository.go       # Storage interface
└── testdata/
    ├── fallback_chain.json
    └── fallback_chain_output.json
```

### Key Types

```go
// Locale represents a language/region configuration
type Locale struct {
    ID               string
    Code             string  // "en" or "en-US"
    Name             string  // "English" or "English (US)"
    IsActive         bool
    IsDefault        bool
    FallbackLocaleID *string // Optional: for regional fallback
    Metadata         map[string]any // Optional: regional metadata
    CreatedAt        time.Time
}

// LocaleConfig groups all locale-related configuration
type LocaleConfig struct {
    Groups      []LocaleGroup      // Custom fallback groups (Level 3)
    Resolver    LocaleResolver     // Strategy for resolving locale from request
    Formatter   LocaleFormatter    // Strategy for formatting dates, numbers, currency
    URLStrategy URLPatternStrategy // Strategy for generating locale-specific URLs
}

// LocaleGroup defines custom fallback chains for related locales
type LocaleGroup struct {
    Name      string   // Group identifier (e.g., "north-america")
    Fallbacks []string // Ordered list of locale codes to try
}

// Translation represents a translatable content item
type Translation struct {
    ID              string
    EntityType      string // "content", "block", "widget", "page"
    EntityID        string
    LocaleID        string
    Content         map[string]any
    AttributeOverrides map[string]any // For locale-specific media
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// TranslationStatus tracks translation completeness
type TranslationStatus struct {
    ID           string
    EntityType   string
    EntityID     string
    LocaleID     string
    Status       string // "missing", "draft", "review", "approved"
    Completeness int    // 0-100
    LastUpdated  time.Time
    TranslatorID *string
    ReviewerID   *string
}
```

### Understanding Translation Types

The i18n system uses different translation types for different purposes. Understanding when to use each is crucial:

#### 1. Generic `Translation` (Base Type)

**Purpose**: A generic translation record in the database. Rarely used directly in application code.

**Use Case**: Database-level representation for any translatable entity.

```go
// Translation is the base database record
type Translation struct {
    ID              string
    EntityType      string // "content", "block", "widget", "page"
    EntityID        string
    LocaleID        string
    Content         map[string]any
    AttributeOverrides map[string]any
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

**When to use**: Internal repository layer, database queries, generic translation operations.

#### 2. `ContentTranslation` (Specific Type)

**Purpose**: Translation for page/post content with metadata.

**Use Case**: Main content body, titles, meta descriptions for pages and posts.

```go
// ContentTranslation is specifically for page/post content
type ContentTranslation struct {
    ID              string
    ContentID       string
    LocaleCode      string
    Title           string           // Page/post title
    Data            map[string]any   // Main content fields
    MetaTitle       string           // SEO title
    MetaDescription string           // SEO description
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// Example usage
func (s *ContentService) GetTranslated(ctx context.Context, contentID, locale string) (*ContentTranslation, error) {
    // Fetch with fallback
    trans, err := s.repo.GetContentTranslation(ctx, contentID, locale)
    if err != nil {
        // Try fallback chain
        for _, fallbackLocale := range s.i18n.GetFallbackChain(locale) {
            trans, err = s.repo.GetContentTranslation(ctx, contentID, fallbackLocale)
            if err == nil {
                break
            }
        }
    }
    return trans, err
}
```

#### 3. `BlockTranslation` (Specific Type)

**Purpose**: Translation for block content with optional attribute overrides.

**Use Case**: Translatable text in blocks, plus locale-specific media references.

```go
// BlockTranslation is for block content with attribute overrides
type BlockTranslation struct {
    ID                 string
    BlockInstanceID    string
    LocaleCode         string
    Content            map[string]any // Translatable text fields
    AttributeOverrides map[string]any // Locale-specific config (e.g., media_id)
    CreatedAt          time.Time
    UpdatedAt          time.Time
}

// Example: Hero block with locale-specific image
func (s *BlockService) GetWithTranslation(ctx context.Context, blockID, locale string) (*BlockWithTranslation, error) {
    block, err := s.repo.GetBlock(ctx, blockID)
    if err != nil {
        return nil, err
    }

    trans, err := s.repo.GetBlockTranslation(ctx, blockID, locale)
    if err != nil {
        // Try fallback
        trans, _ = s.repo.GetBlockTranslation(ctx, blockID, s.defaultLocale)
    }

    // Merge base attributes with locale-specific overrides
    mergedAttrs := block.Attributes
    if trans.AttributeOverrides != nil {
        for k, v := range trans.AttributeOverrides {
            mergedAttrs[k] = v
        }
    }

    return &BlockWithTranslation{
        Block:              block,
        TranslatedContent:  trans.Content,
        MergedAttributes:   mergedAttrs,
        Locale:             locale,
    }, nil
}

// Example data structure:
// {
//   "block": {
//     "attributes": {
//       "layout": "centered",
//       "hero_image_id": "default-image-uuid"  // Base image
//     }
//   },
//   "translation_en": {
//     "content": {
//       "headline": "Welcome",
//       "button_text": "Get Started"
//     }
//   },
//   "translation_ja": {
//     "content": {
//       "headline": "ようこそ",
//       "button_text": "始める"
//     },
//     "attribute_overrides": {
//       "hero_image_id": "japanese-image-uuid"  // Japanese image with Japanese text
//     }
//   }
// }
```

#### 4. `WidgetTranslation` (Specific Type)

**Purpose**: Translation for widget titles, labels, and content.

**Use Case**: Widget display text, UI labels, widget-specific content.

```go
// WidgetTranslation is for widget UI and content
type WidgetTranslation struct {
    ID                   string
    WidgetInstanceID     string
    LocaleCode           string
    Title                string         // Widget title
    TranslatableSettings map[string]any // UI labels
    Content              map[string]any // Widget-specific content
    CreatedAt            time.Time
    UpdatedAt            time.Time
}

// Example usage
func (s *WidgetService) GetTranslated(ctx context.Context, widgetID, locale string) (*WidgetTranslation, error) {
    trans, err := s.repo.GetWidgetTranslation(ctx, widgetID, locale)
    if err != nil {
        // Fallback to default
        trans, err = s.repo.GetWidgetTranslation(ctx, widgetID, s.defaultLocale)
    }
    return trans, err
}

// Example: Recent Posts widget
// {
//   "widget": {
//     "settings": {
//       "count": 5,              // Not translated (behavior)
//       "show_date": true        // Not translated (behavior)
//     }
//   },
//   "translation_en": {
//     "title": "Latest Articles",
//     "translatable_settings": {
//       "date_label": "Published",
//       "read_more_text": "Read more"
//     }
//   },
//   "translation_es": {
//     "title": "Últimos Artículos",
//     "translatable_settings": {
//       "date_label": "Publicado",
//       "read_more_text": "Leer más"
//     }
//   }
// }
```

#### 5. `TranslationContext` (Composite Type)

**Purpose**: Aggregates all translations needed to render a complete page.

**Use Case**: Page rendering, building full translation context in one operation.

```go
// TranslationContext contains everything needed to render a page
type TranslationContext struct {
    Page               *Page                    // Page structure
    PageTranslation    *PageTranslation        // Page path, slug, meta
    ContentTranslation *ContentTranslation     // Page content
    Blocks             []*BlockWithTranslation // All blocks with translations
    Widgets            []*WidgetWithTranslation // All widgets with translations
    Locale             *Locale                  // Current locale info
    FallbackChain      []string                 // Fallback locales used
}

// BlockWithTranslation combines block with its translation
type BlockWithTranslation struct {
    Block             *Block
    TranslatedContent map[string]any
    MergedAttributes  map[string]any // Base attributes + overrides
    Locale            string
}

// WidgetWithTranslation combines widget with its translation
type WidgetWithTranslation struct {
    Widget            *Widget
    Translation       *WidgetTranslation
    Locale            string
}

// Example: Building complete page context
func (s *CMSService) BuildPageContext(
    ctx context.Context,
    pageID, locale string,
) (*TranslationContext, error) {
    // Fetch page structure
    page, err := s.pages.GetByID(ctx, pageID)
    if err != nil {
        return nil, err
    }

    // Fetch page translation (path, slug, meta)
    pageTrans, err := s.i18n.GetPageTranslation(ctx, pageID, locale)
    if err != nil {
        // Try fallback
        pageTrans, err = s.i18n.GetPageTranslation(ctx, pageID, s.defaultLocale)
    }

    // Fetch content translation (title, body, etc.)
    contentTrans, err := s.i18n.GetContentTranslation(ctx, page.ContentID, locale)
    if err != nil {
        contentTrans, _ = s.i18n.GetContentTranslation(ctx, page.ContentID, s.defaultLocale)
    }

    // Fetch all blocks with translations (in a single query)
    blocks, err := s.blocks.GetForContentWithTranslations(ctx, page.ContentID, locale)
    if err != nil {
        return nil, err
    }

    // Fetch active widgets with translations
    widgets, err := s.widgets.GetActiveForPage(ctx, pageID, locale)
    if err != nil {
        return nil, err
    }

    return &TranslationContext{
        Page:               page,
        PageTranslation:    pageTrans,
        ContentTranslation: contentTrans,
        Blocks:             blocks,
        Widgets:            widgets,
        Locale:             s.i18n.GetLocale(locale),
        FallbackChain:      s.i18n.GetFallbackChain(locale),
    }, nil
}

// Example: Using TranslationContext in template rendering
func (s *RenderService) RenderPage(ctx *TranslationContext) (string, error) {
    data := map[string]any{
        "title":       ctx.ContentTranslation.Title,
        "meta_title":  ctx.ContentTranslation.MetaTitle,
        "meta_desc":   ctx.ContentTranslation.MetaDescription,
        "path":        ctx.PageTranslation.Path,
        "locale":      ctx.Locale.Code,
        "content":     ctx.ContentTranslation.Data,
        "blocks":      ctx.Blocks,
        "widgets":     ctx.Widgets,
    }

    return s.template.Render("page", data)
}
```

### Type Comparison Matrix

| Type | Scope | Primary Use | Contains | Fallback Handling |
|------|-------|-------------|----------|-------------------|
| `Translation` | Generic | Database layer | Entity type + generic content | Repository layer |
| `ContentTranslation` | Content entity | Page/post content | Title, body, meta data | Service layer |
| `BlockTranslation` | Block instance | Block text + media | Content + attribute overrides | Service layer |
| `WidgetTranslation` | Widget instance | Widget UI | Title, labels, widget content | Service layer |
| `TranslationContext` | Full page | Page rendering | Everything above combined | Built during fetch |

### When to Use Each Type

**Use `Translation`**:
- Writing generic repository methods
- Database migration scripts
- Low level translation operations

**Use `ContentTranslation`**:
- Fetching page/post content
- Displaying article titles and bodies
- SEO meta tag generation

**Use `BlockTranslation`**:
- Rendering blocks with translated content
- Handling locale-specific images/media
- Block preview/editing interfaces

**Use `WidgetTranslation`**:
- Rendering widget UI
- Displaying widget titles and labels
- Widget configuration interfaces

**Use `TranslationContext`**:
- Full page rendering
- Building complete page data for templates
- Server-side rendering with all translations
- Minimizing database queries (fetch everything once)

### Translation Type Hierarchy

```
                    TranslationContext (Composite)
                            |
                ┌───────────┴───────────┐
                |                       |
         Page Rendering            API Response
                |                       |
    ┌───────────┼───────────┬───────────┴──────┐
    |           |           |                  |
PageTrans   ContentTrans  BlockTrans      WidgetTrans
    |           |           |                  |
  (path,     (title,     (content,          (title,
   slug,      body,       attribute          labels,
   meta)      meta)       overrides)         content)
    |           |           |                  |
    └───────────┴───────────┴──────────────────┘
                            |
                      Translation (Base)
                      (Database Record)
```

### Data Flow Example

```
Request: GET /en/about-us
    |
    v
1. Resolve Locale: "en"
    |
    v
2. Resolve Page: path="/about-us" + locale="en"
    |
    v
3. Build TranslationContext:
    |
    ├─> Fetch PageTranslation (path, slug, meta)
    ├─> Fetch ContentTranslation (title, body, meta)
    ├─> Fetch BlockTranslation[] (all blocks + content)
    └─> Fetch WidgetTranslation[] (all widgets + content)
    |
    v
4. Render Template with TranslationContext
    |
    v
Response: Fully rendered HTML page
```

## Database Schema

### Locales Table

```sql
CREATE TABLE locales (
    id UUID PRIMARY KEY,
    code VARCHAR(10) UNIQUE NOT NULL,  -- "en" OR "en-US"
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
4. Fallback behavior is inactive if `fallback_locale_id` is `NULL`

### Translation Status Table

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

### URL Redirects Table

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

### Locale Groups Table (Level 3 only)

```sql
-- Only create this if you need complex fallback chains
CREATE TABLE locale_groups (
    id UUID PRIMARY KEY,
    name VARCHAR(100),
    primary_locale_id UUID REFERENCES locales(id),
    fallback_order JSONB  -- ["fr-CA", "fr-FR", "fr", "en"]
);
```

## Translation Service

### Service Interface

```go
package i18n

import "context"

// Service handles all translation operations
type Service interface {
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
```

### Implementation with Fallback Logic

```go
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
```

## Locale Resolution

### LocaleResolver Interface

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

### Simple Implementation

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

### Regional Implementation (opt-in)

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
    // 2. Check IP geolocation (opt-in!)
    if r.ipGeolocation != nil {
        if locale := r.ipGeolocation.GetLocale(req.RemoteAddr); locale != "" {
            return locale
        }
    }
    // 3. Fall back to simple logic
    return r.SimpleLocaleResolver.ResolveFromRequest(req)
}
```

## Locale Formatting

### LocaleFormatter Interface

```go
type LocaleFormatter interface {
    // Format date according to locale conventions
    FormatDate(t time.Time, locale string) string

    // Format number with locale-specific separators
    FormatNumber(n float64, locale string) string

    // Format currency
    FormatCurrency(amount int, currency, locale string) string
}
```

### Simple Implementation

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

### Regional Implementation

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

### Formatting Examples

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

## URL Routing

### Multilingual URL Strategy

**Simple Mode** (Level 1): locale-specific paths with simple codes
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

### URL Pattern Strategies

#### Path Prefix Strategy (default)

```go
// URLs: /en/about, /es/acerca
type PathPrefixStrategy struct{}

func (s *PathPrefixStrategy) GenerateURL(locale, path string) string {
    return fmt.Sprintf("/%s%s", locale, path)
}
```

#### Subdomain Strategy (opt-in)

```go
// URLs: en.example.com/about, es.example.com/about
type SubdomainStrategy struct {
    baseDomain string
}

func (s *SubdomainStrategy) GenerateURL(locale, path string) string {
    return fmt.Sprintf("https://%s.%s%s", locale, s.baseDomain, path)
}
```

#### Domain Strategy (advanced)

```go
// URLs: example.com/about (en), example.es/acerca (es)
type DomainStrategy struct {
    domainMap map[string]string  // {"en": "example.com", "es": "example.es"}
}
```

### Router Implementation

```go
type MultilingualRouter interface {
    // ResolvePage finds page by path and locale
    ResolvePage(ctx context.Context, path, locale string) (*Page, error)

    // GetLocaleURLs returns all locale URLs for a page
    GetLocaleURLs(ctx context.Context, pageID string) map[string]string

    // GenerateURL creates locale-specific URL for a page
    GenerateURL(ctx context.Context, pageID, locale string) (string, error)

    // RedirectToLocale finds equivalent page in target locale
    RedirectToLocale(ctx context.Context, currentPath, fromLocale, toLocale string) (string, error)
}

func (r *router) ResolvePage(ctx context.Context, path, locale string) (*Page, error) {
    // Cache key includes both path and locale
    cacheKey := fmt.Sprintf("page:path:%s:locale:%s", path, locale)
    if cached, ok := r.cache.Get(cacheKey); ok {
        return cached.(*Page), nil
    }

    // Query with join to get page via locale-specific path
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
```

## Fallback Strategy

### Fallback Resolution Logic

```go
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

### Progressive Fallback Examples

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

## Performance Optimization

### Caching Strategies

1. **Translation Cache**: Cache translations by `entity_type:entity_id:locale`
2. **Fallback Cache**: Cache resolved fallback chains
3. **URL Cache**: Cache locale-specific URL mappings
4. **Fragment Caching**: Cache rendered blocks with locale in key

**Cache Key Pattern**:
```
block:{block_id}:{locale}:v{version}
page:{page_id}:{locale}:v{version}
widget:{widget_id}:{locale}:v{version}
```

### Query Optimization

**Problem**: N+1 queries when loading page with multiple blocks and translations.

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

### Efficient Batch Loading

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

    // Execute query and scan results...
}
```

### CDN Distribution

- Serve locale-specific assets from regional CDNs
- URL pattern: `cdn.example.com/{locale}/assets/...`
- Automatic cache warming for popular locales
- Locale-based cache TTLs

## Implementation Roadmap

### Phase 1: Core i18n Infrastructure (Sprint 1)

**Goals**:
- Basic locale management
- Simple locale resolver
- Translation service interface
- Level 1 support (simple codes)

**Tasks**:
1. Create `internal/i18n/types.go` with core types
2. Implement `LocaleRepository` interface
3. Create `SimpleLocaleResolver`
4. Implement basic `TranslationService`
5. Add database migrations for `locales` table
6. Write unit tests with fixtures

**Deliverables**:
- Simple locale codes working (`en`, `es`, `fr`)
- Default locale resolution
- Basic translation lookup (no fallbacks yet)

### Phase 2: Fallback Chain Support (Sprint 2)

**Goals**:
- Automatic fallback for regional codes
- Level 2 support (regional locales)
- Fallback chain resolution

**Tasks**:
1. Extend `Locale` type with `FallbackLocaleID`
2. Implement `GetFallbackChain()` logic
3. Update `TranslationService` to use fallback chains
4. Add `RegionalLocaleResolver`
5. Write fallback chain tests

**Deliverables**:
- Regional locale codes working (`en-US`, `fr-CA`)
- Automatic fallback (`en-US` → `en` → default)
- Cached fallback chains

### Phase 3: URL Routing (Sprint 3)

**Goals**:
- Multilingual URL support
- Path resolution by locale
- URL redirects for locale switching

**Tasks**:
1. Create `MultilingualRouter` interface
2. Implement `PathPrefixStrategy`
3. Add `page_translations` support for paths
4. Implement `ResolvePage()` with locale
5. Add URL redirect logic
6. Write routing tests

**Deliverables**:
- Locale-specific paths working
- Page resolution by path + locale
- Locale switching with redirects

### Phase 4: Formatting (Sprint 4)

**Goals**:
- Locale-aware formatting
- Level 2 formatter (regional)

**Tasks**:
1. Create `LocaleFormatter` interface
2. Implement `SimpleFormatter`
3. Implement `RegionalFormatter` with `golang.org/x/text`
4. Add date/number/currency formatting
5. Write formatter tests

**Deliverables**:
- Date formatting per locale
- Number formatting per locale
- Currency display per locale

### Phase 5: Advanced Features (Sprint 5)

**Goals**:
- Custom fallback chains (Level 3)
- Translation status tracking
- RTL language support

**Tasks**:
1. Add `locale_groups` table
2. Implement custom fallback chain logic
3. Add `translation_status` table
4. Implement status tracking
5. Add RTL detection and metadata
6. Write advanced feature tests

**Deliverables**:
- Custom fallback chains working
- Translation completeness tracking
- RTL language support

### Phase 6: Performance & Production (Sprint 6)

**Goals**:
- Production-ready performance
- Comprehensive caching
- Monitoring and observability

**Tasks**:
1. Implement translation cache with TTL
2. Add fallback chain caching
3. Implement URL cache
4. Add cache warming strategies
5. Add metrics and logging
6. Performance testing
7. Documentation

**Deliverables**:
- Cache hit rates > 95%
- Query optimization complete
- Production monitoring ready

## Usage Examples

### Simple Setup (Level 1)

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

### Regional Setup (Level 2)

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
    })
}
```

### Advanced Setup (Level 3)

```go
package main

func main() {
    cms := NewCMS(&Config{
        DefaultLocale: "en-US",
        Locales: []Locale{ /* same as before */ },

        // Locale-specific configuration
        Locale: &LocaleConfig{
            Groups: []LocaleGroup{
                {
                    Name: "north-america",
                    Fallbacks: []string{"en-US", "en-CA", "fr-CA", "es-MX"},
                },
            },

            // Optional: custom implementations
            Resolver: &RegionalLocaleResolver{
                IPGeolocation: myIPService,
            },
            Formatter: &RegionalFormatter{},
            URLStrategy: &SubdomainStrategy{
                BaseDomain: "example.com",
            },
        },
    })
}
```

## Best Practices

### 1. Configuration vs Content Separation

**Decision Criteria**: Determine if field value varies by language.

**Configuration (Not Translated)**:
- Layout settings (alignment, columns, padding)
- Behavior flags (show_date, count, sorting)
- Technical configuration
- Numeric thresholds
- CSS classes

**Content (Translated)**:
- All user-visible text
- UI labels and button text
- URLs and slugs
- Meta descriptions and titles
- Alt text for images

### 2. Translation Workflow

**Establish Clear Workflow Stages**:
```
Content Creation → Translation Request → Translation → Review → Approval → Publish
```

**Use Translation Status Tracking**:
- Verify all required locale translations reach 100% completeness
- Use `draft` status for partial translations
- Assign translators and reviewers for accountability
- Configure automated notifications for review

### 3. Handling Missing Translations

**Fallback Strategy**:
- Display content in fallback language
- Add visual indicator: `<span class="fallback-locale">Content in English</span>`
- Log missing translations for tracking

### 4. SEO Best Practices

**Hreflang Implementation**:
```html
<link rel="alternate" hreflang="en-US" href="https://example.com/about-us" />
<link rel="alternate" hreflang="fr-FR" href="https://example.fr/a-propos" />
<link rel="alternate" hreflang="x-default" href="https://example.com/about-us" />
```

**Canonical URLs**:
```html
<link rel="canonical" href="https://example.fr/a-propos" />
```

**Sitemap Generation**: Generate separate sitemaps per locale

### 5. RTL Language Support

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

### 6. Testing Strategy

**Test Checklist**:
- [ ] All text is translatable (no hardcoded strings)
- [ ] Fallback chain works correctly
- [ ] Layout doesn't break with longer text
- [ ] RTL languages display correctly
- [ ] Numbers, dates, currency format correctly
- [ ] Locale-specific media loads correctly
- [ ] URLs work in all locales
- [ ] Form validation messages are translated
- [ ] SEO tags are translated

### 7. Performance Optimization

**Eager Loading**:
```sql
-- Load page with translations in one query
SELECT p.*, pt.path, pt.meta, l.code as locale
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

### 8. Migration Path

1. Start with simple codes: `"en"`, `"es"`, `"fr"`
2. Add regions if needed: change `"en"` to `"en-US"` (automatic fallback)
3. Add locale groups for custom fallback logic

**Note**: Database schema and existing translations remain unchanged across upgrades. Only configuration changes.
