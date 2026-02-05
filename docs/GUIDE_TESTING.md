# Testing Guide

This guide covers testing strategies, conventions, and utilities for projects built with `go-cms`. By the end you will understand how to write unit tests with memory repositories, contract tests with JSON fixtures, integration tests with SQLite, activity emission assertions, and migration verification tests.

## Testing Architecture Overview

`go-cms` organises tests into four categories, each with a distinct file naming convention:

```
*_test.go                          Unit tests (memory repositories, no DB)
*_contract_test.go                 Contract tests (JSON fixtures, interface compliance)
*_integration_test.go              Integration tests (BunDB + real database)
*_storage_integration_test.go      Storage-layer tests (profile switching, caching)
```

The architecture follows a layered strategy: start with fast unit tests using in-memory storage, add contract tests to verify both memory and Bun implementations satisfy the same interface, then integration tests to exercise real SQL queries.

```
Unit tests (memory repos)           ~ms   No external dependencies
  └── Contract tests (fixtures)     ~ms   Verify both implementations
        └── Integration tests       ~s    Real SQLite, BunDB
              └── Storage tests     ~s    Profile switching, cache layer
```

### Test Utilities

Two packages provide shared test infrastructure:

| Package | Purpose |
|---------|---------|
| `pkg/testsupport` | Database setup, fixture loading, golden file comparison |
| `internal/di/testing` | Generator-specific helpers, in-memory storage adapter |

---

## Test Types and Naming Conventions

### Unit Tests (`*_test.go`)

Unit tests use memory repositories and test service-layer logic directly. They are the fastest tests and require no database or external dependencies.

**Characteristics:**
- Construct services with `New*MemoryRepository()` functions
- Use deterministic clocks via `WithClock(func() time.Time { ... })`
- Test business rules: validation, error paths, side effects
- Run in milliseconds

**Example:** Testing content creation with memory repositories:

```go
package content_test

import (
    "context"
    "testing"
    "time"

    "github.com/goliatone/go-cms/internal/content"
    "github.com/google/uuid"
)

func TestServiceCreateSuccess(t *testing.T) {
    // 1. Set up memory repositories
    contentStore := content.NewMemoryContentRepository()
    typeStore := content.NewMemoryContentTypeRepository()
    localeStore := content.NewMemoryLocaleRepository()

    // 2. Seed required entities
    contentTypeID := uuid.New()
    seedContentType(t, typeStore, &content.ContentType{
        ID:     contentTypeID,
        Name:   "page",
        Schema: map[string]any{"fields": []any{"body"}},
    })

    enID := uuid.New()
    localeStore.Put(&content.Locale{
        ID:      enID,
        Code:    "en",
        Display: "English",
    })

    // 3. Create service with deterministic clock
    svc := content.NewService(contentStore, typeStore, localeStore,
        content.WithClock(func() time.Time {
            return time.Unix(0, 0)
        }),
    )

    // 4. Execute and assert
    result, err := svc.Create(context.Background(), content.CreateContentRequest{
        ContentTypeID: contentTypeID,
        Slug:          "about-us",
        Status:        "draft",
        CreatedBy:     uuid.New(),
        UpdatedBy:     uuid.New(),
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "About us", Content: map[string]any{"body": "Welcome"}},
        },
    })
    if err != nil {
        t.Fatalf("create: %v", err)
    }

    if result.Slug != "about-us" {
        t.Fatalf("expected slug %q got %q", "about-us", result.Slug)
    }
    if len(result.Translations) != 1 {
        t.Fatalf("expected 1 translation got %d", len(result.Translations))
    }
}
```

**Common unit test patterns across modules:**

| Module | Service Constructor | Key Options |
|--------|-------------------|-------------|
| `content` | `content.NewService(contentRepo, typeRepo, localeRepo, ...opts)` | `WithClock`, `WithActivityEmitter`, `WithRequireTranslations` |
| `pages` (legacy) | `pages.NewService(pageRepo, contentRepo, localeRepo, ...opts)` | `WithPageClock`, `WithBlockService`, `WithWorkflowEngine`, `WithRequireTranslations` |
| `blocks` | `blocks.NewService(defRepo, instRepo, trRepo, ...opts)` | `WithClock`, `WithIDGenerator`, `WithVersioningEnabled`, `WithMediaService` |
| `widgets` | `widgets.NewService(defRepo, instRepo, trRepo, areaDefRepo, placementRepo, ...opts)` | `WithWidgetClock`, `WithRequireTranslations` |
| `menus` | `menus.NewService(menuRepo, itemRepo, trRepo, localeRepo, ...opts)` | `WithURLResolver` |

