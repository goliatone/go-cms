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

## Features

- **Composable services**: opt into content, page, widget, or menu modules independently.
- **Storage flexibility**: switch between "in memory" or Bun backed SQL repositories without touching application code.
- **Localization first**: locale aware translations, fallbacks, and translation grouping for content/pages.
- **Authoring experience**: versioning, scheduling, visibility rules, and reusable blocks keep editors productive.
- **Menu locations**: bind menus to theme-defined locations and resolve navigation by location.
- **Static publishing**: generate locale aware static bundles or wire services into a dynamic site.
- **Observability hooks**: structured logging inside commands; optional adapter wiring for telemetry callbacks.

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

Pages form the site map. They link to content, choose templates, and emit locale aware routes with SEO metadata.

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

Widgets add behavioral components with scheduling, visibility rules, and per-area placement.

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

Enable builtin definitions and version retention through configuration:

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

Menus generate navigation trees with locale aware labels, translation keys, and UI hints for groups/separators/collapsible items.

`cms.DefaultConfig()` enables order-independent menu upserts (`cfg.Menus.AllowOutOfOrderUpserts=true`), so modules can insert parents/children in any order and persist collapsible intent before children exist. Set it to `false` if you want strict validation (missing parents and `Collapsible` without children will error).

Menus can also be bound to a location string (often a theme menu location code) so templates can resolve navigation without hardcoding menu codes. Theme manifests declare locations via `menu_locations` (or `ThemeConfig.MenuLocations`).

```go
menuSvc := module.Menus()

if _, err := menuSvc.UpsertMenuWithLocation(ctx, "primary", "site.primary", nil, authorID); err != nil {
	log.Fatal(err)
}

pos0 := 0
pos1 := 1
pos2 := 2

// Menus are addressed by a stable code (e.g. "primary").
// Items are addressed by dot-paths that include the menu code prefix (e.g. "primary.content.pages").
if err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
	Menus:    menuSvc,
	MenuCode: "primary",
	Locale:   "en",
	Actor:    authorID,
	Items: []cms.SeedMenuItem{
		{
			Path:     "primary.home",
			Position: &pos0,
			Type:     "item",
			Target:   map[string]any{"type": "url", "url": "/"},
			Translations: []cms.MenuItemTranslationInput{
				{Locale: "en", Label: "Home"},
				{Locale: "es", Label: "Inicio"},
			},
		},
		{
			Path:     "primary.content",
			Position: &pos1,
			Type:     "group",
			Translations: []cms.MenuItemTranslationInput{
				{Locale: "en", GroupTitleKey: "menu.group.content"},
			},
		},
		{
			Path:     "primary.content.pages",
			Position: &pos0,
			Type:     "item",
			Target:   map[string]any{"type": "url", "url": "/pages"},
			Translations: []cms.MenuItemTranslationInput{
				{Locale: "en", LabelKey: "menu.pages"},
			},
		},
		{
			Path:     "primary.separator",
			Position: &pos2,
			Type:     "separator",
		},
	},
}); err != nil {
	log.Fatal(err)
}

navigation, _ := menuSvc.ResolveNavigationByLocation(ctx, "site.primary", "en")
_ = navigation
```

Menu item types:

- `item` (default): clickable row, may have children and optional `Collapsible/Collapsed` hints.
- `group`: non-clickable header; no target/icon/badge; children only; use `GroupTitle`/`GroupTitleKey` for display. Groups with no children are still returned when they contain presentation data (label/title/metadata/etc); "empty" groups without meaningful data are omitted.
- `separator`: visual divider; no target/children/icon/badge/translations.

Translation precedence: `LabelKey` (or `GroupTitleKey`) → translated value → `Label`/`GroupTitle` fallback. URL resolution only runs for `item` types.

Migration note: menu features rely on migrations:

- `data/sql/migrations/20250209000000_menu_navigation_enhancements.up.sql` (menu item/translation fields: type, collapsible flags, metadata, styling, translation keys, group titles)
- `data/sql/migrations/20250301000000_menu_item_canonical_dedupe.up.sql` (canonical key + uniqueness)
- `data/sql/migrations/20251213000000_menu_item_external_parent_refs.up.sql` (external_code + parent_ref for out-of-order upserts)
- `data/sql/migrations/20260301000001_menu_locations.up.sql` (menus.location + index)

When using BunDB, these migrations are embedded and registered via `cms.GetMigrationsFS()` (see "Database Migrations").

### Localization Helpers

Locales, translations, and fallbacks are available across services. `cfg.I18N.Locales` drives validation, and helpers such as `generator.TemplateContext.Helpers.WithBaseURL` simplify template routing. Use `cfg.I18N.RequireTranslations` (defaults to `true`) to keep the legacy "at least one translation" guard, or flip it to `false` for staged rollouts; pair it with `cfg.I18N.DefaultLocaleRequired` when you need to relax the fallback locale constraint. Both flags are ignored when `cfg.I18N.Enabled` is `false`. Every create/update DTO exposes `AllowMissingTranslations` so workflow transitions or importers can bypass enforcement for a single operation while global defaults remain strict.

Translation grouping: content/page translations store `TranslationGroupID` (backed by `translation_group_id` in SQL). The services default it to the owning content/page ID and preserve it across updates so export pipelines or translation workflows can treat locales as a single group.

Migration note: `data/sql/migrations/20260301000000_translation_grouping.up.sql` (content/page translation group columns + indexes).

## Static Site Generation

The generator composes CMS services to emit prerendered HTML, assets, and sitemaps. It honors locale routing, draft visibility, and storage abstractions so you can stream output to disk, S3 compatible buckets, or custom storage backends.

Programmatic usage: import `github.com/goliatone/go-cms/pkg/generator` (the CLI is a thin wrapper).

```go
package main

import (
	"context"
	"log"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/pkg/generator"
)

func main() {
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

	gen := generator.NewService(
		generator.Config{
			OutputDir:       cfg.Generator.OutputDir,
			BaseURL:         cfg.Generator.BaseURL,
			Incremental:     cfg.Generator.Incremental,
			CopyAssets:      cfg.Generator.CopyAssets,
			GenerateSitemap: cfg.Generator.GenerateSitemap,
			DefaultLocale:   cfg.I18N.DefaultLocale,
			Locales:         cfg.I18N.Locales,
		},
		generator.Dependencies{
			Pages:      module.Pages(),
			Content:    module.Content(),
			Blocks:     module.Blocks(),
			Widgets:    module.Widgets(),
			Menus:      module.Menus(),
			Themes:     module.Themes(),
			I18N:       module.I18N(),
			Renderer:   module.Templates(),
			Storage:    module.Storage(),
			Locales:    module.I18N(),
			Assets:     generator.NoOpAssetResolver{}, // inject theme aware resolver in production
			Logger:     module.Logger(),
			Shortcodes: module.Shortcodes(),
		},
	)

	result, err := gen.Build(context.Background(), generator.BuildOptions{})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("built %d pages across %d locales", result.PagesBuilt, len(result.Locales))
}
```

Contracts:

- `generator.Service` exposing `Build`, `BuildPage`, `BuildAssets`, `BuildSitemap`, and `Clean`.
- `generator.Config`/`BuildOptions`/`BuildResult`/`BuildMetrics` for behavior toggles and reporting.
- `generator.Dependencies` to inject CMS services, renderer, storage, logger, optional hooks, and asset resolver (`AssetResolver` or `NoOpAssetResolver`).

Templates receive `generator.TemplateContext` with resolved dependencies:

```html
{{ define "page" }}
<html lang="{{ .Page.Locale.Code }}">
  <head>
    <title>{{ .Page.Translation.Title }}</title>
    <link rel="stylesheet" href="{{ .Helpers.WithBaseURL (.Theme.AssetURL
    "style") }}">
    <style>
      :root { {{- range $k, $v := .Theme.CSSVars }}{{ $k }}: {{ $v }};{{ end }} }
    </style>
  </head>
  <body>
    {{ range .Page.Blocks }}{{ template .TemplatePath . }}{{ end }} {{ range
    $code, $menu := .Page.Menus }} {{ template "menu" (dict "code" $code "nodes"
    $menu) }} {{ end }}
  </body>
</html>
{{ end }}
```

