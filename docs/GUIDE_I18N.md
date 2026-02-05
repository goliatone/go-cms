# Internationalization Guide

This guide covers internationalization (i18n) in `go-cms`: locale management, the translation model shared by all entities, per-request flexibility, runtime translation settings, and the i18n service with its template helpers. By the end you will understand how to configure locale support for monolingual, bilingual, and full multi-locale sites.

## I18N Architecture Overview

`go-cms` is locale-first: every entity that carries user-facing text supports per-locale translations. The i18n system is composed of three layers:

```
Configuration Layer
  ├── cfg.I18N.Enabled            (global on/off)
  ├── cfg.I18N.Locales            (["en", "es", "fr"])
  ├── cfg.I18N.RequireTranslations (enforce at least one)
  └── cfg.I18N.DefaultLocaleRequired (enforce default locale)

Enforcement Layer
  ├── Global enforcement via config flags
  ├── Per-request override via AllowMissingTranslations
  └── Runtime admin via module.TranslationAdmin()

Translation Layer (per entity)
  ├── Content   → ContentTranslation   (locale, title, summary, content)
  ├── Page      → PageTranslation      (locale, title, path, summary)
  ├── Block     → BlockTranslation     (locale, content, mediaBindings)
  ├── Widget    → WidgetTranslation    (locale, content)
  └── MenuItem  → MenuItemTranslation  (locale, label, labelKey, urlOverride)
```

The i18n service (`internal/i18n`) provides a `Translator` interface for string lookups, a `CultureService` for locale-aware formatting, and template helpers for use in static site generation.

---

## Configuration

### Basic Setup

Start with `cms.DefaultConfig()` and configure your locales:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es", "fr"}
```

The default configuration is strict:

| Field | Default | Description |
|-------|---------|-------------|
| `I18N.Enabled` | `true` | Enable the translation system |
| `I18N.Locales` | `["en"]` | List of supported locale codes |
| `I18N.RequireTranslations` | `true` | Require at least one translation per entity |
| `I18N.DefaultLocaleRequired` | `true` | Require a translation for the default locale |
| `DefaultLocale` | `"en"` | The default locale code |

Locale codes are normalized to lowercase and deduplicated. `"EN"` becomes `"en"`, and duplicate entries are removed while preserving order.

### Strict Mode (Default)

By default, the CMS enforces translations on every create operation:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es"}
// RequireTranslations = true (default)
// DefaultLocaleRequired = true (default)

module, err := cms.New(cfg)
contentSvc := module.Content()

// This fails: no translations provided
_, err = contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: typeID,
    Slug:          "about",
    Status:        "draft",
    CreatedBy:     authorID,
    UpdatedBy:     authorID,
    // Translations: nil -> ErrNoTranslations
})

// This fails: missing default locale
_, err = contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: typeID,
    Slug:          "about",
    Status:        "draft",
    CreatedBy:     authorID,
    UpdatedBy:     authorID,
    Translations: []content.ContentTranslationInput{
        {Locale: "es", Title: "Acerca de"}, // only Spanish, no English
    },
    // -> ErrDefaultLocaleRequired
})

// This succeeds: default locale translation provided
article, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: typeID,
    Slug:          "about",
    Status:        "draft",
    CreatedBy:     authorID,
    UpdatedBy:     authorID,
    Translations: []content.ContentTranslationInput{
        {Locale: "en", Title: "About Us", Content: map[string]any{"body": "..."}},
        {Locale: "es", Title: "Sobre nosotros", Content: map[string]any{"body": "..."}},
    },
})
```

### Relaxed Mode

