# Markdown Guide

This guide covers importing and syncing markdown content with frontmatter support in `go-cms`. By the end you will understand how to load markdown files from disk, render them to HTML, import them as CMS content entries, and keep your CMS in sync with a directory of markdown files.

## Markdown Architecture Overview

The markdown module bridges filesystem-based content authoring with the CMS data model. Authors write markdown files with YAML frontmatter, and the module converts them into content entries (and optionally pages) inside `go-cms`.

The pipeline has three stages:

```
Filesystem (.md files)
  └── Load (parse frontmatter, detect locale, compute checksum)
        └── Render (convert markdown to HTML via Goldmark)
              └── Import / Sync (create or update CMS content and pages)
```

- **Loading** reads markdown files, extracts YAML frontmatter, detects the locale from the directory structure, and computes a SHA-256 checksum for change detection.
- **Rendering** converts the markdown body to HTML using the Goldmark engine with configurable extensions. Shortcode expansion happens before parsing when enabled.
- **Importing** creates or updates content entries and pages in the CMS, grouping multilingual files by slug. **Syncing** extends import with orphan deletion and repeated-run semantics.

All operations are exposed through the `interfaces.MarkdownService` interface and three CLI tools.

### Accessing the Service

```go
cfg := cms.DefaultConfig()
cfg.Features.Markdown = true
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es"}

module, err := cms.New(cfg)
if err != nil {
    log.Fatal(err)
}

mdSvc := module.Markdown()
```

When `Features.Markdown` is `false`, `module.Markdown()` returns `nil`. The service delegates to in-memory repositories by default, or SQL-backed repositories when `di.WithBunDB(db)` is provided.

---

## Enabling Markdown

Enable the markdown feature flag and configure its behaviour:

```go
cfg := cms.DefaultConfig()
cfg.Features.Markdown = true

cfg.Markdown.ContentDir = "./content"
cfg.Markdown.Pattern = "*.md"
cfg.Markdown.Recursive = true
cfg.Markdown.DefaultLocale = "en"
cfg.Markdown.Locales = []string{"en", "es"}
```

### Configuration Reference

**`cfg.Markdown` fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ContentDir` | `string` | `"content"` | Root directory for markdown files |
| `Pattern` | `string` | `"*.md"` | Glob pattern for file discovery |
| `Recursive` | `bool` | `false` | Traverse subdirectories |
| `DefaultLocale` | `string` | `"en"` | Fallback locale when detection fails |
| `Locales` | `[]string` | `[]` | Known locale codes for directory matching |
| `LocalePatterns` | `map[string]string` | `nil` | Maps locale codes to glob patterns |
| `ProcessShortcodes` | `bool` | `false` | Expand shortcodes before rendering |
| `Parser.Extensions` | `[]string` | `[]` | Goldmark extensions to enable |
| `Parser.Sanitize` | `bool` | `false` | Scrub raw HTML from output |
| `Parser.HardWraps` | `bool` | `false` | Convert line breaks to `<br>` tags |
| `Parser.SafeMode` | `bool` | `false` | Disallow raw HTML in markdown |

---

## Loading Documents

### Loading a Single File

`Load` reads one markdown file relative to the configured `ContentDir`, parses its frontmatter, detects the locale, and renders the body to HTML:

```go
ctx := context.Background()

doc, err := mdSvc.Load(ctx, "en/about.md", interfaces.LoadOptions{})
if err != nil {
    log.Fatalf("load: %v", err)
}

fmt.Printf("Title:  %s\n", doc.FrontMatter.Title)
fmt.Printf("Slug:   %s\n", doc.FrontMatter.Slug)
fmt.Printf("Locale: %s\n", doc.Locale)
fmt.Printf("HTML:   %s\n", string(doc.BodyHTML))
```

The returned `Document` is fully populated -- `BodyHTML` contains the rendered HTML.

### Loading a Directory

`LoadDirectory` walks a directory tree, discovers all matching markdown files, and returns them sorted by file path:

```go
docs, err := mdSvc.LoadDirectory(ctx, ".", interfaces.LoadOptions{})
if err != nil {
    log.Fatalf("load directory: %v", err)
}

