# Repositories Guide

This guide covers the repository layer in `go-cms`: how data access is abstracted through interfaces, the two built-in implementations (memory and BunDB), runtime storage switching via proxies, caching integration, and storage profile management. By the end you will understand how to configure, extend, and test the persistence layer.

## Repository Architecture Overview

Every domain module in `go-cms` follows a three-layer repository pattern:

```
Service Layer
  │
  ▼
Repository Interface   (contract: what operations exist)
  │
  ├── Memory Implementation     (in-memory maps, no database)
  ├── Bun Implementation        (SQL via uptrace/bun ORM)
  │     └── Cache Wrapper       (optional go-repository-cache layer)
  │
  ▼
Storage Proxy          (runtime backend switching)
```

1. **Repository interfaces** define the contract each module requires. Services depend only on the interface, never on a concrete implementation.
2. **Memory repositories** store data in Go maps protected by `sync.RWMutex`. They require no database and are ideal for tests and quick prototypes.
3. **Bun repositories** use the `uptrace/bun` ORM to persist data in PostgreSQL or SQLite. They wrap the generic `go-repository-bun` library and optionally add a `go-repository-cache` layer.
4. **Storage proxies** sit between services and repositories, enabling zero-downtime backend switching at runtime.

All entities use UUID primary keys and UTC timestamps.

---

## Repository Interfaces

Each module declares its repository interfaces alongside its service definition. The interfaces describe CRUD operations plus any module-specific queries.

### Content Repositories

```go
// ContentRepository manages content entry persistence.
type ContentRepository interface {
    Create(ctx context.Context, record *Content) (*Content, error)
    GetByID(ctx context.Context, id uuid.UUID) (*Content, error)
    GetBySlug(ctx context.Context, slug string, contentTypeID uuid.UUID, env ...string) (*Content, error)
    List(ctx context.Context, env ...string) ([]*Content, error)
    Update(ctx context.Context, record *Content) (*Content, error)
    ReplaceTranslations(ctx context.Context, contentID uuid.UUID, translations []*ContentTranslation) error
    Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error
    CreateVersion(ctx context.Context, version *ContentVersion) (*ContentVersion, error)
    ListVersions(ctx context.Context, contentID uuid.UUID) ([]*ContentVersion, error)
    GetVersion(ctx context.Context, contentID uuid.UUID, number int) (*ContentVersion, error)
    GetLatestVersion(ctx context.Context, contentID uuid.UUID) (*ContentVersion, error)
    UpdateVersion(ctx context.Context, version *ContentVersion) (*ContentVersion, error)
}

// ContentTypeRepository resolves content types.
type ContentTypeRepository interface {
    Create(ctx context.Context, record *ContentType) (*ContentType, error)
    GetByID(ctx context.Context, id uuid.UUID) (*ContentType, error)
    GetBySlug(ctx context.Context, slug string, env ...string) (*ContentType, error)
    List(ctx context.Context, env ...string) ([]*ContentType, error)
    Search(ctx context.Context, query string, env ...string) ([]*ContentType, error)
    Update(ctx context.Context, record *ContentType) (*ContentType, error)
    Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error
}

// LocaleRepository resolves locales by code.
type LocaleRepository interface {
    GetByCode(ctx context.Context, code string) (*Locale, error)
}
```

### Page Repository

```go
type PageRepository interface {
    Create(ctx context.Context, record *Page) (*Page, error)
    GetByID(ctx context.Context, id uuid.UUID) (*Page, error)
    GetBySlug(ctx context.Context, slug string, env ...string) (*Page, error)
    List(ctx context.Context, env ...string) ([]*Page, error)
    Update(ctx context.Context, record *Page) (*Page, error)
    ReplaceTranslations(ctx context.Context, pageID uuid.UUID, translations []*PageTranslation) error
    Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error
    CreateVersion(ctx context.Context, version *PageVersion) (*PageVersion, error)
    ListVersions(ctx context.Context, pageID uuid.UUID) ([]*PageVersion, error)
    GetVersion(ctx context.Context, pageID uuid.UUID, number int) (*PageVersion, error)
    GetLatestVersion(ctx context.Context, pageID uuid.UUID) (*PageVersion, error)
    UpdateVersion(ctx context.Context, version *PageVersion) (*PageVersion, error)
}
```

