# Getting Started with go-cms

This guide walks you through setting up `go-cms` and creating your first content type, content entry, and page. By the end you will have a working CMS module running entirely in memory.

## Overview

`go-cms` is a modular, headless CMS library for Go. It is not a standalone service: there are no HTTP handlers, no built-in admin UI, and no database requirement out of the box. You embed it in your own application, wire the services you need, and integrate them into whatever router or framework you already use.

Key characteristics:

- **Library, not a service** -- you import packages and call methods; no server starts automatically.
- **Storage-agnostic** -- without a database, repositories run in memory. Pass `di.WithBunDB(db)` to switch to SQL-backed storage.
- **Locale-first** -- every entity supports translations. The default config requires at least one translation per content entry.
- **UUID primary keys** everywhere, timestamps always in UTC.

## Installation

```bash
go get github.com/goliatone/go-cms
```

Requires Go 1.24.10 or later.

## Minimal Setup

The smallest working setup needs two things: a `Config` and a `Module`. The module wraps a dependency injection container that wires all services.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	// 1. Start with default configuration
	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Locales = []string{"en"}

	// 2. Create the CMS module (in-memory repositories, no DB needed)
	module, err := cms.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Access services via the module facade
	contentSvc := module.Content()
	pageSvc := module.Pages()

	fmt.Println("CMS module ready")
	_ = ctx
	_ = contentSvc
	_ = pageSvc
}
```

`cms.DefaultConfig()` returns a config with sensible defaults:

- `DefaultLocale`: `"en"`
- `I18N.Enabled`: `true`, `RequireTranslations`: `true`
- `Cache.Enabled`: `true`, `Cache.DefaultTTL`: 1 minute
- All feature flags (`Features.Widgets`, `Features.Versioning`, etc.) default to `false`

When no `di.WithBunDB()` option is provided, the container uses in-memory repositories. This is useful for prototyping, testing, and static site generation.

## Creating a Content Type, Content Entry, and Page

This example creates an "Article" content type, publishes one article with an English translation, and wraps it in a page.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Locales = []string{"en"}

	module, err := cms.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	contentSvc := module.Content()
	pageSvc := module.Pages()
	authorID := uuid.New()

	// --- Step 1: Create a content type ---
	articleType, err := contentSvc.CreateContentType(ctx, content.CreateContentTypeRequest{
		Name: "Article",
		Slug: "article",
		Schema: map[string]any{
			"fields": []map[string]any{
				{"name": "title", "type": "string", "required": true},
				{"name": "body", "type": "text", "required": true},
			},
		},
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		log.Fatalf("create content type: %v", err)
	}

	// --- Step 2: Create a content entry with a translation ---
	article, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: articleType.ID,
		Slug:          "hello-world",
		Status:        "published",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Hello World",
				Content: map[string]any{"body": "Welcome to go-cms."},
			},
		},
	})
	if err != nil {
		log.Fatalf("create content: %v", err)
	}

	// --- Step 3: Create a page that references the content ---
	templateID := uuid.New() // placeholder; real templates come from the themes service
	page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  article.ID,
		TemplateID: templateID,
		Slug:       "hello-world",
		Status:     "published",
		CreatedBy:  authorID,
		UpdatedBy:  authorID,
		Translations: []pages.PageTranslationInput{
			{
				Locale: "en",
				Title:  "Hello World",
				Path:   "/hello-world",
			},
		},
	})
	if err != nil {
		log.Fatalf("create page: %v", err)
	}

	fmt.Printf("Content type: %s (id=%s)\n", articleType.Name, articleType.ID)
	fmt.Printf("Content:      %s (id=%s)\n", article.Slug, article.ID)
	fmt.Printf("Page:         %s (id=%s)\n", page.Slug, page.ID)
}
```

Running this prints the created IDs, confirming everything is wired correctly.

### What just happened

1. **Content type** -- defines the schema for a category of content. Every content entry references a content type by ID.
2. **Content entry** -- an individual piece of content. Translations are attached inline via `ContentTranslationInput`. Because `RequireTranslations` is `true` by default, at least one translation must be provided.
3. **Page** -- wraps a content entry with routing information (slug, path) and a template reference. Pages form a hierarchy through `ParentID` and are the primary unit the static generator and URL resolver work with.

## Wiring with BunDB for Persistent Storage

To switch from in-memory to SQL-backed storage, pass `di.WithBunDB()` when creating the module:

```go
import (
	"database/sql"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	sqldb, err := sql.Open("sqlite3", "file:cms.db?cache=shared&_fk=1")
	if err != nil {
		log.Fatal(err)
	}

	db := bun.NewDB(sqldb, sqlitedialect.New())

	cfg := cms.DefaultConfig()
	module, err := cms.New(cfg, di.WithBunDB(db))
	if err != nil {
		log.Fatal(err)
	}

	// Run migrations
	migrations := cms.GetMigrationsFS()
	// Register and run migrations with your bun migrator...

	contentSvc := module.Content()
	// All operations now persist to SQLite
	_ = contentSvc
}
```