The `Theme` block on the context comes from [`go-theme`](https://github.com/goliatone/go-theme): configure `cfg.Themes.DefaultTheme`/`DefaultVariant`, ship a `theme.json` alongside your templates/assets, and call helpers such as `.Theme.AssetURL`, `.Theme.Partials`, and `.Theme.CSSVars` (pair them with `.Helpers.WithBaseURL` to honour your site prefix).

Troubleshooting tips:

- `static: static command handlers not configured`: ensure the generator feature is enabled and that the static command constructors receive the generator service (the provided CLI already injects it); use the adapter submodule only when you need registry/dispatcher/cron wiring.
- `static: static sitemap handler not configured`: enable `Config.Generator.GenerateSitemap` or provide `--output` / `--base-url`.
- Missing telemetry: attach a `ResultCallback` that logs or forwards metrics.
- Commands timing out or missing log fields: pass a deadline in the context you supply to `Execute` or use the per command timeout options (for example, `staticcmd.BuildSiteWithTimeout`); inject a logger provider with `di.WithLoggerProvider` so commands include `operation` and domain identifiers in logs.
- Custom storage integration: set `bootstrap.Options.Storage` to an implementation of `interfaces.StorageProvider`.

## Markdown Import & Sync

Opt into file based content ingestion without committing to a full static workflow.

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

cfg.Features.Shortcodes = true
cfg.Shortcodes.Enabled = true
cfg.Shortcodes.Cache.Enabled = true
cfg.Shortcodes.Cache.Provider = "shortcodes" // resolve via di.WithShortcodeCacheProvider
cfg.Markdown.ProcessShortcodes = true

```

Use `di.WithShortcodeCacheProvider` to register named cache implementations (Redis, in-memory) for shortcodes and `di.WithShortcodeMetrics` to feed render telemetry into your monitoring stack.

### Activity Hooks

Enable activity emission with `cfg.Features.Activity` and `cfg.Activity.Enabled`, set `cfg.Activity.Channel` to tag events. Inject hooks via `di.WithActivityHooks` or pass a go-users sink with `di.WithActivitySink` (internally adapted by `pkg/activity/usersink.Hook`). Activity events fan out to all hooks and carry verb, actor IDs, object type/ID, channel, and module specific metadata (slug, status, locale, path, menu code). When no hooks are provided, emissions noop. In tests, pair `activity.CaptureHook` with `activity.NewEmitter` to assert events without persisting them.

## Commands & Adapters

- Core commands are plain structs with direct constructors (for example, `staticcmd.NewBuildSiteHandler`, `markdowncmd.NewSyncDirectoryHandler`) that satisfy `command.CLICommand`/`command.CronCommand` when exposed via CLI or cron. CLIs in this repo wire those constructors directly; there is no collector or registry inside the core module.
- Cross cutting concerns live on the structs: each command applies a default timeout (`commands.WithCommandTimeout` with `commands.DefaultCommandTimeout`) and expects a logger from DI. Override the timeout with options such as `staticcmd.BuildSiteWithTimeout` or pass a logger provider via `di.WithLoggerProvider` so command logs include `operation` and domain identifiers.
- To layer telemetry or retries, derive a context with your own deadline, invoke `Execute`, and forward the returned error to your monitoring hooks.
- Legacy registry/dispatcher/cron wiring lives in the optional adapter submodule. Install it with `go get github.com/goliatone/go-cms/commands`, then call `commands.RegisterContainerCommands(container, commands.RegistrationOptions{Dispatcher: ..., Cron: ...})` to rebuild the old flow when migrating hosts.

```go
module, _ := cms.New(cfg)

result, err := commands.RegisterContainerCommands(module.Container(), commands.RegistrationOptions{
  Registry:       registry,       // optional
  Dispatcher:     dispatcher,     // optional
  CronRegistrar:  cronRegistrar,  // optional
  LoggerProvider: loggerProvider, // optional
})
_ = result // keep result.Subscriptions for shutdown
```


### Managing Storage Profiles at Runtime

Manage storage profiles at runtime through the storage admin service; wire it into your own router or command stack without importing `internal/` packages:

```go
module, err := cms.New(cfg)
if err != nil {
	log.Fatal(err)
}

storageAdmin := module.StorageAdmin()

profiles, err := storageAdmin.ListProfiles(ctx)
if err != nil {
	log.Fatal(err)
}

preview, err := storageAdmin.PreviewProfile(ctx, storage.Profile{
	Name:     "rotated",
	Provider: "bun",
	Config: storage.Config{
		Name:   "rotated",
		Driver: "sqlite3",
		DSN:    "file:/var/lib/cms/rotated.sqlite?_fk=1",
	},
})
if err != nil {
	log.Fatalf("preview failed: %v", err)
}

log.Printf("provider supports reload=%v", preview.Capabilities.SupportsReload)

err = storageAdmin.ApplyConfig(ctx, cms.StorageConfig{
	Profiles: []storage.Profile{
		{
			Name:        "rotated",
			Provider:    "bun",
			Description: "Primary writer",
			Default:     true,
			Config: storage.Config{
				Name:   "rotated",
				Driver: "sqlite3",
				DSN:    "file:/var/lib/cms/rotated.sqlite?_fk=1",
			},
		},
	},
	Aliases: map[string]string{"content": "rotated"},
})
if err != nil {
	log.Fatalf("apply config failed: %v", err)
}
```

- No routes or controllers ship with the module mount these helpers in your own `go-router`, `chi`, gRPC, or command stacks next to the rest of your admin UI.
- `Schemas()` returns JSON schemas for profile/config payloads so UIs can validate forms client side.
- Audit events (`storage_profile_created/updated/deleted`) and container logs (`storage.profile_activated`, `storage.profile_activate_failed`) provide the telemetry required for the dashboards referenced in `TODO_TSK.md`.

### Workflow Engine Configuration

The workflow subsystem externalises lifecycle decisions so hosts can add review, translation, or bespoke approval steps without touching page services. Enable the default engine or register your own through configuration:

```go
cfg.Workflow.Enabled = true            // enable lifecycle orchestration (default)
cfg.Workflow.Provider = "simple"       // use the built-in engine
cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
	{
		Entity: "page",
		States: []cms.WorkflowStateConfig{
			{Name: "draft", Initial: true},
			{Name: "review"},
			{Name: "translated"},
			{Name: "published", Terminal: true},
		},
		Transitions: []cms.WorkflowTransitionConfig{
			{Name: "submit_review", From: "draft", To: "review"},
			{Name: "translate", From: "review", To: "translated"},
			{Name: "publish", From: "translated", To: "published"},
		},
	},
}
```

When `cfg.Workflow.Provider` is set to `custom`, provide an `interfaces.WorkflowEngine` via `di.WithWorkflowEngine` during module construction.
To pull definitions from storage, implement `interfaces.WorkflowDefinitionStore` and pass it to `di.WithWorkflowDefinitionStore`. Store provided definitions override configuration entries for matching entity types.

```go
engine := myengine.New(customDeps...)
definitions := mystore.NewWorkflowDefinitionStore(db)