### Block Repositories

Blocks split persistence across three focused interfaces:

```go
// DefinitionRepository manages block blueprints.
type DefinitionRepository interface {
    Create(ctx context.Context, definition *Definition) (*Definition, error)
    GetByID(ctx context.Context, id uuid.UUID) (*Definition, error)
    GetBySlug(ctx context.Context, slug string, env ...string) (*Definition, error)
    List(ctx context.Context, env ...string) ([]*Definition, error)
    Update(ctx context.Context, definition *Definition) (*Definition, error)
    Delete(ctx context.Context, id uuid.UUID) error
}

// InstanceRepository manages block placements on pages.
type InstanceRepository interface {
    Create(ctx context.Context, instance *Instance) (*Instance, error)
    GetByID(ctx context.Context, id uuid.UUID) (*Instance, error)
    ListByPage(ctx context.Context, pageID uuid.UUID) ([]*Instance, error)
    ListGlobal(ctx context.Context) ([]*Instance, error)
    ListByDefinition(ctx context.Context, definitionID uuid.UUID) ([]*Instance, error)
    Update(ctx context.Context, instance *Instance) (*Instance, error)
    Delete(ctx context.Context, id uuid.UUID) error
}

// TranslationRepository manages localized block content.
type TranslationRepository interface {
    Create(ctx context.Context, translation *Translation) (*Translation, error)
    GetByInstanceAndLocale(ctx context.Context, instanceID uuid.UUID, localeID uuid.UUID) (*Translation, error)
    ListByInstance(ctx context.Context, instanceID uuid.UUID) ([]*Translation, error)
    Update(ctx context.Context, translation *Translation) (*Translation, error)
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### Common Interface Patterns

All repository interfaces across modules share these conventions:

| Pattern | Description |
|---------|-------------|
| `Create` returns `(*T, error)` | The created record is returned with server-assigned fields (ID, timestamps) |
| `GetByID` / `GetBySlug` | Single-entity lookups that return `NotFoundError` when absent |
| `List` accepts variadic `env ...string` | Environment-scoped listing for multi-environment deployments |
| `Update` returns the updated record | Callers receive the post-mutation state |
| `Delete` accepts `hardDelete bool` | Soft-delete support where applicable |
| Version methods | `CreateVersion`, `ListVersions`, `GetVersion`, `GetLatestVersion`, `UpdateVersion` |

---

## Memory Repositories

Memory repositories are the default backend when no database is configured. They are used in tests, quick prototypes, and when `di.WithBunDB()` is not provided.

### Construction

```go
contentRepo := content.NewMemoryContentRepository()
contentTypeRepo := content.NewMemoryContentTypeRepository()
localeRepo := content.NewMemoryLocaleRepository()
pageRepo := pages.NewMemoryPageRepository()
```

### Implementation Patterns

Memory repositories follow a consistent internal structure:

```go
type MemoryContentRepository struct {
    mu        sync.RWMutex              // Thread-safe access
    contents  map[uuid.UUID]*Content    // Primary storage by ID
    slugIndex map[string]uuid.UUID      // Secondary index for slug lookups
    versions  map[uuid.UUID][]*ContentVersion  // Version history
}
```

**Thread safety**: All methods acquire `sync.RWMutex` locks. Read operations use `RLock`, write operations use `Lock`.

**Deep cloning**: Every return value is a deep clone of the stored data. This prevents external mutations from corrupting the repository state:

```go
func (m *MemoryContentRepository) GetByID(_ context.Context, id uuid.UUID) (*Content, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    rec, ok := m.contents[id]
    if !ok {
        return nil, &NotFoundError{Resource: "content", Key: id.String()}
    }
    return m.attachVersions(cloneContent(rec)), nil
}
```

**Secondary indices**: Slug-based lookups use a separate index map keyed by a composite of environment ID, content type ID, and slug:

```go
func contentSlugKey(envID uuid.UUID, contentTypeID uuid.UUID, slug string) string {
    return envID.String() + "|" + contentTypeID.String() + "|" + strings.TrimSpace(slug)
}
```

**Environment filtering**: Memory repositories resolve the environment ID from the optional `env` parameter and filter results accordingly:

```go
func (m *MemoryContentRepository) List(_ context.Context, env ...string) ([]*Content, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    envID := resolveEnvironmentID(uuid.Nil, resolveEnvironmentKey(env...))
    out := make([]*Content, 0, len(m.contents))
    for _, rec := range m.contents {
        if !matchesEnvironment(rec.EnvironmentID, envID) {
            continue
        }
        out = append(out, m.attachVersions(cloneContent(rec)))
    }
    return out, nil
}
```

### Limitations

- No transaction support. Memory repositories execute operations atomically at the method level but cannot participate in multi-repository transactions.
- No soft-delete for content entries. Calling `Delete` with `hardDelete: false` returns an error.
- No SQL query capabilities. Searches use Go-level string matching.

---

## Bun Repositories

Bun repositories provide SQL-backed persistence using the `uptrace/bun` ORM. They support both PostgreSQL and SQLite dialects.

### Construction

Bun repositories are created with a `*bun.DB` instance and optionally wrap with caching:

```go
// Without caching
contentRepo := content.NewBunContentRepository(db)

