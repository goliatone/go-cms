package generator

import (
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	gotheme "github.com/goliatone/go-theme"
	"github.com/google/uuid"
)

// TemplateContext captures the data contract passed to TemplateRenderer implementations.
type TemplateContext struct {
	Site    SiteMetadata
	Page    PageRenderingContext
	Build   BuildMetadata
	Theme   ThemeContext
	Helpers TemplateHelpers
}

// SiteMetadata exposes locale-aware information required by templates.
type SiteMetadata struct {
	BaseURL       string
	DefaultLocale string
	Locales       []LocaleSpec
	MenuAliases   map[string]string
	Metadata      map[string]any
}

// BuildMetadata surfaces high level build information to templates.
type BuildMetadata struct {
	GeneratedAt time.Time
	Options     BuildOptions
}

// PageRenderingContext contains the resolved dependencies for a single page/locale combination.
type PageRenderingContext struct {
	Page               *pages.Page
	Content            *content.Content
	Translation        *pages.PageTranslation
	ContentTranslation *content.ContentTranslation
	Blocks             []*blocks.Instance
	Widgets            map[string][]*widgets.ResolvedWidget
	Menus              map[string][]menus.NavigationNode
	Template           *themes.Template
	Theme              *themes.Theme
	Locale             LocaleSpec
	Metadata           DependencyMetadata
}

// ThemeContext surfaces go-theme selection data to templates.
type ThemeContext struct {
	Name      string
	Variant   string
	Tokens    map[string]string
	CSSVars   map[string]string
	Partials  map[string]string
	AssetURL  func(string) string
	Template  func(string, string) string
	Selection *gotheme.Selection
}

// TemplateHelpers exposes convenience helpers for template authors.
type TemplateHelpers struct {
	locale        LocaleSpec
	defaultLocale string
	baseURL       string
}

func newTemplateHelpers(defaultLocale string, locale LocaleSpec, baseURL string) TemplateHelpers {
	return TemplateHelpers{
		locale:        locale,
		defaultLocale: defaultLocale,
		baseURL:       strings.TrimRight(baseURL, "/"),
	}
}

// Locale returns the active locale code.
func (h TemplateHelpers) Locale() string {
	return h.locale.Code
}

// IsLocale reports whether the provided locale code matches the active locale.
func (h TemplateHelpers) IsLocale(code string) bool {
	return strings.EqualFold(strings.TrimSpace(code), h.locale.Code)
}

// IsDefaultLocale reports whether the current locale matches the configured default.
func (h TemplateHelpers) IsDefaultLocale() bool {
	return strings.EqualFold(h.locale.Code, h.defaultLocale)
}

// BaseURL returns the configured site base URL.
func (h TemplateHelpers) BaseURL() string {
	return h.baseURL
}

// WithBaseURL prefixes the provided path with the configured base URL.
func (h TemplateHelpers) WithBaseURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return h.baseURL
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if h.baseURL == "" {
		return path
	}
	return h.baseURL + path
}

// LocalePrefix returns the locale aware prefix for paths.
func (h TemplateHelpers) LocalePrefix() string {
	if h.IsDefaultLocale() {
		return ""
	}
	return "/" + strings.TrimPrefix(strings.TrimSpace(h.locale.Code), "/")
}

func buildThemeContext(selection *gotheme.Selection, cfg ThemingConfig) ThemeContext {
	empty := ThemeContext{
		Tokens:   map[string]string{},
		CSSVars:  map[string]string{},
		Partials: map[string]string{},
		AssetURL: func(string) string { return "" },
		Template: func(_ string, fallback string) string { return fallback },
	}
	if selection == nil {
		return empty
	}

	cssPrefix := cfg.CSSVariablePrefix
	tokens := selection.Tokens()
	cssVars := selection.CSSVariables(cssPrefix)
	partials := selection.Partials(cfg.PartialFallbacks)

	return ThemeContext{
		Name:      selection.Theme,
		Variant:   selection.Variant,
		Tokens:    tokens,
		CSSVars:   cssVars,
		Partials:  partials,
		AssetURL:  func(key string) string { url, _ := selection.Asset(key); return url },
		Template:  selection.Template,
		Selection: selection,
	}
}

// RenderedPage captures the rendered HTML output for a page.
type RenderedPage struct {
	PageID   uuid.UUID
	Locale   string
	Route    string
	Output   string
	Template string
	HTML     string
	Metadata DependencyMetadata
	Duration time.Duration
	Checksum string
}

// RenderDiagnostic records rendering timing and errors for individual pages.
type RenderDiagnostic struct {
	PageID   uuid.UUID
	Locale   string
	Route    string
	Template string
	Duration time.Duration
	Skipped  bool
	Err      error
}

type renderOutcome struct {
	page       RenderedPage
	diagnostic RenderDiagnostic
	err        error
	skipped    bool
}
