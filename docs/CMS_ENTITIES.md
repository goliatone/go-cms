# CMS Entities Guide

## Overview
The CMS organizes content through a small set of relational entities. Pages define the site structure, blocks supply reusable chunks of content, widgets deliver dynamic behavior, and menus tie navigation together. Each entity is stored in SQL tables designed to support versioning, scheduling, and translation.

## Core Entities

| Entity            | Purpose                                                     | Key Tables                                     |
|-------------------|-------------------------------------------------------------|------------------------------------------------|
| Locale            | Canonical list of supported languages                       | `locales`                                      |
| Page              | Defines site hierarchy, layout metadata                     | `pages`, `page_translations`, `page_versions`  |
| Block             | Reusable content fragments placed within pages              | `block_definitions`, `block_instances`, `block_translations` |
| Widget            | Behavioural components (forms, carousels, search, etc.)     | `widget_definitions`, `widget_instances`, `widget_translations` |
| Menu              | Navigation structures and their localized labels             | `menus`, `menu_items`, `menu_item_translations` |
| Translation State | Tracks editorial progress for localized content             | `translation_status`                           |

### Pages
Pages carry structural information (routing, templates, scheduling) independent of locale-specific content.

```sql
CREATE TABLE pages (
    id                 UUID PRIMARY KEY,
    parent_id          UUID REFERENCES pages(id),
    slug               VARCHAR(150) NOT NULL,
    template_id        UUID NOT NULL,
    status             VARCHAR(20) NOT NULL DEFAULT 'draft',
    publish_at         TIMESTAMP,
    unpublish_at       TIMESTAMP,
    created_by         UUID NOT NULL,
    updated_by         UUID NOT NULL,
    created_at         TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(parent_id, slug)
);

CREATE TABLE page_versions (
    id          UUID PRIMARY KEY,
    page_id     UUID NOT NULL REFERENCES pages(id),
    version     INTEGER NOT NULL,
    snapshot    JSONB NOT NULL,       -- structural snapshot (layout, block ordering)
    created_by  UUID NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(page_id, version)
);

CREATE TABLE page_translations (
    id                 UUID PRIMARY KEY,
    page_id            UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    locale_id          UUID NOT NULL REFERENCES locales(id),
    title              VARCHAR(200) NOT NULL,
    path               VARCHAR(255) NOT NULL,
    seo_title          VARCHAR(255),
    seo_description    TEXT,
    summary            TEXT,
    created_at         TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(page_id, locale_id),
    UNIQUE(locale_id, path)
);
```

### Blocks
Blocks are reusable content units. Definitions describe schema/options, instances capture placement, and translations supply localized text/media.

```sql
CREATE TABLE block_definitions (
    id            UUID PRIMARY KEY,
    name          VARCHAR(100) UNIQUE NOT NULL,
    schema        JSONB NOT NULL,     -- field definitions
    defaults      JSONB,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE block_instances (
    id                UUID PRIMARY KEY,
    page_id           UUID REFERENCES pages(id) ON DELETE CASCADE,
    region            VARCHAR(50) NOT NULL,      -- e.g. "hero", "sidebar"
    position          INTEGER NOT NULL DEFAULT 0,
    definition_id     UUID NOT NULL REFERENCES block_definitions(id),
    configuration     JSONB NOT NULL DEFAULT '{}'::JSONB,
    is_global         BOOLEAN DEFAULT FALSE,
    created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE block_translations (
    id                   UUID PRIMARY KEY,
    block_instance_id    UUID NOT NULL REFERENCES block_instances(id) ON DELETE CASCADE,
    locale_id            UUID NOT NULL REFERENCES locales(id),
    content              JSONB NOT NULL,          -- translatable fields
    attribute_overrides  JSONB,                   -- media swaps, link overrides, etc.
    created_at           TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(block_instance_id, locale_id)
);
```

### Widgets
Widgets extend blocks with data-driven behaviour (e.g., search, newsletter signup). They follow the same definition/instance/translation pattern.

