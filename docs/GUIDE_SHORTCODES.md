# Shortcodes Guide

This guide covers shortcode registration, processing, caching, security, and custom shortcode authoring in `go-cms`. By the end you will understand how to enable the shortcode subsystem, use the five built-in shortcodes, write your own shortcodes with handler or template logic, configure caching and security, and integrate shortcodes with the markdown pipeline.

## Shortcode Architecture Overview

Shortcodes in `go-cms` let authors embed dynamic, validated content fragments inside markdown and templates without writing raw HTML. The subsystem is built on five layers:

- **Registry** stores shortcode definitions and resolves them by name. It is thread-safe and case-insensitive.
- **Parser** extracts shortcode invocations from content. The primary parser handles Hugo-style `{{< >}}` syntax; an optional preprocessor converts WordPress-style `[]` syntax before parsing.
- **Validator** coerces and validates parameters against the definition's schema before rendering.
- **Renderer** executes definitions via Go templates or handler functions, applies sanitisation, and manages output caching.
- **Service** orchestrates the full pipeline: preprocess, parse, render, and replace placeholders with HTML output.

```
Content string
  │
  ├─ [WordPress preprocessor] ──► Convert [] to {{< >}}
  │
  ├─ Parser.Extract() ──► Placeholders + ParsedShortcodes
  │
  ├─ For each shortcode:
  │     Validator.CoerceParams() ──► Renderer.Render()
  │
  └─ Replace placeholders with rendered HTML
```

When `cfg.Features.Shortcodes` is `false`, all service methods return no-op implementations that pass content through unchanged.

### Accessing the Service

Shortcode operations are exposed through the `cms.Module` facade:

```go
cfg := cms.DefaultConfig()
cfg.Features.Shortcodes = true
cfg.Shortcodes.Enabled = true

module, err := cms.New(cfg)
if err != nil {
    log.Fatal(err)
}

shortcodeSvc := module.Shortcodes()
```

The `shortcodeSvc` variable satisfies the `interfaces.ShortcodeService` interface. The service delegates to in-memory components by default and does not require a database.

---

## Enabling Shortcodes

Shortcodes require two flags to be active:

```go
cfg := cms.DefaultConfig()
cfg.Features.Shortcodes = true   // Feature gate — enables subsystem wiring
cfg.Shortcodes.Enabled = true    // Runtime toggle — activates processing
```

Both default to `false`. When either flag is off, the DI container returns a no-op service that passes content through untouched.

---

## Configuration

### Full Configuration Reference

```go
cfg.Shortcodes = runtimeconfig.ShortcodeConfig{
    Enabled:               true,
    EnableWordPressSyntax: false,
    BuiltIns:              []string{"youtube", "alert", "gallery", "figure", "code"},
    CustomDefinitions:     []runtimeconfig.ShortcodeDefinitionConfig{},
    Security: runtimeconfig.ShortcodeSecurityConfig{
        MaxNestingDepth:    5,
        MaxExecutionTime:   5 * time.Second,
        SanitizeOutput:     true,
        CSPEnabled:         false,
        RateLimitPerMinute: 0,
    },
    Cache: runtimeconfig.ShortcodeCacheConfig{
        Enabled:      false,
        Provider:     "",
        DefaultTTL:   time.Hour,
        PerShortcode: map[string]time.Duration{},
    },
}
```

### Built-In Shortcodes

By default, all five built-in shortcodes are registered. Control which ones are available:

```go
// Register all five (default)
cfg.Shortcodes.BuiltIns = []string{"youtube", "alert", "gallery", "figure", "code"}

// Register a subset
cfg.Shortcodes.BuiltIns = []string{"youtube", "alert"}

// Register none — only custom definitions will be available
cfg.Shortcodes.BuiltIns = []string{}
```

When `BuiltIns` is empty, `RegisterBuiltIns` registers all five. To disable all built-ins while using custom definitions, set the list to an explicit empty slice after calling `DefaultConfig()`.

### WordPress Syntax Toggle

Hugo-style `{{< name >}}` syntax is always supported. To also accept WordPress-style `[name]` syntax:

```go
cfg.Shortcodes.EnableWordPressSyntax = true
```