For monolingual sites or staged rollouts, relax the translation requirements:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en"}
cfg.I18N.RequireTranslations = false   // No translations required
cfg.I18N.DefaultLocaleRequired = false // Default locale not enforced
```

When both flags are `false`, entities can be created without any translations. This is useful for:

- Monolingual sites that don't need translation overhead
- Migration scenarios where content is imported without translations initially
- Draft workflows where translations are added after content creation

### Disabling I18N Entirely

Set `cfg.I18N.Enabled = false` to disable the translation system:

```go
cfg := cms.DefaultConfig()
cfg.I18N.Enabled = false
```

When disabled:

- Both `RequireTranslations` and `DefaultLocaleRequired` are ignored
- Services accept empty translation slices without error
- Per-request `AllowMissingTranslations` overrides are still honored
- Template helpers degrade gracefully (return keys as-is)
- The no-op translator returns the key itself for any lookup

---

## Translation Model Across Entities

Every entity that carries user-facing text follows the same translation pattern: translations are provided as a slice of locale-keyed inputs on create and update requests. Each entity type has its own translation fields appropriate to its purpose.

### Content Translations

Content translations carry the title, summary, and structured content for each locale:

```go
article, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: typeID,
    Slug:          "company-overview",
    Status:        "published",
    CreatedBy:     authorID,
    UpdatedBy:     authorID,
    Translations: []content.ContentTranslationInput{
        {
            Locale:  "en",
            Title:   "Company Overview",
            Summary: stringPtr("A brief overview of our company"),
            Content: map[string]any{
                "body":       "Welcome to our company...",
                "hero_image": "/images/hero-en.jpg",
            },
        },
        {
            Locale:  "es",
            Title:   "Resumen de la empresa",
            Summary: stringPtr("Una breve descripción de nuestra empresa"),
            Content: map[string]any{
                "body":       "Bienvenido a nuestra empresa...",
                "hero_image": "/images/hero-es.jpg",
            },
        },
    },
})
```

**`ContentTranslationInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Locale` | `string` | Yes | Locale code (must be in `cfg.I18N.Locales`) |
| `Title` | `string` | Yes | Localized title |
| `Summary` | `*string` | No | Localized summary text |
| `Content` | `map[string]any` | No | Structured content matching the content type schema |
| `Blocks` | `[]map[string]any` | No | Block data for structured layouts |

### Page Translations (Legacy)

Pages are now content entries with entry-level `metadata.path` as the canonical URL. Localized `Path` values are only used as a legacy fallback when entry metadata is missing. The page translation APIs below apply to the legacy pages service.

```go
page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
    ContentID:  article.ID,
    TemplateID: templateID,
    Slug:       "company-overview",
    Status:     "published",
    CreatedBy:  authorID,
    UpdatedBy:  authorID,
    Translations: []pages.PageTranslationInput{
        {
            Locale:  "en",
            Title:   "Company Overview",
            Path:    "/company-overview",
            Summary: stringPtr("Learn about our company"),
        },
        {
            Locale:  "es",
            Title:   "Resumen de la empresa",
            Path:    "/es/resumen-empresa",
            Summary: stringPtr("Conozca nuestra empresa"),
        },
    },
})
```

**`PageTranslationInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Locale` | `string` | Yes | Locale code |
| `Title` | `string` | Yes | Localized page title |
| `Path` | `string` | Yes | Locale-specific URL path |
| `Summary` | `*string` | No | Localized summary |
| `MediaBindings` | `media.BindingSet` | No | Media attachments for this locale |

### Block Translations

Block translations carry structured content and media bindings per locale:

```go
err := blockSvc.AddTranslation(ctx, blocks.AddTranslationInput{
    BlockInstanceID: instanceID,
    LocaleID:        enLocaleID,
    Content: map[string]any{
        "heading": "Featured Products",
        "cta":     "Shop Now",
    },
    MediaBindings: media.BindingSet{
        {Slot: "background", MediaID: bgImageID},
    },
})
```

**`AddTranslationInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `BlockInstanceID` | `uuid.UUID` | Yes | Block instance to translate |
| `LocaleID` | `uuid.UUID` | Yes | Locale UUID (not code) |
| `Content` | `map[string]any` | Yes | Localized block content |
| `AttributeOverrides` | `map[string]any` | No | Override block attributes per locale |
| `MediaBindings` | `media.BindingSet` | No | Locale-specific media attachments |

### Widget Translations

Widget translations carry localized content for dynamic components:

```go
err := widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
    InstanceID: widgetID,
    LocaleID:   esLocaleID,
    Content: map[string]any{
        "title":   "Oferta especial",
        "message": "20% de descuento este fin de semana",
    },
})
```