For new page/post behavior, test content entries with entry `Metadata` (`path`, `template_id`, `parent_id`, `sort_order`) rather than the legacy pages service.

### Error Path Testing

Services expose typed errors that tests can match with `errors.Is` or `errors.As`:

```go
func TestServiceCreateRejectsInvalidSchemaPayload(t *testing.T) {
    // ... seed a content type with schema: {"fields": [{"name": "body"}]}
    svc := content.NewService(contentStore, typeStore, localeStore)

    _, err := svc.Create(context.Background(), content.CreateContentRequest{
        ContentTypeID: contentTypeID,
        Slug:          "bad-payload",
        CreatedBy:     uuid.New(),
        UpdatedBy:     uuid.New(),
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "Bad", Content: map[string]any{"body": "ok", "extra": "no"}},
        },
    })
    if !errors.Is(err, content.ErrContentSchemaInvalid) {
        t.Fatalf("expected ErrContentSchemaInvalid got %v", err)
    }
}
```

For not-found errors that carry resource metadata, use `errors.As`:

```go
_, err = pageSvc.Get(context.Background(), page.ID)
var notFound *pages.PageNotFoundError
if !errors.As(err, &notFound) {
    t.Fatalf("expected not found error got %v", err)
}
```

---

### Contract Tests (`*_contract_test.go`)

Contract tests verify that both memory and Bun repository implementations satisfy the same interface and produce identical results. They use JSON fixture files to define seed data and expected outcomes.

**Characteristics:**
- Load fixtures from `testdata/` via `testsupport.LoadFixture`
- Seed repositories from fixture data
- Execute operations through the service layer
- Project results into a simplified "expectation view" struct
- Compare with `reflect.DeepEqual`

**Example:** Content service contract test:

```go
package content_test

import (
    "context"
    "encoding/json"
    "reflect"
    "sort"
    "testing"
    "time"

    "github.com/goliatone/go-cms/internal/content"
    "github.com/goliatone/go-cms/pkg/testsupport"
    "github.com/google/uuid"
)

// Fixture struct mirrors the JSON file structure
type contentContractFixture struct {
    Locales []struct {
        ID      string `json:"id"`
        Code    string `json:"code"`
        Display string `json:"display"`
    } `json:"locales"`
    ContentTypes []struct {
        ID     string         `json:"id"`
        Name   string         `json:"name"`
        Slug   string         `json:"slug"`
        Schema map[string]any `json:"schema"`
    } `json:"content_types"`
    Request struct {
        ContentTypeID string `json:"content_type_id"`
        Slug          string `json:"slug"`
        Status        string `json:"status"`
        CreatedBy     string `json:"created_by"`
        UpdatedBy     string `json:"updated_by"`
        Translations  []struct {
            Locale  string         `json:"locale"`
            Title   string         `json:"title"`
            Summary string         `json:"summary"`
            Content map[string]any `json:"content"`
        } `json:"translations"`
    } `json:"request"`
    Expectation struct {
        Slug    string            `json:"slug"`
        Status  string            `json:"status"`
        Titles  map[string]string `json:"titles"`
        Locales []string          `json:"locales"`
    } `json:"expectation"`
}

// Simplified view for comparison
type contentExpectationView struct {
    Slug    string
    Status  string
    Locales []string
    Titles  map[string]string
}

func TestContentServiceContract_Phase1Fixture(t *testing.T) {
    fixture := loadContentContractFixture(t, "testdata/phase1_contract.json")

    // Seed repositories from fixture data
    contentRepo := content.NewMemoryContentRepository()
    typeRepo := content.NewMemoryContentTypeRepository()
    localeRepo := content.NewMemoryLocaleRepository()

    localeIndex := make(map[uuid.UUID]string)
    for _, loc := range fixture.Locales {
        id := mustParseUUID(t, loc.ID)
        localeRepo.Put(&content.Locale{ID: id, Code: loc.Code, Display: loc.Display})
        localeIndex[id] = loc.Code
    }

    // ... seed content types from fixture ...

    svc := content.NewService(contentRepo, typeRepo, localeRepo,
        content.WithClock(func() time.Time { return time.Unix(0, 0) }),
    )

    // Build request from fixture, execute, and compare
    result, err := svc.Create(context.Background(), req)
    if err != nil {
        t.Fatalf("Create: %v", err)
    }

    got := projectContent(result, localeIndex) // project to expectation view
    want := contentExpectationView{
        Slug:    fixture.Expectation.Slug,
        Status:  fixture.Expectation.Status,
        Locales: fixture.Expectation.Locales,
        Titles:  fixture.Expectation.Titles,
    }
    sort.Strings(want.Locales)

    if !reflect.DeepEqual(want, got) {
        t.Fatalf("contract mismatch\nwant: %#v\ngot:  %#v", want, got)
    }
}
```