// With caching
contentRepo := content.NewBunContentRepositoryWithCache(db, cacheService, keySerializer)
```

### Three-Layer Wrapping

Each Bun repository builds on three layers:

1. **Base repository** via `go-repository-bun` -- provides generic CRUD with `ModelHandlers`:

```go
func NewContentRepository(db *bun.DB) repository.Repository[*Content] {
    return repository.MustNewRepository(db, repository.ModelHandlers[*Content]{
        NewRecord:          func() *Content { return &Content{} },
        GetID:              func(c *Content) uuid.UUID { return c.ID },
        SetID:              func(c *Content, id uuid.UUID) { c.ID = id },
        GetIdentifier:      func() string { return "slug" },
        GetIdentifierValue: func(c *Content) string { return c.Slug },
    })
}
```

2. **Optional cache wrapper** via `go-repository-cache`:

```go
func wrapWithCache[T any](
    base repository.Repository[T],
    cacheService cache.CacheService,
    keySerializer cache.KeySerializer,
) repository.Repository[T] {
    if cacheService == nil || keySerializer == nil {
        return base
    }
    return repositorycache.New(base, cacheService, keySerializer)
}
```

3. **Domain-specific struct** that composes the wrapped repositories and adds transaction logic:

```go
type BunContentRepository struct {
    db           *bun.DB
    repo         repository.Repository[*Content]
    translations repository.Repository[*ContentTranslation]
    versions     repository.Repository[*ContentVersion]
}
```

### Query Building

Bun repositories use `SelectRawProcessor` to build queries with bun's query builder:

```go
func (r *BunContentRepository) GetBySlug(
    ctx context.Context,
    slug string,
    contentTypeID uuid.UUID,
    env ...string,
) (*Content, error) {
    records, _, err := r.repo.List(ctx,
        repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
            return q.Where("?TableAlias.slug = ?", slug).
                Where("?TableAlias.content_type_id = ?", contentTypeID)
        }),
        repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
            return applyEnvironmentFilter(q, normalizedEnv)
        }),
        repository.SelectPaginate(1, 0),
    )
    if err != nil {
        return nil, mapRepositoryError(err, "content", slug)
    }
    if len(records) == 0 {
        return nil, &NotFoundError{Resource: "content", Key: slug}
    }
    return records[0], nil
}
```

### Update Column Selection

Updates specify exactly which columns to write, preventing accidental overwrites:

```go
updated, err := r.repo.Update(ctx, record,
    repository.UpdateByID(record.ID.String()),
    repository.UpdateColumns(
        "current_version",
        "published_version",
        "status",
        "publish_at",
        "unpublish_at",
        "published_at",
        "published_by",
        "updated_by",
        "updated_at",
    ),
)
```

---

## Transaction Strategy

Complex multi-entity operations use repository-level `db.RunInTx()`. Services do not manage transactions -- repositories do.

### Create with Translations

The `Create` method demonstrates the core transaction pattern. A content entry and its translations are inserted atomically:

```go
func (r *BunContentRepository) Create(ctx context.Context, record *Content) (*Content, error) {
    var created *Content
    err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        var err error
        // Step 1: Insert the main entity using CreateTx (fail-fast on duplicates)
        created, err = r.repo.CreateTx(ctx, tx, record)
        if err != nil {
            return err
        }

        if len(record.Translations) == 0 {
            return nil
        }

        // Step 2: Insert translations in the same transaction
        now := time.Now().UTC()
        toInsert := make([]*ContentTranslation, 0, len(record.Translations))
        for _, tr := range record.Translations {
            cloned := *tr
            if cloned.ID == uuid.Nil {
                cloned.ID = uuid.New()
            }
            cloned.ContentID = created.ID
            if cloned.CreatedAt.IsZero() {
                cloned.CreatedAt = now
            }
            cloned.UpdatedAt = now
            toInsert = append(toInsert, &cloned)
        }

        if _, err := tx.NewInsert().Model(&toInsert).Exec(ctx); err != nil {
            return fmt.Errorf("insert translations: %w", err)
        }

        created.Translations = append([]*ContentTranslation{}, toInsert...)
        return nil
    })
    if err != nil {
        return nil, err
    }
    return created, nil
}
```

### Replace Translations

Translation replacement uses delete-then-insert within a single transaction:

```go
func (r *BunContentRepository) ReplaceTranslations(
    ctx context.Context,
    contentID uuid.UUID,
    translations []*ContentTranslation,
) error {
    return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        // Delete all existing translations
        if _, err := tx.NewDelete().
            Model((*ContentTranslation)(nil)).
            Where("?TableAlias.content_id = ?", contentID).
            Exec(ctx); err != nil {
            return fmt.Errorf("delete translations: %w", err)
        }

        if len(translations) == 0 {
            return nil
        }

        // Insert new translations
        // ... (prepare toInsert slice)

        if _, err := tx.NewInsert().Model(&toInsert).Exec(ctx); err != nil {
            return fmt.Errorf("insert translations: %w", err)
        }
        return nil
    })
}
```

### Cascading Delete

Deleting content cascades through translations and versions in one transaction:

```go
func (r *BunContentRepository) Delete(ctx context.Context, id uuid.UUID, hardDelete bool) error {
    return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        // 1. Delete translations
        tx.NewDelete().Model((*ContentTranslation)(nil)).
            Where("?TableAlias.content_id = ?", id).Exec(ctx)

        // 2. Delete versions
        tx.NewDelete().Model((*ContentVersion)(nil)).
            Where("?TableAlias.content_id = ?", id).Exec(ctx)

        // 3. Delete content record
        result, err := tx.NewDelete().Model((*Content)(nil)).
            Where("?TableAlias.id = ?", id).Exec(ctx)

        affected, _ := result.RowsAffected()
        if affected == 0 {
            return &NotFoundError{Resource: "content", Key: id.String()}
        }
        return nil
    })
}
```

### Key Transaction Semantics

| Method | Behavior |
|--------|----------|
| `CreateTx()` | Fails fast on duplicates. Does **not** upsert. |
| `GetOrCreateTx()` | Returns existing record or creates new. Only used for specific upsert scenarios. |
| `db.RunInTx()` | Wraps multiple statements in a single ACID transaction. Rollback on any error. |

---

## Error Handling

Each module defines a `NotFoundError` type with consistent structure:

```go
type NotFoundError struct {
    Resource string
    Key      string
}