When enabled, the preprocessor converts `[name attr="val"]content[/name]` into `{{< name attr="val" >}}content{{< /name >}}` before the Hugo parser runs. Both syntaxes can coexist in the same content.

You can also enable WordPress syntax per-call without changing the global setting:

```go
html, err := shortcodeSvc.Process(ctx, content, interfaces.ShortcodeProcessOptions{
    EnableWordPress: true,
})
```

### Security Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `MaxNestingDepth` | `5` | Maximum depth for nested shortcodes |
| `MaxExecutionTime` | `5s` | Timeout for a single shortcode render |
| `SanitizeOutput` | `true` | Run output through the sanitizer |
| `CSPEnabled` | `false` | Content Security Policy enforcement |
| `RateLimitPerMinute` | `0` | Render rate limit (0 = unlimited) |
| `AllowedDomains` | `[]` | Restrict embed URLs to listed domains |

The default sanitizer rejects `<script>` tags and inline event handlers (`onclick`, `onload`, etc.), and validates URL schemes against an allow-list of `http`, `https`, and relative paths.

To disable sanitisation (not recommended for production):

```go
cfg.Shortcodes.Security.SanitizeOutput = false
```

---

## Built-In Shortcodes Reference

### youtube

Embeds a responsive YouTube iframe player.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `id` | `string` | Yes | — | YouTube video ID |
| `start` | `int` | No | `0` | Start time in seconds |
| `autoplay` | `bool` | No | `false` | Auto-start playback |

**Cache TTL:** 1 hour

**Usage:**

```
{{< youtube id="dQw4w9WgXcQ" >}}
{{< youtube id="dQw4w9WgXcQ" start=30 autoplay=true >}}
```

**Output:**

```html
<div class="shortcode shortcode--youtube">
  <iframe src="https://www.youtube.com/embed/dQw4w9WgXcQ" title="YouTube video" loading="lazy" allowfullscreen></iframe>
</div>
```

---

### alert

Displays a contextual alert callout with optional title and inner content.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `type` | `string` | Yes | — | One of: `info`, `success`, `warning`, `danger` |
| `title` | `string` | No | — | Alert heading |

**Usage:**

```
{{< alert type="warning" title="Heads Up" >}}
Check your configuration before deploying.
{{< /alert >}}
```

**Output:**

```html
<div class="shortcode shortcode--alert shortcode--alert-warning">
  <div class="shortcode__title">Heads Up</div>
  <div class="shortcode__body">Check your configuration before deploying.</div>
</div>
```

The `type` parameter is validated against the four allowed values. Passing an unsupported type returns an error.

---

### gallery

Renders an image gallery grid from a list of image URLs.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `images` | `array` | Yes | — | Image URLs (comma-separated string or array) |
| `columns` | `int` | No | `3` | Number of grid columns |

**Usage:**

```
{{< gallery images="hero.jpg,team.jpg,office.jpg" columns=2 >}}
```

**Output:**

```html
<div class="shortcode shortcode--gallery columns-2">
  <figure class="shortcode__gallery-item">
    <img src="hero.jpg" loading="lazy" />
  </figure>
  <figure class="shortcode__gallery-item">
    <img src="team.jpg" loading="lazy" />
  </figure>
  <figure class="shortcode__gallery-item">
    <img src="office.jpg" loading="lazy" />
  </figure>
</div>
```

The `images` parameter accepts a comma-separated string which is coerced to an array automatically.

---

### figure

Renders an image wrapped in a `<figure>` element with optional caption.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `src` | `string` | Yes | — | Image URL |
| `alt` | `string` | No | `""` | Alt text |
| `caption` | `string` | No | — | Figure caption |

**Usage:**

```
{{< figure src="/images/sunset.jpg" alt="Sunset" caption="Golden hour at the pier" >}}
```

**Output:**

```html
<figure class="shortcode shortcode--figure">
  <img src="/images/sunset.jpg" alt="Sunset" loading="lazy" />
  <figcaption>Golden hour at the pier</figcaption>
</figure>
```

---

### code

Renders a syntax-highlighted code block with optional title and line numbers.

