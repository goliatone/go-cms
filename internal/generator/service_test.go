package generator

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/google/uuid"
)

func TestBuildRendersTemplateContext(t *testing.T) {
	ctx := context.Background()

	now := time.Date(2024, 2, 5, 14, 30, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)

	renderer := &recordingRenderer{}
	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	expectedLocalized := fixtures.LocalizedCount()
	if result.PagesBuilt != expectedLocalized {
		t.Fatalf("expected %d pages built, got %d", expectedLocalized, result.PagesBuilt)
	}
	if len(result.Rendered) != expectedLocalized {
		t.Fatalf("expected %d rendered outputs, got %d", expectedLocalized, len(result.Rendered))
	}
	if len(result.Diagnostics) != expectedLocalized {
		t.Fatalf("expected %d diagnostics, got %d", expectedLocalized, len(result.Diagnostics))
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %d", len(result.Errors))
	}

	renderer.assertCalls(t, expectedLocalized)
	for _, call := range renderer.calls {
		if call.name != fixtures.Template.TemplatePath {
			t.Fatalf("expected template %q, got %q", fixtures.Template.TemplatePath, call.name)
		}
		if call.ctx.Site.DefaultLocale != fixtures.Config.DefaultLocale {
			t.Fatalf("expected default locale %q, got %q", fixtures.Config.DefaultLocale, call.ctx.Site.DefaultLocale)
		}
		if call.ctx.Helpers.Locale() != call.ctx.Page.Locale.Code {
			t.Fatalf("helper locale mismatch: got %q want %q", call.ctx.Helpers.Locale(), call.ctx.Page.Locale.Code)
		}
		if alias := call.ctx.Site.MenuAliases["main"]; alias != fixtures.Config.Menus["main"] {
			t.Fatalf("expected menu alias %q, got %q", fixtures.Config.Menus["main"], alias)
		}
		if call.ctx.Page.Template == nil {
			t.Fatalf("expected template in page context")
		}
		if call.ctx.Page.Translation == nil || call.ctx.Page.Translation.Path == "" {
			t.Fatalf("expected translation with path")
		}
		if base := call.ctx.Helpers.WithBaseURL("company"); base != "https://example.com/company" {
			t.Fatalf("expected helper base URL to return %q, got %q", "https://example.com/company", base)
		}
	}
}

func TestBuildUsesWorkerPool(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 3, 18, 9, 45, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)
	fixtures.Config.Workers = 4

	renderer := &concurrentRenderer{
		recordingRenderer: recordingRenderer{},
		delay:             2 * time.Millisecond,
	}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	expectedLocalized := fixtures.LocalizedCount()
	renderer.assertCalls(t, expectedLocalized)
	if result.PagesBuilt != expectedLocalized {
		t.Fatalf("expected %d pages built, got %d", expectedLocalized, result.PagesBuilt)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %d", len(result.Errors))
	}
	if renderer.maxConcurrent.Load() < 2 {
		t.Fatalf("expected at least 2 concurrent workers, got %d", renderer.maxConcurrent.Load())
	}
}

func TestBuildDryRunDiagnostics(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 4, 2, 18, 5, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)

	renderer := &recordingRenderer{}
	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{DryRun: true})
	if err != nil {
		t.Fatalf("build dry-run: %v", err)
	}

	expectedLocalized := fixtures.LocalizedCount()
	if !result.DryRun {
		t.Fatalf("expected dry-run flag to be true")
	}
	if result.PagesBuilt != expectedLocalized {
		t.Fatalf("expected %d pages built in dry-run, got %d", expectedLocalized, result.PagesBuilt)
	}
	if len(result.Rendered) != 0 {
		t.Fatalf("expected no rendered outputs in dry-run, got %d", len(result.Rendered))
	}
	if len(result.Diagnostics) != expectedLocalized {
		t.Fatalf("expected %d diagnostics, got %d", expectedLocalized, len(result.Diagnostics))
	}
	for _, diag := range result.Diagnostics {
		if diag.Err != nil {
			t.Fatalf("unexpected diagnostic error: %v", diag.Err)
		}
		if diag.Template != fixtures.Template.TemplatePath {
			t.Fatalf("expected template %q, got %q", fixtures.Template.TemplatePath, diag.Template)
		}
	}
	renderer.assertCalls(t, expectedLocalized)
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %d", len(result.Errors))
	}
}

type renderFixtures struct {
	Config   Config
	Content  *stubContentService
	Pages    *stubPagesService
	Menus    *stubMenusService
	Themes   *stubThemesService
	Locales  *stubLocaleLookup
	Template *themes.Template
	PageIDs  []uuid.UUID
}