The key pattern is projecting domain objects into simplified views that can be compared against fixture expectations. This decouples the test from internal struct details like auto-generated IDs and timestamps.

---

### Integration Tests (`*_integration_test.go`)

Integration tests use BunDB with a real SQLite database to exercise SQL queries, model registration, and transaction boundaries.

**Characteristics:**
- Use `testsupport.NewSQLiteMemoryDB()` for fast in-memory SQLite
- Register Bun models with `db.NewCreateTable().Model(...).IfNotExists().Exec(ctx)`
- Seed data via `db.NewInsert().Model(...).Exec(ctx)`
- Clean up with `t.Cleanup()`

**Example:** Storage integration test with Bun and caching:

```go
package content_test

import (
    "context"
    "testing"
    "time"

    "github.com/goliatone/go-cms/internal/content"
    "github.com/goliatone/go-cms/pkg/testsupport"
    repocache "github.com/goliatone/go-repository-cache/cache"
    "github.com/google/uuid"
    "github.com/uptrace/bun"
    "github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestContentService_WithBunStorageAndCache(t *testing.T) {
    ctx := context.Background()

    // 1. Create in-memory SQLite database
    sqlDB, err := testsupport.NewSQLiteMemoryDB()
    if err != nil {
        t.Fatalf("new sqlite db: %v", err)
    }
    t.Cleanup(func() { _ = sqlDB.Close() })

    // 2. Wrap with BunDB
    bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
    bunDB.SetMaxOpenConns(1)

    // 3. Create tables from Bun models
    registerContentModels(t, bunDB)
    seedContentEntities(t, bunDB)

    // 4. Set up caching layer
    cacheCfg := repocache.DefaultConfig()
    cacheCfg.TTL = time.Minute
    cacheService, _ := repocache.NewCacheService(cacheCfg)
    keySerializer := repocache.NewDefaultKeySerializer()

    // 5. Create cached Bun repositories
    contentRepo := content.NewBunContentRepositoryWithCache(bunDB, cacheService, keySerializer)
    contentTypeRepo := content.NewBunContentTypeRepositoryWithCache(bunDB, cacheService, keySerializer)
    localeRepo := content.NewBunLocaleRepositoryWithCache(bunDB, cacheService, keySerializer)

    svc := content.NewService(contentRepo, contentTypeRepo, localeRepo)

    // 6. Test create and cached reads
    created, err := svc.Create(ctx, content.CreateContentRequest{
        ContentTypeID: mustUUID("00000000-0000-0000-0000-000000000210"),
        Slug:          "company-overview",
        Status:        "published",
        CreatedBy:     mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
        UpdatedBy:     mustUUID("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "Company Overview", Content: map[string]any{"body": "Welcome"}},
        },
    })
    if err != nil {
        t.Fatalf("create content: %v", err)
    }

    // First read hits the database
    if _, err := svc.Get(ctx, created.ID); err != nil {
        t.Fatalf("first get: %v", err)
    }

    // Second read should hit the cache
    if _, err := svc.Get(ctx, created.ID); err != nil {
        t.Fatalf("cached get: %v", err)
    }
}
```

**Helper functions for integration tests:**