**Parameters:**

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `lang` | `string` | Yes | — | Language identifier (`go`, `js`, `python`, etc.) |
| `title` | `string` | No | — | Code block title |
| `line_numbers` | `bool` | No | `true` | Show line numbers |

**Usage:**

```
{{< code lang="go" title="main.go" >}}
package main

func main() {
    fmt.Println("Hello, world!")
}
{{< /code >}}
```

**Output:**

```html
<div class="shortcode shortcode--code">
  <div class="shortcode__code-title">main.go</div>
  <pre class="shortcode__code-block language-go shortcode__code-block--lines"><code>package main

func main() {
    fmt.Println("Hello, world!")
}</code></pre>
</div>
```

---

## Service API

### Process — Batch Content Rendering

`Process` scans a content string for shortcode invocations, renders each one, and returns the content with all shortcodes replaced by their HTML output.

```go
html, err := shortcodeSvc.Process(ctx, markdownContent, interfaces.ShortcodeProcessOptions{
    Locale:          "en",
    EnableWordPress: false,
})
if err != nil {
    log.Fatal(err)
}
fmt.Println(html)
```

**`ShortcodeProcessOptions` fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Locale` | `string` | Locale code passed to handlers via `ShortcodeContext` |
| `Cache` | `CacheProvider` | Override cache provider for this call |
| `Sanitizer` | `ShortcodeSanitizer` | Override sanitizer for this call |
| `EnableWordPress` | `bool` | Enable `[]` syntax for this call only |

**What happens:**

1. If WordPress syntax is enabled (globally or per-call), the preprocessor converts `[]` tags to `{{< >}}` tags
2. The Hugo parser extracts all shortcode invocations and replaces them with numbered placeholders (`<!-- shortcode:0 -->`, `<!-- shortcode:1 -->`, etc.)
3. For each parsed shortcode, parameters are coerced and validated against the definition schema
4. The renderer executes the handler or template and applies sanitisation
5. Placeholders are replaced with rendered HTML
6. Metrics are recorded for each render attempt

If content is empty or contains no shortcodes, it is returned unchanged.

### Render — Single Shortcode Execution

`Render` executes a single shortcode by name with explicit parameters, bypassing the parser.

```go
html, err := shortcodeSvc.Render(
    interfaces.ShortcodeContext{
        Context: ctx,
        Locale:  "en",
    },
    "youtube",
    map[string]any{"id": "dQw4w9WgXcQ", "start": 30},
    "",
)
if err != nil {
    log.Fatal(err)
}
fmt.Println(html)
```

This is useful for programmatic shortcode rendering where the shortcode name and parameters are known ahead of time, without needing to embed them in a content string.

### Registry — Shortcode Registration

The registry is accessible through the service when you need to register, inspect, or remove shortcodes at runtime:

```go
// Access the registry through the service
svc := module.Shortcodes()
registry := svc.(*shortcode.Service).Registry()

// Or access it directly through the container
container := module.Container()
registry := container.ShortcodeRegistry()

// List all registered shortcodes
definitions := registry.List()
for _, def := range definitions {
    fmt.Printf("%s (v%s) - %s\n", def.Name, def.Version, def.Description)
}

// Look up a specific shortcode
if def, ok := registry.Get("youtube"); ok {
    fmt.Printf("youtube has %d parameters\n", len(def.Schema.Params))
}