**`AddTranslationInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `InstanceID` | `uuid.UUID` | Yes | Widget instance to translate |
| `LocaleID` | `uuid.UUID` | Yes | Locale UUID |
| `Content` | `map[string]any` | Yes | Localized widget content |

### Menu Item Translations

Menu item translations carry labels, i18n keys, and optional URL overrides:

```go
item, err := menuSvc.UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
    Path: "primary.about",
    Type: "item",
    Target: map[string]any{"type": "page", "slug": "about"},
    Translations: []cms.MenuItemTranslationInput{
        {
            Locale:   "en",
            Label:    "About Us",
            LabelKey: "nav.about",
        },
        {
            Locale:      "es",
            Label:       "Sobre nosotros",
            LabelKey:    "nav.about",
            URLOverride: stringPtr("/es/sobre-nosotros"),
        },
    },
    Actor: actor,
})
```

**`MenuItemTranslationInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Locale` | `string` | Yes | Locale code |
| `Label` | `string` | Conditional | Display label (required unless `LabelKey` is provided) |
| `LabelKey` | `string` | No | i18n key for host-side translation |
| `GroupTitle` | `string` | No | Section title for `group` menu items |
| `GroupTitleKey` | `string` | No | i18n key for group title |
| `URLOverride` | `*string` | No | Locale-specific URL override |

---

## Updating and Deleting Translations

Each entity exposes methods for managing translations after initial creation.

### Content Translation Management

```go
// Update a specific locale's translation
err := contentSvc.UpdateTranslation(ctx, content.UpdateContentTranslationRequest{
    ContentID: article.ID,
    Locale:    "en",
    Title:     "Updated Company Overview",
    Content:   map[string]any{"body": "Updated content..."},
    UpdatedBy: authorID,
})

// Delete a specific locale's translation
err := contentSvc.DeleteTranslation(ctx, content.DeleteContentTranslationRequest{
    ContentID: article.ID,
    Locale:    "es",
    DeletedBy: authorID,
})
```

### Page Translation Management

```go
// Update
err := pageSvc.UpdateTranslation(ctx, pages.UpdatePageTranslationRequest{
    PageID:    page.ID,
    Locale:    "en",
    Title:     "Updated Title",
    Path:      "/updated-path",
    UpdatedBy: authorID,
})

// Delete
err := pageSvc.DeleteTranslation(ctx, pages.DeletePageTranslationRequest{
    PageID:    page.ID,
    Locale:    "es",
    DeletedBy: authorID,
})
```

### Block Translation Management

```go
// Update
err := blockSvc.UpdateTranslation(ctx, blocks.UpdateTranslationInput{
    BlockInstanceID: instanceID,
    LocaleID:        enLocaleID,
    Content:         map[string]any{"heading": "Updated Heading"},
    UpdatedBy:       authorID,
})

// Delete
err := blockSvc.DeleteTranslation(ctx, blocks.DeleteTranslationRequest{
    BlockInstanceID: instanceID,
    LocaleID:        esLocaleID,
    DeletedBy:       authorID,
})
```

---

## Per-Request Flexibility

Every create and update request type exposes an `AllowMissingTranslations` field. When set to `true`, it bypasses global translation enforcement for that single operation, without changing the global config.

```go
// Global config requires translations, but this request skips enforcement
article, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID:            typeID,
    Slug:                     "draft-article",
    Status:                   "draft",
    CreatedBy:                authorID,
    UpdatedBy:                authorID,
    Translations:             nil, // No translations provided
    AllowMissingTranslations: true, // Override: allow it
})
```

This is available on:

| Entity | Request types with `AllowMissingTranslations` |
|--------|-----------------------------------------------|
| Content | `CreateContentRequest`, `UpdateContentRequest` |
| Pages | `CreatePageRequest`, `UpdatePageRequest` |
| Blocks | `DeleteTranslationRequest` |
| Menus | `AddMenuItemInput`, `UpsertMenuItemInput` |

Use cases for per-request overrides:

- **Draft workflows** -- create content in draft status, add translations later before publishing
- **Status transitions** -- update status without touching translations
- **Migration scripts** -- import content without full locale coverage initially
- **Staging environments** -- test content structure before localizing

When `AllowMissingTranslations` is `true`:

- Empty translation slices are treated as no-ops on update paths
- Services skip the "at least one translation" and "default locale" checks
- Existing translations are preserved; only new/changed translations are applied

---

## Global Opt-Out for Monolingual Sites

If your site uses a single language and you want to eliminate translation overhead entirely, flip the global flags:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en"}
cfg.I18N.RequireTranslations = false
cfg.I18N.DefaultLocaleRequired = false
```

