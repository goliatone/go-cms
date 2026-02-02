# Migrations Guide

This guide covers database migrations in `go-cms`: how SQL schema files are organized, how they support dual PostgreSQL and SQLite dialects, how they are embedded and registered with Bun, and how to add and test new migrations. By the end you will understand the full schema, the execution order, and the patterns used for safe data backfills.

## Migration Architecture Overview

`go-cms` uses embedded SQL migrations managed through the `uptrace/bun` ORM. The migration system has several key properties:

```
data/sql/migrations/
  ├── {timestamp}_{description}.up.sql      (PostgreSQL)
  ├── {timestamp}_{description}.down.sql
  └── sqlite/
      ├── {timestamp}_{description}.up.sql  (SQLite variant)
      └── {timestamp}_{description}.down.sql
          │
          ▼
    embed.FS (cms.GetMigrationsFS())
          │
          ▼
    go-persistence-bun (RegisterDialectMigrations)
          │
          ▼
    bun_migrations table (tracks applied state)
```

- **Timestamp-versioned**: Each migration uses a `YYYYMMDDhhmmss` UTC timestamp prefix for strict ordering.
- **Dual-dialect**: PostgreSQL (default directory) and SQLite (`sqlite/` subdirectory) variants exist for every migration.
- **Reversible**: Every `.up.sql` has a matching `.down.sql` for rollbacks.
- **Embedded**: Migrations are bundled into the binary via Go's `embed.FS` -- no external files needed at runtime.
- **Statement-splitting**: Multiple SQL statements within a single file are separated by `---bun:split` markers.

---

## Migration File Structure

All migrations live under `data/sql/migrations/`:

```
data/sql/migrations/
├── 20250102000000_initial_schema.up.sql
├── 20250102000000_initial_schema.down.sql
├── 20250209000000_menu_navigation_enhancements.up.sql
├── 20250209000000_menu_navigation_enhancements.down.sql
├── 20250301000000_menu_item_canonical_dedupe.up.sql
├── 20250301000000_menu_item_canonical_dedupe.down.sql
├── ... (additional migrations)
└── sqlite/
    ├── 20250102000000_initial_schema.up.sql
    ├── 20250102000000_initial_schema.down.sql
    ├── 20250209000000_menu_navigation_enhancements.up.sql
    └── ... (matching SQLite variants)
```

### Naming Convention

```
{TIMESTAMP}_{description}.{direction}.sql
```

| Component | Format | Example |
|-----------|--------|---------|
| Timestamp | `YYYYMMDDhhmmss` | `20250102000000` |
| Description | `snake_case` | `initial_schema` |
| Direction | `up` or `down` | `up` |

Migrations execute in timestamp order. Bun tracks which migrations have been applied in the `bun_migrations` system table to prevent re-execution.

### Statement Splitting

Multiple SQL statements within a single migration file are separated by `---bun:split`:

```sql
CREATE TABLE locales (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    ...
);

CREATE INDEX idx_locales_code ON locales(code);

---bun:split

CREATE TABLE storage_profiles (
    name TEXT PRIMARY KEY,
    ...
);
```

Each chunk between markers is executed as a separate statement.

---

## Dialect Differences

PostgreSQL and SQLite migrations express the same logical schema but differ in syntax:

| Feature | PostgreSQL | SQLite |
|---------|-----------|--------|
| UUID columns | `UUID` type | `TEXT` |
| JSON columns | `JSONB` | `TEXT` |
| Boolean columns | `BOOLEAN` | `INTEGER` (0/1) |
| JSON defaults | `'{}'::jsonb` | `'{}'` |
| Partial unique index | `WHERE condition` | `WHERE condition` (limited) |
| `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` | Supported | Requires workaround |
| Triggers / plpgsql | Full support | Simple SQL triggers only |

**PostgreSQL** (`data/sql/migrations/20250102000000_initial_schema.up.sql`):

```sql
CREATE TABLE locales (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_default BOOLEAN NOT NULL DEFAULT false,
    metadata JSONB,
    ...
);
```

**SQLite** (`data/sql/migrations/sqlite/20250102000000_initial_schema.up.sql`):

```sql
CREATE TABLE locales (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    is_default INTEGER NOT NULL DEFAULT 0,
    metadata TEXT,
    ...
);
```

---

## Embedding and Accessing Migrations

Migrations are embedded into the binary at compile time via `migrations.go` in the package root:

