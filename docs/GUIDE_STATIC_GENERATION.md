# Static Site Generation Guide

This guide covers building static sites with the locale-aware generator in `go-cms`. By the end you will understand how to configure the generator, execute full and incremental builds, use the CLI, work with template context variables, and integrate the generator into CI/CD pipelines.

## Generator Architecture Overview

The static site generator transforms managed content into static HTML files. It resolves all dependencies for every page/locale combination -- content, translations, blocks, widgets, menus, themes -- renders them through a template engine, and writes the output to a storage backend.

```
Pages + Content + Translations
         |
         v
  BuildContext (resolve locales, load pages, fetch dependencies)
         |
         v
  TemplateContext (site metadata, page data, theme, helpers)
         |
         v
  TemplateRenderer (render HTML from template + context)
         |
         v
  ArtifactWriter (write HTML, assets, sitemap, feeds, robots.txt)
         |
         v
  BuildManifest (track checksums for incremental builds)
```

Key characteristics:

- **Multi-locale**: Generates separate HTML files for each locale. Default locale pages render at the root; non-default locales render under `/{locale}/`.
- **Incremental**: Tracks page and asset checksums in a `.generator-manifest.json` to skip unchanged content on subsequent builds.
- **Concurrent**: Renders pages in parallel using a configurable worker pool. Workers are grouped by locale for optimal cache locality.
- **Storage-agnostic**: Writes artifacts through a `StorageProvider` interface. Can target the file system, S3, or any custom backend.
- **Observable**: Returns detailed build metrics, per-page diagnostics, and supports lifecycle hooks for external integrations.

### Accessing the Service

The generator is exposed through the `cms.Module` facade:

```go
cfg := cms.DefaultConfig()
cfg.Generator.Enabled = true
cfg.Generator.OutputDir = "./dist"
cfg.Generator.BaseURL = "https://example.com"

module, err := cms.New(cfg, di.WithBunDB(db), di.WithTemplateRenderer(renderer))
if err != nil {
    log.Fatal(err)
}

gen := module.Generator()
```

The `gen` variable satisfies the `generator.Service` interface. When `cfg.Generator.Enabled` is `false`, the service returns a no-op implementation where all operations return `ErrServiceDisabled`.

A `TemplateRenderer` implementation is required. Without one, `Build` returns `errRendererRequired`.

---

## Configuration

### Generator Config

The `GeneratorConfig` section controls generator behavior:

```go
cfg := cms.DefaultConfig()

cfg.Generator.Enabled          = true
cfg.Generator.OutputDir        = "./dist"              // Required: output directory
cfg.Generator.BaseURL          = "https://example.com" // Site base URL for links and sitemaps
cfg.Generator.CleanBuild       = false                 // Remove all outputs before building
cfg.Generator.Incremental      = true                  // Use manifest to skip unchanged pages
cfg.Generator.CopyAssets       = true                  // Copy theme assets to output
cfg.Generator.GenerateSitemap  = true                  // Generate sitemap.xml
cfg.Generator.GenerateRobots   = true                  // Generate robots.txt
cfg.Generator.GenerateFeeds    = true                  // Generate RSS/Atom feeds
cfg.Generator.Workers          = 4                     // Concurrent render workers (0 = NumCPU)
cfg.Generator.RenderTimeout    = 30 * time.Second      // Per-template render timeout
cfg.Generator.AssetCopyTimeout = 60 * time.Second      // Asset copy timeout
cfg.Generator.Menus            = map[string]string{    // Menu code aliases
    "main":   "primary_navigation",
    "footer": "footer_navigation",
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Enabled` | `bool` | `false` | Feature flag for the generator |
| `OutputDir` | `string` | `""` | Required output directory for generated artifacts |
| `BaseURL` | `string` | `""` | Site base URL used for absolute links, sitemaps, and feeds |
| `CleanBuild` | `bool` | `false` | Remove all existing output before building |
| `Incremental` | `bool` | `false` | Enable manifest-based incremental builds |
| `CopyAssets` | `bool` | `false` | Copy theme assets (CSS, JS, images) to the output directory |
| `GenerateSitemap` | `bool` | `false` | Generate `sitemap.xml` |
| `GenerateRobots` | `bool` | `false` | Generate `robots.txt` |
| `GenerateFeeds` | `bool` | `false` | Generate RSS and Atom feeds |
| `Workers` | `int` | `0` | Number of concurrent render workers. `0` defaults to `runtime.NumCPU()` |
| `Menus` | `map[string]string` | `nil` | Maps template-friendly aliases to menu codes |
| `RenderTimeout` | `time.Duration` | `0` | Per-template render timeout. `0` means no timeout |
| `AssetCopyTimeout` | `time.Duration` | `0` | Asset copy timeout. `0` means no timeout |

