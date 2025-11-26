# CMS Implementation Reference

This document consolidates the code-level examples and wiring details that support the architectural plan described in `CMS_TDD.md`. Each section mirrors a conceptual area in the TDD and provides the corresponding implementation snippets.

## Module Layout

```
cms/
├── go.mod
├── go.sum
├── cms.go                      # Public API wrapper
├── config.go                   # Configuration types
│
├── cmd/
│   └── example/
│       └── main.go             # Example CLI with DI wiring
│
├── internal/
│   ├── di/
│   │   └── container.go        # Dependency injection container
│   │
│   ├── domain/
│   │   └── types.go            # Core domain types (Status, etc.)
│   │
│   ├── content/
│   │   ├── types.go            # Content-specific types
│   │   ├── service.go          # Interface + implementation
│   │   ├── repository.go       # Storage interface
│   │   └── testdata/           # Fixtures for contract tests
│   │       ├── basic_content.json
│   │       └── basic_content_output.json
│   │
│   ├── pages/
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── testdata/
│   │       ├── hierarchical_pages.json
│   │       └── hierarchical_pages_output.json
│   │
│   ├── blocks/
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── registry.go
│   │   ├── repository.go
│   │   └── testdata/
│   │       ├── nested_blocks.json
│   │       └── nested_blocks_output.json
│   │
│   ├── menus/
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── testdata/
│   │
│   ├── widgets/
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── registry.go
│   │   ├── repository.go
│   │   └── testdata/
│   │
│   ├── themes/
│   │   ├── types.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── testdata/
│   │
│   └── i18n/
│       ├── service.go          # Wraps go-i18n translator/culture services
│       ├── config.go           # Maps CMS config to go-i18n options
│       ├── loader.go           # Optional DB-backed loader plugged into go-i18n
│       ├── template_helpers.go # Augments go-i18n template helpers
│       └── testdata/
│           ├── fallback_chain.json
│           └── fallback_chain_output.json
│
└── pkg/
    ├── interfaces/             # External dependency interfaces
    │   ├── storage.go
    │   ├── cache.go
    │   ├── logger.go
    │   ├── template.go
    │   ├── media.go
    │   └── auth.go
    │
    └── testsupport/            # Shared test utilities
        ├── fixtures.go         # LoadFixture, LoadGolden, WriteGolden
        └── dbtest.go           # Test database setup helpers
```

> Storage adapters provided with the CMS wrap `github.com/goliatone/go-persistence-bun` for connection/migrations and `github.com/goliatone/go-repository-bun` for repositories so they fulfil the `StorageProvider` interface without coupling domain services to a specific ORM.
>
> Cache decorators provided with the CMS wrap `github.com/goliatone/go-repository-cache`, giving any registered repository transparent read caching while still honouring the `CacheProvider` interface defined here.

### Phase 1 Scaffolding Workflow

During Phase 1 we maintain an isolated prototype of the module under `.tmp/cms/` so we can iterate quickly while core contracts settle. Bootstrap or review the scaffolding with:

1. Inspect `.tmp/cms/go.mod` to confirm the module path (`github.com/goliatone/go-cms`) and Go toolchain version.
2. Review `.tmp/cms/config.go` for baseline configuration defaults and feature flags.
3. Explore `.tmp/cms/internal/` for placeholder services (`content`, `pages`, `i18n`) and the DI container stub in `internal/di/container.go`.
4. Reuse shared testing helpers in `.tmp/cms/pkg/testsupport/` when sketching contract tests.
5. Run the example bootstrap `go run ./cmd/example` from `.tmp/cms/` (using the full Go binary path provided in the task plan) to verify the scaffolding compiles.

Once the interfaces stabilise, we will promote the `.tmp` implementation into the production tree following the guidelines in `CMS_TSK.md`.

### Content Module (Phase 1 snapshot)

- Domain structs for `locales`, `content_types`, `contents`, and `content_translations` live in `.tmp/cms/internal/content/types.go`, mirroring the schemas documented in `CMS_ENTITIES.md`.
- Bun repositories are declared in `.tmp/cms/internal/content/repository.go` using `github.com/goliatone/go-repository-bun` handlers so the DI container can supply storage-backed behaviour later.
- Application service logic with validation rules is staged in `.tmp/cms/internal/content/service.go`, backed by in-memory repositories (`.tmp/cms/internal/content/memory.go`) and unit tests in `.tmp/cms/internal/content/service_test.go`.
- Fixture-driven contract tests reside in `.tmp/cms/internal/content/repository_test.go` with JSON samples under `.tmp/cms/internal/content/testdata/` to guide future implementation work (currently skipped until the repositories are wired to real persistence).

### Pages Module (Phase 1 snapshot)

- Page domain models (`pages`, `page_versions`, `page_translations`) are defined in `.tmp/cms/internal/pages/types.go`, with relations back to the shared content entities.
- Repository constructors in `.tmp/cms/internal/pages/repository.go` wrap Bun repositories and align with the slug/path uniqueness rules referenced in `CMS_ENTITIES.md`.
- Page service scaffolding lives in `.tmp/cms/internal/pages/service.go` with in-memory repositories (`.tmp/cms/internal/pages/memory.go`) and targeted unit tests at `.tmp/cms/internal/pages/service_test.go` covering slug and locale validation.
- Contract fixtures and pending tests live in `.tmp/cms/internal/pages/testdata/` and `.tmp/cms/internal/pages/repository_test.go`, ensuring hierarchical paths and translations remain consistent once persistence logic is implemented.

## Static Generator Integration

The static generator introduced in the Static SSG initiative reuses the CMS service layer to emit pre-rendered HTML, assets, and metadata without introducing hard dependencies for existing embedders. All orchestration code lives under `internal/generator/` and is surfaced via `cms.Module.Generator()`.

### Configuration Overview

Enable the generator through `cms.Config.Generator`—it remains opt-in so modules that do not require static builds incur no overhead:

```go
cfg := cms.DefaultConfig()
cfg.Generator.Enabled = true
cfg.Generator.OutputDir = "./dist"
cfg.Generator.BaseURL = "https://example.com"
cfg.Generator.Incremental = true        // Skip unchanged pages when manifest matches
cfg.Generator.CopyAssets = true         // Mirror theme assets to the output storage
cfg.Generator.GenerateSitemap = true    // Emit sitemap.xml through the storage provider
cfg.Generator.Workers = runtime.NumCPU()

module, err := cms.New(cfg)
if err != nil {
    return err
}

result, err := module.Generator().Build(ctx, generator.BuildOptions{
    Locales: []string{"en", "es"},
})
if err != nil {
    return err
}
```

The generator respects incremental manifests (`internal/generator/manifest.go`) and storage streaming via `pkg/interfaces.StorageProvider`. When no template renderer or generator storage is configured, DI falls back to the defaults wired in `internal/di/container.go`.

### Workflow Configuration

`cms.Config.Workflow` selects the lifecycle engine that coordinates page status transitions:

```go
cfg := cms.DefaultConfig()

// Built-in simple engine (default behaviour)
cfg.Workflow.Enabled = true
cfg.Workflow.Provider = "simple"

module, _ := cms.New(cfg)
defaultEngine := module.WorkflowEngine() // *simple.Engine

// Custom engine injected through DI
cfg.Workflow.Provider = "custom"

module, err := cms.New(cfg, di.WithWorkflowEngine(myEngine))
if err != nil {
    return err
}
```

- When the provider is omitted or set to `simple`, the DI container wires `internal/workflow/simple` automatically.
- Setting `Provider` to `custom` requires an explicit `interfaces.WorkflowEngine` via `di.WithWorkflowEngine`.
- Disabling workflows (`cfg.Workflow.Enabled = false`) reverts to legacy status strings without invoking the engine.
- Register configuration-driven definitions through `cfg.Workflow.Definitions` to replace or extend the default state machine. Definitions are validated and normalised during container construction:

```go
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

- Supply definitions from a backing store (database, files, API) by implementing `interfaces.WorkflowDefinitionStore` and wiring it through `di.WithWorkflowDefinitionStore`. Store-provided definitions override configuration entries when entity types collide.
- Transition guards are prepared for authorisation callbacks via the placeholder `interfaces.WorkflowAuthorizer`; custom engines can adopt the interface ahead of Phase 5 security work.

#### Data Migration Considerations

1. **Inventory legacy statuses** – run a quick report against the `pages` (and `contents`, if applicable) table to list distinct `status` values. Normalise them with `domain.NormalizeWorkflowState` in a scratch script to confirm they collapse to the expected enum (`draft`, `published`, `scheduled`, `archived`, etc.).
2. **Clean inconsistent data** – trim whitespace and lower-case persisted statuses before enabling the engine. For relational stores, a one-off update such as `UPDATE pages SET status = LOWER(TRIM(status))` keeps values compatible with the enum adapters.
3. **Review scheduled entries** – ensure records with a `"scheduled"` status also carry a non-null `publish_on`. The simple engine treats missing timestamps as validation errors, so correct or cancel those jobs ahead of the switch.
4. **Backfill workflow metadata** – if you plan to seed richer workflows, preload definitions into `cfg.Workflow.Definitions` (or your `WorkflowDefinitionStore`) and run `module.WorkflowEngine().RegisterWorkflow(...)` in a maintenance window so the new state machine is active before requests hit the services.
5. **Dry-run validation** – execute `/Users/goliatone/.g/go/bin/go test ./internal/workflow/... ./internal/pages/...` against a staging copy to verify transitions succeed with the migrated data. For production data sets, run a read-only publish simulation (for example, invoking `PageService.PublishDraft` on representative records) in a staging environment to confirm the engine returns the expected states.
6. **Cutover safeguards** – enable `cfg.Workflow.Enabled` and deploy behind a feature flag. Monitor audit/event hooks (see `WorkflowResult.Events`) so custom dashboards or log pipelines confirm transitions occur as expected. Keep the legacy status list handy in case you must toggle the feature off; the enum adapters continue to map new engine states back to persisted strings, so fallback simply bypasses the engine.

### Template Context Reference

Template authors receive a strongly-typed payload defined in `internal/generator/render.go`:

- `TemplateContext.Site` exposes `BaseURL`, `DefaultLocale`, and locale metadata, plus menu alias mappings configured via `cms.Config.Generator.Menus`.
- `TemplateContext.Page` bundles the resolved `pages.Page`, `content.Content`, translations, and the active `themes.Template`.

## Logging & Observability

Phase 7 consolidated logging across services and commands. The DI container promotes the `pkg/interfaces.Logger` contract and supplies defaults when no provider is configured.

### Console Logger (Default)

- Implemented in `internal/logging/console`.
- Enabled automatically when no external provider is supplied.
- Emits leveled logs with lightweight key/value formatting—ideal for tests and examples.

### go-logger Adapter

- Located in `internal/logging/gologger`.
- Activated when `cfg.Logging.Provider == "go-logger"` and `di.WithLoggerProvider` receives a go-logger-backed provider.
- Supports JSON output, leveled filtering, and structured context propagation.

```go
provider := gologger.NewProvider(gologger.Config{
    Level:  "info",
    Format: "json",
})

cfg := cms.DefaultConfig()
cfg.Logging.Provider = "go-logger"

module, err := cms.New(cfg, di.WithLoggerProvider(provider))
if err != nil {
    return err
}
```

### Command Handler Telemetry

All command handlers (e.g., static generator commands) wrap execution with `internal/commands.Handler`, which:

- Validates messages before execution.
- Enforces per-command timeouts (`commands.WithTimeout`).
- Emits structured telemetry via `commands.DefaultTelemetry`.
- Provides optional callbacks for result envelopes, enabling CLIs to log metrics without coupling to service internals (see `cmd/static/main.go`).

Refer to the dedicated [Logging Integration Guide](./LOGGING_GUIDE.md) for provider wiring, CLI bootstrap overrides, and troubleshooting tips.
- `TemplateContext.Helpers` supplies locale-aware utilities (`Locale()`, `IsDefaultLocale()`, `WithBaseURL(..)`, `LocalePrefix()`), making it easier to render language switchers and canonical URLs.
- `TemplateContext.Build` embeds timing data and the `BuildOptions` used for the current run, enabling templates to surface diagnostics or cache-busting metadata.

A minimal Handlebars/Go template can iterate the blocks and navigation supplied by the context:

```gotemplate
{{ define "page" }}
<!doctype html>
<html lang="{{ .Page.Locale.Code }}">
  <head>
    <meta charset="utf-8">
    <title>{{ .Page.Translation.Title }}</title>
    <link rel="stylesheet" href="{{ .Helpers.WithBaseURL (printf "/assets/%s.css" .Page.Template.Handle) }}">
  </head>
  <body>
    {{ range .Page.Blocks }}{{ template .TemplatePath . }}{{ end }}
    {{ range $code, $items := .Page.Menus }}
      <nav data-menu="{{ $code }}">
        {{ range $items }}<a href="{{ $.Helpers.WithBaseURL .Path }}">{{ .Label }}</a>{{ end }}
      </nav>
    {{ end }}
  </body>
</html>
{{ end }}
```

### CLI Workflow

`cmd/static` wraps the generator with a purpose-built CLI that respects DI wiring and feature flags. Typical invocations:

- `go run ./cmd/static build --output=./dist --locale=en,es` renders all published pages for the specified locales and writes them to the configured storage backend.
- `go run ./cmd/static diff --dry-run` executes a build without persisting artifacts, surfacing diagnostics for templates or data issues.
- `go run ./cmd/static build --assets` copies theme assets exclusively—useful for CDNs or when page HTML is handled separately.
- `go run ./cmd/static clean` clears the output directory via the storage abstraction and triggers the configured hooks.

Both CLI and programmatic usage emit structured logs (`module=static`, `operation=build`) and timing metrics aligned with the telemetry conventions introduced in Phase 7 of the command work.

### Multi-locale Output Example

Running the sample walkthrough under `examples/static` produces the following tree in the configured storage provider:

```
dist/
├── sitemap.xml
├── robots.txt
├── assets/
│   ├── theme.css
│   └── logo.png
├── en/
│   ├── index.html
│   └── company/
│       └── index.html
└── es/
    ├── index.html
    └── empresa/
        └── index.html
```

The generator automatically promotes the default locale to the root (`dist/index.html`) while nesting non-default locales beneath their locale codes.

### Migration Checklist for Existing Deployments

1. **Configuration** – enable the generator (`cfg.Generator.Enabled = true`), point `OutputDir` at an isolated artifact location, copy locale codes from existing runtime configuration, and map menu aliases to canonical menu codes (for example, `{"primary": "main"}`).
2. **Template renderer** – register a concrete `interfaces.TemplateRenderer` (Handlebars, Go templates, etc.) and ensure theme templates expose stable `TemplatePath` values. Audit templates against the `TemplateContext` contract so locale, menu, and helper lookups do not rely on repository internals.
3. **Theme assets** – populate `themes.Theme.Config.Assets` with the canonical asset list and supply a `generator.AssetResolver` that can open those files from your asset store (local disk, embedded FS, object storage).
4. **Storage provider** – decide whether generator artifacts land on local disk or an external bucket. Provide the matching `interfaces.StorageProvider` (for example via `di.WithGeneratorStorage`) so the writer can stream HTML and manifest files without intermediate buffers.
5. **Deployment pipeline** – wire `cmd/static` into existing build or CD jobs (`go run ./cmd/static build ...`) and publish the generated directory to your hosting target. When running incremental builds, persist the manifest (`.generator/manifest.json`) between runs to benefit from skip logic.
6. **Rollback and validation** – extend existing deployment checks to diff the generator output (use `go run ./cmd/static diff`) and verify sitemap/robots content before promotion. This keeps static artifacts in lockstep with runtime templates and content approval workflows.

## External Dependency Interfaces

### Storage Provider

```go
// pkg/interfaces/storage.go
package interfaces

import "context"

// StorageProvider defines the interface for data persistence
type StorageProvider interface {
    Query(ctx context.Context, query string, args ...any) (Rows, error)
    Exec(ctx context.Context, query string, args ...any) (Result, error)
    Transaction(ctx context.Context, fn func(tx Transaction) error) error
}

// Rows represents query result rows
type Rows interface {
    Next() bool
    Scan(dest ...any) error
    Close() error
}

// Result represents execution result
type Result interface {
    RowsAffected() (int64, error)
    LastInsertId() (int64, error)
}

// Transaction represents a database transaction
type Transaction interface {
    StorageProvider
    Commit() error
    Rollback() error
}
```

Implementations based on `github.com/goliatone/go-persistence-bun` and `github.com/goliatone/go-repository-bun` satisfy `StorageProvider` out of the box; the adapters in `adapters/storage` simply expose those packages through this interface so domain services can remain persistence-agnostic.

### Storage Admin Service

- `internal/admin/storage/service.go` encapsulates storage profile management. The service stays transport-agnostic: it offers `ApplyConfig`, `ListProfiles`, `GetProfile`, `ValidateConfig`, `PreviewProfile`, and `Schemas` so host packages can build their own HTTP/CLI handlers without importing unexported packages.
- The preview helper accepts a profile payload, runs it through the DI-registered storage factory, and returns a `PreviewResult` containing provider capabilities plus a diagnostics map. Active handles are untouched when previews fail.
- `di.Container` wires the service automatically by passing the shared storage repository, audit recorder, and a preview closure that reuses registered factories. The container exposes it via `StorageAdminService()`, and `cms.Module.StorageAdmin()` re-exports the pointer so applications can mount it next to other services.
- Audit integration comes for free: creates, updates, deletes, and alias changes emit `storage_profile_*` audit events. The container already logs `storage.profile_activated` / `storage.profile_activate_failed`, which is enough to feed dashboards for reload success/failure metrics.
- Integration coverage lives in `internal/integration/storage_admin_test.go`. The test primes two SQLite databases, runs concurrent writes, applies an admin config through the service, and asserts that writes continue against the rotated profile without surfacing errors to callers.

### Cache Provider

```go
// pkg/interfaces/cache.go
package interfaces