```sql
CREATE TABLE widget_definitions (
    id         UUID PRIMARY KEY,
    name       VARCHAR(100) UNIQUE NOT NULL,
    schema     JSONB NOT NULL,
    defaults   JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE widget_instances (
    id              UUID PRIMARY KEY,
    block_instance_id UUID REFERENCES block_instances(id) ON DELETE CASCADE,
    definition_id   UUID NOT NULL REFERENCES widget_definitions(id),
    configuration   JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE widget_translations (
    id                UUID PRIMARY KEY,
    widget_instance_id UUID NOT NULL REFERENCES widget_instances(id) ON DELETE CASCADE,
    locale_id         UUID NOT NULL REFERENCES locales(id),
    content           JSONB NOT NULL,
    created_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(widget_instance_id, locale_id)
);
```

### Menus
Menus provide navigational structures with localized labels.

```sql
CREATE TABLE menus (
    id          UUID PRIMARY KEY,
    code        VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE menu_items (
    id         UUID PRIMARY KEY,
    menu_id    UUID NOT NULL REFERENCES menus(id) ON DELETE CASCADE,
    parent_id  UUID REFERENCES menu_items(id) ON DELETE CASCADE,
    position   INTEGER NOT NULL DEFAULT 0,
    target     JSONB NOT NULL,            -- {"type": "page", "id": "..."}
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE menu_item_translations (
    id            UUID PRIMARY KEY,
    menu_item_id  UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    locale_id     UUID NOT NULL REFERENCES locales(id),
    label         VARCHAR(150) NOT NULL,
    url_override  VARCHAR(255),
    created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(menu_item_id, locale_id)
);
```

### Translation Status
Editorial teams need visibility into translation progress across entities.

```sql
CREATE TABLE translation_status (
    id             UUID PRIMARY KEY,
    entity_type    VARCHAR(50) NOT NULL,  -- e.g. 'page', 'block', 'widget'
    entity_id      UUID NOT NULL,
    locale_id      UUID NOT NULL REFERENCES locales(id),
    status         VARCHAR(20) NOT NULL DEFAULT 'missing',
    completeness   INTEGER NOT NULL DEFAULT 0,
    last_updated   TIMESTAMP NOT NULL DEFAULT NOW(),
    translator_id  UUID,
    reviewer_id    UUID,
    UNIQUE(entity_type, entity_id, locale_id)
);
```

## Locale Configuration (Reference)
Locales are declared in configuration files. A typical configuration file looks like:

```json
{
  "default_locale": "en",
  "locales": {
    "en": { "display_name": "English", "active": true },
    "es": { "display_name": "Español", "active": true, "fallbacks": ["en"] }
  }
}
```

The CMS ingests this file at boot, hydrates locale metadata. The SQL schema holds opaque locale identifiers (`locale_id` references).


### Locale Catalog
Locales are optional for the CMS core, but a simple lookup table keeps metadata aligned with configuration.

```sql
CREATE TABLE locales (
    id            UUID PRIMARY KEY,
    code          VARCHAR(20) UNIQUE NOT NULL,  -- e.g. "en", "es"
    display_name  VARCHAR(100) NOT NULL,
    is_active     BOOLEAN DEFAULT TRUE,
    is_default    BOOLEAN DEFAULT FALSE,
    metadata      JSONB,
    created_at    TIMESTAMP NOT NULL DEFAULT NOW()
);
```

## Entity Relationships
- A page owns many block instances; blocks can be flagged as global to appear on multiple pages.
- Widgets attach to block instances and inherit their placement.
- Menus use hierarchical menu items; translations decorate items with localized labels.
- Locale IDs link all translation tables back to configuration-driven locale metadata.

## Implementation Notes
1. **Configuration First**: Load locale metadata from configuration files and seed the `locales` table accordingly. Keep SQL schema agnostic to regional rules.
2. **JSON Schemas**: `schema` columns describe allowed fields for blocks/widgets; use them to validate editorial input.
3. **Versioning**: `page_versions` captures structural snapshots. Extend the same pattern to blocks if draft/preview workflows require it.
4. **Indices**: Add indexes on `(page_id, region, position)` for block instances, `(menu_id, parent_id, position)` for menu items, and `(entity_type, locale_id)` for translation status to support dashboard queries.
5. **Soft Deletes**: Consider `deleted_at` columns on pages, blocks, and menus if audit requirements demand reversible deletions.

## Repository Integration (go-repository-bun)
### Model Registration
- Register every struct that maps to the tables above before creating the persistence client, including join tables for relations (`persistence.RegisterModel((*Page)(nil), (*PageTranslation)(nil), ...)`).
- If you rely on many-to-many helpers (e.g., widgets shared across pages) ensure the join tables are registered first with `persistence.RegisterMany2ManyModel`.