func (e *NotFoundError) Error() string {
    if e.Key == "" {
        return fmt.Sprintf("%s not found", e.Resource)
    }
    return fmt.Sprintf("%s %q not found", e.Resource, e.Key)
}
```

### Error Mapping

Bun repositories map low-level database errors to domain-specific types using `mapRepositoryError`:

```go
func mapRepositoryError(err error, resource, key string) error {
    if err == nil {
        return nil
    }
    if goerrors.IsCategory(err, repository.CategoryDatabaseNotFound) {
        return &NotFoundError{
            Resource: resource,
            Key:      key,
        }
    }
    return fmt.Errorf("%s repository error: %w", resource, err)
}
```

### Checking Error Types

Consumers should use type assertions to distinguish error categories:

```go
result, err := contentSvc.Get(ctx, id)
if err != nil {
    var nfe *content.NotFoundError
    if errors.As(err, &nfe) {
        // Handle missing content
        return nil, fmt.Errorf("content %s not found", id)
    }
    // Handle other errors
    return nil, err
}
```

The same pattern applies across modules: `pages.PageNotFoundError`, `blocks.NotFoundError`, etc.

---

## Storage Proxies

Storage proxies enable runtime backend switching without restarting the application. Each repository type has a corresponding proxy in `internal/di/storage_proxies.go`.

### Proxy Structure

Every proxy follows the same pattern:

```go
type contentRepositoryProxy struct {
    mu   sync.RWMutex
    repo content.ContentRepository
}