// Remove a shortcode
registry.Remove("gallery")
```

The registry is thread-safe. Names are normalized to lowercase. `List()` returns definitions sorted alphabetically. `Remove()` on an unknown name is a no-op.

---

## Shortcode Caching

### Enabling the Cache

```go
cfg.Shortcodes.Cache.Enabled = true
cfg.Shortcodes.Cache.DefaultTTL = time.Hour
```

When caching is enabled, the renderer generates a cache key from the locale, shortcode name, all parameter values (sorted), and inner content using a SHA-1 hash. Cache keys follow the format `shortcode:<hash>`.

### Named Cache Providers

Register a named cache provider via the DI container:

```go
module, err := cms.New(cfg,
    di.WithShortcodeCacheProvider("redis", redisCache),
)
```

Then reference it in configuration:

```go
cfg.Shortcodes.Cache.Provider = "redis"
```

When no named provider is found, the container falls back to the global cache provider registered via `di.WithCache()`. If neither is available, caching is silently disabled.

An empty provider name selects the default (global) cache:

```go
di.WithShortcodeCacheProvider("", memoryCache)
```

### Per-Shortcode TTL Overrides

Each shortcode definition carries its own `CacheTTL`. The built-in `youtube` shortcode caches for 1 hour by default. Override TTLs per shortcode:

```go
cfg.Shortcodes.Cache.PerShortcode = map[string]time.Duration{
    "youtube": 24 * time.Hour,   // Embeds change rarely
    "code":    5 * time.Minute,  // Code blocks may update more often
}
```

Setting `CacheTTL` to `0` on a definition disables caching for that shortcode regardless of global settings.

### Cache Bypass

Caching is skipped when any of these conditions hold:

- `cfg.Shortcodes.Cache.Enabled` is `false`
- The definition's `CacheTTL` is `0`
- No cache provider is available (named or global)

---

## Metrics

### Wiring Metrics

Register a metrics implementation via the DI container:

```go
module, err := cms.New(cfg,
    di.WithShortcodeMetrics(myPrometheusMetrics),
)
```

The `ShortcodeMetrics` interface exposes three methods:

```go
type ShortcodeMetrics interface {
    ObserveRenderDuration(shortcode string, duration time.Duration)
    IncrementRenderError(shortcode string)
    IncrementCacheHit(shortcode string)
}
```

| Method | Emitted when | Purpose |
|--------|-------------|---------|
| `ObserveRenderDuration` | Every render attempt (success or failure) | Track latency per shortcode |
| `IncrementRenderError` | Render returns an error | Count failures |
| `IncrementCacheHit` | Cache returns a valid entry | Track cache effectiveness |

When no metrics implementation is provided, a no-op recorder is used. All calls are zero-cost in that case.

### Structured Logging

The service emits structured log entries at key points:

| Event | Level | Fields |
|-------|-------|--------|
| `shortcode.service.parse_failed` | Error | `operation`, `error` |
| `shortcode.service.render_failed` | Error | `shortcode`, `index`, `duration_ms`, `error` |
| `shortcode.service.render_succeeded` | Debug | `shortcode`, `index`, `duration_ms` |
| `shortcode.service.process_completed` | Debug | `shortcodes` (count) |

---

## Integration with Markdown Processing

When both the markdown and shortcode features are enabled, shortcodes can be processed during markdown import and sync operations.

```go
cfg := cms.DefaultConfig()
cfg.Features.Markdown = true
cfg.Features.Shortcodes = true
cfg.Shortcodes.Enabled = true
cfg.Markdown.ProcessShortcodes = true

module, err := cms.New(cfg)
if err != nil {
    log.Fatal(err)
}
```

With `cfg.Markdown.ProcessShortcodes = true`, the markdown service calls `shortcodeService.Process()` on content before passing it to the Goldmark renderer. This means shortcodes are expanded to HTML before markdown-to-HTML conversion takes place.

The processing order is:

1. Markdown file is loaded and frontmatter is parsed
2. Shortcodes in the markdown body are processed (if enabled)
3. The resulting content is rendered through Goldmark
4. Final HTML is returned

This approach lets authors mix markdown syntax and shortcode invocations:

```markdown
---
title: "About Us"
---

# Our Team

Here is a video introduction:

{{< youtube id="abc123" >}}

{{< alert type="info" title="Note" >}}
We are hiring! See our [careers page](/careers) for open positions.
{{< /alert >}}

{{< gallery images="team-1.jpg,team-2.jpg,team-3.jpg" columns=3 >}}
```

---

## Writing Custom Shortcodes

### Method 1: Template-Based Definitions

Define a shortcode using a Go template string. Parameters and inner content are available as template variables.

```go
registry := container.ShortcodeRegistry()