### Theming Config

Theme variant selection is configured via the `Themes` section:

```go
cfg.Themes.DefaultTheme      = "aurora"   // Theme name for go-theme manifest loading
cfg.Themes.DefaultVariant     = "light"    // Variant name for token/CSS variable selection
cfg.Themes.CSSVariablePrefix  = "--cms-"   // Prefix for generated CSS custom properties
cfg.Themes.PartialFallbacks   = map[string]string{
    "sidebar": "default-sidebar",          // Partial template fallback mappings
}
```

### Dependencies

The generator requires several services wired through the DI container:

| Dependency | Required | Description |
|------------|----------|-------------|
| Pages service | Yes | Loads pages and page translations |
| Content service | Yes | Loads content records and content translations |
| Locale lookup | Yes | Resolves locale codes to locale records |
| Template renderer | Yes | Renders templates with the `TemplateContext` |
| Blocks service | No | Loads block instances for pages |
| Widgets service | No | Resolves widgets by area |
| Menus service | No | Resolves navigation trees by code and locale |
| Themes service | No | Loads templates and themes |
| Storage provider | No | Writes artifacts to disk/S3/etc. Without one, builds are effectively dry-runs |
| Asset resolver | No | Opens and resolves theme asset paths |
| Shortcode service | No | Processes shortcodes in content during rendering |
| Logger | No | Structured logging (defaults to no-op) |

Wire dependencies using `di.With*` options:

```go
module, err := cms.New(cfg,
    di.WithBunDB(db),
    di.WithTemplateRenderer(renderer),
    di.WithGeneratorStorage(storageProvider),
    di.WithAssetResolver(assetResolver),
    di.WithLoggerProvider(loggerProvider),
)
```

---

## Build Operations

The `Service` interface exposes five operations:

```go
type Service interface {
    Build(ctx context.Context, opts BuildOptions) (*BuildResult, error)
    BuildPage(ctx context.Context, pageID uuid.UUID, locale string) error
    BuildAssets(ctx context.Context) error
    BuildSitemap(ctx context.Context) error
    Clean(ctx context.Context) error
}
```

### Full Build

`Build` is the primary entry point. It resolves all page/locale combinations, renders them, copies assets, and generates sitemaps and feeds:

```go
result, err := gen.Build(ctx, generator.BuildOptions{
    Force:  false,   // Respect incremental manifest
    DryRun: false,   // Write artifacts
})
if err != nil {
    log.Fatalf("build failed: %v", err)
}

fmt.Printf("Built %d pages, skipped %d, copied %d assets\n",
    result.PagesBuilt, result.PagesSkipped, result.AssetsBuilt)
```

**Build phases:**

1. **Context loading** -- Resolve locales, load pages, fetch content/translations/blocks/widgets/menus/templates/themes for each page/locale pair.
2. **Rendering** -- Render each page through the template engine. Concurrent when `Workers > 1` and multiple pages exist. Pages are grouped by locale for worker distribution.
3. **Persistence** -- Write rendered HTML to the storage backend.
4. **Asset copying** -- Copy theme assets (CSS, JS, images) to the `assets/` subdirectory.
5. **Sitemap generation** -- Write `sitemap.xml` with all rendered page URLs.
6. **Feed generation** -- Write RSS (`feed.xml`) and Atom (`feed.atom.xml`) feeds, plus per-locale variants.
7. **Robots generation** -- Write `robots.txt` with sitemap reference.
8. **Manifest update** -- Persist `.generator-manifest.json` with checksums for incremental builds.