```go
// registerContentModels creates tables from Bun model structs
func registerContentModels(t *testing.T, db *bun.DB) {
    t.Helper()
    ctx := context.Background()
    models := []any{
        (*content.Locale)(nil),
        (*content.ContentType)(nil),
        (*content.Content)(nil),
        (*content.ContentTranslation)(nil),
    }
    for _, model := range models {
        if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
            t.Fatalf("create table %T: %v", model, err)
        }
    }
}

// seedContentEntities inserts required reference data
func seedContentEntities(t *testing.T, db *bun.DB) {
    t.Helper()
    ctx := context.Background()

    locale := &content.Locale{
        ID: mustUUID("00000000-0000-0000-0000-000000000201"),
        Code: "en", Display: "English", IsActive: true, IsDefault: true,
    }
    if _, err := db.NewInsert().Model(locale).Exec(ctx); err != nil {
        t.Fatalf("insert locale: %v", err)
    }

    ct := &content.ContentType{
        ID: mustUUID("00000000-0000-0000-0000-000000000210"),
        Name: "page", Slug: "page",
        Schema: map[string]any{"fields": []map[string]any{{"name": "body", "type": "richtext"}}},
    }
    if _, err := db.NewInsert().Model(ct).Exec(ctx); err != nil {
        t.Fatalf("insert content type: %v", err)
    }
}
```

---

### Storage Integration Tests (`*_storage_integration_test.go`)

Storage integration tests verify repository behaviour with different storage backends, including cache wrappers and profile switching. They follow the same BunDB + SQLite setup pattern as integration tests but focus on storage-specific concerns.

---

## Test Utilities

### `pkg/testsupport`

The `testsupport` package provides shared helpers for test setup:

**`NewSQLiteMemoryDB()`** -- creates an in-memory SQLite database for integration tests:

```go
func NewSQLiteMemoryDB() (*sql.DB, error) {
    return sql.Open("sqlite3", "file::memory:?cache=shared")
}
```

**`LoadFixture(path)`** -- reads raw bytes from a fixture file for contract tests:

```go
func LoadFixture(path string) ([]byte, error) {
    return os.ReadFile(path)
}
```

**`LoadGolden(path, v)`** -- reads and unmarshals a JSON golden file into a target struct:

```go
func LoadGolden(path string, v any) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, v)
}
```

### `internal/di/testing`

The `ditesting` package provides a `MemoryStorage` adapter that records all SQL interactions for assertions in generator tests:

```go
storage := ditesting.NewMemoryStorage()

// ... run generator operations against storage ...

// Assert recorded interactions
execCalls := storage.ExecCalls()
if len(execCalls) != 1 {
    t.Fatalf("expected 1 exec call, got %d", len(execCalls))
}
if !strings.Contains(execCalls[0].Query, "INSERT") {
    t.Fatalf("expected INSERT query, got %s", execCalls[0].Query)
}

// Check transaction awareness
queryCalls := storage.QueryCalls()
for _, call := range queryCalls {
    if call.InTransaction {
        // query was executed inside a transaction
    }
}
```

---

## Testing Activity Emission

Services that emit activity events can be tested using `activity.CaptureHook` and `activity.NewEmitter`. The capture hook collects all emitted events in memory for assertion.

**Pattern:**

```go
import "github.com/goliatone/go-cms/pkg/activity"

func TestServiceCreateEmitsActivityEvent(t *testing.T) {
    contentStore := content.NewMemoryContentRepository()
    typeStore := content.NewMemoryContentTypeRepository()
    localeStore := content.NewMemoryLocaleRepository()

    // Seed required entities
    contentTypeID := uuid.New()
    seedContentType(t, typeStore, &content.ContentType{ID: contentTypeID, Name: "page"})
    localeStore.Put(&content.Locale{ID: uuid.New(), Code: "en", Display: "English"})

    // Create capture hook and emitter
    hook := &activity.CaptureHook{}
    emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{
        Enabled: true,
        Channel: "cms",
    })

    actorID := uuid.New()
    svc := content.NewService(
        contentStore, typeStore, localeStore,
        content.WithActivityEmitter(emitter),
    )

    // Perform the operation
    record, err := svc.Create(context.Background(), content.CreateContentRequest{
        ContentTypeID: contentTypeID,
        Slug:          "activity-hooks",
        Status:        "draft",
        CreatedBy:     actorID,
        UpdatedBy:     actorID,
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "Hello"},
        },
    })
    if err != nil {
        t.Fatalf("create content: %v", err)
    }

    // Assert captured events
    if len(hook.Events) != 1 {
        t.Fatalf("expected 1 activity event, got %d", len(hook.Events))
    }

    event := hook.Events[0]
    if event.Verb != "create" {
        t.Fatalf("expected verb create got %q", event.Verb)
    }
    if event.ActorID != actorID.String() {
        t.Fatalf("expected actor %s got %s", actorID, event.ActorID)
    }
    if event.ObjectType != "content" || event.ObjectID != record.ID.String() {
        t.Fatalf("unexpected object fields: %s %s", event.ObjectType, event.ObjectID)
    }
    if event.Channel != "cms" {
        t.Fatalf("expected channel cms got %q", event.Channel)
    }
    if slug, ok := event.Metadata["slug"].(string); !ok || slug != "activity-hooks" {
        t.Fatalf("expected slug metadata got %v", event.Metadata["slug"])
    }
}
```