With this configuration:

- All create/update operations succeed without translations
- Existing translations are preserved if already present
- The translation system remains available for later activation
- Template helpers still function (returning keys as fallback)

For a complete opt-out, disable i18n:

```go
cfg.I18N.Enabled = false
```

This makes the entire translation pipeline no-op. Services still accept translation slices (they're just ignored), so code that provides translations continues to work without modification.

---

## Translation Grouping

Content translations support an optional `TranslationGroupID` field. Translation groups link related translations across entities, enabling workflows that require coordinated updates:

```go
type ContentTranslation struct {
    ID                 uuid.UUID
    ContentID          uuid.UUID
    LocaleID           uuid.UUID
    TranslationGroupID *uuid.UUID // Optional grouping identifier
    Title              string
    Summary            *string
    Content            map[string]any
    // ... timestamps
}
```

When present, `TranslationGroupID` associates translations that should be published, reviewed, or updated together. This is an advanced feature for workflows that require multi-entity translation coordination.

---

## i18n.Service and Template Helpers

The `i18n.Service` provides translation lookups and locale-aware formatting for use in code and templates.

### Accessing the Service

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es"}

module, err := cms.New(cfg)
i18nSvc := module.I18n()
```

### Translator Interface

The core translation interface:

```go
type Translator interface {
    Translate(locale, key string, args ...any) (string, error)
}
```

Usage:

```go
translator := i18nSvc.Translator()

// Simple lookup
msg, err := translator.Translate("en", "greeting")
// msg = "Hello"

// With interpolation arguments
msg, err := translator.Translate("es", "welcome_user", "María")
// msg = "Bienvenida, María"
```

When a translation key is missing, the translator returns `ErrMissingTranslation`. The default fallback strategy returns the key itself, so templates degrade gracefully rather than breaking.

### Culture Service

The culture service provides locale-aware formatting:

```go
culture := i18nSvc.Culture()

// Currency code for a locale
code, err := culture.GetCurrencyCode("en-US")
// code = "USD"

// Currency details
currency, err := culture.GetCurrency("en-US")

// Measurement preferences
pref, err := culture.GetMeasurementPreference("en-US", "length")

// Unit conversion
value, unit, display, err := culture.ConvertMeasurement("en-US", 100, "cm", "length")
```

### Template Helpers

The i18n service registers template helpers for use with `go-template`:

```go
cfg := i18nSvc.HelperConfig()
helpers := i18nSvc.TemplateHelpers(cfg)
```

Helpers include locale-aware formatting for dates, times, numbers, percentages, currency, measurements, lists, ordinals, and phone numbers. Each helper accepts a locale string as the first argument:

```html
<!-- In templates -->
{{ t "en" "page.title" }}
{{ formatDate "en" .CreatedAt }}
{{ formatCurrency "en" 29.99 }}
{{ formatNumber "es" 1234567.89 }}
```

The default locale is used when the locale argument is empty. Missing translations return the key itself rather than causing template errors.

### In-Memory Service

For testing or simple setups, create an in-memory i18n service with embedded translations:

```go
translations := map[string]map[string]string{
    "en": {
        "greeting":    "Hello",
        "farewell":    "Goodbye",
        "welcome_user": "Welcome, %s",
    },
    "es": {
        "greeting":    "Hola",
        "farewell":    "Adiós",
        "welcome_user": "Bienvenido, %s",
    },
}

i18nCfg := i18n.Config{
    DefaultLocale: "en",
    Locales: []i18n.LocaleConfig{
        {Code: "en"},
        {Code: "es", Fallbacks: []string{"en"}},
    },
}

svc, err := i18n.NewInMemoryService(i18nCfg, translations)
```

### Fallback Chains

Configure locale fallback chains so missing translations in one locale fall back to another:

```go
i18nCfg := i18n.Config{
    DefaultLocale: "en",
    Locales: []i18n.LocaleConfig{
        {Code: "en"},
        {Code: "es", Fallbacks: []string{"en"}},
        {Code: "fr", Fallbacks: []string{"en"}},
        {Code: "pt-BR", Fallbacks: []string{"pt", "es", "en"}},
    },
}
```

Resolution order for `pt-BR`:
1. Look up key in `pt-BR`
2. Fall back to `pt`
3. Fall back to `es`
4. Fall back to `en` (default locale)
5. Return key as-is if all fail

The default locale is always the ultimate fallback, even if not listed explicitly in the chain.

---

## Translation Admin Service

The translation admin service enables runtime management of translation settings without restarting the application.

### Accessing the Admin Service

```go
module, err := cms.New(cfg)
translationAdmin := module.TranslationAdmin()
```

### Reading Current Settings

```go
settings, err := translationAdmin.GetSettings(ctx)
fmt.Printf("Enabled: %v, Required: %v\n",
    settings.TranslationsEnabled,
    settings.RequireTranslations,
)
```

### Applying Settings at Runtime

```go
// Relax translation requirements at runtime
err := translationAdmin.ApplySettings(ctx, translationconfig.Settings{
    TranslationsEnabled: true,
    RequireTranslations: false,
})
```

When settings are applied:

- The change takes effect immediately across all services
- An audit event is emitted (`"translation_settings_updated"`)
- Existing content is not modified
- The new enforcement rules apply to subsequent create/update operations

### Resetting to Defaults

```go
// Reset to config-based defaults
err := translationAdmin.Reset(ctx)
```

Resetting removes persisted overrides and reverts to the values from `cms.DefaultConfig()`. An audit event (`"translation_settings_deleted"`) is emitted.

### Runtime vs Config

The relationship between config and runtime settings:

| Source | Takes effect | Persisted | Audit logged |
|--------|-------------|-----------|--------------|
| `cfg.I18N.*` | At startup | No (code/env) | No |
| `TranslationAdmin.ApplySettings()` | Immediately | Yes (repository) | Yes |
| `TranslationAdmin.Reset()` | Immediately | Clears persisted | Yes |

Runtime settings override config values. When the application restarts, it loads the config first, then applies any persisted runtime settings from the repository.

---

## The Locale Model

Locales are stored as first-class entities in the database:

```go
type Locale struct {
    ID         uuid.UUID
    Code       string       // e.g., "en", "es", "fr"
    Display    string       // e.g., "English", "Spanish"
    NativeName *string      // e.g., "Español", "Français"
    IsActive   bool         // Whether this locale is currently active
    IsDefault  bool         // Whether this is the default locale
    Metadata   map[string]any
    DeletedAt  *time.Time
    CreatedAt  time.Time
}
```

Locale records are created automatically when `cfg.I18N.Locales` is configured. Content, page, and menu translations reference locale codes (strings), while block and widget translations reference locale UUIDs. The service layer handles the mapping transparently.

---

## Error Handling

Translation-related errors across modules:

### Content Errors

| Error | Cause |
|-------|-------|
| `ErrNoTranslations` | No translations provided when `RequireTranslations` is `true` |
| `ErrDefaultLocaleRequired` | Missing translation for default locale when `DefaultLocaleRequired` is `true` |
| `ErrDuplicateLocale` | Same locale code appears twice in translations slice |
| `ErrUnknownLocale` | Locale code not in `cfg.I18N.Locales` |

### Menu Errors

| Error | Cause |
|-------|-------|
| `ErrMenuItemTranslations` | No translations provided when required |
| `ErrMenuItemDuplicateLocale` | Same locale appears twice |
| `ErrUnknownLocale` | Locale not configured |
| `ErrTranslationLabelRequired` | Translation needs a label or label key |
| `ErrTranslationExists` | Translation already exists for this locale |

### I18N Service Errors

| Error | Cause |
|-------|-------|
| `ErrMissingTranslation` | Translation key not found in any locale (including fallbacks) |

### Config Validation Errors

| Error | Cause |
|-------|-------|
| `ErrDefaultLocaleRequired` | `DefaultLocaleRequired` or `RequireTranslations` is `true` but `DefaultLocale` is empty |

---

## Common Patterns

### Monolingual Site

A single-language site with no translation overhead:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en"}
cfg.I18N.RequireTranslations = false
cfg.I18N.DefaultLocaleRequired = false

module, err := cms.New(cfg)
contentSvc := module.Content()

// Create content without translations
article, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: typeID,
    Slug:          "about",
    Status:        "published",
    CreatedBy:     authorID,
    UpdatedBy:     authorID,
    // No translations needed
})
```

### Bilingual Site

A two-language site with strict enforcement:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es"}
// RequireTranslations = true (default)
// DefaultLocaleRequired = true (default)

module, err := cms.New(cfg)
contentSvc := module.Content()

// Both locales provided
article, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: typeID,
    Slug:          "about",
    Status:        "published",
    CreatedBy:     authorID,
    UpdatedBy:     authorID,
    Translations: []content.ContentTranslationInput{
        {Locale: "en", Title: "About Us", Content: map[string]any{"body": "..."}},
        {Locale: "es", Title: "Sobre nosotros", Content: map[string]any{"body": "..."}},
    },
})
```

### Full Multi-Locale Site

A site supporting many locales with fallback chains:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es", "fr", "de", "pt", "pt-BR", "ja"}

module, err := cms.New(cfg)

// Content with all locales
article, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: typeID,
    Slug:          "welcome",
    Status:        "published",
    CreatedBy:     authorID,
    UpdatedBy:     authorID,
    Translations: []content.ContentTranslationInput{
        {Locale: "en", Title: "Welcome"},
        {Locale: "es", Title: "Bienvenido"},
        {Locale: "fr", Title: "Bienvenue"},
        {Locale: "de", Title: "Willkommen"},
        {Locale: "pt", Title: "Bem-vindo"},
        {Locale: "pt-BR", Title: "Bem-vindo"},
        {Locale: "ja", Title: "ようこそ"},
    },
})
```