import (
    "context"
    "time"
)

// CacheProvider defines the interface for caching
type CacheProvider interface {
    Get(ctx context.Context, key string) (any, error)
    Set(ctx context.Context, key string, value any, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
}
```

`github.com/goliatone/go-repository-cache` provides ready-made decorators that implement this interface and can be plugged into the CMS container, ensuring cached repositories stay stampede-safe without leaking implementation details into services.

### Logger

```go
// pkg/interfaces/logger.go
package interfaces

import "context"

type Logger interface {
    Trace(msg string, args ...any)
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
    Fatal(msg string, args ...any)
    WithContext(ctx context.Context) Logger
}

type LoggerProvider interface {
    GetLogger(name string) Logger
}

type FieldsLogger interface {
    WithFields(fields map[string]any) Logger
}
```

The CMS uses this contract for all runtime diagnostics. Because it mirrors `github.com/goliatone/go-logger`, host applications can inject that package directly (or provide any compatible implementation) without adding a hard dependency to the module. The example bootstrap can wire a simple console logger for development, while production systems usually pass a configured go-logger provider through the DI container.

```go
cfg := cms.DefaultConfig()
cfg.Features.Logger = true
cfg.Logging.Provider = "gologger"
cfg.Logging.Level = "debug"
cfg.Logging.Format = "json"   // console | json | pretty
cfg.Logging.AddSource = true  // include caller metadata

module, err := cms.New(cfg)
```

When `cfg.Logging.Provider` is omitted or set to `console`, the module falls back to the built-in structured console logger. Setting the provider to `gologger` enables the adapter in `internal/logging/gologger`, which wraps `github.com/goliatone/go-logger/glog` so deployments can reuse their existing logger configuration without touching business services.

### Template Renderer

```go
// pkg/interfaces/template.go
package interfaces

import "io"

// TemplateRenderer mirrors github.com/goliatone/go-template's engine contract.
type TemplateRenderer interface {
    Render(name string, data any, out ...io.Writer) (string, error)
    RenderTemplate(name string, data any, out ...io.Writer) (string, error)
    RenderString(templateContent string, data any, out ...io.Writer) (string, error)
    RegisterFilter(name string, fn func(input any, param any) (any, error)) error
    GlobalContext(data any) error
}
```

### i18n Service

```go
// pkg/interfaces/i18n.go
package interfaces

type Translator interface {
    Translate(locale, key string, args ...any) (string, error)
}

type CultureService interface {
    GetCurrencyCode(locale string) (string, error)
    GetCurrency(locale string) (any, error)
    GetSupportNumber(locale string) (string, error)
    GetList(locale, name string) ([]string, error)
    GetMeasurementPreference(locale, measurementType string) (any, error)
    ConvertMeasurement(locale string, value float64, fromUnit, measurementType string) (float64, string, string, error)
}

type HelperConfig struct {
    LocaleKey         string
    TemplateHelperKey string
    OnMissing         MissingTranslationHandler
    Registry          FormatterRegistry
}

type Service interface {
    Translator() Translator
    Culture() CultureService
    TemplateHelpers(cfg HelperConfig) map[string]any
    DefaultLocale() string
}
```

### Media Provider

```go
// pkg/interfaces/media.go
package interfaces

import "context"

// MediaProvider defines the interface for media/asset handling
type MediaProvider interface {
    GetURL(ctx context.Context, path string) (string, error)
    GetMetadata(ctx context.Context, id string) (MediaMetadata, error)
}

// MediaMetadata represents media file metadata
type MediaMetadata struct {
    ID       string
    MimeType string
    Size     int64
    Width    int
    Height   int
}
```

### Auth Service

```go
// pkg/interfaces/auth.go
package interfaces

import (
    "context"
    "time"

    "github.com/google/uuid"
)

type AuthClaims interface {
    Subject() string
    UserID() string
    Role() string
    CanRead(resource string) bool
    CanEdit(resource string) bool
    CanCreate(resource string) bool
    CanDelete(resource string) bool
    HasRole(role string) bool
    IsAtLeast(minRole string) bool
    Expires() time.Time
    IssuedAt() time.Time
}

type Session interface {
    GetUserID() string
    GetUserUUID() (uuid.UUID, error)
    GetAudience() []string
    GetIssuer() string
    GetIssuedAt() *time.Time
    GetData() map[string]any
}

type TokenService interface {
    Generate(identity Identity, resourceRoles map[string]string) (string, error)
    Validate(token string) (AuthClaims, error)
}

type Authenticator interface {
    Login(ctx context.Context, identifier, password string) (string, error)
    Impersonate(ctx context.Context, identifier string) (string, error)
    SessionFromToken(token string) (Session, error)
    IdentityFromSession(ctx context.Context, session Session) (Identity, error)
    TokenService() TokenService
}

type Identity interface {
    ID() string
    Username() string
    Email() string
    Role() string
}

type AuthService interface {
    Authenticator() Authenticator
    TokenService() TokenService
    TemplateHelpers() map[string]any
}
```

## Configuration Layer

```go
// config.go
package cms

import (
    "fmt"
    "time"

    "github.com/goliatone/cms/pkg/interfaces"
)

// Config defines what the CMS should use (Layer 1: Configuration)
type Config struct {
    // Required infrastructure
    Storage interfaces.StorageProvider

    // Optional infrastructure (can be nil)
    Cache    interfaces.CacheProvider
    Template interfaces.TemplateRenderer
    Media    interfaces.MediaProvider
    Auth     interfaces.AuthService

    // Locale configuration
    DefaultLocale string
    Locales       []Locale

    // Feature flags (progressive complexity)
    Features Features

    // Performance tuning
    CacheTTL      time.Duration
    MaxPageDepth  int
    MaxBlockDepth int
}

// Locale represents a language/region configuration
type Locale struct {
    Code             string
    Name             string
    IsDefault        bool
    FallbackLocaleID string // Optional: for regional fallback
}

// Features controls which CMS features are enabled
type Features struct {
    // Sprint 1 (always enabled)
    BasicContent bool // Always true
    BasicPages   bool // Always true

    // Sprint 2
    Blocks       bool
    NestedBlocks bool

    // Sprint 3
    Menus             bool
    HierarchicalMenus bool

    // Sprint 4
    Widgets bool

    // Sprint 5
    Themes    bool
    Templates bool

    // Sprint 6
    Versioning    bool
    Scheduling    bool
    MediaLibrary  bool
    AdvancedCache bool
}

// Validate ensures config is usable
func (c *Config) Validate() error {
    if c.Storage == nil {
        return fmt.Errorf("storage provider is required")
    }

    if c.DefaultLocale == "" {
        c.DefaultLocale = "en"
    }

    if len(c.Locales) == 0 {
        c.Locales = []Locale{
            {Code: c.DefaultLocale, Name: "English", IsDefault: true},
        }
    }

    if c.CacheTTL == 0 {
        c.CacheTTL = 5 * time.Minute
    }

    if c.MaxPageDepth == 0 {
        c.MaxPageDepth = 5
    }

    if c.MaxBlockDepth == 0 {
        c.MaxBlockDepth = 10
    }

    // Validate feature dependencies
    if c.Features.NestedBlocks && !c.Features.Blocks {
        return fmt.Errorf("NestedBlocks requires Blocks feature")
    }

    if c.Features.HierarchicalMenus && !c.Features.Menus {
        return fmt.Errorf("HierarchicalMenus requires Menus feature")
    }

    if c.Features.Templates && !c.Features.Themes {
        return fmt.Errorf("Templates requires Themes feature")
    }

    return nil
}

// ApplyDefaults sets reasonable defaults for all features
func (c *Config) ApplyDefaults() {
    if c.Features == (Features{}) {
        c.Features = Features{
            BasicContent: true,
            BasicPages:   true,
            // Rest are false by default (progressive complexity)
        }
    }
}
```

## Public API Layer

```go
// cms.go
package cms

import (
    "context"
    "fmt"

    "github.com/goliatone/cms/internal/di"
    "github.com/goliatone/cms/internal/content"
    "github.com/goliatone/cms/internal/pages"
    "github.com/goliatone/cms/internal/blocks"
    "github.com/goliatone/cms/internal/menus"
    "github.com/goliatone/cms/internal/widgets"
    "github.com/goliatone/cms/internal/themes"
)

// CMS is the main entry point for the CMS module
type CMS struct {
    container *di.Container
}

// New creates a new CMS instance
func New(config Config) (*CMS, error) {
    container, err := di.NewContainer(&config)
    if err != nil {
        return nil, fmt.Errorf("failed to create CMS: %w", err)
    }

    return &CMS{
        container: container,
    }, nil
}

// Content returns the content service
func (c *CMS) Content() content.Service {
    return c.container.ContentService()
}

// Pages returns the page service
func (c *CMS) Pages() pages.Service {
    return c.container.PageService()
}

// Blocks returns the block service
// Returns no-op service if blocks feature not enabled
func (c *CMS) Blocks() blocks.Service {
    return c.container.BlockService()
}

// Menus returns the menu service
// Returns no-op service if menus feature not enabled
func (c *CMS) Menus() menus.Service {
    return c.container.MenuService()
}

// Widgets returns the widget service
// Returns no-op service if widgets feature not enabled
func (c *CMS) Widgets() widgets.Service {
    return c.container.WidgetService()
}

// Themes returns the theme service
// Returns no-op service if themes feature not enabled
func (c *CMS) Themes() themes.Service {
    return c.container.ThemeService()
}

// Close cleans up CMS resources
func (c *CMS) Close() error {
    return c.container.Close()
}

// HealthCheck verifies CMS is healthy
func (c *CMS) HealthCheck(ctx context.Context) error {
    // Check storage connectivity
    if err := c.container.Storage().Ping(ctx); err != nil {
        return fmt.Errorf("storage unhealthy: %w", err)
    }

    // Check if any services failed to initialize
    if errors := c.container.GetErrors(); len(errors) > 0 {
        return fmt.Errorf("service initialization errors: %v", errors)
    }

    return nil
}
```

## Dependency Injection Container

```go
// internal/di/container.go
package di

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/internal/content"
    "github.com/goliatone/cms/internal/pages"
    "github.com/goliatone/cms/internal/blocks"
    "github.com/goliatone/cms/internal/menus"
    "github.com/goliatone/cms/internal/widgets"
    "github.com/goliatone/cms/internal/themes"
    "github.com/goliatone/cms/internal/i18n"
    exti18n "github.com/goliatone/go-i18n"
    "github.com/goliatone/cms/pkg/interfaces"
)

