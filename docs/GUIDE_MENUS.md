# Menus Guide

This guide covers navigation structures in `go-cms`: creating menus, managing menu items with hierarchy and translations, resolving navigation trees for rendering, and configuring URL resolution with `go-urlkit`. By the end you will understand the full menu lifecycle from bootstrapping to localized navigation output.

## Menu Architecture Overview

Menus in `go-cms` model navigation structures as hierarchical trees of items. Each menu is identified by a unique code (e.g., `"primary"`, `"footer"`, `"admin"`) and contains items organized in a parent-child hierarchy using dot-path identifiers.

```
Menu (code, location, description)
  ├── MenuItem (path: "primary.home", type: item)
  │     └── MenuItemTranslation (locale: "en", label: "Home")
  │     └── MenuItemTranslation (locale: "es", label: "Inicio")
  ├── MenuItem (path: "primary.about", type: group, collapsible)
  │     ├── MenuItemTranslation (locale: "en", label: "About")
  │     ├── MenuItem (path: "primary.about.team", type: item)
  │     └── MenuItem (path: "primary.about.history", type: item)
  └── MenuItem (path: "primary.sep1", type: separator)
```

Three entity types compose a menu:

- **Menu** -- a named container identified by code. Optionally bound to a theme location.
- **MenuItem** -- a node in the tree. Items are identified by dot-paths (e.g., `"primary.about.team"`), have a type (`item`, `group`, or `separator`), and carry type-specific data such as targets, icons, badges, and permissions.
- **MenuItemTranslation** -- per-locale labels and optional URL overrides for a menu item.

The public API addresses menus and items by code and path, never by UUID. UUIDs are internal implementation details.

### Accessing the Service

Menu operations are exposed through the `cms.Module` facade:

```go
cfg := cms.DefaultConfig()
cfg.DefaultLocale = "en"
cfg.I18N.Locales = []string{"en", "es"}

module, err := cms.New(cfg)
if err != nil {
    log.Fatal(err)
}

menuSvc := module.Menus()
```

The `menuSvc` variable satisfies the `cms.MenuService` interface. The service delegates to in-memory repositories by default, or SQL-backed repositories when `di.WithBunDB(db)` is provided.

---

## Menu CRUD

### Creating a Menu

Menus are created implicitly through `GetOrCreateMenu` or `UpsertMenu`. Both accept a code and return the menu, creating it if it does not exist:

```go
actor := uuid.New()
description := "Main site navigation"

// GetOrCreateMenu -- creates if absent, returns existing if present
menu, err := menuSvc.GetOrCreateMenu(ctx, "primary", &description, actor)
```

To also bind a menu to a theme location (e.g., `"header"`, `"footer"`), use `GetOrCreateMenuWithLocation`:

```go
menu, err := menuSvc.GetOrCreateMenuWithLocation(ctx, "footer", "footer", &description, actor)
```

`UpsertMenu` behaves similarly but will update the description if the menu already exists:

```go
menu, err := menuSvc.UpsertMenu(ctx, "primary", &description, actor)
menu, err := menuSvc.UpsertMenuWithLocation(ctx, "primary", "header", &description, actor)
```

**Menu codes** are canonicalized: lowercased, trimmed, and restricted to letters, numbers, hyphens, and underscores. `"Primary"` becomes `"primary"`, `"My-Menu"` becomes `"my-menu"`.

### Querying Menus

```go
// By code
menu, err := menuSvc.GetMenuByCode(ctx, "primary")

// By theme location
menu, err := menuSvc.GetMenuByLocation(ctx, "header")

// List all items for a menu
items, err := menuSvc.ListMenuItemsByCode(ctx, "primary")
```

All methods return `cms.MenuInfo`:

| Field | Type | Description |
|-------|------|-------------|
| `Code` | `string` | Unique menu identifier |
| `Location` | `string` | Optional theme location binding |
| `Description` | `*string` | Human-readable description |

### Deleting and Resetting Menus

```go
// Reset removes all items from a menu, keeping the menu record
err := menuSvc.ResetMenuByCode(ctx, "primary", actor, false)

// Force reset ignores theme binding checks
err := menuSvc.ResetMenuByCode(ctx, "primary", actor, true)
```