### BuildOptions

`BuildOptions` narrows the scope of a build:

```go
type BuildOptions struct {
    Locales    []string    // Build only these locales (empty = all configured locales)
    PageIDs    []uuid.UUID // Build only these pages (empty = all visible pages)
    DryRun     bool        // Execute without writing artifacts
    Force      bool        // Ignore manifest cache, rebuild everything
    AssetsOnly bool        // Copy theme assets only, skip page rendering
}
```

Examples:

```go
// Build only English pages
gen.Build(ctx, generator.BuildOptions{Locales: []string{"en"}})

// Rebuild a specific page in all locales
gen.Build(ctx, generator.BuildOptions{PageIDs: []uuid.UUID{pageID}, Force: true})

// Preview what would change without writing
gen.Build(ctx, generator.BuildOptions{DryRun: true})

// Copy only theme assets
gen.Build(ctx, generator.BuildOptions{AssetsOnly: true})
```

### Single Page Build

`BuildPage` rebuilds a single page (all its locales or a specific one). It always forces a rebuild, ignoring the manifest cache:

```go
// Rebuild all locales for a page
err := gen.BuildPage(ctx, pageID, "")

// Rebuild a specific locale
err := gen.BuildPage(ctx, pageID, "fr")
```

### Asset Build

`BuildAssets` copies theme assets without rendering pages:

```go
err := gen.BuildAssets(ctx)
```

### Sitemap Build

`BuildSitemap` regenerates `sitemap.xml` and `robots.txt` without rebuilding pages. It uses the manifest to include previously rendered pages:

```go
err := gen.BuildSitemap(ctx)
```

### Clean

`Clean` removes all artifacts from the configured output directory:

```go
err := gen.Clean(ctx)
```

---

## BuildResult

Every `Build` call returns a `BuildResult` with detailed metrics:

```go
type BuildResult struct {
    PagesBuilt    int                // Pages successfully rendered
    PagesSkipped  int                // Pages skipped via incremental cache
    AssetsBuilt   int                // Assets copied
    AssetsSkipped int                // Assets skipped via cache
    FeedsBuilt    int                // Feed documents generated
    Locales       []string           // Active locales in this build
    Duration      time.Duration      // Total build time
    Rendered      []RenderedPage     // Details for each rendered page
    Diagnostics   []RenderDiagnostic // Per-page timing and error info
    Errors        []error            // Accumulated errors (build may partially succeed)
    DryRun        bool               // Whether this was a dry-run
    Metrics       BuildMetrics       // Detailed timing breakdown
}
```

### Build Metrics

```go
type BuildMetrics struct {
    ContextDuration       time.Duration  // Time to load all dependencies
    RenderDuration        time.Duration  // Time to render all pages
    PersistDuration       time.Duration  // Time to write artifacts
    AssetDuration         time.Duration  // Time to copy assets
    SitemapDuration       time.Duration  // Time to generate sitemap
    RobotsDuration        time.Duration  // Time to generate robots.txt
    FeedDuration          time.Duration  // Time to generate feeds
    PagesPerSecond        float64        // Render throughput
    AssetsPerSecond       float64        // Asset copy throughput
    SkippedPagesPerSecond float64        // Cache skip rate
}
```

### Rendered Page

Each successfully rendered page produces a `RenderedPage`:

```go
type RenderedPage struct {
    PageID   uuid.UUID
    Locale   string
    Route    string             // URL path (e.g., "/about")
    Output   string             // File path (e.g., "dist/about/index.html")
    Template string             // Template name used
    HTML     string             // Rendered HTML content
    Metadata DependencyMetadata // Dependency hashes for incremental tracking
    Duration time.Duration      // Render time for this page
    Checksum string             // SHA-256 of the rendered HTML
}
```

### Render Diagnostics

Each page (including skipped ones) produces a diagnostic:

```go
type RenderDiagnostic struct {
    PageID   uuid.UUID
    Locale   string
    Route    string
    Template string
    Duration time.Duration
    Skipped  bool  // True if skipped via incremental cache
    Err      error // Non-nil if rendering failed
}
```

### Inspecting Results