func (p *contentRepositoryProxy) swap(repo content.ContentRepository) {
    p.mu.Lock()
    defer p.mu.Unlock()
    if repo != nil {
        p.repo = repo
    }
}

func (p *contentRepositoryProxy) current() content.ContentRepository {
    p.mu.RLock()
    defer p.mu.RUnlock()
    return p.repo
}

// All interface methods delegate to the current implementation:
func (p *contentRepositoryProxy) Create(ctx context.Context, record *content.Content) (*content.Content, error) {
    return p.current().Create(ctx, record)
}
```

### Available Proxies

The DI container maintains proxies for every repository type:

| Proxy | Repository Interface |
|-------|---------------------|
| `contentRepositoryProxy` | `content.ContentRepository` |
| `contentTypeRepositoryProxy` | `content.ContentTypeRepository` |
| `localeRepositoryProxy` | `content.LocaleRepository` |
| `environmentRepositoryProxy` | `environments.EnvironmentRepository` |
| `pageRepositoryProxy` | `pages.PageRepository` |
| `blockDefinitionRepositoryProxy` | `blocks.DefinitionRepository` |
| `blockDefinitionVersionRepositoryProxy` | `blocks.DefinitionVersionRepository` |
| `blockInstanceRepositoryProxy` | `blocks.InstanceRepository` |
| `blockTranslationRepositoryProxy` | `blocks.TranslationRepository` |
| `blockVersionRepositoryProxy` | `blocks.InstanceVersionRepository` |

### How Swapping Works

When a storage profile changes, the DI container:

1. Creates new Bun repositories backed by the new database connection
2. Optionally wraps them with caching
3. Calls `swap()` on each proxy to atomically replace the backing implementation
4. Closes the previous database connection

```
Container.swapStorageHandle(ctx, newHandle)
  ├── Update bunDB reference
  ├── configureRepositories()
  │     ├── contentRepo.swap(NewBunContentRepositoryWithCache(...))
  │     ├── pageRepo.swap(NewBunPageRepositoryWithCache(...))
  │     └── ... (all other proxies)
  └── Close previous handle