**Key `activity.Event` fields for assertions:**

| Field | Type | Description |
|-------|------|-------------|
| `Verb` | `string` | Operation: `"create"`, `"update"`, `"delete"`, `"publish"` |
| `ActorID` | `string` | UUID of the user who triggered the action |
| `ObjectType` | `string` | Entity type: `"content"`, `"page"`, `"block"`, `"menu"` |
| `ObjectID` | `string` | UUID of the affected entity |
| `Channel` | `string` | Channel tag from `activity.Config` |
| `OccurredAt` | `time.Time` | Timestamp of the event |
| `Metadata` | `map[string]any` | Module-specific metadata (slug, status, locales, etc.) |

To verify that failed operations do not emit events:

```go
func TestServiceCreateSkipsActivityOnError(t *testing.T) {
    hook := &activity.CaptureHook{}
    emitter := activity.NewEmitter(activity.Hooks{hook}, activity.Config{Enabled: true})

    svc := content.NewService(
        content.NewMemoryContentRepository(),
        content.NewMemoryContentTypeRepository(),
        content.NewMemoryLocaleRepository(),
        content.WithActivityEmitter(emitter),
    )

    // Create with invalid request (missing content type)
    _, _ = svc.Create(context.Background(), content.CreateContentRequest{
        Slug:      "should-fail",
        CreatedBy: uuid.New(),
        UpdatedBy: uuid.New(),
    })

    if len(hook.Events) != 0 {
        t.Fatalf("expected no events on error, got %d", len(hook.Events))
    }
}
```

---

## Testing Migrations

Migration tests verify that SQL migrations apply correctly and produce the expected schema changes. They use the embedded migration filesystem and in-memory SQLite.

**Pattern:**

```go
package content_test

import (
    "database/sql"
    "io/fs"
    "strings"
    "testing"

    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/pkg/testsupport"
    "github.com/google/uuid"
)

func TestContentTypeSlugMigrationBackfill(t *testing.T) {
    // 1. Open in-memory SQLite
    db, err := testsupport.NewSQLiteMemoryDB()
    if err != nil {
        t.Fatalf("open sqlite db: %v", err)
    }
    defer db.Close()

    // 2. Apply the initial schema migration
    applyMigrationFile(t, db, "20250102000000_initial_schema.up.sql")

    // 3. Insert rows without the new column
    schema := `{"fields":[]}`
    id1, id2 := uuid.NewString(), uuid.NewString()
    db.Exec(`INSERT INTO content_types (id, name, schema) VALUES (?, ?, ?)`, id1, "Landing Page", schema)
    db.Exec(`INSERT INTO content_types (id, name, schema) VALUES (?, ?, ?)`, id2, "Landing Page", schema)

    // 4. Apply the migration under test
    applyMigrationFile(t, db, "20260126000000_content_type_slug.up.sql")

    // 5. Verify backfill results
    slug1 := fetchContentTypeSlug(t, db, id1)
    slug2 := fetchContentTypeSlug(t, db, id2)

    if !strings.HasPrefix(slug1, "landing-page") {
        t.Fatalf("expected slug1 to start with landing-page, got %q", slug1)
    }
    if slug1 == slug2 {
        t.Fatalf("expected unique slugs after backfill, got %q", slug1)
    }
}
```