err := registry.Register(interfaces.ShortcodeDefinition{
    Name:        "callout",
    Version:     "1.0.0",
    Description: "Highlighted callout box",
    Category:    "content",
    AllowInner:  true,
    Schema: interfaces.ShortcodeSchema{
        Params: []interfaces.ShortcodeParam{
            {
                Name:     "title",
                Type:     interfaces.ShortcodeParamString,
                Required: true,
            },
            {
                Name:    "color",
                Type:    interfaces.ShortcodeParamString,
                Default: "blue",
            },
        },
    },
    Template: `<div class="callout callout--{{ .color }}">
  <h3>{{ .title }}</h3>
  <div>{{ .Inner }}</div>
</div>`,
})
```

In the template:
- Named parameters are available as `.paramName` (e.g. `.title`, `.color`)
- Inner content (between opening and closing tags) is available as `.Inner`
- The template is parsed via Go's `html/template` package, providing automatic escaping

**Usage:**

```
{{< callout title="Did you know?" color="green" >}}
Go templates provide automatic HTML escaping.
{{< /callout >}}
```

### Method 2: Handler-Based Definitions

For shortcodes that need programmatic logic — API calls, complex formatting, conditional rendering — use a handler function.

```go
err := registry.Register(interfaces.ShortcodeDefinition{
    Name:        "price",
    Version:     "1.0.0",
    Description: "Formats a price with currency symbol",
    Category:    "commerce",
    Schema: interfaces.ShortcodeSchema{
        Params: []interfaces.ShortcodeParam{
            {
                Name:     "amount",
                Type:     interfaces.ShortcodeParamInt,
                Required: true,
            },
            {
                Name:    "currency",
                Type:    interfaces.ShortcodeParamString,
                Default: "USD",
            },
        },
    },
    Handler: func(ctx interfaces.ShortcodeContext, params map[string]any, inner string) (template.HTML, error) {
        amount := params["amount"].(int)
        currency := params["currency"].(string)

        symbols := map[string]string{
            "USD": "$", "EUR": "€", "GBP": "£",
        }
        symbol := symbols[currency]
        if symbol == "" {
            symbol = currency + " "
        }

        html := fmt.Sprintf(`<span class="price">%s%d.00</span>`, symbol, amount)
        return template.HTML(html), nil
    },
})
```

The handler receives:
- `ctx` — a `ShortcodeContext` with the request context, locale, cache, and sanitizer
- `params` — coerced and validated parameters
- `inner` — inner content as a raw string (empty for self-closing shortcodes)

When both `Handler` and `Template` are set, the handler takes priority.

### Method 3: Configuration-Based Definitions

Register shortcodes through configuration without writing Go code:

```go
cfg.Shortcodes.CustomDefinitions = []runtimeconfig.ShortcodeDefinitionConfig{
    {
        Name:     "badge",
        Template: `<span class="badge badge--{{ .type }}">{{ .text }}</span>`,
        Schema: map[string]any{
            "type": map[string]any{
                "type":     "string",
                "required": true,
            },
            "text": map[string]any{
                "type":     "string",
                "required": true,
            },
        },
    },
}
```

Configuration-based definitions are registered during container initialisation after the built-in shortcodes. If a custom definition has the same name as a built-in, the registration will fail with `ErrDuplicateDefinition` and a warning will be logged.

### Parameter Types

Five parameter types are supported:

| Type | Go Type | Coercion Rules |
|------|---------|----------------|
| `string` | `string` | Any value via `fmt.Sprintf` |
| `int` | `int` | Parses strings, truncates floats, converts uint variants |
| `bool` | `bool` | `"true"`, `"1"`, `"yes"`, `"on"` → `true`; `"false"`, `"0"`, `"no"`, `"off"` → `false` |
| `array` | `[]any` | Splits comma-separated strings; converts typed slices |
| `url` | `string` | Validates via `url.ParseRequestURI`; allows http, https, and relative paths |

### Custom Parameter Validation

Attach a `Validate` function to any parameter for domain-specific validation:

```go
{
    Name:     "type",
    Type:     interfaces.ShortcodeParamString,
    Required: true,
    Validate: func(value any) error {
        str, ok := value.(string)
        if !ok {
            return fmt.Errorf("type must be a string")
        }
        allowed := map[string]bool{"info": true, "warning": true, "danger": true}
        if !allowed[str] {
            return fmt.Errorf("type %q is not supported", str)
        }
        return nil
    },
}
```

The validator runs after type coercion, so the value is already the correct Go type.

### Caching Custom Shortcodes

Set `CacheTTL` on the definition to enable per-shortcode caching:

```go
{
    Name:     "weather",
    CacheTTL: 10 * time.Minute,
    Handler: func(ctx interfaces.ShortcodeContext, params map[string]any, inner string) (template.HTML, error) {
        // Expensive API call — cached for 10 minutes
        // ...
    },
}
```

---

## Syntax Reference

### Hugo-Style Syntax (Always Available)

**Self-closing:**

```
{{< youtube id="abc123" >}}
{{< figure src="/img.jpg" caption="Title" >}}
```

**With inner content:**

```
{{< alert type="warning" >}}
Be careful with this operation.
{{< /alert >}}
```

**Nested shortcodes:**

```
{{< alert type="info" >}}
See this image: {{< figure src="/example.jpg" >}}
{{< /alert >}}
```

### WordPress-Style Syntax (Optional)

Requires `cfg.Shortcodes.EnableWordPressSyntax = true` or `EnableWordPress: true` in process options.

**Self-closing:**

```
[youtube id="abc123"]
[figure src="/img.jpg" caption="Title"]
```

**With inner content:**

```
[alert type="warning"]Be careful with this operation.[/alert]
```

The preprocessor converts WordPress syntax to Hugo syntax before parsing. Both syntaxes can coexist in the same document.

---

## Error Handling

The shortcode subsystem defines these error types:

| Error | Condition |
|-------|-----------|
| `ErrDuplicateDefinition` | Attempting to register a shortcode name that is already taken |
| `ErrInvalidDefinition` | Definition fails validation (missing name, invalid schema, etc.) |
| `ErrUnknownParameter` | Supplied parameter not declared in the definition schema |
| `ErrMissingParameter` | Required parameter not provided |
| `ErrParameterType` | Parameter value cannot be coerced to the declared type |

Rendering errors include:

- `"shortcode: unknown <name>"` — the shortcode is not registered
- `"shortcode: script tags are not allowed"` — the sanitizer blocked the output
- `"shortcode: url scheme <scheme> not permitted"` — URL validation failed
- `"shortcode: attribute <attr> not permitted"` — inline event handler detected

All errors from `Process` and `Render` should be checked. The service stops processing on the first render error and returns it immediately.

---

## Common Patterns

### Replacing a Built-In

To override a built-in shortcode's behaviour, remove the original and register a replacement:

```go
registry := container.ShortcodeRegistry()

