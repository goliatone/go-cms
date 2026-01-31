# Themes Guide

This guide covers theme management, template registration, region definitions, asset resolution, and the theme manifest in `go-cms`. By the end you will understand how to register themes, configure templates with named regions, manage assets, and integrate themes with the static site generator.

## Theme Architecture Overview

Themes in `go-cms` organize the visual presentation layer through three concepts:

- **Themes** describe a named visual identity with a version, file system path, and configuration. Each theme declares widget areas, menu locations, and static assets. Only one theme can be active at a time.
- **Templates** belong to a theme and define page layouts. Each template has a slug, a file path within the theme directory, and a set of named regions where blocks and widgets can be placed.
- **Regions** are named content surfaces within a template. Each region declares whether it accepts blocks, widgets, or both, and can specify fallback regions for content inheritance.

```
Theme (name, version, themePath, config)
  ├── ThemeConfig
  │     ├── WidgetAreas (code, name, scope)
  │     ├── MenuLocations (code, name)
  │     └── Assets (styles, scripts, images)
  └── Template (slug, templatePath, regions)
        ├── Region (name, acceptsBlocks, acceptsWidgets)
        ├── Region (name, acceptsBlocks, acceptsWidgets)
        └── Region (name, acceptsBlocks, acceptsWidgets)
```

Themes integrate with the `go-theme` library for manifest loading, variant selection, and asset resolution during static site generation. The CMS theme models and `go-theme` manifests are separate structures; the generator bridges them at build time.

All entities use UUID primary keys and UTC timestamps. Theme IDs are deterministically derived from the theme path, and template IDs from the combination of theme ID and slug.

### Accessing the Service

Theme operations are exposed through the `cms.Module` facade:

```go
cfg := cms.DefaultConfig()
cfg.Features.Themes = true
cfg.Themes.DefaultTheme = "aurora"
cfg.Themes.DefaultVariant = "light"

module, err := cms.New(cfg)
if err != nil {
    log.Fatal(err)
}

themeSvc := module.Themes()
```

The `themeSvc` variable satisfies the `themes.Service` interface. The service delegates to in-memory repositories by default, or SQL-backed repositories when `di.WithBunDB(db)` is provided.

When `cfg.Features.Themes` is `false` (the default), the service returns a no-op implementation where all operations return `ErrFeatureDisabled`.

---

## Enabling and Configuring Themes

### Feature Flag

Themes are disabled by default. Enable via the feature flag:

```go
cfg := cms.DefaultConfig()
cfg.Features.Themes = true
```

### Configuration Options

The `ThemeConfig` section controls theme behavior:

```go
cfg.Themes.BasePath = "./themes"           // Base directory for theme files
cfg.Themes.DefaultTheme = "aurora"         // Default theme name for selection
cfg.Themes.DefaultVariant = "light"        // Default variant (for go-theme)
cfg.Themes.CSSVariablePrefix = "--cms-"    // Prefix for CSS custom properties
cfg.Themes.PartialFallbacks = map[string]string{
    "sidebar": "default-sidebar",          // Fallback partial template mappings
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `BasePath` | `string` | `""` | Base directory for theme files on disk |
| `DefaultTheme` | `string` | `""` | Theme name used when no explicit selection is made |
| `DefaultVariant` | `string` | `""` | Variant name passed to `go-theme` for variant selection |
| `CSSVariablePrefix` | `string` | `""` | Prefix for generated CSS custom properties |
| `PartialFallbacks` | `map[string]string` | `nil` | Maps partial names to fallback partial names |

---

## Theme Lifecycle

### Registering a Theme

Register a theme with its name, version, file system path, and optional configuration:

```go
theme, err := themeSvc.RegisterTheme(ctx, themes.RegisterThemeInput{
    Name:        "Aurora",
    Description: stringPtr("Default marketing theme with hero-focused layout"),
    Version:     "1.0.0",
    Author:      stringPtr("Go CMS"),
    ThemePath:   "themes/aurora",
    Config: themes.ThemeConfig{
        WidgetAreas: []themes.ThemeWidgetArea{
            {Code: "header.global", Name: "Global Header", Scope: "global"},
            {Code: "hero", Name: "Hero Banner", Scope: "template"},
            {Code: "sidebar", Name: "Sidebar", Scope: "page"},
        },
        MenuLocations: []themes.ThemeMenuLocation{
            {Code: "primary_navigation", Name: "Primary Navigation"},
            {Code: "footer_navigation", Name: "Footer Navigation"},
        },
        Assets: &themes.ThemeAssets{
            BasePath: stringPtr("public"),
            Styles:   []string{"css/base.css", "css/aurora.css"},
            Scripts:  []string{"js/site.js"},
            Images:   []string{"images/logo.png"},
        },
    },
    Activate: true, // Attempt immediate activation after registration
})
```

**`RegisterThemeInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Name` | `string` | Yes | Unique display name for the theme |
| `Description` | `*string` | No | Human-readable description |
| `Version` | `string` | Yes | Semantic version string (e.g. "1.0.0") |
| `Author` | `*string` | No | Theme author name |
| `ThemePath` | `string` | Yes | File system path to the theme directory |
| `Config` | `ThemeConfig` | No | Widget areas, menu locations, and assets |
| `Activate` | `bool` | No | If true, activate the theme immediately after registration |

**What happens:**

1. Name, version, and theme path are validated (all required, non-empty)
2. Widget areas are validated: each must have both `Code` and `Name`
3. Menu locations are validated: each must have both `Code` and `Name`
4. A deterministic UUID is generated from the theme path via `identity.ThemeUUID(themePath)`
5. The theme is persisted to the repository
6. If `Activate` is true and the theme has at least one template, activation is attempted

### Retrieving Themes

```go
// Get by ID
theme, err := themeSvc.GetTheme(ctx, themeID)

// Get by name
theme, err := themeSvc.GetThemeByName(ctx, "Aurora")
```

Both return `ErrThemeNotFound` when the theme does not exist.

### Listing Themes

```go
// List all themes
allThemes, err := themeSvc.ListThemes(ctx)
for _, t := range allThemes {
    fmt.Printf("Theme: %s v%s (active=%v)\n", t.Name, t.Version, t.IsActive)
}