```

Services continue operating without interruption because they hold references to the proxy, not the underlying implementation.

---

## Repository Caching

When both `cfg.Cache.Enabled` and `cfg.Features.AdvancedCache` are `true`, repositories are automatically wrapped with `go-repository-cache`.

### Configuration

```go
cfg := cms.DefaultConfig()
cfg.Cache.Enabled = true
cfg.Cache.DefaultTTL = 5 * time.Minute
cfg.Features.AdvancedCache = true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Cache.Enabled` | `bool` | `false` | Master switch for caching |
| `Cache.DefaultTTL` | `time.Duration` | `1m` | Default cache entry lifetime |
| `Features.AdvancedCache` | `bool` | `false` | Enable repository-level caching (requires `Cache.Enabled`) |

### How It Works

The caching layer is transparent. The generic `wrapWithCache` function wraps any `repository.Repository[T]`:

```go
func wrapWithCache[T any](
    base repository.Repository[T],
    cacheService cache.CacheService,
    keySerializer cache.KeySerializer,
) repository.Repository[T] {
    if cacheService == nil || keySerializer == nil {
        return base
    }
    return repositorycache.New(base, cacheService, keySerializer)
}
```

When caching is active:
- Read operations (`GetByID`, `List`, etc.) check the cache first
- Write operations (`Create`, `Update`, `Delete`) invalidate relevant cache entries
- The cache TTL defaults to `cfg.Cache.DefaultTTL`

### Custom Cache Provider

Override the cache service via the DI container:

```go
module, err := cms.New(cfg,
    di.WithBunDB(db),
    di.WithCache(myCacheService, myKeySerializer),
)
```

---

## Storage Profiles

Storage profiles define database connection configurations that can be managed at runtime.

### Profile Structure

```go
type Profile struct {
    Name        string            `json:"name"`
    Description string            `json:"description,omitempty"`
    Provider    string            `json:"provider"`
    Config      Config            `json:"config"`
    Fallbacks   []string          `json:"fallbacks,omitempty"`
    Labels      map[string]string `json:"labels,omitempty"`
    Default     bool              `json:"default,omitempty"`
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Name` | `string` | Yes | Unique identifier for the profile |
| `Description` | `string` | No | Human-readable description |
| `Provider` | `string` | Yes | Storage provider type (e.g., `"bun"`) |
| `Config` | `Config` | Yes | Database connection configuration (name, driver, DSN, options) |
| `Fallbacks` | `[]string` | No | Ordered list of fallback profile names |
| `Labels` | `map[string]string` | No | Arbitrary key-value metadata |
| `Default` | `bool` | No | Whether this is the default profile |

### Configuration

Define profiles in the CMS configuration:

```go
cfg := cms.DefaultConfig()
cfg.Storage.Provider = "bun"
cfg.Storage.Profiles = []storage.Profile{
    {
        Name:     "primary",
        Provider: "bun",
        Default:  true,
        Config: storage.Config{
            Name:   "primary",
            Driver: "postgres",
            DSN:    "postgres://user:pass@localhost:5432/cms?sslmode=disable",
        },
    },
    {
        Name:     "readonly",
        Provider: "bun",
        Config: storage.Config{
            Name:     "readonly",
            Driver:   "postgres",
            DSN:      "postgres://reader:pass@replica:5432/cms?sslmode=disable",
            ReadOnly: true,
        },
        Fallbacks: []string{"primary"},
    },
}
cfg.Storage.Aliases = map[string]string{
    "default": "primary",
    "reports": "readonly",
}
```

### Profile Aliases

Aliases map logical names to physical profile names:

```go
cfg.Storage.Aliases = map[string]string{
    "default": "primary",
    "staging": "primary",
}
```

### StorageAdmin Service

The `StorageAdmin` service provides runtime profile management. Access it via the module facade:

```go
module, err := cms.New(cfg, di.WithBunDB(db))
storageAdmin := module.StorageAdmin()
```

**Available operations:**

```go
// List all configured profiles
profiles, err := storageAdmin.ListProfiles(ctx)

// Get a single profile by name
profile, err := storageAdmin.GetProfile(ctx, "primary")

// Validate a profile without applying
result, err := storageAdmin.PreviewProfile(ctx, newProfile)

// Apply a complete storage configuration
err := storageAdmin.ApplyConfig(ctx, runtimeconfig.StorageConfig{
    Profiles: []storage.Profile{...},
    Aliases:  map[string]string{...},
})

// Resolve an alias to its target profile name
target, ok := storageAdmin.ResolveAlias("default")

// Get JSON schemas for admin UIs
schemas := storageAdmin.Schemas()
```

### Validation

Profiles are validated before being applied. The following rules are enforced:

1. Profile names must be non-empty and unique
2. Provider and config driver fields are required
3. Config DSN is required
4. Only one profile may be marked as default
5. Fallback references must point to existing profiles
6. Fallback chains cannot reference the profile itself
7. Alias names must not collide with profile names
8. Alias targets must reference existing profiles

---

## DI Container Wiring

The DI container (`di.Container`) orchestrates repository creation and wiring.

### Initialization Flow

```go
cfg := cms.DefaultConfig()
module, err := cms.New(cfg, di.WithBunDB(db))
```

Internally, the container:

1. Creates memory repositories for all modules
2. Wraps each memory repository in a proxy
3. Applies `di.With*` options (BunDB, caching, etc.)
4. When BunDB is available, calls `configureRepositories()` to swap proxies to Bun implementations
5. Creates services with proxy references
6. Subscribes to storage profile change events

### Repository Setup

When BunDB is available:

```go
func (c *Container) configureRepositories() {
    if c.bunDB != nil {
        c.contentRepo.swap(
            content.NewBunContentRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer),
        )
        c.pageRepo.swap(
            pages.NewBunPageRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer),
        )
        // ... repeat for all other repositories
    }
}
```

Without BunDB, the memory repositories remain active. This allows prototyping and testing without a database.

### Override Order

When providing multiple DI options, call `di.WithBunDB()` first:

```go
module, err := cms.New(cfg,
    di.WithBunDB(db),                          // Must be first
    di.WithCache(cacheService, keySerializer),  // Then other overrides
    di.WithStorage(storageProvider),
    di.WithWorkflowEngine(engine),
)
```

### Lazy Initialization

Services are created on first access, not at container construction time. This avoids circular dependencies and allows optional features to remain uninitialized when disabled:

```go
contentSvc := module.Content()  // Service created here, not at cms.New()
pageSvc := module.Pages()       // Each service initialized independently
```

---

## Testing with Repositories

### Unit Tests: Memory Repositories

For fast, database-free tests, use memory repositories directly:

```go
func TestContentCreate(t *testing.T) {
    cfg := cms.DefaultConfig()
    cfg.DefaultLocale = "en"
    cfg.I18N.Locales = []string{"en"}

    module, err := cms.New(cfg)
    require.NoError(t, err)

    svc := module.Content()

    created, err := svc.Create(ctx, content.CreateContentRequest{
        ContentTypeID: articleTypeID,
        Slug:          "test-article",
        Status:        "draft",
        CreatedBy:     authorID,
        UpdatedBy:     authorID,
        Translations: []content.ContentTranslationInput{
            {Locale: "en", Title: "Test Article"},
        },
    })
    require.NoError(t, err)
    assert.Equal(t, "test-article", created.Slug)
}
```

### Integration Tests: BunDB with SQLite

For integration tests, use an in-memory SQLite database:

```go
func TestContentCreateIntegration(t *testing.T) {
    db, err := bun.Open(sqlitedialect.New(), "file::memory:?cache=shared")
    require.NoError(t, err)
    t.Cleanup(func() { db.Close() })

    // Run migrations
    // ...

    cfg := cms.DefaultConfig()
    module, err := cms.New(cfg, di.WithBunDB(db))
    require.NoError(t, err)

    svc := module.Content()
    // ... test with real SQL queries
}
```

For file-based SQLite (useful when you need persistence across test steps):

```go
dbPath := filepath.Join(t.TempDir(), "test.db")
db, err := bun.Open(sqlitedialect.New(), dbPath)
t.Cleanup(func() { db.Close() })
```

### Contract Tests

Contract tests verify that both memory and Bun implementations satisfy the same interface. They run the same test logic against both backends:

```go
// *_contract_test.go files test interface compliance
// across memory and Bun implementations
```

### Testing Patterns Summary

| Test Type | File Suffix | Backend | Use Case |
|-----------|-------------|---------|----------|
| Unit | `*_test.go` | Memory | Fast logic tests, no DB |
| Contract | `*_contract_test.go` | Memory + Bun | Interface compliance |
| Integration | `*_integration_test.go` | BunDB (SQLite) | Full SQL path testing |
| Storage | `*_storage_integration_test.go` | BunDB | Profile switching tests |

---

## Common Gotchas

### Create vs GetOrCreate

`Create()` methods use `CreateTx()` internally, which fails fast on duplicates. If you need upsert semantics, implement a separate `GetOrCreate()` method. Do not assume `Create()` will silently succeed on conflicts.

### Transaction Boundaries

Transactions live at the repository level, not the service level. Services call single repository methods that internally manage their own transaction scope. If you need cross-module atomicity, coordinate at the application level.

### Feature Flag Dependencies

Caching requires two flags:

```go
cfg.Cache.Enabled = true         // Master cache switch
cfg.Features.AdvancedCache = true // Repository-level caching
```

Setting only `AdvancedCache` without `Cache.Enabled` produces a validation error.

### Memory Repository Gotchas

- Memory repositories do not support soft-delete for content entries
- No transaction isolation between concurrent operations beyond mutex locking
- Slug uniqueness is enforced per environment and content type combination
- Data does not survive process restart

### BunDB Dialect Support

Both PostgreSQL and SQLite are supported. Migrations in `data/sql/migrations/` ship dual variants (`*_pg.up.sql` / `*_sqlite.up.sql`). Ensure you run the correct dialect's migrations for your database.