// Remove the built-in
registry.Remove("youtube")

// Register a customised version
registry.Register(interfaces.ShortcodeDefinition{
    Name:     "youtube",
    Version:  "2.0.0",
    CacheTTL: 48 * time.Hour,
    Schema:   /* same or modified schema */,
    Template: /* custom template */,
})
```

### Locale-Aware Rendering

Pass the locale through `ShortcodeContext` for handlers that need locale-specific logic:

```go
html, err := shortcodeSvc.Process(ctx, content, interfaces.ShortcodeProcessOptions{
    Locale: "es",
})
```

Inside a handler, access it via `ctx.Locale`:

```go
Handler: func(ctx interfaces.ShortcodeContext, params map[string]any, inner string) (template.HTML, error) {
    if ctx.Locale == "es" {
        return template.HTML(`<span>Precio: €10</span>`), nil
    }
    return template.HTML(`<span>Price: $10</span>`), nil
},
```

### Migrating WordPress Content

Enable WordPress syntax and process existing content:

```go
cfg.Shortcodes.EnableWordPressSyntax = true

// Existing WordPress content works without modification
content := `Check out this video: [youtube id="abc123"]
[alert type="info"]This is an informational note.[/alert]`

html, err := shortcodeSvc.Process(ctx, content, interfaces.ShortcodeProcessOptions{})
```

Both `[youtube id="..."]` and `{{< youtube id="..." >}}` render identically.

---

## Next Steps

- See the [Markdown Guide](GUIDE_MARKDOWN.md) for markdown import workflows that process shortcodes
- See the [Configuration Guide](GUIDE_CONFIGURATION.md) for full `ShortcodeConfig` reference
- Refer to `docs/SHORTCODE_TDD.md` for the technical design document
- Explore `site/internal/shortcodes/definitions.go` for real-world custom shortcode examples from the COLABS reference site