**The `applyMigrationFile` helper:**

```go
func applyMigrationFile(t *testing.T, db *sql.DB, name string) {
    t.Helper()
    // Try SQLite-specific path first, fall back to generic
    paths := []string{
        "data/sql/migrations/sqlite/" + name,
        "data/sql/migrations/" + name,
    }
    var raw []byte
    var err error
    for _, path := range paths {
        raw, err = fs.ReadFile(cms.GetMigrationsFS(), path)
        if err == nil {
            break
        }
    }
    if err != nil {
        t.Fatalf("read migration %s: %v", name, err)
    }

    content := string(raw)
    // SQLite doesn't understand Postgres JSONB casts
    content = strings.ReplaceAll(content, "::jsonb", "")
    content = strings.ReplaceAll(content, "::JSONB", "")

    // Execute each statement separated by ---bun:split
    for _, chunk := range strings.Split(content, "---bun:split") {
        statement := strings.TrimSpace(chunk)
        if statement == "" {
            continue
        }
        if _, err := db.Exec(statement); err != nil {
            t.Fatalf("exec migration %s: %v", name, err)
        }
    }
}
```

This pattern allows sequential application of migrations, which is useful for testing backfill logic that depends on data inserted before a migration runs.

---

## Testing Service Options and Feature Flags

Many services accept functional options that control behaviour. Tests should exercise these options explicitly.

**Versioning:**

```go
svc := blocks.NewService(defRepo, instRepo, trRepo,
    blocks.WithVersioningEnabled(true),
    blocks.WithVersionRetentionLimit(5),
)
```

**Translation flexibility:**

```go
// Global opt-out
svc := content.NewService(contentRepo, typeRepo, localeRepo,
    content.WithRequireTranslations(false),
)

// Per-request override (global requirement stays on)
_, err := svc.Create(ctx, content.CreateContentRequest{
    // ...
    AllowMissingTranslations: true,
})
```

**Workflow engine injection:**

```go
fake := &fakeWorkflowEngine{
    states: []interfaces.WorkflowState{"review", "approved", "published"},
}
pageSvc := pages.NewService(pageRepo, contentRepo, localeRepo,
    pages.WithWorkflowEngine(fake),
)
```

**Deterministic ID generation (for predictable test assertions):**

```go
counter := 0
idFn := func() uuid.UUID {
    counter++
    return uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012x", counter))
}

svc := blocks.NewService(defRepo, instRepo, trRepo,
    blocks.WithIDGenerator(idFn),
)
```

---

## Running Tests

### Standard Commands

```bash
# Run all tests
go test ./...

# Run a single test by name
go test ./internal/content/... -run TestServiceCreate

# Run tests for a specific package
go test ./internal/content/...
go test ./internal/pages/...
go test ./internal/blocks/...
go test ./internal/widgets/...
go test ./internal/menus/...

# Run tests with race detection
./taskfile dev:test race

# Run tests with coverage
./taskfile dev:cover
```

### Integration Tests

```bash
# Run integration tests (require database)
go test -v ./internal/pages/... -run Integration

# Run workflow regression suite
./taskfile workflow:test

# With custom workflow engine:
CMS_WORKFLOW_PROVIDER=custom \
CMS_WORKFLOW_ENGINE_ADDR=http://localhost:8080 \
go test ./internal/workflow/... ./internal/integration/...
```

### Regression Tests

```bash
# Generator regression tests
./taskfile dev:test:regression

# Command module tests
./taskfile command:test

# CRUD readiness tests
./taskfile readiness:crud
```

### COLABS Site Tests

The `site/` directory is a separate Go module with its own test suite:

```bash
# Run COLABS tests
./taskfile colabs:test

# Vet and format checks
./taskfile colabs:vet
./taskfile colabs:fmt:check

# Build static site
./taskfile colabs:build:static

# Playwright regression tests
./taskfile colabs:verify:regressions
```

### CI Suite

```bash
# Run full CI suite
./taskfile ci
```

---

## Common Testing Patterns

### Helper Function Conventions

Tests share helper functions within their `_test` package. Common patterns include seed functions and UUID parsers:

