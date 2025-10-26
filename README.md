# go-cms

A modular headless CMS library for Go providing content management capabilities including pages, blocks, widgets, menus, and internationalization.

## Installation

```bash
go get github.com/goliatone/go-cms
```

## Features

- **Content Management**: Structured content with custom content types and locale support
- **Page Hierarchy**: Nested pages with routing paths and SEO metadata
- **Blocks**: Reusable content fragments with schema definitions and translations
- **Widgets**: Dynamic components with area-based placement and visibility rules
- **Menus**: Navigation structures with URL resolution and internationalization
- **I18N**: Multi-language support with translation management
- **Caching**: Optional caching layer for repository operations
- **Flexible Storage**: Memory-based or SQL-backed (via Bun ORM) repositories

## Quick Start

```go
package main

import (
    "context"
    "github.com/goliatone/go-cms"
    "github.com/goliatone/go-cms/internal/di"
    "github.com/google/uuid"
)

func main() {
    ctx := context.Background()

    // Configure CMS
    cfg := cms.DefaultConfig()
    cfg.DefaultLocale = "en"
    cfg.I18N.Locales = []string{"en", "es"}

    // Create container
    container := di.NewContainer(cfg)

    // Use services
    contentSvc := container.ContentService()
    pageSvc := container.PageService()

    // Create content
    authorID := uuid.New()
    content, err := contentSvc.Create(ctx, content.CreateContentRequest{
        ContentTypeID: typeID,
        Slug:          "example",
        Status:        "published",
        CreatedBy:     authorID,
        UpdatedBy:     authorID,
        Translations: []content.ContentTranslationInput{
            {
                Locale:  "en",
                Title:   "Example Page",
                Content: map[string]any{"body": "Content here"},
            },
        },
    })
}
```

## Architecture

### Module Structure

```
internal/
├── content/     # Content entities and content types
├── pages/       # Page hierarchy and routing
├── blocks/      # Reusable content fragments
├── widgets/     # Dynamic behavioral components
├── menus/       # Navigation structures
├── i18n/        # Internationalization
├── adapters/    # External integrations
└── di/          # Dependency injection container

pkg/
├── interfaces/  # External dependency abstractions
└── testsupport/ # Shared test utilities
```

### Repository Pattern

Each domain module provides two repository implementations:

- **Memory**: In-memory storage for testing or simple use cases
- **Bun**: SQL-backed storage using uptrace/bun ORM with optional caching

The container automatically selects the appropriate implementation based on configuration.

### Dependency Injection

The `di.Container` wires all dependencies. Override defaults using functional options:

```go
container := di.NewContainer(cfg,
    di.WithBunDB(db),                    // Use SQL storage
    di.WithCache(cache, serializer),     // Custom cache
    di.WithPageService(customPageSvc),   // Custom service
)
```

## Configuration

```go
cfg := cms.DefaultConfig()

// Content settings
cfg.DefaultLocale = "en"
cfg.Content.PageHierarchy = true

// I18N settings
cfg.I18N.Enabled = true
cfg.I18N.Locales = []string{"en", "es", "fr"}

// Storage
cfg.Storage.Provider = "bun"  // or "memory"

// Caching
cfg.Cache.Enabled = true
cfg.Cache.DefaultTTL = time.Minute * 5

// Features
cfg.Features.Widgets = true

// Navigation (requires go-urlkit)
cfg.Navigation.RouteConfig = &urlkit.Config{...}
cfg.Navigation.URLKit.DefaultGroup = "frontend"
cfg.Navigation.URLKit.LocaleGroups = map[string]string{
    "es": "frontend.es",
}
```

## Usage Examples

### Creating Pages with Blocks

```go
// Register block definition
definition, _ := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
    Name: "hero",
    Schema: map[string]any{
        "fields": []any{"title", "body"},
    },
})

// Create page
page, _ := pageSvc.Create(ctx, pages.CreatePageRequest{
    ContentID:  contentID,
    TemplateID: templateID,
    Slug:       "about",
    Status:     "published",
    CreatedBy:  authorID,
    UpdatedBy:  authorID,
    Translations: []pages.PageTranslationInput{
        {Locale: "en", Title: "About Us", Path: "/about"},
        {Locale: "es", Title: "Acerca de", Path: "/es/acerca-de"},
    },
})

// Add block instance to page
instance, _ := blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
    DefinitionID: definition.ID,
    PageID:       &page.ID,
    Region:       "hero",
    Position:     0,
    CreatedBy:    authorID,
    UpdatedBy:    authorID,
})
```

### Widget Areas

```go
// Bootstrap widget areas
widgets.Bootstrap(ctx, widgetSvc, widgets.BootstrapConfig{
    Areas: []widgets.RegisterAreaDefinitionInput{
        {Code: "sidebar.primary", Name: "Primary Sidebar", Scope: widgets.AreaScopeGlobal},
    },
})

// Create widget instance
widget, _ := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
    DefinitionID: definitionID,
    Configuration: map[string]any{"headline": "Welcome"},
    VisibilityRules: map[string]any{"audience": []any{"guest"}},
    CreatedBy: authorID,
    UpdatedBy: authorID,
})

// Assign to area
widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
    AreaCode:   "sidebar.primary",
    InstanceID: widget.ID,
    Position:   intPtr(0),
})

// Resolve widgets for area
resolved, _ := widgetSvc.ResolveArea(ctx, widgets.ResolveAreaInput{
    AreaCode: "sidebar.primary",
    Audience: []string{"guest"},
    Now:      time.Now().UTC(),
})
```

### Menus with URL Resolution

```go
// Create menu
menu, _ := menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
    Code:      "primary",
    CreatedBy: authorID,
    UpdatedBy: authorID,
})

// Add menu item
menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
    MenuID:   menu.ID,
    Position: 0,
    Target: map[string]any{
        "type": "page",
        "slug": "about",
    },
    CreatedBy: authorID,
    UpdatedBy: authorID,
    Translations: []menus.MenuItemTranslationInput{
        {Locale: "en", Label: "About"},
        {Locale: "es", Label: "Acerca de"},
    },
})

// Resolve navigation with URLs
nav, _ := menuSvc.ResolveNavigation(ctx, "primary", "en")
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./internal/content/...
go test ./internal/pages/...
go test ./internal/blocks/...
go test ./internal/widgets/...
go test ./internal/themes/...
go test ./internal/menus/...

# Run with coverage
./taskfile dev:cover

# Run integration tests (requires database)
go test -v ./internal/pages/... -run Integration
```

## Requirements

- Go 1.24+
- Optional: Database supported by uptrace/bun (PostgreSQL, MySQL, SQLite)

## Dependencies

- [github.com/uptrace/bun](https://github.com/uptrace/bun) - SQL ORM
- [github.com/goliatone/go-urlkit](https://github.com/goliatone/go-urlkit) - URL routing (for menu resolution)
- [github.com/goliatone/go-repository-cache](https://github.com/goliatone/go-repository-cache) - Repository caching
- [github.com/google/uuid](https://github.com/google/uuid) - UUID generation

## Example

See [cmd/example/main.go](cmd/example/main.go) for a complete working example demonstrating all major features.

## License

See [LICENSE](LICENSE) file for details.