If a menu is bound to an active theme, reset and delete operations fail unless `force` is `true`. This prevents accidentally breaking a live site's navigation.

---

## Menu Items

### Item Types

Three item types are supported:

| Type | Description | Allowed fields |
|------|-------------|----------------|
| `item` | Regular navigational link | Target, icon, badge, children, permissions, collapsible |
| `group` | Non-clickable container for children | Label, children, collapsible; **no** target, icon, or badge |
| `separator` | Visual divider between sections | Position only; **no** target, children, labels, icons, or badges |

### Creating and Upserting Items

Items are identified by dot-paths. The first segment is the menu code, subsequent segments form the hierarchy:

```
"primary.home"           -> menu: primary, root item: home
"primary.about.team"     -> menu: primary, parent: about, item: team
"admin.content.pages"    -> menu: admin, parent: content, item: pages
```

Use `UpsertMenuItemByPath` to create or update an item idempotently:

```go
item, err := menuSvc.UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
    Path:     "primary.home",
    Type:     "item",
    Position: intPtr(0),
    Target: map[string]any{
        "type": "page",
        "slug": "home",
    },
    Icon: "home",
    Translations: []cms.MenuItemTranslationInput{
        {Locale: "en", Label: "Home"},
        {Locale: "es", Label: "Inicio"},
    },
    Actor: actor,
})
```

**`UpsertMenuItemByPathInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Path` | `string` | Yes | Dot-path identifier (e.g., `"primary.about.team"`) |
| `ParentPath` | `string` | No | Override parent path; derived from `Path` if omitted |
| `Type` | `string` | Yes | `"item"`, `"group"`, or `"separator"` |
| `Position` | `*int` | No | 0-based insertion index; `nil` appends to end |
| `Target` | `map[string]any` | Conditional | Required for `item`; prohibited for `group`/`separator` |
| `Icon` | `string` | No | Icon identifier (e.g., `"home"`, `"settings"`) |
| `Badge` | `map[string]any` | No | Badge data (e.g., `{"count": 5, "color": "red"}`) |
| `Permissions` | `[]string` | No | Required permissions to see this item |
| `Classes` | `[]string` | No | CSS classes for styling |
| `Styles` | `map[string]string` | No | Inline styles |
| `Collapsible` | `bool` | No | Whether the item can be collapsed (requires children) |
| `Collapsed` | `bool` | No | Initial collapsed state (requires `Collapsible`) |
| `Metadata` | `map[string]any` | No | Arbitrary key-value data |
| `Translations` | `[]MenuItemTranslationInput` | Conditional | At least one required unless `AllowMissingTranslations` is set |
| `AllowMissingTranslations` | `bool` | No | Bypass translation requirement |
| `Actor` | `uuid.UUID` | Yes | Actor performing the operation |

### Updating Items

Update an existing item by path:

```go
updated, err := menuSvc.UpdateMenuItemByPath(ctx, "primary", "primary.home", cms.UpdateMenuItemByPathInput{
    Icon:  stringPtr("house"),
    Actor: actor,
})
```

`UpdateMenuItemByPathInput` uses pointer fields for partial updates. A `nil` field means "leave unchanged":

| Field | Type | Description |
|-------|------|-------------|
| `ParentPath` | `*string` | New parent path; `nil` = unchanged |
| `Type` | `*string` | New type; `nil` = unchanged |
| `Position` | `*int` | New position; `nil` = unchanged |
| `Target` | `map[string]any` | New target data |
| `Icon` | `*string` | New icon |
| `Badge` | `map[string]any` | New badge |
| `Permissions` | `[]string` | New permissions |
| `Classes` | `[]string` | New CSS classes |
| `Styles` | `map[string]string` | New inline styles |
| `Collapsible` | `*bool` | New collapsible state |
| `Collapsed` | `*bool` | New collapsed state |
| `Metadata` | `map[string]any` | New metadata |
| `Actor` | `uuid.UUID` | Actor performing the operation |

### Deleting Items

```go
// Delete a single item (fails if it has children)
err := menuSvc.DeleteMenuItemByPath(ctx, "primary", "primary.about.team", actor, false)

// Cascade delete: removes the item and all descendants
err := menuSvc.DeleteMenuItemByPath(ctx, "primary", "primary.about", actor, true)
```