### Repository Setup
- Initialize the persistence client and wrap its `DB()` with repositories from `github.com/goliatone/go-repository-bun`.
- Model handlers must supply ID getters/setters and, when required, an alternate unique identifier such as a slug or email.

```go
type Page struct {
    bun.BaseModel `bun:"table:pages,alias:p"`
    ID            uuid.UUID  `bun:",pk,type:uuid"`
    Slug          string     `bun:"slug,notnull"`
    Status        string     `bun:"status,notnull,default:'draft'"`
    ParentID      *uuid.UUID `bun:"parent_id"`
    Versions      []*PageVersion     `bun:"rel:has-many,join:id=page_id"`
    Translations  []*PageTranslation `bun:"rel:has-many,join:id=page_id"`
    Blocks        []*BlockInstance   `bun:"rel:has-many,join:id=page_id"`
}

func NewPageRepository(db *bun.DB) repository.Repository[*Page] {
    return repository.MustNewRepository[*Page](db, repository.ModelHandlers[*Page]{
        NewRecord: func() *Page { return &Page{} },
        GetID:     func(p *Page) uuid.UUID { return p.ID },
        SetID:     func(p *Page, id uuid.UUID) { p.ID = id },
        GetIdentifier: func() string { return "slug" },
        GetIdentifierValue: func(p *Page) string {
            if p == nil {
                return ""
            }
            return p.Slug
        },
    })
}
```

### Example Queries
Use repository criteria to keep queries composable and locale-aware.

```go
func (r PageRepository) LoadPage(ctx context.Context, slug, locale string, version *int) (*Page, error) {
    selectors := []repository.SelectCriteria{
        repository.SelectBy("slug", "=", slug),
        repository.SelectRelation("Translations",
            repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
                return q.Join("JOIN locales AS l ON l.id = pt.locale_id").
                    Where("l.code = ?", locale)
            }),
        ),
        repository.SelectRelation("Blocks",
            repository.OrderBy("bi.region ASC", "bi.position ASC"),
            repository.SelectRelation("Translations",
                repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
                    return q.Join("JOIN locales AS l ON l.id = bt.locale_id").
                        Where("l.code = ?", locale)
                }),
            ),
            repository.SelectRelation("Widgets",
                repository.SelectRelation("Translations",
                    repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
                        return q.Join("JOIN locales AS l ON l.id = wt.locale_id").
                            Where("l.code = ?", locale)
                    }),
                ),
            ),
        ),
    }

    selectors = append(selectors, repository.SelectRelation("Versions",
        repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
            if version != nil {
                return q.Where("pv.version = ?", *version)
            }
            return q.Order("pv.version DESC").Limit(1)
        }),
    ))

    return r.repo.Get(ctx, selectors...)
}
```

```go
func (r MenuRepository) GetWithLocale(ctx context.Context, code, locale string) (*Menu, error) {
    return r.repo.Get(ctx,
        repository.SelectBy("code", "=", code),
        repository.SelectRelation("Items",
            repository.OrderBy("mi.parent_id NULLS FIRST", "mi.position ASC"),
            repository.SelectRelation("Translations",
                repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
                    return q.Join("JOIN locales AS l ON l.id = mit.locale_id").
                        Where("l.code = ?", locale)
                }),
            ),
            repository.SelectRelation("Children",
                repository.OrderBy("mi_child.position ASC"),
                repository.SelectRelation("Translations",
                    repository.SelectRawProcessor(func(q *bun.SelectQuery) *bun.SelectQuery {
                        return q.Join("JOIN locales AS l ON l.id = mit.locale_id").
                            Where("l.code = ?", locale)
                    }),
                ),
            ),
        ),
    )
}
```

## Next Steps
- Align the SQL DDL above with migrations in your project.
- Ensure configuration files include all locales used in seed data.
- Implement services that map configuration metadata (fallbacks, display names) onto the relational entities.

The refactored schema keeps localization flexible while concentrating on durable CMS concepts: pages, blocks, widgets, and navigation. As requirements grow, extend configuration files or add optional tables (e.g., scheduling rules, personalization traits) without revisiting the core entity design.