// Container manages service lifecycle and dependencies
type Container struct {
    config *cms.Config

    // Infrastructure (provided by user)
    storage  interfaces.StorageProvider
    cache    interfaces.CacheProvider
    template interfaces.TemplateRenderer
    media    interfaces.MediaProvider
    auth     interfaces.AuthService

    // Services (lazy-initialized, singleton pattern)
    i18nService    i18n.Service
    contentService content.Service
    pageService    pages.Service
    blockService   blocks.Service
    menuService    menus.Service
    widgetService  widgets.Service
    themeService   themes.Service

    // Synchronization for lazy initialization
    once struct {
        i18n    sync.Once
        content sync.Once
        page    sync.Once
        block   sync.Once
        menu    sync.Once
        widget  sync.Once
        theme   sync.Once
    }

    // Error tracking for initialization failures
    initErrors map[string]error
    mu         sync.RWMutex
}

// NewContainer creates a new DI container
func NewContainer(config *cms.Config) (*Container, error) {
    // Config validation happens here
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }

    config.ApplyDefaults()

    // Extract infrastructure
    storage := config.Storage // typically provided by go-persistence-bun/go-repository-bun adapter
    cache := config.Cache
    template := config.Template
    media := config.Media
    auth := config.Auth

    // Provide no-op implementations if not provided
    if cache == nil {
        cache = newNoOpCache()
    }

    return &Container{
        config:     config,
        storage:    storage,
        cache:      cache,
        template:   template,
        media:      media,
        auth:       auth,
        initErrors: make(map[string]error),
    }, nil
}

// I18nService returns the i18n service (lazy init, singleton)
func (c *Container) I18nService() i18n.Service {
    c.once.i18n.Do(func() {
        cfg, err := exti18n.NewConfig(
            exti18n.WithDefaultLocale(c.config.DefaultLocale),
            exti18n.WithLocales(c.config.LocaleCodes()...), // helper returns []string of locale codes
            exti18n.WithFallbackResolver(i18n.BuildFallbackResolver(c.config.Locales)),
            exti18n.WithLoader(i18n.NewLoader(c.storage)), // optional DB loader
            exti18n.WithCultureData(c.config.CultureDataPath),
        )
        if err != nil {
            c.recordError("i18n", fmt.Errorf("build i18n config: %w", err))
            c.i18nService = i18n.NewNoOpService()
            return
        }

        translator, err := cfg.BuildTranslator()
        if err != nil {
            c.recordError("i18n", fmt.Errorf("build translator: %w", err))
            c.i18nService = i18n.NewNoOpService()
            return
        }

        helpers := cfg.TemplateHelpers(translator, exti18n.HelperConfig{
            TemplateHelperKey: c.config.TemplateLocaleKey,
        })

        c.i18nService = i18n.NewService(i18n.ServiceOptions{
            Translator:    translator,
            Culture:       cfg.CultureService(),
            Helpers:       helpers,
            DefaultLocale: c.config.DefaultLocale,
        })
    })

    return c.i18nService
}

// ContentService returns the content service (lazy init, singleton)
func (c *Container) ContentService() content.Service {
    c.once.content.Do(func() {
        repo := content.NewRepository(c.storage) // adapter uses go-repository-bun under the hood

        c.contentService = content.NewService(
            repo,
            c.cache,
            c.I18nService(), // Dependency: i18n
            content.Options{
                CacheTTL: c.config.CacheTTL,
            },
        )
    })

    return c.contentService
}

// PageService returns the page service (lazy init, singleton)
func (c *Container) PageService() pages.Service {
    c.once.page.Do(func() {
        repo := pages.NewRepository(c.storage)

        c.pageService = pages.NewService(
            repo,
            c.cache,
            c.I18nService(),    // Dependency: i18n
            c.ContentService(), // Dependency: content
            pages.Options{
                MaxDepth: c.config.MaxPageDepth,
                CacheTTL: c.config.CacheTTL,
            },
        )
    })

    return c.pageService
}

// BlockService returns the block service (lazy init, singleton)
func (c *Container) BlockService() blocks.Service {
    if !c.config.Features.Blocks {
        c.recordError("block", fmt.Errorf("blocks feature not enabled"))
        return blocks.NewNoOpService()
    }

    c.once.block.Do(func() {
        repo := blocks.NewRepository(c.storage)
        registry := blocks.NewRegistry()

        // Register built-in block types
        registerBuiltInBlocks(registry)

        opts := blocks.Options{
            MaxDepth:     c.config.MaxBlockDepth,
            AllowNesting: c.config.Features.NestedBlocks,
            CacheTTL:     c.config.CacheTTL,
        }

        c.blockService = blocks.NewService(
            repo,
            registry,
            c.cache,
            c.I18nService(),
            opts,
        )
    })

    return c.blockService
}

// MenuService returns the menu service (lazy init, singleton)
func (c *Container) MenuService() menus.Service {
    if !c.config.Features.Menus {
        c.recordError("menu", fmt.Errorf("menus feature not enabled"))
        return menus.NewNoOpService()
    }

    c.once.menu.Do(func() {
        repo := menus.NewRepository(c.storage)

        opts := menus.Options{
            AllowHierarchical: c.config.Features.HierarchicalMenus,
            CacheTTL:          c.config.CacheTTL,
        }

        c.menuService = menus.NewService(
            repo,
            c.cache,
            c.I18nService(),
            c.PageService(), // Dependency: pages (for menu item validation)
            opts,
        )
    })

    return c.menuService
}

### Navigation Routing (go-urlkit)