```go
// seedContentType is a test helper that inserts a content type
// into the memory repository. It auto-generates a slug if empty.
func seedContentType(t *testing.T, store *content.MemoryContentTypeRepository, ct *content.ContentType) {
    t.Helper()
    if ct != nil && ct.Slug == "" {
        ct.Slug = content.DeriveContentTypeSlug(ct)
    }
    if err := store.Put(ct); err != nil {
        t.Fatalf("seed content type: %v", err)
    }
}

// mustParseUUID fails the test if the string is not a valid UUID.
func mustParseUUID(t *testing.T, value string) uuid.UUID {
    t.Helper()
    parsed, err := uuid.Parse(value)
    if err != nil {
        t.Fatalf("parse uuid %q: %v", value, err)
    }
    return parsed
}

// ptr returns a pointer to the given string value.
func ptr(value string) *string {
    return &value
}
```

### Cross-Module Test Setup (Legacy Pages)

Legacy page tests require content infrastructure. The pattern is to create content through the content service first, then pass the content ID to the page service:

```go
// Create content first
contentSvc := content.NewService(contentStore, contentTypeStore, localeStore)
contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
    ContentTypeID: typeID,
    Slug:          "welcome",
    CreatedBy:     uuid.New(),
    UpdatedBy:     uuid.New(),
    Translations: []content.ContentTranslationInput{
        {Locale: "en", Title: "Welcome"},
    },
})

// Then create a page referencing that content
pageSvc := pages.NewService(pageStore, contentStore, localeStore)
page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
    ContentID:  contentRecord.ID,
    TemplateID: uuid.New(),
    Slug:       "home",
    CreatedBy:  uuid.New(),
    UpdatedBy:  uuid.New(),
    Translations: []pages.PageTranslationInput{
        {Locale: "en", Title: "Home", Path: "/"},
    },
})
```

### Fake Implementations for Testing

When a service depends on an external interface (workflow engine, media provider, logger), tests provide lightweight fakes:

```go
// Fake workflow engine that returns pre-configured states
type fakeWorkflowEngine struct {
    states []interfaces.WorkflowState
    calls  []interfaces.TransitionInput
}

func (f *fakeWorkflowEngine) Transition(ctx context.Context, input interfaces.TransitionInput) (*interfaces.TransitionResult, error) {
    f.calls = append(f.calls, input)

    var target interfaces.WorkflowState
    if len(f.states) > 0 {
        target = f.states[0]
        f.states = f.states[1:]
    } else {
        target = input.CurrentState
    }

    return &interfaces.TransitionResult{
        EntityID:   input.EntityID,
        EntityType: input.EntityType,
        FromState:  input.CurrentState,
        ToState:    target,
    }, nil
}
```

After the test, assert against the recorded calls:

```go
if len(fake.calls) != 3 {
    t.Fatalf("expected 3 workflow calls got %d", len(fake.calls))
}
if fake.calls[0].EntityType != workflow.EntityTypePage {
    t.Fatalf("expected entity type %s got %s", workflow.EntityTypePage, fake.calls[0].EntityType)
}
```

---

## Testing Gotchas

### SQLite vs PostgreSQL Differences

- SQLite uses `TEXT` where PostgreSQL uses `UUID` and `BOOLEAN`
- SQLite does not support `::jsonb` casts -- strip them when applying PostgreSQL migrations to SQLite (see `applyMigrationFile`)
- Set `bunDB.SetMaxOpenConns(1)` for in-memory SQLite to prevent locking issues with concurrent connections

### Memory Repository Behaviour

- Memory repositories use `sync.RWMutex` and deep clone values on read/write
- Slug uniqueness is enforced via secondary indices within memory repositories
- Memory repositories do not support SQL-specific features like `LIKE` queries or complex joins

### Test Isolation

- Always use `t.Cleanup()` to close database connections
- Use `t.TempDir()` for file-based SQLite if you need persistent storage during a test
- Use `file::memory:?cache=shared` for in-memory SQLite that can be shared across connections within the same test
- Each test function should create its own service instances and repositories to avoid shared state

### Deterministic Testing

- Use `WithClock(func() time.Time { return time.Unix(0, 0) })` to make timestamps deterministic
- Use `WithIDGenerator(fn)` to make UUID generation predictable
- Sort slices before comparison when order is not guaranteed
