# go-cms Web Example

This example demonstrates how to use go-cms with a web interface powered by [go-router](https://github.com/goliatone/go-router) and the Fiber adapter.

The example uses the Django template engine (via gofiber/template/django) for HTML rendering, following the same pattern as the go-router examples.

## Features Demonstrated

This example showcases real-world usage of go-cms entities:

### Content Types
- **Page**: Basic pages with rich text body
- **Blog Post**: Posts with author, excerpt, tags, and featured image
- **Product**: Products with description, price, features, and specs

### Blocks (Reusable Page Components)
- **Hero Block**: Eye-catching banner with title, subtitle, CTA, and background image
- **Features Grid**: Showcases key features with icons and descriptions
- **Call-to-Action**: Compelling CTAs with headline, description, and button

### Widgets (Dynamic Sidebar Components)
- **Newsletter Widget**: Subscription form with headline and description (guest-only)
- **Promo Banner**: Time-limited promotional offers with badges and scheduling

### Other Features
- **Pages**: Hierarchical page management with content
- **Menus**: Navigation structures with internationalization
- **Themes**: Template system with regions
- **i18n**: Multi-language support (English and Spanish)

## Running the Example

From the `examples/web` directory:

```bash
# Install dependencies
go mod download

# Run the server (IMPORTANT: use '.' not 'main.go')
go run .

# Or build and run
go build -o web-server .
./web-server
```

The server will start on `http://localhost:3000`.

## Available Routes

### Web Pages

- `GET /` - Home page with feature overview
- `GET /pages/:slug` - View a page by slug
- `GET /switch-locale/:locale` - Switch locale (en/es)

### API Endpoints

- `GET /api/pages` - List all pages
- `GET /api/pages/:id` - Get page by ID
- `GET /api/menus/:code` - Get menu by code (e.g., `/api/menus/primary?locale=en`)

## Project Structure

```
examples/web/
├── main.go                      # Main application entry point
├── go.mod                       # Go module dependencies
├── README.md                    # This file
├── views/                       # HTML templates (Django syntax)
│   ├── layout.html              # Base layout template
│   ├── index.html               # Home page template
│   ├── page.html                # Page detail template
│   └── error.html               # Error page template
└── static/                      # Static assets
    ├── css/
    │   └── style.css            # Stylesheet
    └── js/
        └── main.js              # JavaScript
```

## Template System

The example uses the Django template engine from `gofiber/template/django/v3` for HTML rendering. The template engine is configured when creating the Fiber adapter:

```go
engine := django.New("./views", ".html")
server := router.NewFiberAdapter(func(a *fiber.App) *fiber.App {
    return fiber.New(fiber.Config{
        Views: engine,
        PassLocalsToViews: true,
    })
})
```

### Templates

Templates use Django/Jinja2 syntax with `{% %}` for logic and `{{ }}` for variables:

- **[views/layout.html](views/layout.html)**: Base layout with header, navigation, and footer
- **[views/index.html](views/index.html)**: Home page showing features and available pages
- **[views/page.html](views/page.html)**: Individual page view with content, blocks, and widgets
- **[views/error.html](views/error.html)**: Error page template

## Demo Data

The example automatically creates comprehensive demo data including:

### Content Types
- **page**: Basic page content type with rich text body
- **blog_post**: Blog posts with excerpt, author, tags, and featured image
- **product**: Product content type with price, features, and specifications

### Pages with Blocks
- **About Page**: Demonstrates hero and features grid blocks
  - Hero block with title, subtitle, and CTA
  - Features grid showcasing 4 key features with icons
- **Blog Post**: "Getting Started with go-cms"
  - Call-to-action block encouraging users to view documentation
  - Full i18n support with Spanish translations

### Widgets
- **Newsletter Widget**: Subscription form (visible to guests only)
- **Promo Banner**: Limited-time offer with badge and 30-day expiration

### Navigation
- Primary menu with Home, About, and Blog links
- Full internationalization support (English and Spanish)

## Internationalization

Switch between English and Spanish using the language switcher in the header, or by adding `?locale=es` to any URL.

## Development

To modify the example:

1. Edit templates in `views/` for UI changes (Django/Jinja2 syntax)
2. Edit styles in `static/css/style.css`
3. Edit JavaScript in `static/js/main.js`
4. Modify `main.go` to add new routes or change CMS configuration

## API Testing

Use curl or your favorite HTTP client to test the API:

```bash
# List all pages
curl http://localhost:3000/api/pages

# Get menu in Spanish
curl http://localhost:3000/api/menus/primary?locale=es

# Get page by ID
curl http://localhost:3000/api/pages/<page-id>
```