```go
package cms

import "embed"

//go:embed data/sql/migrations
var migrationsFS embed.FS

// GetMigrationsFS returns the embedded migration files for this package.
func GetMigrationsFS() embed.FS {
    return migrationsFS
}
```

Consumers call `cms.GetMigrationsFS()` to obtain the embedded filesystem containing all migration files for both dialects.

---

## Registering Migrations with Bun

The recommended approach uses `go-persistence-bun` for dialect-aware migration management:

```go
import (
    "context"
    "database/sql"
    "io/fs"

    "github.com/goliatone/go-cms"
    persistence "github.com/goliatone/go-persistence-bun"
    "github.com/uptrace/bun"
    "github.com/uptrace/bun/dialect/sqlitedialect"
    _ "github.com/mattn/go-sqlite3"
)

func setupDatabase() (*bun.DB, error) {
    // Open database connection
    sqlDB, err := sql.Open("sqlite3", "file:cms.db?cache=shared")
    if err != nil {
        return nil, err
    }

    // Create Bun client with migrations
    client, err := persistence.New(cfg.Persistence, sqlDB, sqlitedialect.New())
    if err != nil {
        return nil, err
    }

    // Register CMS migrations (dialect-aware)
    migrationsFS, err := fs.Sub(cms.GetMigrationsFS(), "data/sql/migrations")
    if err != nil {
        return nil, err
    }
    client.RegisterDialectMigrations(
        migrationsFS,
        persistence.WithDialectSourceLabel("data/sql/migrations"),
        persistence.WithValidationTargets("postgres", "sqlite"),
    )

    // Validate both dialect variants exist for each migration
    if err := client.ValidateDialects(context.Background()); err != nil {
        return nil, err
    }

    // Run pending migrations
    if err := client.Migrate(context.Background()); err != nil {
        return nil, err
    }

    // Check migration status
    if report := client.Report(); report != nil && !report.IsZero() {
        fmt.Printf("Applied migrations: %s\n", report.String())
    }

    return client.DB(), nil
}
```

**Key steps:**

1. `fs.Sub()` extracts the `data/sql/migrations` subdirectory from the embedded FS.
2. `RegisterDialectMigrations()` registers migrations for multiple dialects.
3. `WithValidationTargets("postgres", "sqlite")` ensures both variants exist for every timestamp.
4. `client.Migrate()` executes all pending migrations and records their state in `bun_migrations`.

---

## Schema Overview

The initial migration (`20250102000000_initial_schema`) creates the complete core schema. Subsequent migrations evolve it incrementally. All tables use UUID primary keys and UTC timestamps.

### Localization

**locales** -- Supported languages for the CMS.

| Column | Type | Description |
|--------|------|-------------|
| `id` | `UUID PK` | Locale identifier |
| `code` | `TEXT UNIQUE` | Language code (e.g., `"en"`, `"es"`) |
| `display_name` | `TEXT` | Human-readable name |
| `native_name` | `TEXT` | Name in the locale's own language |
| `is_active` | `BOOLEAN` | Whether the locale is available |
| `is_default` | `BOOLEAN` | Whether this is the default locale |
| `metadata` | `JSONB` | Additional locale configuration |

### Storage

**storage_profiles** -- Runtime storage configuration for backend switching.

| Column | Type | Description |
|--------|------|-------------|
| `name` | `TEXT PK` | Profile identifier |
| `provider` | `TEXT` | Storage provider type |
| `config` | `JSONB` | Connection configuration |
| `fallbacks` | `JSONB` | Ordered fallback profile names |
| `labels` | `JSONB` | Arbitrary key-value metadata |
| `is_default` | `BOOLEAN` | Whether this is the default profile |

### Content Types

**content_types** -- Blueprint entities defining editorial structure.

| Column | Type | Description |
|--------|------|-------------|
| `id` | `UUID PK` | Content type identifier |
| `name` | `TEXT` | Display name |
| `slug` | `TEXT UNIQUE` | URL-safe identifier (added by `20260126000000`) |
| `description` | `TEXT` | Human-readable description |
| `schema` | `JSONB` | JSON schema defining content structure |
| `ui_schema` | `JSONB` | UI rendering hints (added by `20260128000000`) |
| `capabilities` | `JSONB` | Feature toggles per content type |
| `schema_version` | `TEXT` | Semantic version of the schema (added by `20260128000000`) |
| `status` | `TEXT` | Lifecycle status (added by `20260128000000`) |
| `environment_id` | `UUID FK` | Environment scope (added by `20260415000000`) |