When `cascadeChildren` is `false` and the item has children, the operation returns `ErrMenuItemHasChildren`.

### Reordering Items

Move items within their sibling list:

```go
// Move to top of siblings
err := menuSvc.MoveMenuItemToTop(ctx, "primary", "primary.contact", actor)

// Move to bottom of siblings
err := menuSvc.MoveMenuItemToBottom(ctx, "primary", "primary.contact", actor)

// Move before another sibling
err := menuSvc.MoveMenuItemBefore(ctx, "primary", "primary.contact", "primary.about", actor)

// Move after another sibling
err := menuSvc.MoveMenuItemAfter(ctx, "primary", "primary.contact", "primary.home", actor)

// Set exact sibling order under a parent
err := menuSvc.SetMenuSiblingOrder(ctx, "primary", "primary.about", []string{
    "primary.about.history",
    "primary.about.team",
    "primary.about.values",
}, actor)
```

`SetMenuSiblingOrder` accepts a slice of sibling paths in the desired order. Items not included in the list retain their relative positions after the listed items.

---

## Menu Item Translations

Each menu item supports per-locale translations. Translations carry labels, i18n keys, and optional URL overrides.

### Adding Translations

Translations are typically provided inline when upserting items. To add or update a translation separately:

```go
err := menuSvc.UpsertMenuItemTranslationByPath(ctx, "primary", "primary.home", cms.MenuItemTranslationInput{
    Locale:   "fr",
    Label:    "Accueil",
    LabelKey: "nav.home", // optional i18n key for host-side translation
})
```

**`MenuItemTranslationInput` fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Locale` | `string` | Yes | Locale code (must be in `cfg.I18N.Locales`) |
| `Label` | `string` | Conditional | Display label; required unless `LabelKey` is provided |
| `LabelKey` | `string` | No | i18n key for host-side translation |
| `GroupTitle` | `string` | No | Section title for `group` items |
| `GroupTitleKey` | `string` | No | i18n key for group title |
| `URLOverride` | `*string` | No | Override the resolved URL for this locale |

### Translation Resolution Precedence

When resolving navigation, translations are applied with this precedence:

1. **`LabelKey`** -- if present, passed through for the host application to translate
2. **`Label`** -- fallback display text when the host does not translate the key

The same precedence applies to `GroupTitleKey` / `GroupTitle` for group items. If a group item lacks group-specific fields, it falls back to `Label` / `LabelKey`.

### URL Overrides

`URLOverride` lets you provide a locale-specific URL that replaces the auto-resolved URL. This is useful when a locale uses a different domain or path structure:

```go
err := menuSvc.UpsertMenuItemTranslationByPath(ctx, "primary", "primary.home", cms.MenuItemTranslationInput{
    Locale:      "es",
    Label:       "Inicio",
    URLOverride: stringPtr("https://es.example.com/"),
})
```

---

## Out-of-Order Upserts

When bootstrapping menus from configuration files or seed scripts, items may reference parents that have not been created yet. Enable forgiving mode with:

```go
cfg.Menus.AllowOutOfOrderUpserts = true
```

When enabled:

1. **Deferred parent resolution** -- items referencing non-existent parents are stored with a `ParentRef` (the parent's external code). They appear as root-level items until reconciled.
2. **Automatic reconciliation** -- after writes and before navigation resolution, the service runs reconciliation to link deferred items to their now-existing parents.
3. **Collapsible intent** -- parent items declared as collapsible are persisted before their children, so collapsible state is preserved regardless of insertion order.

This lets you define menus in any order without worrying about dependency ordering.

### Manual Reconciliation

You can also trigger reconciliation explicitly:

```go
result, err := menuSvc.ReconcileMenuByCode(ctx, "primary", actor)
fmt.Printf("Resolved: %d, Remaining: %d\n", result.Resolved, result.Remaining)
```

`ReconcileMenuResult` tells you how many items were linked and how many still have unresolved parents.

---

## Navigation Resolution

Navigation resolution transforms a menu's raw items into a localized, hierarchical tree ready for rendering. This is the primary read path for consuming menus.

### Resolving by Menu Code

```go
nodes, err := menuSvc.ResolveNavigation(ctx, "primary", "en")
if err != nil {
    log.Fatal(err)
}