```go
result, err := gen.Build(ctx, generator.BuildOptions{})
if err != nil {
    // Build had errors, but may have partially succeeded
    log.Printf("Build completed with errors: %v", err)
}

// Log summary
log.Printf("Built %d pages in %v (%.2f pages/sec)",
    result.PagesBuilt, result.Duration, result.Metrics.PagesPerSecond)

// Inspect individual pages
for _, page := range result.Rendered {
    log.Printf("  %s (%s) -> %s", page.Route, page.Locale, page.Output)
}

// Check diagnostics for errors
for _, diag := range result.Diagnostics {
    if diag.Err != nil {
        log.Printf("  ERROR: page %s [%s]: %v", diag.PageID, diag.Locale, diag.Err)
    }
}

// Detailed timing breakdown
m := result.Metrics
log.Printf("Timing: context=%v render=%v persist=%v assets=%v sitemap=%v feeds=%v",
    m.ContextDuration, m.RenderDuration, m.PersistDuration,
    m.AssetDuration, m.SitemapDuration, m.FeedDuration)
```

---

## Template Context Variables

Templates receive a `TemplateContext` struct with four top-level sections:

```go
type TemplateContext struct {
    Site    SiteMetadata          // Site-level information
    Page    PageRenderingContext  // Page and content data
    Build   BuildMetadata         // Build metadata
    Theme   ThemeContext           // Theme and variant information
    Helpers TemplateHelpers       // Convenience helper methods
}
```

### Site Metadata

| Variable | Type | Description |
|----------|------|-------------|
| `{{ .Site.BaseURL }}` | `string` | Configured site base URL |
| `{{ .Site.DefaultLocale }}` | `string` | Primary locale code |
| `{{ .Site.Locales }}` | `[]LocaleSpec` | All configured locales (`.Code`, `.IsDefault`) |
| `{{ .Site.MenuAliases }}` | `map[string]string` | Menu alias-to-code mappings |
| `{{ .Site.Metadata }}` | `map[string]any` | Custom site metadata |

### Page Context

| Variable | Type | Description |
|----------|------|-------------|
| `{{ .Page.Page }}` | `*pages.Page` | Page record (ID, slug, status, visibility) |
| `{{ .Page.Content }}` | `*content.Content` | Content record (ID, slug, status) |
| `{{ .Page.Translation }}` | `*pages.PageTranslation` | Localized page data (path, title, summary) |
| `{{ .Page.ContentTranslation }}` | `*content.ContentTranslation` | Localized content data (title, body, summary) |
| `{{ .Page.Blocks }}` | `[]*blocks.Instance` | Block instances attached to this page |
| `{{ .Page.Widgets }}` | `map[string][]*widgets.ResolvedWidget` | Widgets grouped by area name |
| `{{ .Page.Menus }}` | `map[string][]menus.NavigationNode` | Navigation trees keyed by alias |
| `{{ .Page.Template }}` | `*themes.Template` | Template record (slug, path, regions) |
| `{{ .Page.Theme }}` | `*themes.Theme` | Theme record (name, version, config) |
| `{{ .Page.Locale }}` | `LocaleSpec` | Active locale (`.Code`, `.IsDefault`) |
| `{{ .Page.Metadata }}` | `DependencyMetadata` | Dependency hash and last modified time |

### Build Metadata

| Variable | Type | Description |
|----------|------|-------------|
| `{{ .Build.GeneratedAt }}` | `time.Time` | Build timestamp |
| `{{ .Build.Options }}` | `BuildOptions` | Original build options |

### Theme Context

| Variable | Type | Description |
|----------|------|-------------|
| `{{ .Theme.Name }}` | `string` | Active theme name |
| `{{ .Theme.Variant }}` | `string` | Selected variant |
| `{{ .Theme.Tokens }}` | `map[string]string` | Design tokens (key-value pairs) |
| `{{ .Theme.CSSVars }}` | `map[string]string` | CSS custom properties with prefix |
| `{{ .Theme.Partials }}` | `map[string]string` | Partial template paths |
| `{{ .Theme.AssetURL "key" }}` | `func(string) string` | Resolve asset URL by key |
| `{{ .Theme.Template "name" "fallback" }}` | `func(string, string) string` | Get template path with fallback |