### Content Entries

**contents** -- Individual content records belonging to a content type.

| Column | Type | Description |
|--------|------|-------------|
| `id` | `UUID PK` | Content identifier |
| `content_type_id` | `UUID FK` | References `content_types(id)` |
| `slug` | `TEXT` | URL-safe identifier |
| `status` | `TEXT` | One of `draft`, `published`, `archived`, `scheduled` |
| `current_version` | `INTEGER` | Active version number |
| `published_version` | `INTEGER` | Published version number |
| `publish_at` / `unpublish_at` | `TIMESTAMP` | Scheduling timestamps |
| `environment_id` | `UUID FK` | Environment scope (added by `20260415000000`) |

**content_translations** -- Localized content payloads.

| Column | Type | Description |
|--------|------|-------------|
| `content_id` | `UUID FK` | References `contents(id)`, CASCADE delete |
| `locale_id` | `UUID FK` | References `locales(id)` |
| `title` | `TEXT` | Localized title |
| `summary` | `TEXT` | Localized summary |
| `content` | `JSONB` | Localized content payload |
| `translation_group_id` | `UUID` | Translation grouping (added by `20260301000000`) |
| | | UNIQUE on `(content_id, locale_id)` |

**content_versions** -- Immutable snapshots of content entries.

| Column | Type | Description |
|--------|------|-------------|
| `content_id` | `UUID FK` | References `contents(id)`, CASCADE delete |
| `version` | `INTEGER` | Version number |
| `status` | `TEXT` | Version status |
| `snapshot` | `JSONB` | Complete content snapshot |
| | | UNIQUE on `(content_id, version)` |

### Pages

**pages** -- Hierarchical page metadata with self-referential parent.

| Column | Type | Description |
|--------|------|-------------|
| `id` | `UUID PK` | Page identifier |
| `content_id` | `UUID FK` | References `contents(id)`, RESTRICT delete |
| `parent_id` | `UUID FK` | References `pages(id)`, SET NULL on delete |
| `template_id` | `UUID FK` | References `templates(id)` |
| `slug` | `TEXT` | URL-safe identifier |
| `status` | `TEXT` | One of `draft`, `published`, `archived`, `scheduled` |
| `environment_id` | `UUID FK` | Environment scope (added by `20260415000000`) |

**page_translations** -- Locale-specific page routing and SEO metadata.

| Column | Type | Description |
|--------|------|-------------|
| `page_id` | `UUID FK` | References `pages(id)`, CASCADE delete |
| `locale_id` | `UUID FK` | References `locales(id)` |
| `title` | `TEXT` | Localized title |
| `path` | `TEXT` | Canonical URL path per locale |
| `seo_title` | `TEXT` | SEO title override |
| `seo_description` | `TEXT` | SEO description |
| `translation_group_id` | `UUID` | Translation grouping (added by `20260301000000`) |
| | | UNIQUE on `(page_id, locale_id)` |

**page_versions** -- Snapshots of page structure. Same pattern as `content_versions`.

### Themes and Templates

**themes** -- Complete site designs.

| Column | Type | Description |
|--------|------|-------------|
| `id` | `UUID PK` | Theme identifier |
| `name` | `TEXT UNIQUE` | Theme name |
| `version` | `TEXT` | Semantic version |
| `is_active` | `BOOLEAN` | Whether this theme is active |
| `theme_path` | `TEXT` | Filesystem path to theme assets |
| `config` | `JSONB` | Theme configuration |

**templates** -- Layout surfaces within themes.

| Column | Type | Description |
|--------|------|-------------|
| `id` | `UUID PK` | Template identifier |
| `theme_id` | `UUID FK` | References `themes(id)`, CASCADE delete |
| `slug` | `TEXT` | URL-safe identifier |
| `template_path` | `TEXT` | Path to template file |
| `regions` | `JSONB` | Named regions (e.g., `"hero"`, `"sidebar"`) |

### Blocks

**block_definitions** -- Reusable block type blueprints.

| Column | Type | Description |
|--------|------|-------------|
| `id` | `UUID PK` | Definition identifier |
| `name` | `TEXT` | Display name |
| `slug` | `TEXT` | URL-safe identifier (added by `20260415000000`) |
| `schema` | `JSONB` | Block content schema |
| `defaults` | `JSONB` | Default configuration values |
| `environment_id` | `UUID FK` | Environment scope (added by `20260415000000`) |
| `metadata` | `JSONB` | Extra metadata (added by `20260420000000`) |

