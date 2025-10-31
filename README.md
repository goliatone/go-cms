# go-cms

go-cms is a modular, headless CMS toolkit for Go. It bundles reusable services for content, pages, blocks, widgets, menus, localization, and static generation so you can embed editorial workflows in any Go application.

## Table of Contents
- [Why go-cms](#why-go-cms)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [Static Site Generation](#static-site-generation)
- [Markdown Import & Sync](#markdown-import--sync)
- [Configuration](#configuration)
- [Architecture & Extensibility](#architecture--extensibility)
- [CLI Reference](#cli-reference)
- [Development](#development)
- [Requirements & Dependencies](#requirements--dependencies)
- [Further Reading](#further-reading)

## Why go-cms
- **Composable services** &mdash; opt into content, page, widget, or menu modules independently.
- **Storage flexibility** &mdash; switch between in-memory or Bun-backed SQL repositories without touching application code.
- **Localization first** &mdash; every entity carries locale-aware translations and fallbacks.
- **Authoring experience** &mdash; versioning, scheduling, visibility rules, and reusable blocks keep editors productive.
- **Static publishing** &mdash; generate locale-aware static bundles or wire services into a dynamic site.
- **Observability hooks** &mdash; structured logging interfaces and command callbacks integrate with existing telemetry.

## Installation

```bash
go get github.com/goliatone/go-cms
```

## Quick Start

```go
package main

import (
	"context"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Locales = []string{"en", "es"}

	container := di.NewContainer(cfg)
	contentSvc := container.ContentService()
	pageSvc := container.PageService()

	authorID := uuid.New()
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
		panic(err)
	}

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
				Content: map[string]any{"body": "Content goes here"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	_, err = pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  article.ID,
		Slug:       "hello-world",
		Status:     "published",
		CreatedBy:  authorID,
		UpdatedBy:  authorID,
		Translations: []pages.PageTranslationInput{
			{Locale: "en", Title: "Hello World", Path: "/hello-world"},
		},
	})
	if err != nil {
		panic(err)
	}
}
```

See `cmd/example/main.go` for a more complete walkthrough.

## Core Concepts

### Content Types & Content
Define schemas that describe editorial data. Content records reference a type and store localized payloads.

```go
contentType, _ := contentSvc.CreateContentType(ctx, content.CreateContentTypeRequest{
	Name: "Article",
	Slug: "article",
	Schema: map[string]any{
		"fields": []map[string]any{
			{"name": "title", "type": "string", "required": true},
			{"name": "body", "type": "text", "required": true},
			{"name": "tags", "type": "array"},
		},
	},
	CreatedBy: authorID,
	UpdatedBy: authorID,
})
```

### Pages
Pages form the site map. They link to content, choose templates, and emit locale-aware routes with SEO metadata.

```go
page, _ := pageSvc.Create(ctx, pages.CreatePageRequest{
	ContentID:  article.ID,
	TemplateID: articleTemplateID,
	Slug:       "getting-started",
	Status:     "published",
	ParentID:   &docsPageID,
	CreatedBy:  authorID,
	UpdatedBy:  authorID,
	Translations: []pages.PageTranslationInput{
		{
			Locale: "en",
			Title:  "Getting Started",
			Path:   "/docs/getting-started",
			MetaDescription: "Learn how to get started",
		},
	},
})
```

### Blocks
Blocks are reusable fragments that can be attached to pages or content regions with translations.

```go
definition, _ := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
	Name: "call_to_action",
	Schema: map[string]any{
		"fields": []string{"headline", "description", "button_text", "button_url"},
	},
})

instance, _ := blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
	DefinitionID: definition.ID,
	PageID:       &page.ID,
	Region:       "main",
	Position:     1,
	CreatedBy:    authorID,
	UpdatedBy:    authorID,
})
```

### Widgets
Widgets add behavioural components with scheduling, visibility rules, and per-area placement.

```go
widgetSvc.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
	Code:  "sidebar.primary",
	Name:  "Primary Sidebar",
	Scope: widgets.AreaScopeGlobal,
})

widget, _ := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
	DefinitionID: newsletterWidgetDefID,
	Configuration: map[string]any{
		"headline": "Stay Updated",
	},
	VisibilityRules: map[string]any{
		"audience": []string{"guest"},
	},
	CreatedBy: authorID,
	UpdatedBy: authorID,
})

widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
	AreaCode:   "sidebar.primary",
	InstanceID: widget.ID,
})
```

Enable built-in definitions and version retention through configuration:

```go
cfg := cms.DefaultConfig()
cfg.Features.Widgets = true
cfg.Widgets.Definitions = []cms.WidgetDefinitionConfig{
	{
		Name: "promo_banner",
		Schema: map[string]any{
			"fields": []any{
				map[string]any{"name": "headline"},
				map[string]any{"name": "cta_text"},
			},
		},
		Defaults: map[string]any{"cta_text": "Sign up"},
		Category: "marketing",
	},
}
cfg.Features.Versioning = true
cfg.Retention = cms.RetentionConfig{Content: 5, Pages: 3, Blocks: 2}
```

### Menus
Menus generate navigation trees with locale-aware labels and URL resolution.

```go
menu, _ := menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
	Code:      "primary",
	CreatedBy: authorID,
	UpdatedBy: authorID,
})

menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
	MenuID:   menu.ID,
	Position: 0,
	Target: map[string]any{
		"type": "page",
		"slug": "about",
	},
	Translations: []menus.MenuItemTranslationInput{
		{Locale: "en", Label: "About Us"},
		{Locale: "es", Label: "Acerca de"},
	},
})
```

### Localization Helpers
Locales, translations, and fallbacks are available across services. `cfg.I18N.Locales` drives validation, and helpers such as `generator.TemplateContext.Helpers.WithBaseURL` simplify template routing.

## Static Site Generation

The generator composes CMS services to emit pre-rendered HTML, assets, and sitemaps. It honours locale routing, draft visibility, and storage abstractions so you can stream output to disk, S3-compatible buckets, or custom storage backends.

Programmatic usage requires `github.com/goliatone/go-cms/internal/generator`.

```go
cfg := cms.DefaultConfig()
cfg.Generator.Enabled = true
cfg.Generator.OutputDir = "./dist"
cfg.Generator.BaseURL = "https://example.com"
cfg.Generator.Incremental = true
cfg.Generator.CopyAssets = true

module, err := cms.New(cfg)
if err != nil {
	log.Fatal(err)
}

result, err := module.Generator().Build(context.Background(), generator.BuildOptions{})
if err != nil {
	log.Fatal(err)
}

log.Printf("built %d pages across %d locales", result.PagesBuilt, len(result.Locales))
```

Templates receive `generator.TemplateContext` with resolved dependencies:

```gotemplate
{{ define "page" }}
<html lang="{{ .Page.Locale.Code }}">
  <head>
    <title>{{ .Page.Translation.Title }}</title>
  </head>
  <body>
    {{ range .Page.Blocks }}{{ template .TemplatePath . }}{{ end }}
    {{ range $code, $menu := .Page.Menus }}
      {{ template "menu" (dict "code" $code "nodes" $menu) }}
    {{ end }}
  </body>
</html>
{{ end }}
```

Troubleshooting tips:
- `static: static command handlers not configured` &mdash; ensure `bootstrap.Options.EnableCommands` is true and the generator feature is enabled.
- `static: static sitemap handler not configured` &mdash; enable `Config.Generator.GenerateSitemap` or provide `--output` / `--base-url`.
- Missing telemetry &mdash; attach a `ResultCallback` that logs or forwards metrics.
- Custom storage integration &mdash; set `bootstrap.Options.Storage` to an implementation of `interfaces.StorageProvider`.

## Markdown Import & Sync

Opt into file-based content ingestion without committing to a full static workflow.

```go
cfg := cms.DefaultConfig()
cfg.Features.Markdown = true
cfg.Markdown = cms.MarkdownConfig{
	Enabled:        true,
	ContentDir:     "./content",
	DefaultLocale:  "en",
	Locales:        []string{"en", "es"},
	LocalePatterns: map[string]string{"es": "es/**/*.md"},
	Pattern:        "**/*.md",
	Recursive:      true,
}

module, err := cms.New(cfg)
if err != nil {
	log.Fatal(err)
}

mdSvc := module.Markdown()
```

CLI helpers live under `cmd/markdown`:

```bash
# Import a single document without touching pages
go run ./cmd/markdown/import \
  --path ./content/en/about.md \
  --content-type $CONTENT_TYPE_ID \
  --author $AUTHOR_ID

# Sync a directory, updating content and optionally creating pages
go run ./cmd/markdown/sync \
  --dir ./content \
  --content-type $CONTENT_TYPE_ID \
  --author $AUTHOR_ID \
  --create-pages \
  --template $TEMPLATE_ID \
  --update-existing
```

`examples/web/` shows how to wire the markdown service into startup and cron flows. The default adapter currently performs a delete-and-recreate for page updates; swap in an alternative once granular update hooks land in `pages.Service`.

## Configuration

Most features are toggled on the shared configuration struct.

```go
cfg := cms.DefaultConfig()

cfg.DefaultLocale = "en"
cfg.Content.PageHierarchy = true

cfg.I18N.Enabled = true
cfg.I18N.Locales = []string{"en", "es", "fr"}

cfg.Storage.Provider = "bun" // or "memory"

cfg.Cache.Enabled = true
cfg.Cache.DefaultTTL = time.Minute * 5

cfg.Features.Widgets = true

cfg.Navigation.RouteConfig = &urlkit.Config{...}
cfg.Navigation.URLKit.DefaultGroup = "frontend"
cfg.Navigation.URLKit.LocaleGroups = map[string]string{
	"es": "frontend.es",
}
```

Additional guides:
- Observability & logging: `docs/LOGGING_GUIDE.md`
- Static bootstrapper: `cmd/static/internal/bootstrap`
- DI wiring options: `internal/di/options.go`

## Architecture & Extensibility

```
internal/
├── content/     # Content entities and content types
├── pages/       # Page hierarchy and routing
├── blocks/      # Reusable content fragments
├── widgets/     # Dynamic behavioral components
├── menus/       # Navigation structures
├── i18n/        # Internationalization helpers
├── adapters/    # Integrations (storage, rendering)
└── di/          # Dependency injection container

pkg/
├── interfaces/  # Public abstractions
└── testsupport/ # Shared fixtures and helpers
```

- **Repository pattern** &mdash; every module ships in-memory and Bun-backed repositories; the container picks based on `cfg.Storage.Provider`.
- **Dependency injection** &mdash; `di.NewContainer` wires services. Override dependencies with functional options:

```go
container := di.NewContainer(cfg,
	di.WithBunDB(db),
	di.WithCache(cache, serializer),
	di.WithPageService(customPageSvc),
)
```

- **Commands** &mdash; `cmd/static` and `cmd/markdown` expose features through go-command handlers; register additional commands through the same container.

## CLI Reference

```bash
# Static generator commands
go run ./cmd/static build   --output ./dist --locale en,es
go run ./cmd/static diff    --page <page-id> --locale en
go run ./cmd/static build   --assets
go run ./cmd/static sitemap

# Markdown import/sync
go run ./cmd/markdown import ...
go run ./cmd/markdown sync ...

# Example application
go run ./cmd/example
```

## Development

```bash
# Unit tests
go test ./...

# Package-specific tests
go test ./internal/content/...
go test ./internal/pages/...
go test ./internal/blocks/...
go test ./internal/widgets/...
go test ./internal/menus/...
go test ./internal/generator ./cms

# Coverage
./taskfile dev:cover

# Integration tests (require database)
go test -v ./internal/pages/... -run Integration
```

## Requirements & Dependencies

- Go 1.24+
- Optional SQL backend supported by uptrace/bun (PostgreSQL, MySQL, SQLite)

Key modules:
- [github.com/uptrace/bun](https://github.com/uptrace/bun)
- [github.com/goliatone/go-urlkit](https://github.com/goliatone/go-urlkit)
- [github.com/goliatone/go-repository-cache](https://github.com/goliatone/go-repository-cache)
- [github.com/google/uuid](https://github.com/google/uuid)

## Further Reading

- Examples: `cmd/example/main.go`, `examples/web/`
- Logging & observability: `docs/LOGGING_GUIDE.md`
- Feature walkthroughs: `docs/FEAT_STATIC.md`, `docs/FEAT_MARKDOWN.md`
- Task-driven design: `docs/CMS_TDD.md`, `docs/CMD_TDD.md`

## License

Licensed under the terms of [LICENSE](LICENSE).