### Template Helpers

| Helper | Return Type | Description |
|--------|-------------|-------------|
| `{{ .Helpers.Locale }}` | `string` | Active locale code |
| `{{ .Helpers.IsLocale "en" }}` | `bool` | Check if current locale matches |
| `{{ .Helpers.IsDefaultLocale }}` | `bool` | Whether this is the default locale |
| `{{ .Helpers.BaseURL }}` | `string` | Site base URL |
| `{{ .Helpers.WithBaseURL "/path" }}` | `string` | Prepend base URL to path (handles absolute URLs) |
| `{{ .Helpers.LocalePrefix }}` | `string` | Locale path prefix (empty for default locale, `"/fr"` for non-default) |

### Template Example

```html
<!DOCTYPE html>
<html lang="{{ .Helpers.Locale }}">
<head>
    <meta charset="utf-8">
    <title>{{ .Page.ContentTranslation.Title }} - {{ .Site.BaseURL }}</title>
    <link rel="stylesheet" href="{{ .Theme.AssetURL "styles" }}">

    <style>
        :root {
            {{ range $key, $value := .Theme.CSSVars }}
            {{ $key }}: {{ $value }};
            {{ end }}
        }
    </style>
</head>
<body>
    <nav>
        {{ range $node := index .Page.Menus "main" }}
        <a href="{{ $.Helpers.WithBaseURL $node.URL }}">{{ $node.Label }}</a>
        {{ end }}
    </nav>

    <main>
        <h1>{{ .Page.ContentTranslation.Title }}</h1>
        {{ range $key, $value := .Page.ContentTranslation.Content }}
            <div class="content-{{ $key }}">{{ $value }}</div>
        {{ end }}
    </main>

    {{ if not .Helpers.IsDefaultLocale }}
    <p>Viewing in {{ .Helpers.Locale }}</p>
    {{ end }}

    <footer>
        <p>Generated {{ .Build.GeneratedAt.Format "2006-01-02" }}</p>
    </footer>

    <script src="{{ .Theme.AssetURL "scripts" }}"></script>
</body>
</html>
```

---

## Output Path Structure

The generator produces a locale-aware directory structure. The default locale renders at the root; non-default locales render under a `/{locale}/` prefix.

```
dist/
  index.html                          # Default locale root page
  about/index.html                    # Default locale "/about"
  blog/my-post/index.html             # Default locale "/blog/my-post"
  fr/
    index.html                        # French root page
    about/index.html                  # French "/about"
    blog/my-post/index.html           # French "/blog/my-post"
  es/
    index.html                        # Spanish root page
  assets/
    css/base.css                      # Theme stylesheet
    js/site.js                        # Theme script
    images/logo.png                   # Theme image
  sitemap.xml                         # XML sitemap
  robots.txt                          # Robots file
  feed.xml                            # Default RSS feed
  feed.atom.xml                       # Default Atom feed
  feeds/
    en.rss.xml                        # Per-locale RSS feed
    fr.rss.xml
    en.atom.xml                       # Per-locale Atom feed
    fr.atom.xml
  .generator-manifest.json            # Incremental build manifest
```

Path resolution rules:
- Default locale pages render directly under the output directory
- Non-default locale pages render under `/{locale}/`
- If a route already starts with the locale code, the prefix is not duplicated
- All pages use `index.html` for clean URL support (directory-style)

---

## Incremental Builds

When `cfg.Generator.Incremental = true`, the generator maintains a `.generator-manifest.json` that tracks checksums for every rendered page and copied asset.

### How It Works

1. Before rendering a page, the generator computes a `DependencyMetadata` hash from:
   - Page record (ID, slug, status, updated timestamp, published version)
   - Page translation (ID, path, title, updated timestamp)
   - Content record (ID, slug, status, updated timestamp, published version)
   - Content translation (ID, title, content hash, updated timestamp)
   - Block instances (IDs, regions, positions, versions, updated timestamps)
   - Widget instances (IDs, area placements, positions, updated timestamps)
   - Menu navigation trees (node IDs, labels, URLs, children)
   - Template (ID, name, updated timestamp)
   - Theme (ID, name, version)