// List only active themes
activeThemes, err := themeSvc.ListActiveThemes(ctx)
```

### Active Theme Summaries

Summaries include resolved asset paths for rendering:

```go
summaries, err := themeSvc.ListActiveSummaries(ctx)
for _, s := range summaries {
    fmt.Printf("Theme: %s\n", s.Theme.Name)
    fmt.Printf("  Styles:  %v\n", s.Assets.Styles)
    fmt.Printf("  Scripts: %v\n", s.Assets.Scripts)
    fmt.Printf("  Images:  %v\n", s.Assets.Images)
}
```

The `ThemeSummary` struct bundles the theme record with a `ThemeAssetsSummary` containing resolved style, script, and image paths.

### Activating a Theme

Activation makes a theme the current active theme. Activation constraints:

1. The theme must exist
2. `ThemePath` must be non-empty
3. At least one template must be registered for the theme
4. All widget areas must have both `Code` and `Name`
5. All menu locations must have both `Code` and `Name`

```go
activated, err := themeSvc.ActivateTheme(ctx, themeID)
if err != nil {
    // ErrThemeActivationMissingTemplates -- no templates registered
    // ErrThemeActivationPathInvalid -- theme path is empty
    // ErrThemeWidgetAreaInvalid -- widget area validation failed
    log.Fatalf("activate theme: %v", err)
}
fmt.Printf("Active: %s (isActive=%v)\n", activated.Name, activated.IsActive)
```

### Deactivating a Theme

```go
deactivated, err := themeSvc.DeactivateTheme(ctx, themeID)
```

Deactivation sets `IsActive` to `false`. This does not remove the theme or its templates.

---

## Theme Manifest

Themes can be described by a `theme.json` manifest file in the theme directory. The manifest provides a declarative way to define theme metadata, widget areas, menu locations, and assets.

### Manifest Structure

```json
{
    "name": "aurora",
    "description": "Default marketing theme with hero-focused layout",
    "version": "1.0.0",
    "author": "Go CMS",
    "widget_areas": [
        {
            "code": "header.global",
            "name": "Global Header",
            "scope": "global"
        },
        {
            "code": "hero",
            "name": "Hero Banner",
            "scope": "template"
        }
    ],
    "menu_locations": [
        {
            "code": "primary_navigation",
            "name": "Primary Navigation"
        },
        {
            "code": "footer_navigation",
            "name": "Footer Navigation"
        }
    ],
    "assets": {
        "base_path": "public",
        "styles": ["css/base.css", "css/aurora.css"],
        "scripts": ["js/site.js"],
        "images": ["images/logo.png"]
    },
    "metadata": {
        "preview_image": "images/previews/landing.png",
        "support_email": "support@example.com"
    }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | `string` | Yes | Theme identifier |
| `description` | `string` | No | Human-readable description |
| `version` | `string` | Yes | Semantic version |
| `author` | `string` | No | Theme author |
| `widget_areas` | `array` | No | Declared widget placement areas |
| `menu_locations` | `array` | No | Declared menu insertion points |
| `assets` | `object` | No | Static asset declarations |
| `metadata` | `object` | No | Arbitrary metadata (stored as JSONB) |

### Loading a Manifest

```go
// Load from a file path
manifest, err := themes.LoadManifest("themes/aurora/theme.json")
if err != nil {
    log.Fatalf("load manifest: %v", err)
}

// Parse from an io.Reader
manifest, err := themes.ParseManifest(jsonReader)
```

### Converting a Manifest to a Registration Input

Convert a loaded manifest into a `RegisterThemeInput` for programmatic registration:

```go
input, err := themes.ManifestToThemeInput("themes/aurora", manifest)
if err != nil {
    log.Fatalf("convert manifest: %v", err)
}

theme, err := themeSvc.RegisterTheme(ctx, input)
```

### Real-World Example

The Collective Labs theme demonstrates a production manifest:

```json
{
    "assets": {
        "scripts": ["assets/main-Bs7R2_QW.js"],
        "styles": ["assets/main-CcZ-R0Hw.css"]
    },
    "label": "Collective Labs Marketing Theme",
    "name": "collective-labs",
    "version": "0.1.0"
}
```

A typical theme directory structure:

```
collective-labs/
├── theme.json
├── templates/
│   ├── layout.tpl
│   ├── landing.tpl
│   ├── case-study.tpl
│   └── partials/
│       ├── head.tpl
│       ├── header.tpl
│       └── footer.tpl
└── assets/
    ├── main-Bs7R2_QW.js
    ├── main-CcZ-R0Hw.css
    └── img/
        ├── logo/
        └── pattern_003.svg
```

---

## Template Management

Templates define page layouts within a theme. Each template has a slug unique within its theme, a file path, and named regions.

### Registering a Template

```go
template, err := themeSvc.RegisterTemplate(ctx, themes.RegisterTemplateInput{
    ThemeID:      theme.ID,
    Name:         "Landing Page",
    Slug:         "landing",
    Description:  stringPtr("Full-width landing page with hero and content sections"),
    TemplatePath: "templates/landing.html.tmpl",
    Regions: map[string]themes.TemplateRegion{
        "hero": {
            Name:          "Hero Banner",
            Description:   stringPtr("Full-width hero section at the top"),
            AcceptsBlocks: true,
        },
        "content": {
            Name:           "Main Content",
            AcceptsBlocks:  true,
            AcceptsWidgets: true,
        },
        "sidebar": {
            Name:            "Sidebar",
            AcceptsWidgets:  true,
            FallbackRegions: []string{"content"},
        },
    },
    Metadata: map[string]any{
        "preview_image": "previews/landing.png",
        "category":      "marketing",
    },
})
```

**`RegisterTemplateInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ThemeID` | `uuid.UUID` | Yes | Parent theme identifier |
| `Name` | `string` | Yes | Human-readable template name |
| `Slug` | `string` | Yes | URL-safe identifier, unique per theme |
| `Description` | `*string` | No | Template description |
| `TemplatePath` | `string` | Yes | Path within theme directory (e.g. "templates/landing.html.tmpl") |
| `Regions` | `map[string]TemplateRegion` | Yes | Named content regions (at least one required) |
| `Metadata` | `map[string]any` | No | Arbitrary metadata stored as JSONB |

**What happens:**

1. Theme ID, name, slug, and template path are validated
2. The slug is normalized to lowercase and trimmed
3. Regions are validated: at least one region required, each must accept blocks or widgets (or both)
4. A deterministic UUID is generated from `identity.TemplateUUID(themeID, slug)`
5. Slug uniqueness is enforced per theme (composite unique constraint)
6. The template is persisted to the repository

### Template Region Structure

Each region defines a named content surface:

```go
type TemplateRegion struct {
    Name            string   // Human-readable region name (required)
    Description     *string  // Optional description
    AcceptsWidgets  bool     // True if widgets can be placed here
    AcceptsBlocks   bool     // True if content blocks can be placed here
    FallbackRegions []string // Fallback region keys for content inheritance
}
```

**Region validation rules:**
- At least one of `AcceptsWidgets` or `AcceptsBlocks` must be true
- Both can be true for flexible regions that accept either type
- `Name` cannot be empty
- `FallbackRegions` define a chain for content fallback

### Updating a Template

Update uses pointer fields so only specified fields are modified:

```go
updated, err := themeSvc.UpdateTemplate(ctx, themes.UpdateTemplateInput{
    TemplateID:   template.ID,
    Name:         stringPtr("Landing Page v2"),
    TemplatePath: stringPtr("templates/landing-v2.html.tmpl"),
    Regions: map[string]themes.TemplateRegion{
        "hero": {
            Name:          "Hero Banner",
            AcceptsBlocks: true,
        },
        "content": {
            Name:           "Main Content",
            AcceptsBlocks:  true,
            AcceptsWidgets: true,
        },
        "sidebar": {
            Name:           "Sidebar",
            AcceptsWidgets: true,
        },
        "footer": {
            Name:           "Footer",
            AcceptsWidgets: true,
        },
    },
})
```

**`UpdateTemplateInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `TemplateID` | `uuid.UUID` | Yes | Template identifier |
| `Name` | `*string` | No | Updated display name |
| `Description` | `*string` | No | Updated description |
| `TemplatePath` | `*string` | No | Updated file path |
| `Regions` | `map[string]TemplateRegion` | No | Updated region definitions |
| `Metadata` | `map[string]any` | No | Updated metadata |

### Deleting a Template

```go
err := themeSvc.DeleteTemplate(ctx, templateID)
if err != nil {
    // Returns ErrTemplateNotFound if the template does not exist
    log.Fatalf("delete template: %v", err)
}
```

### Getting a Template

```go
template, err := themeSvc.GetTemplate(ctx, templateID)
if err != nil {
    log.Fatalf("get template: %v", err)
}
fmt.Printf("Template: %s (slug=%s, path=%s)\n", template.Name, template.Slug, template.TemplatePath)
```

### Listing Templates

```go
templates, err := themeSvc.ListTemplates(ctx, themeID)
for _, t := range templates {
    fmt.Printf("  %s (slug=%s) -- %d regions\n", t.Name, t.Slug, len(t.Regions))
}
```

---

## Regions

Regions provide a structured way to inspect the content surfaces available in templates and across an entire theme.

### Template Regions

Inspect the regions available in a single template:

```go
regions, err := themeSvc.TemplateRegions(ctx, templateID)
for _, r := range regions {
    fmt.Printf("Region: %s (%s)\n", r.Key, r.Name)
    fmt.Printf("  Accepts blocks:  %v\n", r.AcceptsBlocks)
    fmt.Printf("  Accepts widgets: %v\n", r.AcceptsWidgets)
    if len(r.Fallbacks) > 0 {
        fmt.Printf("  Fallbacks: %v\n", r.Fallbacks)
    }
}
```

The `RegionInfo` struct provides a flattened view:

| Field | Type | Description |
|-------|------|-------------|
| `Key` | `string` | Region map key |
| `Name` | `string` | Human-readable region name |
| `AcceptsBlocks` | `bool` | Whether blocks can be placed here |
| `AcceptsWidgets` | `bool` | Whether widgets can be placed here |
| `Fallbacks` | `[]string` | Fallback region keys |

### Theme Region Index

Get a map of all regions across all templates in a theme:

```go
regionIndex, err := themeSvc.ThemeRegionIndex(ctx, themeID)
for templateSlug, regions := range regionIndex {
    fmt.Printf("Template: %s\n", templateSlug)
    for _, r := range regions {
        fmt.Printf("  %s: blocks=%v widgets=%v\n", r.Key, r.AcceptsBlocks, r.AcceptsWidgets)
    }
}
```

The returned map keys are template slugs, and values are slices of `RegionInfo` for that template. This is useful for admin UIs that need to show all available placement targets across a theme.

---

## Asset Resolution

Themes provide a file-system-based asset resolver for loading CSS, JavaScript, and image files.

### FileSystemAssetResolver

```go
resolver := themes.FileSystemAssetResolver{
    FS:       os.DirFS("themes"),
    BasePath: "aurora/public",
}

// Resolve an asset path
path, err := resolver.ResolvePath("css/main.css")
// Returns: "aurora/public/css/main.css"

// Open an asset file
file, err := resolver.Open("css/main.css")
if err != nil {
    log.Fatalf("open asset: %v", err)
}
defer file.Close()
```

The resolver includes path traversal protection via `filepath.Clean()` and prefix checks. Relative paths that attempt to escape the base path are rejected.

---

## Template Helpers in Rendering

When rendering templates during static site generation, several theme-related helpers are available in the template context via the `go-theme` integration.

### Available Helpers

| Helper | Description |
|--------|-------------|
| `.Theme.AssetURL(path)` | Resolves a relative asset path to a full URL based on the theme's asset base path |
| `.Theme.Partials` | Access partial templates registered with the theme |
| `.Theme.CSSVars` | Get CSS custom properties defined by the theme configuration |
| `.Helpers.WithBaseURL(url)` | Prepend the site's base URL prefix to a given URL |

### Usage in Templates

```html
<!DOCTYPE html>
<html>
<head>
    <!-- Resolve theme asset URLs -->
    <link rel="stylesheet" href="{{ .Theme.AssetURL "css/base.css" }}">
    <link rel="stylesheet" href="{{ .Theme.AssetURL "css/aurora.css" }}">

    <!-- CSS custom properties from theme config -->
    <style>
        :root {
            {{ range $key, $value := .Theme.CSSVars }}
            {{ $key }}: {{ $value }};
            {{ end }}
        }
    </style>
</head>
<body>
    <!-- Include a partial template -->
    {{ template "header" .Theme.Partials }}

    <main>
        {{ .Content }}
    </main>

    {{ template "footer" .Theme.Partials }}

    <!-- Resolve script URLs -->
    <script src="{{ .Theme.AssetURL "js/site.js" }}"></script>
</body>
</html>
```

### Base URL Handling

Use `.Helpers.WithBaseURL` to ensure URLs respect the configured site base URL:

```html
<a href="{{ .Helpers.WithBaseURL "/about" }}">About</a>
<!-- With base URL "/site": renders as "/site/about" -->
<!-- Without base URL: renders as "/about" -->
```

### Generator Integration

The static site generator loads theme manifests from disk using the `go-theme` library. At build time the generator:

1. Reads the `go-theme` manifest from the theme directory using `os.DirFS(themePath)`
2. Caches manifests in memory by theme ID
3. Creates a `go-theme.Selection` that provides asset URLs, partials, and CSS variables
4. Passes the selection into the template context as `.Theme`

The generator's theme selector is configured from the CMS config:

```go
cfg.Themes.DefaultTheme = "aurora"    // Selects which theme to render with
cfg.Themes.DefaultVariant = "light"   // Selects which variant of the theme
```

---

## Bootstrap and Seeding

The theme registry enables programmatic registration of themes and templates at application startup.

### Creating a Theme Seed

```go
registry := themes.NewRegistry()

registry.Register(themes.ThemeSeed{
    Theme: themes.RegisterThemeInput{
        Name:      "Aurora",
        Version:   "1.0.0",
        ThemePath: "themes/aurora",
        Config: themes.ThemeConfig{
            WidgetAreas: []themes.ThemeWidgetArea{
                {Code: "header", Name: "Header", Scope: "global"},
                {Code: "hero", Name: "Hero Banner", Scope: "template"},
                {Code: "sidebar", Name: "Sidebar", Scope: "page"},
            },
            MenuLocations: []themes.ThemeMenuLocation{
                {Code: "primary_navigation", Name: "Primary Navigation"},
            },
            Assets: &themes.ThemeAssets{
                BasePath: stringPtr("public"),
                Styles:   []string{"css/base.css", "css/aurora.css"},
                Scripts:  []string{"js/site.js"},
            },
        },
        Activate: true,
    },
    Templates: []themes.RegisterTemplateInput{
        {
            Name:         "Landing Page",
            Slug:         "landing",
            TemplatePath: "templates/landing.html.tmpl",
            Regions: map[string]themes.TemplateRegion{
                "hero":    {Name: "Hero Banner", AcceptsBlocks: true},
                "content": {Name: "Main Content", AcceptsBlocks: true, AcceptsWidgets: true},
                "sidebar": {Name: "Sidebar", AcceptsWidgets: true},
            },
        },
        {
            Name:         "Blog Post",
            Slug:         "blog-post",
            TemplatePath: "templates/blog-post.html.tmpl",
            Regions: map[string]themes.TemplateRegion{
                "content": {Name: "Article Body", AcceptsBlocks: true},
                "sidebar": {Name: "Sidebar", AcceptsWidgets: true},
            },
        },
    },
})
```

### Applying Seeds at Startup

```go
seeds := registry.List()
err := themes.Bootstrap(ctx, themeSvc, seeds)
if err != nil {
    log.Fatalf("bootstrap themes: %v", err)
}
```

`Bootstrap` iterates over all seeds, registers each theme and its templates, and tolerates duplicates. This makes it safe to call on every application startup for declarative theme management.

---

## Database Schema

Themes and templates are stored in two tables when using BunDB.

### Themes Table

```sql
CREATE TABLE themes (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    version TEXT NOT NULL,
    author TEXT,
    is_active BOOLEAN NOT NULL DEFAULT false,
    theme_path TEXT NOT NULL,
    config JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_themes_name ON themes(name);
CREATE INDEX idx_themes_is_active ON themes(is_active);
```

### Templates Table

```sql
CREATE TABLE templates (
    id UUID PRIMARY KEY,
    theme_id UUID NOT NULL REFERENCES themes(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,
    template_path TEXT NOT NULL,
    regions JSONB NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_templates_theme_id ON templates(theme_id);
CREATE UNIQUE INDEX idx_templates_theme_slug ON templates(theme_id, slug);
```

Key points:
- The `config` column stores the entire `ThemeConfig` as JSONB (widget areas, menu locations, assets, metadata)
- The `regions` column stores the `map[string]TemplateRegion` as JSONB
- A composite unique index on `(theme_id, slug)` enforces slug uniqueness per theme
- Cascade delete from themes to templates ensures cleanup when a theme is removed

---

## Error Reference

| Error | Cause |
|-------|-------|
| `ErrFeatureDisabled` | Theme operations called but `cfg.Features.Themes` is false |
| `ErrThemeRepositoryRequired` | Missing theme repository dependency in DI container |
| `ErrTemplateRepositoryRequired` | Missing template repository dependency in DI container |
| `ErrThemeNameRequired` | `RegisterTheme` called without a name |
| `ErrThemeVersionRequired` | `RegisterTheme` called without a version |
| `ErrThemePathRequired` | `RegisterTheme` called without a theme path |
| `ErrThemeExists` | A theme with the same name already exists |
| `ErrThemeNotFound` | Theme lookup failed (by ID or name) |
| `ErrThemeActivationMissingTemplates` | Activation attempted on a theme with no templates |
| `ErrThemeActivationPathInvalid` | Activation attempted on a theme with an empty path |
| `ErrThemeWidgetAreaInvalid` | Widget area missing required `Code` or `Name` |
| `ErrTemplateNotFound` | Template lookup failed |
| `ErrTemplateThemeRequired` | `RegisterTemplate` called without a theme ID |
| `ErrTemplateNameRequired` | `RegisterTemplate` called without a name |
| `ErrTemplateSlugRequired` | `RegisterTemplate` called without a slug |
| `ErrTemplatePathRequired` | `RegisterTemplate` called without a template path |
| `ErrTemplateSlugConflict` | Another template in the same theme uses this slug |
| `ErrTemplateRegionsInvalid` | Region validation failed (no regions, or a region accepts neither blocks nor widgets) |

---

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/themes"
)

func main() {
    ctx := context.Background()

    // 1. Configure with themes enabled
    cfg := cms.DefaultConfig()
    cfg.Features.Themes = true
    cfg.Themes.DefaultTheme = "aurora"
    cfg.Themes.DefaultVariant = "light"
    cfg.Themes.BasePath = "./themes"

    module, err := cms.New(cfg)
    if err != nil {
        log.Fatal(err)
    }

    themeSvc := module.Themes()

    // 2. Register a theme
    theme, err := themeSvc.RegisterTheme(ctx, themes.RegisterThemeInput{
        Name:        "Aurora",
        Description: stringPtr("Default marketing theme"),
        Version:     "1.0.0",
        Author:      stringPtr("Go CMS"),
        ThemePath:   "themes/aurora",
        Config: themes.ThemeConfig{
            WidgetAreas: []themes.ThemeWidgetArea{
                {Code: "header.global", Name: "Global Header", Scope: "global"},
                {Code: "hero", Name: "Hero Banner", Scope: "template"},
                {Code: "sidebar", Name: "Sidebar", Scope: "page"},
            },
            MenuLocations: []themes.ThemeMenuLocation{
                {Code: "primary_navigation", Name: "Primary Navigation"},
                {Code: "footer_navigation", Name: "Footer Navigation"},
            },
            Assets: &themes.ThemeAssets{
                BasePath: stringPtr("public"),
                Styles:   []string{"css/base.css", "css/aurora.css"},
                Scripts:  []string{"js/site.js"},
            },
        },
    })
    if err != nil {
        log.Fatalf("register theme: %v", err)
    }
    fmt.Printf("Theme: %s v%s (id=%s)\n", theme.Name, theme.Version, theme.ID)

    // 3. Register templates
    landing, err := themeSvc.RegisterTemplate(ctx, themes.RegisterTemplateInput{
        ThemeID:      theme.ID,
        Name:         "Landing Page",
        Slug:         "landing",
        Description:  stringPtr("Full-width landing page layout"),
        TemplatePath: "templates/landing.html.tmpl",
        Regions: map[string]themes.TemplateRegion{
            "hero": {
                Name:          "Hero Banner",
                Description:   stringPtr("Full-width hero section"),
                AcceptsBlocks: true,
            },
            "content": {
                Name:           "Main Content",
                AcceptsBlocks:  true,
                AcceptsWidgets: true,
            },
            "sidebar": {
                Name:            "Sidebar",
                AcceptsWidgets:  true,
                FallbackRegions: []string{"content"},
            },
        },
    })
    if err != nil {
        log.Fatalf("register landing template: %v", err)
    }
    fmt.Printf("Template: %s (slug=%s)\n", landing.Name, landing.Slug)

    blogPost, err := themeSvc.RegisterTemplate(ctx, themes.RegisterTemplateInput{
        ThemeID:      theme.ID,
        Name:         "Blog Post",
        Slug:         "blog-post",
        TemplatePath: "templates/blog-post.html.tmpl",
        Regions: map[string]themes.TemplateRegion{
            "content": {
                Name:          "Article Body",
                AcceptsBlocks: true,
            },
            "sidebar": {
                Name:           "Sidebar",
                AcceptsWidgets: true,
            },
        },
    })
    if err != nil {
        log.Fatalf("register blog-post template: %v", err)
    }
    fmt.Printf("Template: %s (slug=%s)\n", blogPost.Name, blogPost.Slug)

    // 4. Activate the theme (requires at least one template)
    activated, err := themeSvc.ActivateTheme(ctx, theme.ID)
    if err != nil {
        log.Fatalf("activate theme: %v", err)
    }
    fmt.Printf("Activated: %s (isActive=%v)\n", activated.Name, activated.IsActive)

    // 5. Inspect regions for the landing template
    regions, err := themeSvc.TemplateRegions(ctx, landing.ID)
    if err != nil {
        log.Fatalf("template regions: %v", err)
    }
    fmt.Println("Landing Page regions:")
    for _, r := range regions {
        fmt.Printf("  %s (%s): blocks=%v widgets=%v\n", r.Key, r.Name, r.AcceptsBlocks, r.AcceptsWidgets)
    }

    // 6. Get the full region index for the theme
    regionIndex, err := themeSvc.ThemeRegionIndex(ctx, theme.ID)
    if err != nil {
        log.Fatalf("theme region index: %v", err)
    }
    fmt.Println("\nTheme region index:")
    for templateSlug, templateRegions := range regionIndex {
        fmt.Printf("  %s:\n", templateSlug)
        for _, r := range templateRegions {
            fmt.Printf("    %s: blocks=%v widgets=%v\n", r.Key, r.AcceptsBlocks, r.AcceptsWidgets)
        }
    }

    // 7. List all templates for the theme
    templates, err := themeSvc.ListTemplates(ctx, theme.ID)
    if err != nil {
        log.Fatalf("list templates: %v", err)
    }
    fmt.Printf("\nTheme has %d template(s)\n", len(templates))

    // 8. Get active theme summaries with resolved assets
    summaries, err := themeSvc.ListActiveSummaries(ctx)
    if err != nil {
        log.Fatalf("list summaries: %v", err)
    }
    for _, s := range summaries {
        fmt.Printf("\nActive theme: %s v%s\n", s.Theme.Name, s.Theme.Version)
        fmt.Printf("  Styles:  %v\n", s.Assets.Styles)
        fmt.Printf("  Scripts: %v\n", s.Assets.Scripts)
    }
}

func stringPtr(s string) *string { return &s }
```

---

## Next Steps

- [GUIDE_STATIC_GENERATION.md](GUIDE_STATIC_GENERATION.md) -- building static sites with the locale-aware generator
- [GUIDE_BLOCKS.md](GUIDE_BLOCKS.md) -- reusable content fragments placed in template regions
- [GUIDE_WIDGETS.md](GUIDE_WIDGETS.md) -- dynamic behavioral components with area-based placement
- [GUIDE_PAGES.md](GUIDE_PAGES.md) -- page hierarchy, routing paths, and page-block integration
- [GUIDE_CONFIGURATION.md](GUIDE_CONFIGURATION.md) -- full config reference and DI container wiring