for _, node := range nodes {
    fmt.Printf("%s: %s -> %s\n", node.Type, node.Label, node.URL)
    for _, child := range node.Children {
        fmt.Printf("  %s: %s -> %s\n", child.Type, child.Label, child.URL)
    }
}
```

### Resolving by Theme Location

When menus are bound to theme locations, resolve by location instead of code:

```go
nodes, err := menuSvc.ResolveNavigationByLocation(ctx, "header", "en")
```

### NavigationNode Structure

Each node in the resolved tree contains:

| Field | Type | Description |
|-------|------|-------------|
| `Position` | `int` | 0-based sibling order |
| `Type` | `string` | `"item"`, `"group"`, or `"separator"` |
| `Label` | `string` | Translated display label |
| `LabelKey` | `string` | i18n key for host-side translation |
| `GroupTitle` | `string` | Section title (group items only) |
| `GroupTitleKey` | `string` | i18n key for group title |
| `URL` | `string` | Resolved URL (from target + URL resolver) |
| `Target` | `map[string]any` | Raw target data |
| `Icon` | `string` | Icon identifier |
| `Badge` | `map[string]any` | Badge data |
| `Permissions` | `[]string` | Required permissions |
| `Classes` | `[]string` | CSS classes |
| `Styles` | `map[string]string` | Inline styles |
| `Collapsible` | `bool` | Whether the node can be collapsed |
| `Collapsed` | `bool` | Initial collapsed state |
| `Metadata` | `map[string]any` | Arbitrary metadata |
| `Children` | `[]NavigationNode` | Child nodes (recursive) |

### Resolution Steps

When `ResolveNavigation` is called:

1. Load the menu, its items, and all translations
2. For each item, select the translation matching the requested locale
3. Apply translation precedence rules (key -> label fallback)
4. Resolve URLs via the configured URL resolver (see next section)
5. Apply `URLOverride` from translation if present
6. Build the parent-child hierarchy
7. Normalize separators (strip leading, trailing, and consecutive separators)
8. Return the `[]NavigationNode` tree

---

## URL Resolution with go-urlkit

Menu items with `"type": "page"` targets have their URLs resolved dynamically using `go-urlkit`. This enables locale-aware, route-based URL generation.

### Configuration

Configure URL resolution through `cfg.Navigation`:

```go
import "github.com/goliatone/go-urlkit"

cfg := cms.DefaultConfig()
cfg.Navigation.RouteConfig = &urlkit.Config{
    Groups: []urlkit.GroupConfig{
        {
            Name:   "frontend",
            Prefix: "",
            Routes: []urlkit.RouteConfig{
                {Name: "page", Path: "/{slug}"},
                {Name: "home", Path: "/"},
            },
        },
        {
            Name:   "frontend.es",
            Prefix: "/es",
            Routes: []urlkit.RouteConfig{
                {Name: "page", Path: "/{slug}"},
                {Name: "home", Path: "/"},
            },
        },
    },
}
cfg.Navigation.URLKit = cms.URLKitResolverConfig{
    DefaultGroup: "frontend",
    LocaleGroups: map[string]string{
        "es": "frontend.es",
    },
    DefaultRoute: "page",
    SlugParam:    "slug",
}
```

### How URL Resolution Works

When a menu item has a target like:

```go
Target: map[string]any{
    "type": "page",
    "slug": "about-us",
}
```

The resolver:

1. Extracts the slug from `Target["slug"]`
2. Selects the route group for the requested locale (e.g., `"frontend.es"` for Spanish)
3. Builds the URL using the configured route pattern (e.g., `/{slug}` -> `/es/about-us`)
4. Returns the resolved URL string

### URLKit Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `DefaultGroup` | `string` | Base route group (e.g., `"frontend"`) |
| `LocaleGroups` | `map[string]string` | Maps locale codes to route groups |
| `DefaultRoute` | `string` | Route name for page links (e.g., `"page"`) |
| `SlugParam` | `string` | Parameter name for page slugs (default: `"slug"`) |
| `LocaleParam` | `string` | Parameter name for locale codes |
| `LocaleIDParam` | `string` | Parameter name for locale UUIDs |
| `RouteField` | `string` | Target field containing route name (default: `"route"`) |
| `ParamsField` | `string` | Target field containing route params (default: `"params"`) |
| `QueryField` | `string` | Target field containing query string params (default: `"query"`) |

### Custom Targets

Items can use custom route names and parameters through their target:

```go
// Using a named route with parameters
item, err := menuSvc.UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
    Path: "primary.blog",
    Type: "item",
    Target: map[string]any{
        "type":   "page",
        "route":  "blog-post",
        "params": map[string]any{"category": "tech", "slug": "hello"},
    },
    Translations: []cms.MenuItemTranslationInput{
        {Locale: "en", Label: "Blog"},
    },
    Actor: actor,
})