### Staged Translation Rollout

Create content first, add translations later:

```go
// Step 1: Create with default locale only, skip other translations
article, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: typeID,
    Slug:          "new-feature",
    Status:        "draft",
    CreatedBy:     authorID,
    UpdatedBy:     authorID,
    Translations: []content.ContentTranslationInput{
        {Locale: "en", Title: "New Feature Announcement"},
    },
    AllowMissingTranslations: true, // Spanish translation comes later
})

// Step 2: Add Spanish translation when ready
err = contentSvc.UpdateTranslation(ctx, content.UpdateContentTranslationRequest{
    ContentID: article.ID,
    Locale:    "es",
    Title:     "Anuncio de nueva función",
    Content:   map[string]any{"body": "..."},
    UpdatedBy: authorID,
})

// Step 3: Publish with all translations
_, err = contentSvc.Update(ctx, content.UpdateContentRequest{
    ID:        article.ID,
    Status:    "published",
    UpdatedBy: authorID,
})
```

### Runtime Translation Toggle

Operators can toggle translation enforcement without restarting:

```go
module, err := cms.New(cfg)
admin := module.TranslationAdmin()

// Temporarily relax enforcement for a bulk import
err := admin.ApplySettings(ctx, translationconfig.Settings{
    TranslationsEnabled: true,
    RequireTranslations: false,
})

// ... run bulk import ...

// Restore strict enforcement
err = admin.ApplySettings(ctx, translationconfig.Settings{
    TranslationsEnabled: true,
    RequireTranslations: true,
})
```

---

## Next Steps

- **GUIDE_CONTENT.md** -- content types, entries, and content-specific translation details
- **GUIDE_PAGES.md** -- page translations with locale-specific paths
- **GUIDE_MENUS.md** -- menu item translations and locale-aware navigation resolution
- **GUIDE_BLOCKS.md** -- block translations with media bindings
- **GUIDE_WIDGETS.md** -- widget translations for dynamic components
- **GUIDE_CONFIGURATION.md** -- full config reference including `I18NConfig`
- `docs/I18N_TDD.md` -- technical design notes for the i18n module
- `docs/TRANS_FIX.md` -- translation flexibility design decisions