container := di.NewContainer(cfg,
	di.WithWorkflowEngine(engine),
	di.WithWorkflowDefinitionStore(definitions),
)

pageSvc := container.PageService()
```

For go-command/flow-powered state machines, wrap the external engine with the CMS adapter in `internal/workflow/adapter` to preserve DTOs, guard hooks, and action-generated events/notifications:

```go
import (
	cmsadapter "github.com/goliatone/go-cms/internal/workflow/adapter"
)

flowEngine := buildFlowStateMachine() // engine exposing Transition/AvailableTransitions/RegisterWorkflow

workflowEngine, _ := cmsadapter.NewEngine(flowEngine,
	cmsadapter.WithAuthorizer(myAuthorizer{}), // evaluates guard strings on transitions
	cmsadapter.WithActionRegistry(cmsadapter.ActionRegistry{
		"page::publish": publishAction, // actions can emit events/notifications into TransitionResult
	}),
)

cfg.Workflow.Provider = "custom"
container := di.NewContainer(cfg,
	di.WithWorkflowEngine(workflowEngine),
)
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

- **Repository pattern** &mdash; every module ships "in memory" and Bun backed repositories; the container picks based on `cfg.Storage.Provider`.
- **Dependency injection** &mdash; `di.NewContainer` wires services. Override dependencies with functional options:

```go
container := di.NewContainer(cfg,
	di.WithBunDB(db),
	di.WithCache(cache, serializer),
	di.WithPageService(customPageSvc),
)
```

- **Commands** &mdash; `cmd/static` and `cmd/markdown` invoke direct command structs; construct handlers in core or use the adapter module (`github.com/goliatone/go-cms/commands`) if you need registry/cron wiring.

### Database Migrations

When using BunDB as the storage provider, the CMS provides embedded SQL migrations to create all required tables. The migrations follow Bun's naming convention and include dialect-specific overrides under `data/sql/migrations/sqlite`, so register them via the dialect-aware loader.

```go
import (
	"context"
	"database/sql"
	"io/fs"

	"github.com/goliatone/go-cms"
	persistence "github.com/goliatone/go-persistence-bun"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// Open database connection
db, err := sql.Open(sqliteshim.ShimName, "file:cms.db?cache=shared")
if err != nil {
	panic(err)
}

// Create Bun client with migrations
client, err := persistence.New(cfg.Persistence, db, sqlitedialect.New())
if err != nil {
	panic(err)
}

// Register CMS migrations (dialect-aware)
migrationsFS, err := fs.Sub(cms.GetMigrationsFS(), "data/sql/migrations")
if err != nil {
	panic(err)
}
client.RegisterDialectMigrations(
	migrationsFS,
	persistence.WithDialectSourceLabel("data/sql/migrations"),
	persistence.WithValidationTargets("postgres", "sqlite"),
)
if err := client.ValidateDialects(context.Background()); err != nil {
	panic(err)
}

// Run migrations
if err := client.Migrate(context.Background()); err != nil {
	panic(err)
}

// Check migration status
if report := client.Report(); report != nil && !report.IsZero() {
	fmt.Printf("Applied migrations: %s\n", report.String())
}
```

The CMS includes migrations for all core tables:

- Locales and content types
- Contents with translations and versions
- Themes and templates
- Pages with translations and versions
- Block definitions, instances, translations, and versions
- Widget definitions, instances, translations, areas, and placements
- Menus, menu items, and menu item translations

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

# Translation enforcement toggles (optional; defaults are strict)
go run ./cmd/markdown import --translations-enabled=false --require-translations=false ...
go run ./cmd/static build --translations-enabled=false --require-translations=false ...

# Example application
go run ./cmd/example
go run ./cmd/example shortcodes
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

## Verification

Run the workflow regression suite before shipping workflow changes. These commands exercise the externalized workflow engine (including generator integration) and require the full Go binary path provided in the task plan.

```bash
CMS_WORKFLOW_PROVIDER=custom \
CMS_WORKFLOW_ENGINE_ADDR=http://localhost:8080 \
go test ./internal/workflow/... ./internal/integration/...
```

Translation-related changes should also pass the full suite with the pinned toolchain:

```bash
go test ./...
```

To run the same suite via the task runner:

```bash
./taskfile workflow:test
```

When using the built-in engine, the environment variables can be omitted.

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
- Menu canonicalization (go-admin alignment): `MENU_CANONICALIZATION.md`
- Task-driven design: `docs/CMS_TDD.md`, `docs/CMD_TDD.md`

## License

Copyright © 2025 goliatone - Licensed under the terms of [LICENSE](LICENSE).
