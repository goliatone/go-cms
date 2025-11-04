package shortcode

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	parserpkg "github.com/goliatone/go-cms/internal/shortcode/parser"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

type memoryCache struct {
	store map[string]cacheEntry
}

type cacheEntry struct {
	value any
	ttl   time.Duration
}

func newMemoryCache() *memoryCache {
	return &memoryCache{store: map[string]cacheEntry{}}
}

func (c *memoryCache) Get(ctx context.Context, key string) (any, error) {
	if entry, ok := c.store[key]; ok {
		return entry.value, nil
	}
	return nil, fmt.Errorf("cache miss")
}

func (c *memoryCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	c.store[key] = cacheEntry{value: value, ttl: ttl}
	return nil
}

func (c *memoryCache) Delete(ctx context.Context, key string) error {
	delete(c.store, key)
	return nil
}

func (c *memoryCache) Clear(ctx context.Context) error {
	c.store = map[string]cacheEntry{}
	return nil
}

func TestRenderer_RenderTemplate(t *testing.T) {
	registry := NewRegistry(NewValidator())
	for _, def := range BuiltInDefinitions() {
		if err := registry.Register(def); err != nil {
			t.Fatalf("register built-in: %v", err)
		}
	}

	renderer := NewRenderer(registry, NewValidator())

	ctx := interfaces.ShortcodeContext{Locale: "en"}
	html, err := renderer.Render(ctx, "youtube", map[string]any{"id": "dQw4w9WgXcQ"}, "")
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	if !strings.Contains(string(html), "youtube.com/embed/dQw4w9WgXcQ") {
		t.Fatalf("expected iframe embed, got %s", html)
	}
}

func TestRenderer_SanitizerBlocksScript(t *testing.T) {
	registry := NewRegistry(NewValidator())
	malicious := interfaces.ShortcodeDefinition{
		Name:     "bad",
		Schema:   interfaces.ShortcodeSchema{},
		Template: `<script>alert('xss')</script>`,
	}
	if err := registry.Register(malicious); err != nil {
		t.Fatalf("register: %v", err)
	}

	renderer := NewRenderer(registry, NewValidator())
	_, err := renderer.Render(interfaces.ShortcodeContext{}, "bad", nil, "")
	if err == nil {
		t.Fatal("expected sanitizer error")
	}
}

func TestRenderer_CacheHit(t *testing.T) {
	registry := NewRegistry(NewValidator())
	def := interfaces.ShortcodeDefinition{
		Name:     "cached",
		Schema:   interfaces.ShortcodeSchema{},
		Template: "<p>cached</p>",
		CacheTTL: time.Hour,
	}
	if err := registry.Register(def); err != nil {
		t.Fatalf("register: %v", err)
	}

	cache := newMemoryCache()
	renderer := NewRenderer(registry, NewValidator(), WithRendererCache(cache))

	ctx := interfaces.ShortcodeContext{Locale: "en"}
	if _, err := renderer.Render(ctx, "cached", nil, ""); err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	if _, err := renderer.Render(ctx, "cached", nil, ""); err != nil {
		t.Fatalf("Render() second call error: %v", err)
	}

	if len(cache.store) != 1 {
		t.Fatalf("expected cache to store 1 item, got %d", len(cache.store))
	}
}

func TestRenderer_EndToEnd(t *testing.T) {
	registry := NewRegistry(NewValidator())
	for _, def := range BuiltInDefinitions() {
		if err := registry.Register(def); err != nil {
			t.Fatalf("register built-in: %v", err)
		}
	}

	renderer := NewRenderer(registry, NewValidator())
	parser := parserpkg.NewHugoParser()

	content := "Before {{< alert type=\"info\" >}}Watch {{< figure src=\"image.jpg\" alt=\"Alt\" >}}{{< /alert >}} After"
	transformed, parsed, err := parser.Extract(content)
	if err != nil {
		t.Fatalf("Extract() error: %v", err)
	}

	ctx := interfaces.ShortcodeContext{Locale: "en"}
	output := transformed
	for idx, sc := range parsed {
		html, err := renderer.Render(ctx, sc.Name, sc.Params, sc.Inner)
		if err != nil {
			t.Fatalf("Render shortcode %s: %v", sc.Name, err)
		}
		placeholder := fmt.Sprintf("<!-- shortcode:%d -->", idx)
		output = strings.ReplaceAll(output, placeholder, string(html))
	}

	if !strings.Contains(output, "shortcode--alert") {
		t.Fatalf("expected alert markup, got %s", output)
	}
	if !strings.Contains(output, "figure") {
		t.Fatalf("expected figure markup in nested shortcode output")
	}
}