// External URL (no resolution needed)
item, err := menuSvc.UpsertMenuItemByPath(ctx, cms.UpsertMenuItemByPathInput{
    Path: "primary.docs",
    Type: "item",
    Target: map[string]any{
        "type": "url",
        "href": "https://docs.example.com",
    },
    Translations: []cms.MenuItemTranslationInput{
        {Locale: "en", Label: "Documentation"},
    },
    Actor: actor,
})
```

---

## Seeding Menus

For bootstrapping menus from configuration or seed scripts, use `cms.SeedMenu`:

```go
actor := uuid.New()

err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
    Menus:    menuSvc,
    MenuCode: "primary",
    Description: stringPtr("Primary site navigation"),
    Locale:   "en",
    Actor:    actor,
    Items: []cms.SeedMenuItem{
        {
            Path:     "primary.home",
            Position: intPtr(0),
            Type:     "item",
            Target:   map[string]any{"type": "page", "slug": "home"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Home"},
                {Locale: "es", Label: "Inicio"},
            },
        },
        {
            Path:     "primary.about",
            Position: intPtr(1),
            Type:     "group",
            Collapsible: true,
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "About", GroupTitle: "About Us"},
                {Locale: "es", Label: "Acerca", GroupTitle: "Sobre Nosotros"},
            },
        },
        {
            Path:     "primary.about.team",
            Position: intPtr(0),
            Type:     "item",
            Target:   map[string]any{"type": "page", "slug": "team"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Our Team"},
                {Locale: "es", Label: "Nuestro Equipo"},
            },
        },
        {
            Path:     "primary.about.history",
            Position: intPtr(1),
            Type:     "item",
            Target:   map[string]any{"type": "page", "slug": "history"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "History"},
                {Locale: "es", Label: "Historia"},
            },
        },
        {
            Path:     "primary.sep1",
            Position: intPtr(2),
            Type:     "separator",
            AllowMissingTranslations: true,
        },
        {
            Path:     "primary.contact",
            Position: intPtr(3),
            Type:     "item",
            Target:   map[string]any{"type": "page", "slug": "contact"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Contact"},
                {Locale: "es", Label: "Contacto"},
            },
        },
    },
    AutoCreateParents: true,
    Ensure:            true,
})
```

**`SeedMenuOptions` fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Menus` | `MenuService` | The menu service to use |
| `MenuCode` | `string` | Menu code to create/update |
| `Description` | `*string` | Menu description |
| `Locale` | `string` | Default locale for translations |
| `Actor` | `uuid.UUID` | Actor performing the seed |
| `Items` | `[]SeedMenuItem` | Items to create |
| `AutoCreateParents` | `bool` | Auto-scaffold missing parent groups |
| `Ensure` | `bool` | Run reconciliation and enforce ordering after seeding |
| `PruneUnspecified` | `bool` | Delete items not in the spec |

When `Ensure` is `true`, `SeedMenu` runs reconciliation after upserting all items to link deferred parents and enforce the specified ordering. When `PruneUnspecified` is `true`, items present in the database but missing from the seed spec are deleted.

---

## Cache Invalidation

Menu navigation resolution results are cached by default. When you modify a menu's items or translations, the cache is invalidated automatically. To invalidate manually:

```go
err := menuSvc.InvalidateCache(ctx)
```

This is useful when external changes (e.g., page slug updates) affect resolved URLs but are not made through the menu service.

---

## Error Handling

The menu service returns typed errors for common failure cases:

| Error | Cause |
|-------|-------|
| `cms.ErrMenuCodeRequired` | Empty menu code |
| `cms.ErrMenuNotFound` | Menu code does not exist |
| `cms.ErrMenuInUse` | Menu is bound to an active theme (delete/reset blocked) |
| `cms.ErrMenuItemPathRequired` | Empty item path |
| `cms.ErrMenuItemPathInvalid` | Path does not match `<menuCode>.<segment>...` format |
| `cms.ErrMenuItemPathMismatch` | Path prefix does not match the menu code |

Additional validation errors from the internal service:

| Error | Cause |
|-------|-------|
| `ErrMenuItemTypeInvalid` | Type is not `"item"`, `"group"`, or `"separator"` |
| `ErrMenuItemTargetMissing` | `item` type missing target data |
| `ErrMenuItemGroupFields` | `group` has target, icon, or badge |
| `ErrMenuItemSeparatorFields` | `separator` has target, children, labels, icons, or badges |
| `ErrMenuItemHasChildren` | Deleting an item with children without cascade |
| `ErrMenuItemCycle` | Reparenting would create a hierarchy cycle |
| `ErrMenuItemCollapsibleWithoutChildren` | `Collapsible` set on an item without children |
| `ErrMenuItemTranslations` | No translations provided when required |
| `ErrMenuItemDuplicateLocale` | Same locale provided twice in translations |
| `ErrUnknownLocale` | Locale not in `cfg.I18N.Locales` |

---

## Common Patterns

### Primary Site Navigation

A typical site navigation with pages, groups, and external links:

```go
err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
    Menus:    menuSvc,
    MenuCode: "primary",
    Locale:   "en",
    Actor:    actor,
    Items: []cms.SeedMenuItem{
        {Path: "primary.home", Position: intPtr(0), Type: "item",
            Target: map[string]any{"type": "page", "slug": "home"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Home"},
            }},
        {Path: "primary.products", Position: intPtr(1), Type: "group",
            Collapsible: true,
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Products", GroupTitle: "Our Products"},
            }},
        {Path: "primary.products.software", Position: intPtr(0), Type: "item",
            Target: map[string]any{"type": "page", "slug": "software"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Software"},
            }},
        {Path: "primary.products.hardware", Position: intPtr(1), Type: "item",
            Target: map[string]any{"type": "page", "slug": "hardware"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Hardware"},
            }},
        {Path: "primary.blog", Position: intPtr(2), Type: "item",
            Target: map[string]any{"type": "url", "href": "https://blog.example.com"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Blog"},
            }},
    },
    Ensure: true,
})
```

### Footer Navigation

A simpler flat menu for footer links:

```go
err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
    Menus:    menuSvc,
    MenuCode: "footer",
    Locale:   "en",
    Actor:    actor,
    Items: []cms.SeedMenuItem{
        {Path: "footer.privacy", Position: intPtr(0), Type: "item",
            Target: map[string]any{"type": "page", "slug": "privacy-policy"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Privacy Policy"},
            }},
        {Path: "footer.terms", Position: intPtr(1), Type: "item",
            Target: map[string]any{"type": "page", "slug": "terms-of-service"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Terms of Service"},
            }},
        {Path: "footer.contact", Position: intPtr(2), Type: "item",
            Target: map[string]any{"type": "page", "slug": "contact"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Contact Us"},
            }},
    },
    Ensure: true,
})
```

### Multi-Locale Navigation

Resolving the same menu in different locales:

```go
// Resolve English navigation
navEN, err := menuSvc.ResolveNavigation(ctx, "primary", "en")

// Resolve Spanish navigation (URLs use locale-specific routes)
navES, err := menuSvc.ResolveNavigation(ctx, "primary", "es")

// Each locale gets its own labels and URL paths
// EN: "About Us" -> "/about-us"
// ES: "Sobre Nosotros" -> "/es/sobre-nosotros"
```

### Admin Sidebar Navigation

A collapsible admin menu with sections, permissions, and badges:

```go
err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
    Menus:    menuSvc,
    MenuCode: "admin",
    Locale:   "en",
    Actor:    actor,
    Items: []cms.SeedMenuItem{
        {Path: "admin.main", Position: intPtr(0), Type: "group",
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", GroupTitle: "Main Menu", GroupTitleKey: "admin.section.main"},
            }},
        {Path: "admin.main.dashboard", Position: intPtr(0), Type: "item",
            Target: map[string]any{"type": "page", "slug": "dashboard"},
            Icon: "dashboard",
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Dashboard", LabelKey: "admin.nav.dashboard"},
            }},
        {Path: "admin.main.content", Position: intPtr(1), Type: "item",
            Target:      map[string]any{"type": "page", "slug": "content"},
            Icon:        "file-text",
            Collapsible: true,
            Permissions: []string{"content:read"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Content", LabelKey: "admin.nav.content"},
            }},
        {Path: "admin.main.content.pages", Position: intPtr(0), Type: "item",
            Target:      map[string]any{"type": "page", "slug": "pages"},
            Permissions: []string{"pages:read"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Pages", LabelKey: "admin.nav.pages"},
            }},
        {Path: "admin.sep1", Position: intPtr(1), Type: "separator",
            AllowMissingTranslations: true},
        {Path: "admin.others", Position: intPtr(2), Type: "group",
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", GroupTitle: "Others", GroupTitleKey: "admin.section.others"},
            }},
        {Path: "admin.others.settings", Position: intPtr(0), Type: "item",
            Target:      map[string]any{"type": "page", "slug": "settings"},
            Icon:        "settings",
            Permissions: []string{"admin:settings"},
            Translations: []cms.MenuItemTranslationInput{
                {Locale: "en", Label: "Settings", LabelKey: "admin.nav.settings"},
            }},
    },
    AutoCreateParents: true,
    Ensure:            true,
})
```

### Rendering Navigation in Templates

When using the static generator, resolved navigation is available in templates:

```go
// In your generator setup, resolve navigation and pass to templates
navEN, _ := menuSvc.ResolveNavigation(ctx, "primary", "en")

// Template data
data := map[string]any{
    "navigation": navEN,
}
```

In templates (Go `html/template`):

```html
<nav>
    <ul>
    {{ range .navigation }}
        {{ if eq .Type "separator" }}
            <li class="separator"><hr></li>
        {{ else if eq .Type "group" }}
            <li class="group">
                <span class="group-title">{{ .GroupTitle }}</span>
                {{ if .Children }}
                <ul>
                    {{ range .Children }}
                    <li><a href="{{ .URL }}">{{ .Label }}</a></li>
                    {{ end }}
                </ul>
                {{ end }}
            </li>
        {{ else }}
            <li>
                <a href="{{ .URL }}">
                    {{ if .Icon }}<i class="icon-{{ .Icon }}"></i>{{ end }}
                    {{ .Label }}
                </a>
                {{ if .Children }}
                <ul>
                    {{ range .Children }}
                    <li><a href="{{ .URL }}">{{ .Label }}</a></li>
                    {{ end }}
                </ul>
                {{ end }}
            </li>
        {{ end }}
    {{ end }}
    </ul>
</nav>
```

---

## Path Utilities

`go-cms` exports utility functions for working with menu item paths:

```go
// Canonicalize a menu code
code := cms.CanonicalMenuCode("Primary") // "primary"

// Parse a menu item path
parsed, err := cms.ParseMenuItemPath("primary.about.team")
// parsed.Path       = "primary.about.team"
// parsed.MenuCode   = "primary"
// parsed.ParentPath = "primary.about"
// parsed.Key        = "team"

// Canonicalize a full item path
path, err := cms.CanonicalMenuItemPath("primary", "About.Team")
// path = "primary.about.team"

// Derive path and parent from components
derived, err := cms.DeriveMenuItemPaths("primary", "team", "about", "Team")
// derived.Path       = "primary.about.team"
// derived.ParentPath = "primary.about"
```

---

## Next Steps

- **GUIDE_I18N.md** -- internationalization, locale management, and translation workflows
- **GUIDE_THEMES.md** -- theme management, template registration, and asset resolution
- **GUIDE_STATIC_GENERATION.md** -- building static sites with locale-aware rendering
- **GUIDE_CONFIGURATION.md** -- full config reference and DI container wiring
- `cmd/example/main.go` -- comprehensive example exercising menus alongside content, pages, and themes
- `docs/MENU_TDD.md` -- technical design notes for the menu module
