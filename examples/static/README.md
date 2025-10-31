# Static Generator Walkthrough

This example seeds a minimal in-memory site and runs the static generator to produce a multi-locale build. It demonstrates how themes, menus, and the template contract compose without relying on external storage or template engines.

## What it does

- Registers English and Spanish locales plus a reusable `Demo Page` content type.
- Seeds five translated pages (`/`, `/company`, `/services`, `/blog`, `/contact`) and wires them into a primary navigation menu.
- Registers a lightweight theme with Go templates and static assets (CSS + SVG logo).
- Streams the generator output to `./dist/static-demo`, including HTML, assets, and sitemap/robots files.

## Run it

```bash
/Users/goliatone/.g/go/bin/go run ./examples/static
```

After completion you should see log output confirming the page/asset counts and the build duration:

```
static build complete: pages=4 assets=2 duration=45.321ms
output written to dist/static-demo
```

## Output layout

```
dist/static-demo/
├── assets/
│   ├── logo.svg
│   └── theme.css
├── en/
│   ├── blog/index.html
│   ├── company/index.html
│   ├── contact/index.html
│   ├── index.html
│   └── services/index.html
├── es/
│   ├── blog/index.html
│   ├── contacto/index.html
│   ├── empresa/index.html
│   ├── index.html
│   └── servicios/index.html
├── index.html
├── robots.txt
└── sitemap.xml
```

Files under `en/` and `es/` mirror the locale-specific routes while the default locale is also promoted to the root for convenience.

## Template contract highlights

The demo theme uses the `generator.TemplateContext` helpers showcased in:

- `templates/layout.tmpl` for locale-aware navigation, asset links via `Helpers.WithBaseURL`, and build metadata.
- `templates/page.tmpl` which simply delegates to the layout for clarity.

Check `support.go` for a reference Go template renderer and filesystem storage adapter—these illustrate how embedders can satisfy the generator interfaces without additional dependencies.