2. The combined SHA-256 hash is compared against the manifest entry for the same page/locale.

3. If the hash matches and the output path is unchanged, the page is skipped.

4. Assets use file-content checksums. Unchanged assets are skipped.

### Manifest Structure

```json
{
  "version": 1,
  "generated_at": "2025-01-30T09:45:00Z",
  "pages": {
    "page-uuid::en": {
      "page_id": "page-uuid",
      "locale": "en",
      "route": "/about",
      "output": "dist/about/index.html",
      "template": "templates/page.tpl",
      "hash": "sha256-of-dependencies...",
      "checksum": "sha256-of-html...",
      "last_modified": "2025-01-30T08:00:00Z",
      "rendered_at": "2025-01-30T09:45:00Z"
    }
  },
  "assets": {
    "theme-uuid::css/base.css": {
      "key": "theme-uuid::css/base.css",
      "theme_id": "theme-uuid",
      "source": "css/base.css",
      "output": "dist/assets/css/base.css",
      "checksum": "sha256...",
      "size": 12345,
      "copied_at": "2025-01-30T09:45:00Z"
    }
  },
  "metadata": {}
}
```

### Forcing a Full Rebuild

Use `Force: true` to ignore the manifest and rebuild everything:

```go
gen.Build(ctx, generator.BuildOptions{Force: true})
```

Or from the CLI:

```bash
go run cmd/static/main.go build --force
```

---

## Lifecycle Hooks

The generator supports lifecycle hooks for extending build behavior:

```go
type Hooks struct {
    BeforeBuild func(context.Context, BuildOptions) error
    AfterBuild  func(context.Context, BuildOptions, *BuildResult) error
    AfterPage   func(context.Context, RenderedPage) error
    BeforeClean func(context.Context, string) error
    AfterClean  func(context.Context, string) error
}
```

Hooks can be used to:
- Validate or modify build options before rendering
- Publish notifications after successful builds
- Trigger CDN cache invalidation after pages are rendered
- Log build outcomes to external systems
- Clean up external caches before or after a clean operation

All hooks are optional. When not provided, they are silently skipped.

---

## Shortcode Processing

When a shortcode service is configured, the generator processes shortcodes in content during rendering. Shortcode syntax is detected in:

- Content translation `Content` map values (recursively, including nested maps and arrays)
- Content translation `Summary` field
- Page translation `Summary` field

Supported syntax patterns:
- Hugo-style: `{{< shortcode >}}` and `{{% shortcode %}}`
- WordPress-style: `[shortcode]` (when enabled)

Enable shortcode processing:

```go
cfg.Features.Shortcodes = true
cfg.Shortcodes.Enabled = true
cfg.Markdown.ProcessShortcodes = true
```

---

## CLI Usage

The `cmd/static/main.go` binary provides four subcommands:

### Build

```bash
go run cmd/static/main.go build [flags]
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output` | `string` | config value | Output directory |
| `--base-url` | `string` | config value | Site base URL |
| `--translations-enabled` | `bool` | `true` | Enable i18n translations |
| `--require-translations` | `bool` | `true` | Require at least one translation |
| `--page` | `string` | `""` | Comma-separated page UUIDs to build |
| `--locale` | `string` | `""` | Comma-separated locales to include |
| `--force` | `bool` | `false` | Force rebuild (ignore manifest cache) |
| `--dry-run` | `bool` | `false` | Execute without writing artifacts |
| `--assets` | `bool` | `false` | Copy theme assets only |

Examples:

```bash
# Full build
go run cmd/static/main.go build --output ./dist --base-url https://example.com

# Build specific pages
go run cmd/static/main.go build --page "uuid1,uuid2"

# Build specific locales
go run cmd/static/main.go build --locale "en,fr"

# Force rebuild with asset copy
go run cmd/static/main.go build --force

# Dry-run preview
go run cmd/static/main.go build --dry-run

# Assets only
go run cmd/static/main.go build --assets
```

### Diff

Runs a dry-run build to preview what would change:

```bash
go run cmd/static/main.go diff [flags]
```