for _, doc := range docs {
    fmt.Printf("%s [%s] — %s\n", doc.FilePath, doc.Locale, doc.FrontMatter.Title)
}
```

### Document Type

Every loaded file becomes a `Document`:

| Field | Type | Description |
|-------|------|-------------|
| `FilePath` | `string` | Relative path within the content root |
| `Locale` | `string` | Detected locale code |
| `FrontMatter` | `FrontMatter` | Parsed YAML metadata |
| `Body` | `[]byte` | Raw markdown (without frontmatter delimiters) |
| `BodyHTML` | `[]byte` | Rendered HTML (populated after rendering) |
| `LastModified` | `time.Time` | File modification timestamp |
| `Checksum` | `[]byte` | SHA-256 digest of the original file |

### LoadOptions

Override discovery behaviour per call:

| Field | Type | Description |
|-------|------|-------------|
| `Recursive` | `*bool` | Override the service-level recursion setting |
| `Pattern` | `string` | Override the file glob pattern |
| `LocalePatterns` | `map[string]string` | Override locale detection patterns |
| `Parser` | `ParseOptions` | Override rendering options for this load |

```go
recursive := true
docs, err := mdSvc.LoadDirectory(ctx, "blog", interfaces.LoadOptions{
    Recursive: &recursive,
    Pattern:   "*.markdown",
})
```

---

## YAML Frontmatter

Every markdown file starts with a YAML block delimited by `---`. The parser extracts structured metadata and passes the remaining body to the renderer.

### Example File

```markdown
---
title: About Us
slug: about
summary: Learn more about our company
status: published
template: standard_page
tags:
  - company
  - info
author: Jane Doe
date: 2024-01-15
draft: false
hero_image: /images/about-hero.jpg
---

# About Us

Welcome to our company. We build tools for content management...
```

### FrontMatter Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Title` | `string` | No | Display title; falls back to slug if empty |
| `Slug` | `string` | **Yes** (for import) | URL-safe identifier; groups multilingual files |
| `Summary` | `string` | No | Short description for listings and SEO |
| `Status` | `string` | No | Content status; defaults to `"draft"` |
| `Template` | `string` | No | Template name for page creation |
| `Tags` | `[]string` | No | Categorisation tags |
| `Author` | `string` | No | Author name or identifier |
| `Date` | `time.Time` | No | Publication date |
| `Draft` | `bool` | No | Draft flag |
| `Custom` | `map[string]any` | No | Extra YAML fields (captured via inline tag) |
| `Raw` | `map[string]any` | -- | Full map of all parsed fields |

The `Custom` map captures any YAML keys that are not part of the standard fields. In the example above, `hero_image` would appear in `doc.FrontMatter.Custom["hero_image"]`. The `Raw` map contains every field including standard ones, useful for passing all metadata downstream.

### Slug Requirement

The `slug` field is **required** for import and sync operations. It determines the content entry's identifier and groups multilingual files together. Files with the same slug but different locales become translations of the same content entry:

```
content/
  en/about.md    (slug: about, locale: en)
  es/about.md    (slug: about, locale: es)
  ↓
  Single content entry "about" with English and Spanish translations
```

---

## Rendering

### Rendering Raw Markdown

`Render` converts raw markdown bytes to HTML without loading from a file:

```go
markdown := []byte("# Hello\n\nThis is **bold** text.")

html, err := mdSvc.Render(ctx, markdown, interfaces.ParseOptions{})
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(html))
// <h1 id="hello">Hello</h1>
// <p>This is <strong>bold</strong> text.</p>
```

### Rendering a Document

`RenderDocument` renders a previously loaded document and populates its `BodyHTML` field in place:

```go
doc, _ := mdSvc.Load(ctx, "en/about.md", interfaces.LoadOptions{})

html, err := mdSvc.RenderDocument(ctx, doc, interfaces.ParseOptions{
    HardWraps: true,
})
if err != nil {
    log.Fatal(err)
}
// doc.BodyHTML is now populated
```

### Goldmark Pipeline Configuration

