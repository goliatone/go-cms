package shortcode

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Renderer executes shortcode definitions and produces sanitised HTML output.
type Renderer struct {
	registry  interfaces.ShortcodeRegistry
	validator *Validator
	sanitizer interfaces.ShortcodeSanitizer
	cache     interfaces.CacheProvider
}

// RendererOption configures the renderer instance.
type RendererOption func(*Renderer)

// WithRendererSanitizer overrides the default sanitizer.
func WithRendererSanitizer(s interfaces.ShortcodeSanitizer) RendererOption {
	return func(r *Renderer) {
		r.sanitizer = s
	}
}

// WithRendererCache supplies a cache provider used when definitions specify a CacheTTL.
func WithRendererCache(cache interfaces.CacheProvider) RendererOption {
	return func(r *Renderer) {
		r.cache = cache
	}
}

// NewRenderer constructs a renderer using the provided registry and validator.
func NewRenderer(registry interfaces.ShortcodeRegistry, validator *Validator, opts ...RendererOption) *Renderer {
	r := &Renderer{
		registry:  registry,
		validator: validator,
		sanitizer: NewSanitizer(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Render executes the shortcode and returns sanitised HTML.
func (r *Renderer) Render(ctx interfaces.ShortcodeContext, shortcode string, params map[string]any, inner string) (template.HTML, error) {
	def, ok := r.registry.Get(shortcode)
	if !ok {
		return "", fmt.Errorf("shortcode: unknown %s", shortcode)
	}

	coerced, err := r.validator.CoerceParams(def, params)
	if err != nil {
		return "", err
	}

	cacheProvider := r.resolveCache(ctx)
	cacheKey := ""
	if cacheProvider != nil && def.CacheTTL > 0 {
		cacheKey = r.buildCacheKey(ctx.Locale, shortcode, coerced, inner)
		if cached, err := cacheProvider.Get(r.background(ctx.Context), cacheKey); err == nil {
			if cachedHTML, ok := cached.(string); ok {
				return template.HTML(cachedHTML), nil
			}
		}
	}

	var output string
	if def.Handler != nil {
		result, err := def.Handler(ctx, coerced, inner)
		if err != nil {
			return "", err
		}
		output = string(result)
	} else if def.Template != "" {
		rendered, err := r.renderTemplate(def, coerced, inner)
		if err != nil {
			return "", err
		}
		output = rendered
	} else {
		return "", fmt.Errorf("shortcode: definition %s has no handler or template", shortcode)
	}

	sanitizer := r.resolveSanitizer(ctx)
	if sanitizer != nil {
		sanitised, err := sanitizer.Sanitize(output)
		if err != nil {
			return "", err
		}
		output = sanitised
	}

	if cacheProvider != nil && def.CacheTTL > 0 {
		_ = cacheProvider.Set(r.background(ctx.Context), cacheKey, output, def.CacheTTL)
	}

	return template.HTML(output), nil
}

// RenderAsync executes Render in a separate goroutine.
func (r *Renderer) RenderAsync(ctx interfaces.ShortcodeContext, shortcode string, params map[string]any, inner string) (<-chan template.HTML, <-chan error) {
	outputCh := make(chan template.HTML, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(outputCh)
		defer close(errCh)

		result, err := r.Render(ctx, shortcode, params, inner)
		if err != nil {
			errCh <- err
			return
		}
		outputCh <- result
	}()

	return outputCh, errCh
}

func (r *Renderer) renderTemplate(def interfaces.ShortcodeDefinition, params map[string]any, inner string) (string, error) {
	data := make(map[string]any, len(params)+1)
	for key, value := range params {
		data[key] = value
	}
	data["Inner"] = inner

	tmpl, err := template.New(def.Name).Parse(def.Template)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (r *Renderer) resolveSanitizer(ctx interfaces.ShortcodeContext) interfaces.ShortcodeSanitizer {
	if ctx.Sanitizer != nil {
		return ctx.Sanitizer
	}
	return r.sanitizer
}

func (r *Renderer) resolveCache(ctx interfaces.ShortcodeContext) interfaces.CacheProvider {
	if ctx.Cache != nil {
		return ctx.Cache
	}
	return r.cache
}

func (r *Renderer) buildCacheKey(locale, shortcode string, params map[string]any, inner string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString(locale)
	builder.WriteString("|")
	builder.WriteString(shortcode)
	for _, key := range keys {
		builder.WriteString("|")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(fmt.Sprintf("%v", params[key]))
	}
	builder.WriteString("|inner=")
	builder.WriteString(inner)

	h := sha1.Sum([]byte(builder.String()))
	return "shortcode:" + hex.EncodeToString(h[:])
}

func (r *Renderer) background(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

// Ensure Renderer implements interfaces.ShortcodeRenderer.
var _ interfaces.ShortcodeRenderer = (*Renderer)(nil)