Both PostgreSQL (`pgdialect`) and SQLite (`sqlitedialect`) are supported. The repository proxies in the DI container handle the switch transparently -- service code does not change.

## Feature Flags

Optional subsystems are controlled via `cfg.Features`. All default to `false`:

```go
cfg.Features.Widgets = true       // Widget subsystem with areas and visibility rules
cfg.Features.Themes = true        // Theme management and template registration
cfg.Features.Versioning = true    // Draft/publish/restore for content, pages, blocks
cfg.Features.Scheduling = true    // Timed publish/unpublish (requires Versioning)
cfg.Features.MediaLibrary = true  // Media attachment and binding resolution
cfg.Features.Markdown = true      // Markdown import/sync with frontmatter
cfg.Features.Shortcodes = true    // Shortcode processing in markdown content
cfg.Features.Activity = true      // Activity event emission for audit logging
cfg.Features.Environments = true  // Environment-scoped content and configuration
cfg.Features.Logger = true        // Structured logging with module-level loggers
cfg.Features.AdvancedCache = true // Repository-level caching (requires Cache.Enabled)
```

When a feature is disabled, the corresponding service returns a no-op implementation. You do not need to nil-check services -- they are always safe to call, they just do nothing when disabled.

## Environments

Enable environments to scope content types, content entries, pages, menus, and block definitions. When enabled, all lookups accept an optional environment key; if omitted, the default environment is used.

```go
cfg.Features.Environments = true
cfg.Environments = cms.EnvironmentsConfig{
    DefaultKey:       "dev",
    RequireExplicit:  false, // if true, writes must include EnvironmentKey
    RequireActive:    false, // if true, inactive envs are rejected
    EnforceDefault:   false, // if true, default env cannot be unset/deleted
    PermissionScoped: false, // if true, permissions include @environment
    PermissionStrategy: "env_first",
    Definitions: []cms.EnvironmentConfig{
        {Key: "dev", Name: "Development", Default: true},
        {Key: "staging", Name: "Staging"},
        {Key: "prod", Name: "Production"},
    },
}
```

Defaults and fallbacks:

- If no definitions are provided, a single `default` environment is created and used.
- When `RequireExplicit=false`, create/update requests may omit `EnvironmentKey`.
- Environment scoping is opt-in; leaving `Features.Environments=false` preserves legacy behaviour.

## Accessing Services

The `cms.Module` facade exposes all services:

```go
module.Content()        // Content and content types
module.Pages()          // Page hierarchy and routing
module.Blocks()         // Block definitions and instances
module.Widgets()        // Widget definitions, instances, and areas
module.Menus()          // Menu management and navigation resolution
module.Themes()         // Theme and template management
module.Media()          // Media attachment and binding
module.Generator()      // Static site generation
module.Markdown()       // Markdown import/sync
module.Shortcodes()     // Shortcode processing
module.Scheduler()      // Publish scheduling
module.WorkflowEngine() // Content lifecycle state machine
module.StorageAdmin()   // Runtime storage profile switching
module.TranslationAdmin() // Translation settings management
module.BlocksAdmin()    // Block admin operations
```

For advanced integrations, `module.Container()` returns the underlying DI container.

## Multi-Locale Content

To support multiple locales, list them in the config and supply translations for each:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es", "fr"}

// When creating content, provide translations for each locale:
article, err := contentSvc.Create(ctx, content.CreateContentRequest{
	ContentTypeID: typeID,
	Slug:          "about-us",
	Status:        "published",
	CreatedBy:     authorID,
	UpdatedBy:     authorID,
	Translations: []content.ContentTranslationInput{
		{Locale: "en", Title: "About Us", Content: map[string]any{"body": "..."}},
		{Locale: "es", Title: "Sobre nosotros", Content: map[string]any{"body": "..."}},
		{Locale: "fr", Title: "A propos", Content: map[string]any{"body": "..."}},
	},
})
```

By default, at least one translation is required. To allow creating content without translations (e.g. during a draft workflow), set `AllowMissingTranslations: true` on the request, or globally set `cfg.I18N.RequireTranslations = false`.

## Next Steps

- **GUIDE_CONTENT.md** -- deep dive into content types, content entries, versioning, and scheduling
- **GUIDE_PAGES.md** -- page hierarchy, parent-child relationships, path resolution
- **GUIDE_BLOCKS.md** -- reusable content fragments with definitions and instances
- **GUIDE_CONFIGURATION.md** -- full config reference and DI container wiring
- **GUIDE_STATIC_GENERATION.md** -- building static sites with the generator service
- `cmd/example/main.go` -- comprehensive example exercising content, pages, blocks, widgets, menus, and themes
- `site/` -- the COLABS site module, a full integration example with static generation and Playwright tests