**block_instances** -- Concrete block placements on pages.

| Column | Type | Description |
|--------|------|-------------|
| `id` | `UUID PK` | Instance identifier |
| `page_id` | `UUID FK` | References `pages(id)`, nullable for global blocks |
| `definition_id` | `UUID FK` | References `block_definitions(id)` |
| `region` | `TEXT` | Template region name |
| `position` | `INTEGER` | Sort order within region |
| `configuration` | `JSONB` | Instance-specific configuration |
| `is_global` | `BOOLEAN` | Whether this block appears on all pages |

**block_translations** -- Localized block content. UNIQUE on `(block_instance_id, locale_id)`.

**block_versions** -- Block instance snapshots. UNIQUE on `(block_instance_id, version)`.

**block_definition_versions** -- Definition schema evolution tracking (added by `20260401000000`).

### Widgets

**widget_definitions** -- Widget type definitions with schema and defaults.

**widget_instances** -- Concrete widget placements with visibility rules and scheduling.

| Column | Type | Description |
|--------|------|-------------|
| `definition_id` | `UUID FK` | References `widget_definitions(id)` |
| `block_instance_id` | `UUID FK` | Optional attachment to a block |
| `area_code` | `TEXT` | Widget area placement |
| `visibility_rules` | `JSONB` | Conditional display logic |
| `publish_on` / `unpublish_on` | `TIMESTAMP` | Time-based visibility |

**widget_translations** -- Localized widget content. UNIQUE on `(widget_instance_id, locale_id)`.

**widget_area_definitions** -- Named regions for widget placement with scope (`global`, `theme`, `template`).

**widget_area_placements** -- Binds widgets to areas with ordering and locale scoping.

### Menus

**menus** -- Navigation containers identified by code.

| Column | Type | Description |
|--------|------|-------------|
| `id` | `UUID PK` | Menu identifier |
| `code` | `TEXT UNIQUE` | Lookup code |
| `location` | `TEXT` | Theme-bound placement (added by `20260301000001`) |
| `environment_id` | `UUID FK` | Environment scope (added by `20260415000000`) |

**menu_items** -- Individual navigation entries with self-referential parent.

| Column | Type | Description |
|--------|------|-------------|
| `menu_id` | `UUID FK` | References `menus(id)`, CASCADE delete |
| `parent_id` | `UUID FK` | References `menu_items(id)`, CASCADE delete |
| `target` | `JSONB` | Navigation target (page reference, URL, etc.) |
| `position` | `INTEGER` | Sort order |
| `type` | `TEXT` | Item type (added by `20250209000000`) |
| `external_code` | `TEXT` | External identifier (added by `20251213000000`) |
| `parent_ref` | `TEXT` | Deferred parent reference (added by `20251213000000`) |

**menu_item_translations** -- Localized menu labels. UNIQUE on `(menu_item_id, locale_id)`.

### Environments

**environments** -- Environment scopes for multi-tenant deployments (added by `20260415000000`).

| Column | Type | Description |
|--------|------|-------------|
| `id` | `UUID PK` | Environment identifier |
| `key` | `TEXT UNIQUE` | Lookup key (e.g., `"default"`, `"staging"`) |
| `name` | `TEXT` | Display name |
| `is_active` | `BOOLEAN` | Whether environment is available |
| `is_default` | `BOOLEAN` | Whether this is the default environment |

A default environment (`key: "default"`) is seeded automatically. The migration retroactively adds `environment_id` columns to `content_types`, `contents`, `pages`, `menus`, `menu_items`, and `block_definitions`, backfilling existing records to the default environment.

---

## Migration Execution Order

Migrations execute in timestamp order:

| # | Timestamp | Description |
|---|-----------|-------------|
| 1 | `20250102000000` | Initial schema: locales, storage profiles, content, pages, themes, templates, blocks, widgets, menus |
| 2 | `20250209000000` | Menu navigation enhancements: item type, collapsible flags, icon, badge, styles, translation keys |
| 3 | `20250301000000` | Menu item canonical deduplication |
| 4 | `20251213000000` | Menu item external parent refs: `external_code`, `parent_ref` |
| 5 | `20251216000000` | I18N settings table |
| 6 | `20260126000000` | Content type slug: backfill and uniqueness |
| 7 | `20260126000010` | Content slug uniqueness constraint |
| 8 | `20260128000000` | Content type builder fields: `ui_schema`, `schema_version`, `status`, `deleted_at` |
| 9 | `20260128000010` | Content type schema history tracking |
| 10 | `20260301000000` | Translation grouping: `translation_group_id` on content and page translations |
| 11 | `20260301000001` | Menu locations: `location` column on menus |
| 12 | `20260315000000` | Content schema version backfill |
| 13 | `20260401000000` | Block definition versions table |
| 14 | `20260415000000` | Environments: table, `environment_id` columns, backfill, triggers |
| 15 | `20260420000000` | Block definition metadata |
| 16 | `20260420000010` | Block definition migration status |

---

## Adding a New Migration

### Step 1: Create Migration Files

Create a new `.up.sql` and `.down.sql` pair in both directories:

```
data/sql/migrations/20260501000000_add_seo_fields.up.sql
data/sql/migrations/20260501000000_add_seo_fields.down.sql
data/sql/migrations/sqlite/20260501000000_add_seo_fields.up.sql
data/sql/migrations/sqlite/20260501000000_add_seo_fields.down.sql
```

Use the current UTC time as the timestamp prefix in `YYYYMMDDhhmmss` format.

### Step 2: Write PostgreSQL DDL

```sql
-- data/sql/migrations/20260501000000_add_seo_fields.up.sql
ALTER TABLE contents ADD COLUMN IF NOT EXISTS seo_keywords JSONB;
ALTER TABLE contents ADD COLUMN IF NOT EXISTS seo_canonical TEXT;

---bun:split

CREATE INDEX idx_contents_seo_canonical ON contents(seo_canonical)
    WHERE seo_canonical IS NOT NULL;
```

### Step 3: Write SQLite Variant

```sql
-- data/sql/migrations/sqlite/20260501000000_add_seo_fields.up.sql
ALTER TABLE contents ADD COLUMN seo_keywords TEXT;
ALTER TABLE contents ADD COLUMN seo_canonical TEXT;

---bun:split

CREATE INDEX idx_contents_seo_canonical ON contents(seo_canonical);
```

Key differences:
- Replace `JSONB` with `TEXT`
- Remove `IF NOT EXISTS` on `ALTER TABLE` (SQLite does not support it)
- Remove PostgreSQL-specific casts (e.g., `::jsonb`)
- Simplify partial index conditions if needed

### Step 4: Write Down Migration

```sql
-- data/sql/migrations/20260501000000_add_seo_fields.down.sql
DROP INDEX IF EXISTS idx_contents_seo_canonical;
ALTER TABLE contents DROP COLUMN IF EXISTS seo_canonical;
ALTER TABLE contents DROP COLUMN IF EXISTS seo_keywords;
```

### Step 5: Validate

After adding migration files, validate both dialect variants:

```go
client.RegisterDialectMigrations(
    migrationsFS,
    persistence.WithDialectSourceLabel("data/sql/migrations"),
    persistence.WithValidationTargets("postgres", "sqlite"),
)
if err := client.ValidateDialects(ctx); err != nil {
    // Reports missing variants
    log.Fatal(err)
}
```

---

## Migration Patterns

### Backfill + Constraint

When adding a column that requires uniqueness, first add the column with no constraint, backfill data, resolve duplicates, then add the constraint:

```sql
-- Step 1: Add column
ALTER TABLE content_types ADD COLUMN slug TEXT;

---bun:split

-- Step 2: Backfill from existing data
UPDATE content_types SET slug = lower(replace(trim(name), ' ', '-'))
    WHERE slug IS NULL OR slug = '';

---bun:split

-- Step 3: Resolve duplicates by appending ID prefix
WITH duplicates AS (
    SELECT slug FROM content_types
    GROUP BY slug HAVING count(*) > 1
)
UPDATE content_types
SET slug = slug || '-' || substr(CAST(id AS TEXT), 1, 8)
WHERE slug IN (SELECT slug FROM duplicates);

---bun:split

-- Step 4: Add uniqueness constraint
CREATE UNIQUE INDEX idx_content_types_slug ON content_types(slug);
```

### Environment Scoping

The environments migration (`20260415000000`) demonstrates retroactive scoping:

1. Create `environments` table and seed a default
2. Add `environment_id` columns with a default value
3. Backfill using relationships (e.g., contents inherit from their content type)
4. Replace global uniqueness indexes with environment-scoped ones

```sql
-- Seed default environment
INSERT INTO environments (id, key, name, is_active, is_default)
VALUES ('00000000-0000-0000-0000-000000000001', 'default', 'Default', TRUE, TRUE)
ON CONFLICT (key) DO NOTHING;

-- Add environment_id to content_types
ALTER TABLE content_types
    ADD COLUMN IF NOT EXISTS environment_id UUID NOT NULL
    DEFAULT '00000000-0000-0000-0000-000000000001';

-- Backfill contents from their content type
UPDATE contents AS c
SET environment_id = ct.environment_id
FROM content_types ct
WHERE c.content_type_id = ct.id;
```

### Multi-Statement Transactions

Use `---bun:split` to separate statements that must execute independently. Each split becomes a separate Bun execution unit:

```sql
-- First statement
ALTER TABLE menus ADD COLUMN location TEXT;

---bun:split

-- Second statement (depends on first)
CREATE INDEX idx_menus_location ON menus(location)
    WHERE location IS NOT NULL;
```

---

## Testing Migrations

### In-Memory SQLite

Tests use `testsupport.NewSQLiteMemoryDB()` for fast, isolated database instances:

```go
import "github.com/goliatone/go-cms/pkg/testsupport"

db, err := testsupport.NewSQLiteMemoryDB()
// Opens: file::memory:?cache=shared
```

### Applying Migrations in Tests

The `applyMigrationFile` helper reads from the embedded FS, strips PostgreSQL-specific syntax, and executes each statement:

```go
func applyMigrationFile(t *testing.T, db *sql.DB, name string) {
    t.Helper()
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
    // Strip PostgreSQL-specific type casts for SQLite
    content = strings.ReplaceAll(content, "::jsonb", "")
    content = strings.ReplaceAll(content, "::JSONB", "")

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

### Testing a Migration Backfill

Migration tests verify that backfill logic handles edge cases correctly:

```go
func TestContentTypeSlugMigrationBackfill(t *testing.T) {
    db, err := testsupport.NewSQLiteMemoryDB()
    if err != nil {
        t.Fatalf("open sqlite db: %v", err)
    }
    defer db.Close()

    // Apply base schema
    applyMigrationFile(t, db, "20250102000000_initial_schema.up.sql")

    // Insert test data with duplicate names
    db.Exec(`INSERT INTO content_types (id, name, schema) VALUES (?, ?, ?)`,
        uuid.NewString(), "Landing Page", `{"fields":[]}`)
    db.Exec(`INSERT INTO content_types (id, name, schema) VALUES (?, ?, ?)`,
        uuid.NewString(), "Landing Page", `{"fields":[]}`)

    // Apply slug migration
    applyMigrationFile(t, db, "20260126000000_content_type_slug.up.sql")

    // Verify: both get slugs starting with "landing-page"
    // Verify: slugs are unique (one gets a suffix)
}
```

### Running Migration Tests

```bash
# Run migration-specific tests
go test ./internal/content/... -run TestContentTypeSlugMigration

# Run all tests (includes migration tests)
go test ./...
```

---

## Common Gotchas

### SQLite `ALTER TABLE` Limitations

SQLite does not support `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`. If the PostgreSQL migration uses this syntax, the SQLite variant must omit `IF NOT EXISTS`. This also means SQLite down migrations that use `DROP COLUMN` require SQLite 3.35.0+.

### JSONB Cast Stripping

When running PostgreSQL migrations against SQLite in tests, strip `::jsonb` and `::JSONB` casts. The `applyMigrationFile` helper handles this automatically, but manual SQL execution must account for it.

### Embedded FS Cache

After adding new migration files, rebuild the binary to update the embedded FS. The `//go:embed data/sql/migrations` directive captures the directory state at build time.

### Migration Order Conflicts

When multiple developers add migrations concurrently, use distinct timestamps to avoid ordering conflicts. The `YYYYMMDDhhmmss` format provides second-level granularity. For closely related migrations, use sub-second suffixes (e.g., `20260126000000` and `20260126000010`).

### Down Migration Safety

Down migrations should be idempotent where possible. Use `DROP INDEX IF EXISTS` and `DROP TABLE IF EXISTS` to handle partial rollback states gracefully.