Supports `--output`, `--base-url`, `--page`, `--locale`, and `--force` flags.

### Clean

Removes all artifacts from the output directory:

```bash
go run cmd/static/main.go clean
```

### Sitemap

Regenerates sitemap.xml and robots.txt without rebuilding pages:

```bash
go run cmd/static/main.go sitemap
```

### CLI Output

The CLI logs structured output with build metrics:

```
module=static operation=build summary pages_built=42 pages_skipped=0
  assets_built=5 assets_skipped=0 duration=1.234s dry_run=false
  context_ms=120 render_ms=800 persist_ms=200 assets_ms=50 sitemap_ms=10
  pages_per_sec=52.50 assets_per_sec=100.00
```

Per-page diagnostics are logged when errors occur:

```
module=static operation=build page=<uuid> locale=en route=/about template=page.tpl status=ok
module=static operation=build page=<uuid> locale=fr route=/about template=page.tpl status=error
module=static operation=build page=<uuid> locale=fr err=generator: render template "page.tpl" ...
```

---

## Sitemap, Robots, and Feeds

### Sitemap

When `cfg.Generator.GenerateSitemap = true`, the generator writes a `sitemap.xml` in W3C XML Sitemap format:

- Includes all rendered pages with `<lastmod>` timestamps
- Deduplicates URLs
- Sorted alphabetically
- Includes previously rendered pages from the manifest (for incremental builds)

### Robots.txt

When `cfg.Generator.GenerateRobots = true`:

```
User-agent: *
Allow: /
Sitemap: https://example.com/sitemap.xml
```

The `Sitemap` directive is only included when sitemap generation is also enabled.

### RSS and Atom Feeds

When `cfg.Generator.GenerateFeeds = true`:

- `feed.xml` -- Default RSS feed with all pages
- `feed.atom.xml` -- Default Atom feed
- `feeds/<locale>.rss.xml` -- Per-locale RSS feeds
- `feeds/<locale>.atom.xml` -- Per-locale Atom feeds

Feeds include page titles, summaries, links, and GUIDs, sorted by published date (descending), with a maximum of 100 items per feed.

---

## Concurrency Model

The generator uses a worker pool for concurrent page rendering:

1. Pages are grouped by locale.
2. Each locale group is dispatched as a batch to the worker pool.
3. Workers process pages within their assigned batch sequentially.
4. Results are collected through a thread-safe callback.

Worker count behavior:
- `cfg.Generator.Workers = 0` (default): Uses `runtime.NumCPU()`
- Workers are capped to the number of locales (no benefit from more workers than locale groups)
- Single page or single worker: Falls back to sequential rendering

Context cancellation is respected at each page boundary. If the context is cancelled mid-build, remaining pages receive a cancellation diagnostic.

---

## COLABS Site Module Reference

The `site/` directory contains a complete reference implementation demonstrating the generator in a production context.

### Key Components

| Path | Purpose |
|------|---------|
| `site/cmd/c8e/static` | Full static generator CLI |
| `site/cmd/c8e/import` | Markdown-to-JSON content importer |
| `site/cmd/c8e/assetsync` | Asset inventory synchronization |
| `site/themes/collective-labs/` | Theme templates and assets |
| `site/content/` | Markdown content fixtures |

### Build Tasks

```bash
# Full build pipeline: markdown sync + static generation + reference assets
./taskfile colabs:build:static

# Sync markdown case studies
./taskfile colabs:test

# Compare output against reference
./taskfile colabs:verify:regressions

# Visual regression tests (Playwright)
./taskfile colabs:verify:regressions
```

The COLABS module demonstrates:
- Multi-locale content with English and Spanish translations
- Template-based rendering with Go `html/template`
- Theme asset management with hashed filenames
- Markdown content ingestion and synchronization
- Visual regression testing against reference output

---

## Integration with CI/CD

### Basic Pipeline

```bash
#!/bin/bash
set -e

# 1. Build the static site
go run cmd/static/main.go build \
  --output ./dist \
  --base-url https://example.com \
  --force

# 2. Verify the build produced output
if [ ! -f ./dist/index.html ]; then
  echo "Build failed: no index.html found"
  exit 1
fi

# 3. Deploy (example: sync to S3)
aws s3 sync ./dist/ s3://my-bucket/ --delete
```