func newRenderFixtures(now time.Time) renderFixtures {
	localeEN := uuid.New()
	localeES := uuid.New()
	themeID := uuid.New()
	templateID := uuid.New()
	contentID := uuid.New()

	contentRecord := &content.Content{
		ID:             contentID,
		Slug:           "company",
		Status:         "published",
		UpdatedAt:      now.Add(-time.Hour),
		CurrentVersion: 3,
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  localeEN,
				Title:     "Company",
				Content: map[string]any{
					"body": "english body",
				},
				UpdatedAt: now.Add(-30 * time.Minute),
			},
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  localeES,
				Title:     "Empresa",
				Content: map[string]any{
					"body": "contenido español",
				},
				UpdatedAt: now.Add(-20 * time.Minute),
			},
		},
	}

	pageBase := &pages.Page{
		ContentID:   contentID,
		TemplateID:  templateID,
		Slug:        "company",
		Status:      "published",
		IsVisible:   true,
		Blocks:      []*blocks.Instance{},
		Widgets:     map[string][]*widgets.ResolvedWidget{},
		PublishedAt: ptrTime(now.Add(-2 * time.Hour)),
		UpdatedAt:   now.Add(-90 * time.Minute),
	}

	page1 := clonePage(pageBase)
	page1.ID = uuid.New()
	page1.Translations = []*pages.PageTranslation{
		{ID: uuid.New(), PageID: page1.ID, LocaleID: localeEN, Title: "Company", Path: "/company", UpdatedAt: now.Add(-80 * time.Minute)},
		{ID: uuid.New(), PageID: page1.ID, LocaleID: localeES, Title: "Empresa", Path: "/es/empresa", UpdatedAt: now.Add(-70 * time.Minute)},
	}

	page2 := clonePage(pageBase)
	page2.ID = uuid.New()
	page2.Slug = "vision"
	page2.Translations = []*pages.PageTranslation{
		{ID: uuid.New(), PageID: page2.ID, LocaleID: localeEN, Title: "Vision", Path: "/vision", UpdatedAt: now.Add(-60 * time.Minute)},
		{ID: uuid.New(), PageID: page2.ID, LocaleID: localeES, Title: "Visión", Path: "/es/vision", UpdatedAt: now.Add(-50 * time.Minute)},
	}

	templateRecord := &themes.Template{
		ID:           templateID,
		ThemeID:      themeID,
		Name:         "detail",
		Slug:         "detail",
		TemplatePath: "themes/detail.html",
	}

	themeRecord := &themes.Theme{
		ID:        themeID,
		Name:      "aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
		Templates: []*themes.Template{templateRecord},
	}

	contentSvc := &stubContentService{
		records: map[uuid.UUID]*content.Content{
			contentID: contentRecord,
		},
	}

	pagesSvc := &stubPagesService{
		records: map[uuid.UUID]*pages.Page{
			page1.ID: page1,
			page2.ID: page2,
		},
		listing: []*pages.Page{page1, page2},
	}

	menuSvc := newStubMenuService()
	themeSvc := &stubThemesService{
		template: templateRecord,
		theme:    themeRecord,
	}
	locales := &stubLocaleLookup{
		records: map[string]*content.Locale{
			"en": {ID: localeEN, Code: "en"},
			"es": {ID: localeES, Code: "es"},
		},
	}

	cfg := Config{
		OutputDir:     "dist",
		BaseURL:       "https://example.com",
		DefaultLocale: "en",
		Locales:       []string{"en", "es"},
		Menus: map[string]string{
			"main": "main-nav",
		},
		Workers: 1,
	}

	return renderFixtures{
		Config:   cfg,
		Content:  contentSvc,
		Pages:    pagesSvc,
		Menus:    menuSvc,
		Themes:   themeSvc,
		Locales:  locales,
		Template: templateRecord,
		PageIDs:  []uuid.UUID{page1.ID, page2.ID},
	}
}

func (f renderFixtures) LocalizedCount() int {
	return len(f.PageIDs) * len(f.Config.Locales)
}

type renderCall struct {
	name string
	ctx  TemplateContext
}

type recordingRenderer struct {
	mu    sync.Mutex
	calls []renderCall
}

func (r *recordingRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	return r.RenderTemplate(name, data, out...)
}

func (r *recordingRenderer) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	ctx, ok := data.(TemplateContext)
	if !ok {
		return "", fmt.Errorf("unexpected render data type %T", data)
	}
	r.mu.Lock()
	r.calls = append(r.calls, renderCall{name: name, ctx: ctx})
	r.mu.Unlock()
	return fmt.Sprintf("<html data-path=\"%s\"></html>", ctx.Page.Translation.Path), nil
}

func (r *recordingRenderer) RenderString(templateContent string, data any, out ...io.Writer) (string, error) {
	return r.RenderTemplate(templateContent, data, out...)
}

func (r *recordingRenderer) RegisterFilter(string, func(any, any) (any, error)) error {
	return nil
}

func (r *recordingRenderer) GlobalContext(any) error {
	return nil
}

func (r *recordingRenderer) assertCalls(t *testing.T, expected int) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) != expected {
		t.Fatalf("expected %d render calls, got %d", expected, len(r.calls))
	}
}

type concurrentRenderer struct {
	recordingRenderer
	delay         time.Duration
	current       atomic.Int32
	maxConcurrent atomic.Int32
}

func (r *concurrentRenderer) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	ctx, ok := data.(TemplateContext)
	if !ok {
		return "", fmt.Errorf("unexpected render data type %T", data)
	}
	cur := r.current.Add(1)
	for {
		max := r.maxConcurrent.Load()
		if cur <= max {
			break
		}
		if r.maxConcurrent.CompareAndSwap(max, cur) {
			break
		}
	}
	time.Sleep(r.delay)
	r.mu.Lock()
	r.calls = append(r.calls, renderCall{name: name, ctx: ctx})
	r.mu.Unlock()
	r.current.Add(-1)
	return fmt.Sprintf("<html locale=\"%s\"></html>", ctx.Page.Locale.Code), nil
}