The markdown module uses [Goldmark](https://github.com/yuin/goldmark) for rendering. When no extensions are specified, the following defaults are enabled:

- **GFM** (GitHub Flavored Markdown) -- tables, strikethrough, autolinks
- **Linkify** -- auto-link bare URLs
- **TaskList** -- checkbox syntax in lists

You can select specific extensions via `ParseOptions.Extensions` or `cfg.Markdown.Parser.Extensions`:

| Extension Name | Aliases | Description |
|---------------|---------|-------------|
| `gfm` | -- | GitHub Flavored Markdown (tables + strikethrough + autolinks) |
| `table` | `tables` | Pipe-delimited tables |
| `strikethrough` | -- | `~~deleted~~` syntax |
| `linkify` | `autolink` | Auto-link bare URLs |
| `tasklist` | -- | `- [x]` checkbox lists |
| `definition` | -- | Definition lists |
| `footnote` | -- | Footnote references |

```go
html, err := mdSvc.Render(ctx, markdown, interfaces.ParseOptions{
    Extensions: []string{"gfm", "footnote", "definition"},
    HardWraps:  true,
    SafeMode:   false,
})
```

**ParseOptions fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Extensions` | `[]string` | `[]` (uses GFM+Linkify+TaskList) | Goldmark extension names |
| `Sanitize` | `bool` | `false` | Scrub raw HTML from output |
| `HardWraps` | `bool` | `false` | Line breaks become `<br>` tags |
| `SafeMode` | `bool` | `false` | Disallow raw HTML passthrough |
| `ProcessShortcodes` | `bool` | `false` | Expand shortcodes before parsing |
| `ShortcodeOptions` | `ShortcodeProcessOptions` | -- | Shortcode-specific settings |

When both `SafeMode` and `Sanitize` are `false`, raw HTML embedded in markdown is passed through unchanged. Heading IDs are always auto-generated from heading text.

---

## Import Workflow

Importing converts loaded markdown documents into CMS content entries. The importer groups files by their frontmatter `slug`, creates or updates content entries with translations, and optionally creates page entities.

### Importing a Single Document

```go
doc, err := mdSvc.Load(ctx, "en/about.md", interfaces.LoadOptions{})
if err != nil {
    log.Fatal(err)
}

result, err := mdSvc.Import(ctx, doc, interfaces.ImportOptions{
    ContentTypeID: articleTypeID,
    AuthorID:      authorID,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Created: %d, Updated: %d, Skipped: %d\n",
    len(result.CreatedContentIDs),
    len(result.UpdatedContentIDs),
    len(result.SkippedContentIDs),
)
```

### Importing a Directory

`ImportDirectory` loads all markdown files in a directory, groups them by slug, and imports each group:

```go
result, err := mdSvc.ImportDirectory(ctx, ".", interfaces.ImportOptions{
    ContentTypeID: articleTypeID,
    AuthorID:      authorID,
    CreatePages:   true,
    TemplateID:    &templateID,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Created %d content entries\n", len(result.CreatedContentIDs))
```

### ImportOptions Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `ContentTypeID` | `uuid.UUID` | **Yes** | Content type for imported entries |
| `AuthorID` | `uuid.UUID` | **Yes** | Actor ID recorded on created/updated records |
| `CreatePages` | `bool` | No | Also create page entities for each content entry |
| `TemplateID` | `*uuid.UUID` | When `CreatePages` | Template ID for created pages |
| `DryRun` | `bool` | No | Preview changes without persisting |
| `EnvironmentKey` | `string` | No | Environment scope for content lookups |
| `ContentAllowMissingTranslations` | `bool` | No | Bypass translation requirements for content |
| `PageAllowMissingTranslations` | `bool` | No | Bypass translation requirements for pages |
| `ProcessShortcodes` | `bool` | No | Expand shortcodes during rendering |

### ImportResult Structure

| Field | Type | Description |
|-------|------|-------------|
| `CreatedContentIDs` | `[]uuid.UUID` | IDs of newly created content entries |
| `UpdatedContentIDs` | `[]uuid.UUID` | IDs of updated content entries |
| `SkippedContentIDs` | `[]uuid.UUID` | IDs of unchanged content entries |
| `Errors` | `[]error` | Errors encountered during import |

### How Import Works

1. **Validate** -- each document must have a `slug` in frontmatter and a detected locale.
2. **Group by slug** -- files with the same slug become translations of one content entry.
3. **Render** -- markdown body is converted to HTML.
4. **Lookup** -- check if a content entry with this slug already exists (`GetBySlug`).
5. **Create or update** -- if new, create a content entry with translations. If existing, compare translations and update only when changes are detected (title, summary, or checksum differ).
6. **Page creation** -- when `CreatePages` is true, a page entity is created or updated alongside the content entry, using the slug as the page path.

Content metadata records the import source:

```json
{
  "source": "markdown",
  "documents": [
    {
      "path": "en/about.md",
      "locale": "en",
      "checksum": "a1b2c3...",
      "template": "standard_page",
      "tags": ["company"],
      "title": "About Us",
      "timestamp": "2024-01-15T00:00:00Z"
    }
  ]
}
```

Translation fields store the markdown body, rendered HTML, checksum, and frontmatter:

```json
{
  "markdown": {
    "body": "# About Us\n\nWelcome...",
    "body_html": "<h1>About Us</h1>\n<p>Welcome...</p>",
    "checksum": "a1b2c3...",
    "frontmatter": { "title": "About Us", "slug": "about", ... },
    "custom": { "hero_image": "/images/about-hero.jpg" }
  },
  "locale": "en"
}
```

### Change Detection

The importer avoids unnecessary writes by comparing existing translations against incoming data. An update is triggered only when:

- A new locale translation appears (or one is removed)
- The title or summary changes
- The file checksum changes (body was edited)

Unchanged content entries are reported as skipped.

---

## Sync Workflow

Syncing extends the import workflow with support for repeated runs. It detects new, changed, and orphaned content, keeping the CMS in sync with the filesystem.

### Syncing a Directory

```go
result, err := mdSvc.Sync(ctx, ".", interfaces.SyncOptions{
    ImportOptions: interfaces.ImportOptions{
        ContentTypeID: articleTypeID,
        AuthorID:      authorID,
        CreatePages:   true,
        TemplateID:    &templateID,
    },
    DeleteOrphaned: true,
    UpdateExisting: true,
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Created: %d, Updated: %d, Deleted: %d, Skipped: %d\n",
    result.Created, result.Updated, result.Deleted, result.Skipped,
)
```

### SyncOptions Reference

`SyncOptions` embeds `ImportOptions` and adds:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `DeleteOrphaned` | `bool` | `false` | Delete CMS content with no matching markdown file |
| `UpdateExisting` | `bool` | `true` | Update CMS entries when markdown content changes |

### SyncResult Structure

| Field | Type | Description |
|-------|------|-------------|
| `Created` | `int` | Number of new content entries created |
| `Updated` | `int` | Number of existing entries updated |
| `Deleted` | `int` | Number of orphaned entries deleted |
| `Skipped` | `int` | Number of unchanged entries skipped |
| `Errors` | `[]error` | Errors encountered during sync |

### Orphan Deletion

When `DeleteOrphaned` is `true`, the sync compares all markdown slugs against existing CMS content. Any content entry (and its associated page, if `CreatePages` is enabled) that has no matching markdown file is deleted with a hard delete. Use `DryRun: true` to preview which entries would be removed.

---

## Shortcode Integration

When shortcodes are enabled, the markdown module expands shortcode tags before passing content to Goldmark. This allows markdown authors to embed dynamic components.

### Configuration

```go
cfg.Features.Shortcodes = true
cfg.Features.Markdown = true
cfg.Shortcodes.Enabled = true
cfg.Markdown.ProcessShortcodes = true
```

### Per-Request Override

Shortcode processing can be toggled per request without changing global configuration:

```go
html, err := mdSvc.Render(ctx, markdown, interfaces.ParseOptions{
    ProcessShortcodes: true,
    ShortcodeOptions: interfaces.ShortcodeProcessOptions{
        Locale: "en",
    },
})
```

When importing, set `ProcessShortcodes: true` in `ImportOptions`:

```go
result, err := mdSvc.Import(ctx, doc, interfaces.ImportOptions{
    ContentTypeID:     articleTypeID,
    AuthorID:          authorID,
    ProcessShortcodes: true,
})
```

The shortcode service is wired automatically by the DI container when both features are enabled.

---

## Locale Detection

The loader determines a document's locale using the following priority:

1. **Per-request `LocalePatterns`** -- glob patterns provided in `LoadOptions.LocalePatterns`.
2. **Service-level `LocalePatterns`** -- patterns from `cfg.Markdown.LocalePatterns`.
3. **Directory name** -- the first path segment is compared against `cfg.Markdown.Locales`. For example, `en/about.md` matches locale `"en"`.
4. **Default locale** -- falls back to `cfg.Markdown.DefaultLocale`.

### Locale Patterns Example

```go
cfg.Markdown.LocalePatterns = map[string]string{
    "en": "*/en/*",
    "es": "*/es/*",
    "fr": "*/fr/*",
}
```

### Recommended Directory Layout

The simplest layout uses locale codes as top-level directories:

```
content/
  en/
    about.md       (slug: about, locale: en)
    blog/
      hello.md     (slug: hello, locale: en)
  es/
    about.md       (slug: about, locale: es)
    blog/
      hello.md     (slug: hello, locale: es)
```

Files with the same `slug` across locale directories are grouped into a single content entry with multiple translations.

---

## CLI Usage

Three CLI tools wrap the markdown service for command-line workflows and CI/CD pipelines.

### Import Command

Import markdown files into the CMS:

```bash
go run cmd/markdown/import/main.go \
  -content-dir=./content \
  -directory=. \
  -pattern="*.md" \
  -default-locale=en \
  -locales=en,es \
  -content-type=<UUID> \
  -author=<UUID> \
  -template=<UUID> \
  -create-pages \
  -dry-run
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-content-dir` | `content` | Root markdown directory |
| `-directory` | `.` | Subdirectory to import, relative to content root |
| `-pattern` | `*.md` | Glob pattern for file discovery |
| `-default-locale` | `en` | Fallback locale |
| `-locales` | -- | Comma-separated known locales |
| `-translations-enabled` | `true` | Enable i18n |
| `-require-translations` | `true` | Enforce at least one translation |
| `-content-type` | -- | Content type UUID (**required**) |
| `-author` | -- | Author UUID (**required**) |
| `-template` | -- | Template UUID for pages |
| `-create-pages` | `false` | Create page entities alongside content |
| `-dry-run` | `false` | Preview without persisting |

### Sync Command

Keep CMS content in sync with markdown files:

```bash
go run cmd/markdown/sync/main.go \
  -content-dir=./content \
  -directory=. \
  -content-type=<UUID> \
  -author=<UUID> \
  -create-pages \
  -template=<UUID> \
  -delete-orphaned \
  -update-existing \
  -dry-run
```

**Additional flags (beyond import flags):**

| Flag | Default | Description |
|------|---------|-------------|
| `-delete-orphaned` | `false` | Delete CMS content with no matching markdown |
| `-update-existing` | `true` | Update CMS entries when markdown changes |

### Preview Command

Preview a single markdown file without importing:

```bash
go run cmd/markdown/preview/main.go \
  -content-dir=./content \
  -file=en/about.md \
  -render-html=true
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-file` | -- | File to preview, relative to content root (**required**) |
| `-render-html` | `true` | Render HTML or show raw markdown body |
| `-content-dir` | `content` | Root markdown directory |
| `-default-locale` | `en` | Fallback locale |
| `-locales` | -- | Comma-separated known locales |

Output includes the file path, locale, checksum, parsed frontmatter (as JSON), and rendered HTML.

---

## Common Patterns

### Monolingual Site

For a single-language site, keep the directory layout flat and set one locale:

```go
cfg.Markdown.DefaultLocale = "en"
cfg.Markdown.Locales = []string{"en"}
cfg.Markdown.Recursive = true
```

```
content/
  about.md
  blog/
    hello-world.md
    second-post.md
```

All files inherit locale `"en"` from the default.

### Multilingual Site

Use locale directories and declare all locales:

```go
cfg.Markdown.DefaultLocale = "en"
cfg.Markdown.Locales = []string{"en", "es", "fr"}
cfg.Markdown.Recursive = true
```

```
content/
  en/
    about.md
    blog/hello-world.md
  es/
    about.md
    blog/hello-world.md
  fr/
    about.md
```

The importer groups `en/about.md`, `es/about.md`, and `fr/about.md` into one content entry with three translations. Missing translations (e.g., `fr/blog/hello-world.md`) are allowed when `ContentAllowMissingTranslations: true`.

### Dry Run Before Import

Preview what an import or sync would do without writing to the CMS:

```go
result, err := mdSvc.ImportDirectory(ctx, ".", interfaces.ImportOptions{
    ContentTypeID: articleTypeID,
    AuthorID:      authorID,
    DryRun:        true,
})
// result shows what would be created/updated/skipped
```

### CI/CD Pipeline Integration

Use the sync command in CI to keep content in sync on every push:

```bash
go run cmd/markdown/sync/main.go \
  -content-dir=./content \
  -content-type=$CONTENT_TYPE_ID \
  -author=$AUTHOR_ID \
  -create-pages \
  -template=$TEMPLATE_ID \
  -update-existing \
  -delete-orphaned
```

Pair with `-dry-run` in pull request checks to validate content without deploying.

### Custom Frontmatter Fields

Extra YAML fields are captured in `FrontMatter.Custom` and stored in the content translation's `Fields["markdown"]["custom"]`:

```yaml
---
title: Product Launch
slug: product-launch
status: published
featured: true
priority: 1
category: announcements
---
```

Access custom fields programmatically:

```go
doc, _ := mdSvc.Load(ctx, "en/product-launch.md", interfaces.LoadOptions{})

featured, _ := doc.FrontMatter.Custom["featured"].(bool)
priority, _ := doc.FrontMatter.Custom["priority"].(int)
category, _ := doc.FrontMatter.Custom["category"].(string)
```

---

## Next Steps

- **GUIDE_GETTING_STARTED.md** -- minimal setup and first content creation
- **GUIDE_CONTENT.md** -- content types, entries, versioning, and scheduling
- **GUIDE_PAGES.md** -- page hierarchy, routing, and templates
- **GUIDE_SHORTCODES.md** -- registering and processing shortcodes in markdown
- **GUIDE_I18N.md** -- translation workflows and locale management
- **GUIDE_STATIC_GENERATION.md** -- building static sites from CMS content
- `cmd/example/main.go` -- comprehensive usage example