Menus can now delegate URL generation to [go-urlkit](https://github.com/goliatone/go-urlkit). The CMS config exposes a `Navigation` block:

```go
type NavigationConfig struct {
    RouteConfig *urlkit.Config           // hierarchy of groups/routes
    URLKit      URLKitResolverConfig     // resolver options
}

type URLKitResolverConfig struct {
    DefaultGroup string            // e.g. "frontend"
    LocaleGroups map[string]string // e.g. {"es": "frontend.es"}
    DefaultRoute string            // e.g. "page"
    SlugParam    string            // parameter name to receive the page slug (default: "slug")
    LocaleParam  string            // optional param for locale
    LocaleIDParam string           // optional param for locale UUID
    RouteField   string            // target field containing an override route name (default: "route")
    ParamsField  string            // target field containing extra params (default: "params")
    QueryField   string            // target field containing query map (default: "query")
}
```

### Service Initialisation Ordering

The container now constructs high-traffic services in a single pass so every dependency is available before the next layer is built:

1. `blocks.Service` (with media/versioning options)
2. `widgets.Service` (pluggable registry or no-op depending on the feature flag)
3. `themes.Service` (no-op when themes are disabled)
4. `pages.Service` – initialised once with block, widget, theme, media, versioning, and scheduling options applied before any command handlers capture it
5. `menus.Service` – created after pages so it can reuse the page repository and URL resolver once

This ordering avoids the previous “build then rebuild” behaviour for the page and menu services and guarantees that feature-gated dependencies (widgets, themes, media) are always injected when enabled.


When `Navigation.RouteConfig` is provided the DI container constructs a shared `urlkit.RouteManager` and injects a `menus.URLResolver`. Menu items continue to use slug-based fallbacks if the resolver cannot produce a URL. Locale-specific groups are optional; omit `LocaleGroups` to share a single group across locales.

Example wiring:

```go
cfg := cms.DefaultConfig()
cfg.Navigation.RouteConfig = &urlkit.Config{
    Groups: []urlkit.GroupConfig{
        {
            Name:    "frontend",
            BaseURL: "https://example.com",
            Paths: map[string]string{
                "page": "/pages/:slug",
            },
            Groups: []urlkit.GroupConfig{
                {
                    Name: "es",
                    Path: "/es",
                    Paths: map[string]string{"page": "/paginas/:slug"},
                },
            },
        },
    },
}
cfg.Navigation.URLKit = cms.URLKitResolverConfig{
    DefaultGroup: "frontend",
    LocaleGroups: map[string]string{"es": "frontend.es"},
    DefaultRoute: "page",
    SlugParam:    "slug",
}

container := di.NewContainer(cfg)
menuSvc := container.MenuService()
```

Menu targets can set optional fields:

- `route` – override the route name resolved through go-urlkit.
- `params` – map of additional route parameters.
- `query` – map (or map of slices) for query string values.

If these fields are absent the resolver uses the configured defaults and the item slug.

### Widgets Integration

Widgets follow the same dependency structure as the other slices: the container wires an in-memory service by default and upgrades to Bun + cache repositories when `di.WithBunDB` is provided. If the widgets feature flag is disabled the container returns `widgets.NewNoOpService()`.

- **Repositories** – `NewContainer` primes memory repositories for definition, instance, translation, area definition, and area placement data. `configureRepositories` swaps in the Bun-backed implementations (`NewBunDefinitionRepositoryWithCache`, `NewBunInstanceRepositoryWithCache`, etc.) and clears the memory locale seed when a database connection is available.
- **Registry** – the container always creates a `widgets.Registry`, hydrates it with `cfg.Widgets.Definitions`, and passes it to the service through `widgets.WithRegistry`. The default configuration keeps the `newsletter_signup` definition so existing deployments retain their built-in widget; hosts can replace the slice or clear it entirely to opt-out. Duplicate names are normalised (case-insensitive) and the last definition wins, making it easy to override the default without editing internal helpers.
- **Area support** – area repositories are optional. When the container can provide `widgetAreaRepo` and `widgetPlacementRepo` (memory or Bun) it includes `widgets.WithAreaDefinitionRepository` and `widgets.WithAreaPlacementRepository`. If neither repository is configured, area APIs return `ErrAreaFeatureDisabled`, making it safe to ship applications that do not need placements yet.
- **Visibility** – the service enforces `publish_on`/`unpublish_on` windows, rule-based scheduling, audience/segment filters, and locale allowlists through `EvaluateVisibility`. `ResolveArea` walks the primary locale, fallbacks, and finally the default locale to return the first visible widgets, cloning placement metadata for presentation.
- **Bootstrap helpers** – `widgets.Bootstrap` wraps `EnsureDefinitions`/`EnsureAreaDefinitions`, allowing hosts to seed types and areas idempotently at startup even when the module is running in no-op mode or when definitions already exist.

#### Registry configuration & migration

Phase 4 externalises the built-in widget definition so applications can control the registry through configuration. Deployments that already used `cms.DefaultConfig()` continue to receive the `newsletter_signup` widget automatically because the default now seeds the same definition via `cfg.Widgets.Definitions`. To remove the default, set `cfg.Widgets.Definitions = nil` or an empty slice; to override it, replace the slice with your own definitions (later entries with the same name override earlier ones).

#### Version retention configuration

Phase 5 introduces the `cfg.Retention` block with per-slice version limits. The container now feeds the configured values into `content.WithVersionRetentionLimit`, `blocks.WithVersionRetentionLimit`, and `pages.WithPageVersionRetentionLimit`. Limits of zero preserve the previous “infinite history” behaviour; any positive value constrains how many drafts each service keeps before emitting `ErrContentVersionRetentionExceeded`, `ErrInstanceVersionRetentionExceeded`, or `ErrVersionRetentionExceeded`. The services log a warning via the module logger whenever a caller hits the ceiling so operators can adjust policies. Block versioning also gains first-class wiring: the container provisions the memory and Bun-backed instance version repositories and enables version retention whenever the `Versioning` feature is active.

### Scheduling Worker

Scheduled publishing relies on the `pkg/interfaces.Scheduler` contract. The interface exposes idempotent operations for enqueuing and cancelling jobs, introspecting their status, and marking the outcome once processed. The repository ships with both a no-op implementation (for feature-disabled configurations) and a deterministic in-memory implementation that powers fixture-driven tests.

The `internal/jobs.Worker` coordinates with the scheduler to process due jobs. It currently understands four job types:

- `cms.content.publish`
- `cms.content.unpublish`
- `cms.page.publish`
- `cms.page.unpublish`

For each job, the worker:

1. Fetches the target record through the content or page repositories.
2. Applies the appropriate state transition (publish clears `publish_at`, stamps `published_at`, and marks the record as `published`; unpublish drops `unpublish_at` and marks the record as `archived`).
3. Records an audit event through the pluggable `jobs.AuditRecorder` interface so host applications can persist scheduling activity.
4. Marks the job as completed or failed on the scheduler to unlock retries.

Host applications that provide their own scheduler must guarantee unique job keys (the worker relies on the `content:`/`page:` scoped keys exposed in `internal/scheduler/jobs.go`) and eventual delivery semantics. The worker is idempotent: it inspects the current publish state before mutating, making it safe to rerun missed jobs or to requeue after transient failures.

Example wiring with Bun storage and a custom registry addition:

```go
cfg := cms.DefaultConfig()
cfg.Features.Widgets = true

container := di.NewContainer(cfg, di.WithBunDB(bunDB))
widgetSvc := container.WidgetService()

// Seed definitions + areas on boot
_ = widgets.Bootstrap(ctx, widgetSvc, widgets.BootstrapConfig{
    Definitions: []widgets.RegisterDefinitionInput{
        {
            Name: "promo_banner",
            Schema: map[string]any{
                "fields": []any{
                    map[string]any{"name": "headline"},
                    map[string]any{"name": "cta_text"},
                },
            },
        },
    },
    Areas: []widgets.RegisterAreaDefinitionInput{
        {Code: "sidebar.primary", Name: "Primary Sidebar"},
        {Code: "footer.global", Name: "Global Footer", Scope: widgets.AreaScopeGlobal},
    },
})

// Fetch placements
resolved, err := widgetSvc.ResolveArea(ctx, widgets.ResolveAreaInput{
    AreaCode: "sidebar.primary",
    LocaleID: &localeID,
    Audience: []string{"guest"},
    Now:      time.Now().UTC(),
})
```

Visibility rules mirror the service tests:

```json
{
  "visibility_rules": {
    "schedule": {"starts_at": "2024-01-01T00:00:00Z", "ends_at": "2024-02-01T00:00:00Z"},
    "audience": ["guest", "anonymous"],
    "segments": ["beta_tester"],
    "locales": ["00000000-0000-0000-0000-000000000201"]
  }
}
```

Widgets assigned to an area can also carry placement metadata (layout hints, theme variants, etc.) which the service preserves when returning `ResolvedWidget` results.

Area definitions accept `AreaScopeGlobal`, `AreaScopeTheme`, and `AreaScopeTemplate`, allowing themes to layer their own slots on top of the global defaults without affecting existing placements.

When implementing a custom registry, prefer factories so host applications can generate instance defaults:

```go
registry := widgets.NewRegistry()
registry.RegisterFactory("latest_posts", widgets.Registration{
    Definition: func() widgets.RegisterDefinitionInput {
        return widgets.RegisterDefinitionInput{
            Name: "latest_posts",
            Schema: map[string]any{"fields": []any{map[string]any{"name": "limit"}}},
            Defaults: map[string]any{"limit": 3},
        }
    },
    InstanceFactory: func(ctx context.Context, def *widgets.Definition, input widgets.CreateInstanceInput) (map[string]any, error) {
        return map[string]any{"limit": 5}, nil
    },
})

container := di.NewContainer(cfg, di.WithWidgetService(
    widgets.NewService(
        widgets.NewMemoryDefinitionRepository(),
        widgets.NewMemoryInstanceRepository(),
        widgets.NewMemoryTranslationRepository(),
        widgets.WithRegistry(registry),
    ),
))
```

### Theme Integration

// ThemeService returns the theme service (lazy init, singleton)
func (c *Container) ThemeService() themes.Service {
    if !c.config.Features.Themes {
        c.recordError("theme", fmt.Errorf("themes feature not enabled"))
        return themes.NewNoOpService()
    }

    if c.template == nil {
        c.recordError("theme", fmt.Errorf("themes feature requires template renderer"))
        return themes.NewNoOpService()
    }

    c.once.theme.Do(func() {
        repo := themes.NewRepository(c.storage)

        c.themeService = themes.NewService(
            repo,
            c.template,
            c.WidgetService(), // Dependency: widgets
            c.I18nService(),
            themes.Options{
                CacheTTL: c.config.CacheTTL,
            },
        )
    })

    return c.themeService
}

// Storage returns the storage provider
func (c *Container) Storage() interfaces.StorageProvider {
    return c.storage
}

// Close cleans up resources
func (c *Container) Close() error {
    var errs []error

    // Close services in reverse dependency order
    if c.themeService != nil {
        if closer, ok := c.themeService.(interface{ Close() error }); ok {
            if err := closer.Close(); err != nil {
                errs = append(errs, fmt.Errorf("theme service: %w", err))
            }
        }
    }

    // Close infrastructure
    if closer, ok := c.storage.(interface{ Close() error }); ok {
        if err := closer.Close(); err != nil {
            errs = append(errs, fmt.Errorf("storage: %w", err))
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("errors during cleanup: %v", errs)
    }

    return nil
}

// GetErrors returns all initialization errors
func (c *Container) GetErrors() map[string]error {
    c.mu.RLock()
    defer c.mu.RUnlock()

    errors := make(map[string]error)
    for k, v := range c.initErrors {
        errors[k] = v
    }
    return errors
}

// recordError stores initialization errors
func (c *Container) recordError(service string, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.initErrors[service] = err
}

// Helper functions (shared by the i18n wrapper + domain services)

func hasRegionalLocales(locales []cms.Locale) bool {
    for _, loc := range locales {
        if len(loc.Code) > 2 && loc.Code[2] == '-' {
            return true
        }
    }
    return false
}

func registerBuiltInBlocks(registry *blocks.Registry) {
    // Register core block types
    registry.Register("text", blocks.TextBlockType)
    registry.Register("image", blocks.ImageBlockType)
    registry.Register("video", blocks.VideoBlockType)
}

func registerBuiltInWidgets(registry *widgets.Registry) {
    // Register core widgets
    registry.Register("recent_posts", widgets.RecentPostsWidget)
    registry.Register("tag_cloud", widgets.TagCloudWidget)
}

type noOpCache struct{}

func newNoOpCache() interfaces.CacheProvider {
    return &noOpCache{}
}

func (n *noOpCache) Get(ctx context.Context, key string) (interface{}, error) {
    return nil, fmt.Errorf("not found")
}

func (n *noOpCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    return nil // No-op
}

func (n *noOpCache) Delete(ctx context.Context, key string) error {
    return nil // No-op
}

func (n *noOpCache) Clear(ctx context.Context) error {
    return nil // No-op
}
```

> The `internal/i18n` package shown above is intentionally lightweight: it builds a `go-i18n` configuration, wires CMS-specific fallbacks/loaders, and then exposes the resulting translator/culture services through our own interfaces. All locale resolution, formatting, and culture data rules live in the external `github.com/goliatone/go-i18n` module.

## Domain Module Examples

### Data Models (Bun ORM)

The Bun models below mirror the latest schema described in `CMS_ENTITIES.md`, including the new tables (`content_types`, `themes`, `templates`) and the additional audit/soft-delete fields introduced across the existing entities.

```go
// internal/models/models.go
package models

import (
    "time"

    "github.com/google/uuid"
    "github.com/uptrace/bun"
)

type ContentType struct {
    bun.BaseModel `bun:"table:content_types,alias:ct"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    Name          string         `bun:"name,notnull"`
    Description   *string        `bun:"description"`
    Schema        map[string]any `bun:"schema,type:jsonb,notnull"`
    Capabilities  map[string]any `bun:"capabilities,type:jsonb"`
    Icon          *string        `bun:"icon"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`
}

type Page struct {
    bun.BaseModel `bun:"table:pages,alias:p"`
    ID            uuid.UUID   `bun:",pk,type:uuid"`
    ParentID      *uuid.UUID  `bun:"parent_id"`
    Slug          string      `bun:"slug,notnull"`
    TemplateID    uuid.UUID   `bun:"template_id,notnull"`
    Status        string      `bun:"status,notnull"`
    PublishAt     *time.Time  `bun:"publish_at"`
    UnpublishAt   *time.Time  `bun:"unpublish_at"`
    CreatedBy     uuid.UUID   `bun:"created_by,notnull"`
    UpdatedBy     uuid.UUID   `bun:"updated_by,notnull"`
    DeletedAt     *time.Time  `bun:"deleted_at"`
    CreatedAt     time.Time   `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time   `bun:"updated_at,nullzero,default:now()"`

    Template     *Template        `bun:"rel:belongs-to,join:template_id=id"`
    Versions     []*PageVersion   `bun:"rel:has-many,join:id=page_id"`
    Translations []*PageTranslation `bun:"rel:has-many,join:id=page_id"`
    Blocks       []*BlockInstance `bun:"rel:has-many,join:id=page_id"`
}

type PageVersion struct {
    bun.BaseModel `bun:"table:page_versions,alias:pv"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    PageID        uuid.UUID      `bun:"page_id,notnull"`
    Version       int            `bun:"version,notnull"`
    Snapshot      map[string]any `bun:"snapshot,type:jsonb,notnull"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedBy     uuid.UUID      `bun:"created_by,notnull"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`

    Page *Page `bun:"rel:belongs-to,join:page_id=id"`
}

type PageTranslation struct {
    bun.BaseModel `bun:"table:page_translations,alias:pt"`
    ID            uuid.UUID  `bun:",pk,type:uuid"`
    PageID        uuid.UUID  `bun:"page_id,notnull"`
    LocaleID      uuid.UUID  `bun:"locale_id,notnull"`
    Title         string     `bun:"title,notnull"`
    Path          string     `bun:"path,notnull"`
    SEOTitle      *string    `bun:"seo_title"`
    SEODescription *string   `bun:"seo_description"`
    Summary       *string    `bun:"summary"`
    DeletedAt     *time.Time `bun:"deleted_at"`
    CreatedAt     time.Time  `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time  `bun:"updated_at,nullzero,default:now()"`

    Page   *Page   `bun:"rel:belongs-to,join:page_id=id"`
    Locale *Locale `bun:"rel:belongs-to,join:locale_id=id"`
}

type BlockDefinition struct {
    bun.BaseModel `bun:"table:block_definitions,alias:bd"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    Name          string         `bun:"name,notnull"`
    Description   *string        `bun:"description"`
    Icon          *string        `bun:"icon"`
    Schema        map[string]any `bun:"schema,type:jsonb,notnull"`
    Defaults      map[string]any `bun:"defaults,type:jsonb"`
    EditorStyleURL   *string     `bun:"editor_style_url"`
    FrontendStyleURL *string     `bun:"frontend_style_url"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`

    Instances []*BlockInstance `bun:"rel:has-many,join:id=definition_id"`
}

type BlockInstance struct {
    bun.BaseModel `bun:"table:block_instances,alias:bi"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    PageID        *uuid.UUID     `bun:"page_id"`
    Region        string         `bun:"region,notnull"`
    Position      int            `bun:"position,notnull"`
    DefinitionID  uuid.UUID      `bun:"definition_id,notnull"`
    Configuration map[string]any `bun:"configuration,type:jsonb,notnull"`
    IsGlobal      bool           `bun:"is_global,notnull"`
    CreatedBy     uuid.UUID      `bun:"created_by,notnull"`
    UpdatedBy     uuid.UUID      `bun:"updated_by,notnull"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Definition   *BlockDefinition     `bun:"rel:belongs-to,join:definition_id=id"`
    Page         *Page                `bun:"rel:belongs-to,join:page_id=id"`
    Translations []*BlockTranslation  `bun:"rel:has-many,join:id=block_instance_id"`
    Widgets      []*WidgetInstance    `bun:"rel:has-many,join:id=block_instance_id"`
}

type BlockTranslation struct {
    bun.BaseModel `bun:"table:block_translations,alias:bt"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    BlockInstanceID uuid.UUID    `bun:"block_instance_id,notnull"`
    LocaleID      uuid.UUID      `bun:"locale_id,notnull"`
    Content       map[string]any `bun:"content,type:jsonb,notnull"`
    AttributeOverrides map[string]any `bun:"attribute_overrides,type:jsonb"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Instance *BlockInstance `bun:"rel:belongs-to,join:block_instance_id=id"`
    Locale   *Locale        `bun:"rel:belongs-to,join:locale_id=id"`
}

type WidgetDefinition struct {
    bun.BaseModel `bun:"table:widget_definitions,alias:wd"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    Name          string         `bun:"name,notnull"`
    Description   *string        `bun:"description"`
    Schema        map[string]any `bun:"schema,type:jsonb,notnull"`
    Defaults      map[string]any `bun:"defaults,type:jsonb"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`

    Instances []*WidgetInstance `bun:"rel:has-many,join:id=definition_id"`
}

type WidgetInstance struct {
    bun.BaseModel `bun:"table:widget_instances,alias:wi"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    BlockInstanceID *uuid.UUID   `bun:"block_instance_id"`
    DefinitionID  uuid.UUID      `bun:"definition_id,notnull"`
    Configuration map[string]any `bun:"configuration,type:jsonb,notnull"`
    CreatedBy     uuid.UUID      `bun:"created_by,notnull"`
    UpdatedBy     uuid.UUID      `bun:"updated_by,notnull"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Definition   *WidgetDefinition    `bun:"rel:belongs-to,join:definition_id=id"`
    Block        *BlockInstance       `bun:"rel:belongs-to,join:block_instance_id=id"`
    Translations []*WidgetTranslation `bun:"rel:has-many,join:id=widget_instance_id"`
}

type WidgetTranslation struct {
    bun.BaseModel `bun:"table:widget_translations,alias:wt"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    WidgetInstanceID uuid.UUID   `bun:"widget_instance_id,notnull"`
    LocaleID      uuid.UUID      `bun:"locale_id,notnull"`
    Content       map[string]any `bun:"content,type:jsonb,notnull"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Instance *WidgetInstance `bun:"rel:belongs-to,join:widget_instance_id=id"`
    Locale   *Locale         `bun:"rel:belongs-to,join:locale_id=id"`
}

type Menu struct {
    bun.BaseModel `bun:"table:menus,alias:m"`
    ID            uuid.UUID   `bun:",pk,type:uuid"`
    Code          string      `bun:"code,notnull"`
    Description   *string     `bun:"description"`
    CreatedBy     uuid.UUID   `bun:"created_by,notnull"`
    UpdatedBy     uuid.UUID   `bun:"updated_by,notnull"`
    CreatedAt     time.Time   `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time   `bun:"updated_at,nullzero,default:now()"`

    Items []*MenuItem `bun:"rel:has-many,join:id=menu_id"`
}

type MenuItem struct {
    bun.BaseModel `bun:"table:menu_items,alias:mi"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    MenuID        uuid.UUID      `bun:"menu_id,notnull"`
    ParentID      *uuid.UUID     `bun:"parent_id"`
    Position      int            `bun:"position,notnull"`
    Target        map[string]any `bun:"target,type:jsonb,notnull"`
    CreatedBy     uuid.UUID      `bun:"created_by,notnull"`
    UpdatedBy     uuid.UUID      `bun:"updated_by,notnull"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Menu         *Menu                `bun:"rel:belongs-to,join:menu_id=id"`
    Parent       *MenuItem            `bun:"rel:belongs-to,join:parent_id=id"`
    Children     []*MenuItem          `bun:"rel:has-many,join:id=parent_id"`
    Translations []*MenuItemTranslation `bun:"rel:has-many,join:id=menu_item_id"`
}

type MenuItemTranslation struct {
    bun.BaseModel `bun:"table:menu_item_translations,alias:mit"`
    ID            uuid.UUID  `bun:",pk,type:uuid"`
    MenuItemID    uuid.UUID  `bun:"menu_item_id,notnull"`
    LocaleID      uuid.UUID  `bun:"locale_id,notnull"`
    Label         string     `bun:"label,notnull"`
    URLOverride   *string    `bun:"url_override"`
    DeletedAt     *time.Time `bun:"deleted_at"`
    CreatedAt     time.Time  `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time  `bun:"updated_at,nullzero,default:now()"`

    Item   *MenuItem `bun:"rel:belongs-to,join:menu_item_id=id"`
    Locale *Locale   `bun:"rel:belongs-to,join:locale_id=id"`
}

type Theme struct {
    bun.BaseModel `bun:"table:themes,alias:th"`
    ID            uuid.UUID   `bun:",pk,type:uuid"`
    Name          string      `bun:"name,notnull"`
    Description   *string     `bun:"description"`
    Version       string      `bun:"version,notnull"`
    Author        *string     `bun:"author"`
    IsActive      bool        `bun:"is_active"`
    ThemePath     string      `bun:"theme_path,notnull"`
    Config        map[string]any `bun:"config,type:jsonb"`
    CreatedAt     time.Time   `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time   `bun:"updated_at,nullzero,default:now()"`

    Templates []*Template `bun:"rel:has-many,join:id=theme_id"`
}

type Template struct {
    bun.BaseModel `bun:"table:templates,alias:tp"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    ThemeID       uuid.UUID      `bun:"theme_id,notnull"`
    Name          string         `bun:"name,notnull"`
    Slug          string         `bun:"slug,notnull"`
    Description   *string        `bun:"description"`
    TemplatePath  string         `bun:"template_path,notnull"`
    Regions       map[string]any `bun:"regions,type:jsonb,notnull"`
    Metadata      map[string]any `bun:"metadata,type:jsonb"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
    UpdatedAt     time.Time      `bun:"updated_at,nullzero,default:now()"`

    Theme *Theme `bun:"rel:belongs-to,join:theme_id=id"`
}

type TranslationStatus struct {
    bun.BaseModel `bun:"table:translation_status,alias:ts"`
    ID            uuid.UUID  `bun:",pk,type:uuid"`
    EntityType    string     `bun:"entity_type,notnull"`
    EntityID      uuid.UUID  `bun:"entity_id,notnull"`
    LocaleID      uuid.UUID  `bun:"locale_id,notnull"`
    Status        string     `bun:"status,notnull"`
    Completeness  int        `bun:"completeness,notnull"`
    LastUpdated   time.Time  `bun:"last_updated,nullzero,default:now()"`
    TranslatorID  *uuid.UUID `bun:"translator_id"`
    ReviewerID    *uuid.UUID `bun:"reviewer_id"`

    Locale *Locale `bun:"rel:belongs-to,join:locale_id=id"`
}

type Locale struct {
    bun.BaseModel `bun:"table:locales,alias:l"`
    ID            uuid.UUID      `bun:",pk,type:uuid"`
    Code          string         `bun:"code,notnull"`
    DisplayName   string         `bun:"display_name,notnull"`
    NativeName    *string        `bun:"native_name"`
    IsActive      bool           `bun:"is_active"`
    IsDefault     bool           `bun:"is_default"`
    Metadata      map[string]any `bun:"metadata,type:jsonb"`
    DeletedAt     *time.Time     `bun:"deleted_at"`
    CreatedAt     time.Time      `bun:"created_at,nullzero,default:now()"`
}
```

These models provide a one-to-one mapping with the SQL DDL in `CMS_ENTITIES.md`, ensuring the Go layer captures newly added audit fields (`created_by`, `updated_by`, `deleted_at`), JSON-backed configuration columns, and the new top-level tables for content types, themes, and templates.

## Testing Infrastructure

```go
// pkg/testsupport/fixtures.go
func LoadFixture(t *testing.T, path string) []byte
func LoadGolden(t *testing.T, path string, v any) error
func WriteGolden(t *testing.T, path string, data any) error

// pkg/testsupport/dbtest.go
func NewMemoryStorage(t *testing.T) interfaces.StorageProvider
func NewPostgresForTest(t *testing.T) interfaces.StorageProvider
```

**Contract Test Pattern**:

```go
// internal/content/service_test.go
func TestContentService_Create_BasicContent(t *testing.T) {
    input := testsupport.LoadFixture(t, "testdata/basic_content.json")
    storage := testdb.NewMemoryStorage(t)
    svc := content.NewService(storage, nil, nil, content.Options{})

    got, err := svc.Create(context.Background(), input)
    if err != nil {
        t.Fatalf("Create: %v", err)
    }

    want := testsupport.LoadGolden(t, "testdata/basic_content_output.json")
    if diff := cmp.Diff(want, got); diff != "" {
        t.Fatalf("mismatch (-want +got):\n%s", diff)
    }
}
```

### Local Test Command

Run the contract and integration suites locally (uses a repo-local build cache to satisfy sandbox restrictions):

```bash
GOCACHE=$(pwd)/.gocache /Users/goliatone/.g/go/bin/go test \
  ./internal/content \
  ./internal/pages \
  ./internal/blocks \
  ./internal/themes \
  ./internal/menus \
  ./internal/widgets
```

When configuring CI, mirror the same command (or ensure your existing `./...` invocation includes the widgets and themes packages) so widget contracts, Bun/cache integrations, and visibility evaluation helpers run beside the established content, pages, blocks, themes, and menus suites.

### Blocks Integration

Phase 2 adds the Blocks module on top of the existing structure:

- `internal/blocks` contains definitions, instances, translations, and the block service.
- The DI container wires block repositories alongside content/pages. Calling
  `di.NewContainer` now exposes `BlockService` by default (memory-backed) and upgrades
  to Bun+cache when `di.WithBunDB` is provided.
- The page service automatically enriches `Get`/`List` results with block instances when a
  block service is configured, keeping consumers unaware of storage details.
- The example CLI seeds a block definition and instance to demonstrate rendering.

### Bun Storage & Repository Cache

Phase 1 hardening introduced production-ready storage and cache wiring:

- `di.WithBunDB(*bun.DB)` switches the container from the in-memory scaffolding
  to the Bun repositories (`content.NewBunContentRepository`,
  `pages.NewBunPageRepository`, etc.).
- Repository caching is implemented via
  `github.com/goliatone/go-repository-cache`. When `cms.Config.Cache.Enabled`
  is true the container creates a default cache service; callers can override
  it with `di.WithCache(cacheService, keySerializer)`.
- TTL defaults to one minute (`Cache.DefaultTTL`) and flows directly into the
  cache.Config passed to the library.

```go
sqlDB, _ := sql.Open("sqlite3", "file::memory:?cache=shared")
bunDB := bun.NewDB(sqlDB, sqlitedialect.New())

cfg := cms.DefaultConfig()
cfg.Cache.DefaultTTL = 5 * time.Minute

cacheCfg := cache.DefaultConfig()
cacheCfg.TTL = cfg.Cache.DefaultTTL
cacheService, _ := cache.NewCacheService(cacheCfg)
keySerializer := cache.NewDefaultKeySerializer()

container := di.NewContainer(
    cfg,
    di.WithBunDB(bunDB),
    di.WithCache(cacheService, keySerializer),
)

contentSvc := container.ContentService() // Bun + cache aware
pageSvc := container.PageService()
```

## Usage Examples

### Sprint 1: Example CLI

A runnable demonstration lives in `.tmp/cms/cmd/example`. It uses the in-memory repositories provided by the DI container, seeds a locale-aware content type, and creates a localized page. Run it from the staged module root with the helper script:

```bash
cd .tmp/cms
./scripts/run_example.sh
```

Sample output:

```json
{
  "content_id": "<uuid>",
  "page_id": "<uuid>",
  "page": {
    "id": "<uuid>",
    "slug": "company-overview",
    "status": "published",
    "contentID": "<uuid>",
    "template": "22222222-2222-2222-2222-222222222222",
    "translations": [
      {"locale": "en", "title": "Company Overview", "path": "/company"},
      {"locale": "es", "title": "Resumen corporativo", "path": "/es/empresa"}
    ]
  },
  "pages": [
    {"id": "<uuid>", "slug": "company-overview", "status": "published"}
  ]
}
```

The UUID values are generated at runtime, so expect different identifiers on each execution.

The script pins `GOCACHE` to `.tmp/cms/.gocache` so it remains sandbox-friendly and forwards additional arguments to `go run` if you need to experiment (for example `./scripts/run_example.sh -v` once flags are added).

### Future Sprints

Example applications for Blocks, Menus, and Widgets will be added as those vertical slices promote from `.tmp/` to production packages. The CLI is structured so additional subcommands can be layered on without changing existing behaviour.

### Sprint 6: Full Features

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/goliatone/cms"
    "github.com/goliatone/cms/adapters/storage/postgres" // wraps go-persistence-bun/go-repository-bun
    "github.com/goliatone/cms/adapters/cache/redis" // built on go-repository-cache decorators
    "github.com/goliatone/cms/adapters/template/handlebars"
    "github.com/goliatone/cms/adapters/media/s3"
    "github.com/goliatone/cms/adapters/auth/jwt"
)

func main() {
    storage, _ := postgres.New("postgres://localhost/cms")
    defer storage.Close()

    cache, _ := redis.New("redis://localhost:6379")
    defer cache.Close()

    template, _ := handlebars.New(handlebars.Config{
        TemplateDir: "./themes",
    })

    media, _ := s3.New(s3.Config{
        Bucket: "my-cms-media",
        Region: "us-east-1",
    })

    auth, _ := jwt.New(jwt.Config{
        Secret: "my-secret",
    })

    // Full feature set
    app, err := cms.New(cms.Config{
        Storage:  storage,
        Cache:    cache,
        Template: template,
        Media:    media,
        Auth:     auth,

        DefaultLocale: "en",
        Locales: []cms.Locale{
            {Code: "en-US", Name: "English (US)", IsDefault: true},
            {Code: "en-GB", Name: "English (UK)", FallbackLocaleID: "en-US"},
            {Code: "es-ES", Name: "Spanish (Spain)"},
            {Code: "es-MX", Name: "Spanish (Mexico)", FallbackLocaleID: "es-ES"},
        },

        Features: cms.Features{
            BasicContent:      true,
            BasicPages:        true,
            Blocks:            true,
            NestedBlocks:      true,
            Menus:             true,
            HierarchicalMenus: true,
            Widgets:           true,
            Themes:            true,
            Templates:         true,
            Versioning:        true,
            Scheduling:        true,
            MediaLibrary:      true,
            AdvancedCache:     true,
        },

        CacheTTL:      10 * time.Minute,
        MaxPageDepth:  10,
        MaxBlockDepth: 15,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer app.Close()

    ctx := context.Background()

    // All services available
    content := app.Content()
    pages := app.Pages()
    blocks := app.Blocks()
    menus := app.Menus()
    widgets := app.Widgets()
    themes := app.Themes()

    // Use advanced features
    scheduled, err := content.Schedule(ctx, "content-123", time.Now().Add(24*time.Hour))
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Scheduled content for: %s", scheduled.PublishAt)

    // Health check
    if err := app.HealthCheck(ctx); err != nil {
        log.Printf("Health check failed: %v", err)
    } else {
        log.Println("CMS is healthy")
    }
}
```

## Command Failure Triage & Rollback

- **Identify the failure**: Command handlers emit structured logs containing the `command` message type, high-level `operation`, and domain identifiers (for example, `content_id`, `page_id`). Filter on these fields to isolate failing requests and determine whether they are safe to retry.
- **Handle retries and timeouts**: Direct commands honour context deadlines and the per-command timeout options (for example, `staticcmd.BuildSiteWithTimeout`). Wrap `Execute` with your scheduler's retry/backoff policy or use the optional adapter module when you need go-command runners to manage retries centrally.
- **Requeue idempotent work**: Content/page publish/schedule commands and audit cleanup are idempotent—rerunning them after remediation will converge state. For non-idempotent flows (for example, asset imports with side effects), confirm downstream state before replaying.
- **Rollback strategy**: Pause cron/dispatcher triggers by removing your scheduling hooks (or by dropping the optional adapter module), deploy the fix, and use direct command execution or the underlying services until the issue is resolved. Once healthy, reinstate the scheduling glue and rerun your CLI/cron smoke tests before redeploying.

## Progressive Complexity Reference

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

**Upgrade Path**: Each level of i18n complexity is enabled by providing additional configuration, without requiring modifications to existing application code or database schema. Behind the scenes the wrapper simply feeds these options into `github.com/goliatone/go-i18n`; increasing complexity means enabling more of the external package's capabilities while keeping the CMS-facing API unchanged.