### Incremental Builds

For incremental CI builds, persist the `.generator-manifest.json` between runs:

```bash
# Restore manifest from cache
cp .cache/.generator-manifest.json ./dist/ 2>/dev/null || true

# Run incremental build
go run cmd/static/main.go build --output ./dist --base-url https://example.com

# Cache manifest for next run
mkdir -p .cache
cp ./dist/.generator-manifest.json .cache/
```

### Programmatic Integration

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/di"
    "github.com/goliatone/go-cms/internal/generator"
)

func main() {
    cfg := cms.DefaultConfig()
    cfg.Generator.Enabled = true
    cfg.Generator.OutputDir = "./dist"
    cfg.Generator.BaseURL = "https://example.com"
    cfg.Generator.GenerateSitemap = true
    cfg.Generator.GenerateRobots = true
    cfg.Generator.GenerateFeeds = true
    cfg.Generator.Incremental = true
    cfg.Generator.Workers = 4
    cfg.Generator.Menus = map[string]string{
        "main":   "primary_navigation",
        "footer": "footer_navigation",
    }

    module, err := cms.New(cfg,
        di.WithBunDB(db),
        di.WithTemplateRenderer(renderer),
        di.WithGeneratorStorage(storageProvider),
    )
    if err != nil {
        log.Fatalf("init: %v", err)
    }

    gen := module.Generator()
    ctx := context.Background()

    // Full build
    result, err := gen.Build(ctx, generator.BuildOptions{})
    if err != nil {
        log.Fatalf("build: %v", err)
    }

    fmt.Printf("Built %d pages in %v\n", result.PagesBuilt, result.Duration)
    fmt.Printf("Skipped %d unchanged pages\n", result.PagesSkipped)
    fmt.Printf("Copied %d assets\n", result.AssetsBuilt)
    fmt.Printf("Throughput: %.2f pages/sec\n", result.Metrics.PagesPerSecond)

    // Check for errors in diagnostics
    for _, diag := range result.Diagnostics {
        if diag.Err != nil {
            log.Printf("Page %s [%s] error: %v", diag.PageID, diag.Locale, diag.Err)
        }
    }
}
```

---

## Error Reference

| Error | Cause |
|-------|-------|
| `ErrServiceDisabled` | Generator operations called but `cfg.Generator.Enabled` is false |
| `ErrNotImplemented` | Operation not yet supported |
| `errRendererRequired` | `Build` called without a `TemplateRenderer` dependency |
| `errTemplateRequired` | Page has no associated template |
| `errTemplateIdentifierMissing` | Template has neither `TemplatePath` nor `Slug` set |
| `errPagesServiceRequired` | Pages service not wired in the DI container |
| `errContentServiceRequired` | Content service not wired in the DI container |
| `errLocaleLookupRequired` | Locale lookup not wired in the DI container |

Build errors are accumulated in `BuildResult.Errors`. A build may partially succeed -- some pages render while others fail. Check `result.Diagnostics` for per-page error details.

---

## Next Steps

- [GUIDE_THEMES.md](GUIDE_THEMES.md) -- theme management, templates, and asset resolution
- [GUIDE_PAGES.md](GUIDE_PAGES.md) -- page hierarchy, routing paths, and page-block integration
- [GUIDE_CONTENT.md](GUIDE_CONTENT.md) -- content types, content entries, and translations
- [GUIDE_BLOCKS.md](GUIDE_BLOCKS.md) -- reusable content fragments placed in template regions
- [GUIDE_MENUS.md](GUIDE_MENUS.md) -- navigation structures and URL resolution
- [GUIDE_I18N.md](GUIDE_I18N.md) -- internationalization and translation workflows
- [GUIDE_MARKDOWN.md](GUIDE_MARKDOWN.md) -- importing and syncing markdown content
- [GUIDE_SHORTCODES.md](GUIDE_SHORTCODES.md) -- shortcode processing in markdown and templates
- [GUIDE_CONFIGURATION.md](GUIDE_CONFIGURATION.md) -- full config reference and DI container wiring
