package generator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/pages"
	shortcodepkg "github.com/goliatone/go-cms/internal/shortcode"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

func TestBuildRendersTemplateContext(t *testing.T) {
	ctx := context.Background()

	now := time.Date(2024, 2, 5, 14, 30, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}
	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
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
	if result.PagesSkipped != 0 {
		t.Fatalf("expected no skipped pages, got %d", result.PagesSkipped)
	}
	if result.AssetsSkipped != 0 {
		t.Fatalf("expected no skipped assets, got %d", result.AssetsSkipped)
	}

	calls := storage.ExecCalls()
	if len(calls) == 0 {
		t.Fatal("expected storage writes")
	}
	var pageOutputs []string
	for _, page := range result.Rendered {
		if page.Output == "" {
			t.Fatalf("expected output path for page %s", page.PageID)
		}
		if page.Checksum == "" {
			t.Fatalf("expected checksum for page %s", page.PageID)
		}
		pageOutputs = append(pageOutputs, page.Output)
	}
	for _, output := range pageOutputs {
		if !strings.HasSuffix(output, "index.html") {
			t.Fatalf("expected output to end with index.html, got %s", output)
		}
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

func TestRenderShortcodesInTemplateContext(t *testing.T) {
	svc := &service{
		deps: Dependencies{Shortcodes: newGeneratorShortcodeService(t)},
	}

	contentSummary := "Summary {{< alert type=\"success\" >}}Done{{< /alert >}}"
	pageSummary := "Page {{< alert type=\"info\" >}}Notice{{< /alert >}}"
	contentMap := map[string]any{
		"body": "Intro {{< alert type=\"warning\" >}}Check{{< /alert >}}",
	}

	tmpl := TemplateContext{
		Page: PageRenderingContext{
			Locale: LocaleSpec{Code: "en"},
			ContentTranslation: &content.ContentTranslation{
				Content: contentMap,
				Summary: &contentSummary,
			},
			Translation: &pages.PageTranslation{
				Summary: &pageSummary,
			},
		},
	}

	svc.renderShortcodesInTemplateContext(context.Background(), &tmpl)

	body, ok := tmpl.Page.ContentTranslation.Content["body"].(string)
	if !ok {
		t.Fatalf("expected body string, got %T", tmpl.Page.ContentTranslation.Content["body"])
	}
	if !strings.Contains(body, "shortcode--alert") {
		t.Fatalf("expected shortcode render in body, got %s", body)
	}
	if tmpl.Page.ContentTranslation.Summary == nil || !strings.Contains(*tmpl.Page.ContentTranslation.Summary, "shortcode--alert") {
		t.Fatalf("expected shortcode render in content summary, got %v", tmpl.Page.ContentTranslation.Summary)
	}
	if tmpl.Page.Translation == nil || tmpl.Page.Translation.Summary == nil || !strings.Contains(*tmpl.Page.Translation.Summary, "shortcode--alert") {
		t.Fatalf("expected shortcode render in page summary, got %v", tmpl.Page.Translation)
	}
}

func TestBuildSkipsPagesWithoutTranslations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2024, 8, 12, 9, 0, 0, 0, time.UTC)

	localeID := uuid.New()
	contentID := uuid.New()
	pageID := uuid.New()

	contentRecord := &content.Content{
		ID:        contentID,
		Slug:      "translation-optional",
		Status:    "published",
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-time.Hour),
	}

	pageRecord := &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: uuid.Nil,
		Slug:       "translation-optional",
		Status:     "published",
		IsVisible:  true,
		CreatedAt:  now.Add(-2 * time.Hour),
		UpdatedAt:  now.Add(-time.Hour),
		Blocks:     nil,
		Widgets:    nil,
		// Translations intentionally omitted to mimic optional translation mode.
	}

	cfg := Config{
		OutputDir:       "dist",
		DefaultLocale:   "en",
		Locales:         []string{"en"},
		CopyAssets:      false,
		GenerateSitemap: false,
		GenerateRobots:  false,
		GenerateFeeds:   false,
	}

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}

	svc := NewService(cfg, Dependencies{
		Pages: &stubPagesService{
			records: map[uuid.UUID]*pages.Page{
				pageID: pageRecord,
			},
			listing: []*pages.Page{pageRecord},
		},
		Content: &stubContentService{
			records: map[uuid.UUID]*content.Content{
				contentID: contentRecord,
			},
		},
		Menus:  newStubMenuService(),
		Themes: &stubThemesService{},
		Locales: &stubLocaleLookup{
			records: map[string]*content.Locale{
				"en": {
					ID:   localeID,
					Code: "en",
				},
			},
		},
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if result.PagesBuilt != 0 {
		t.Fatalf("expected no pages built, got %d", result.PagesBuilt)
	}
	if result.PagesSkipped != 0 {
		t.Fatalf("expected no skipped pages, got %d", result.PagesSkipped)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %d", len(result.Diagnostics))
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no build errors, got %d", len(result.Errors))
	}
	if len(result.Rendered) != 0 {
		t.Fatalf("expected no rendered pages, got %d", len(result.Rendered))
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
	storage := &recordingStorage{}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
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
	if result.PagesSkipped != 0 {
		t.Fatalf("expected no skipped pages, got %d", result.PagesSkipped)
	}
	if result.AssetsSkipped != 0 {
		t.Fatalf("expected no skipped assets, got %d", result.AssetsSkipped)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %d", len(result.Errors))
	}
	if renderer.maxConcurrent.Load() < 2 {
		t.Fatalf("expected at least 2 concurrent workers, got %d", renderer.maxConcurrent.Load())
	}
}

func TestBuildFailsOnRenderTimeout(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 3, 20, 12, 0, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)
	fixtures.Config.RenderTimeout = time.Millisecond

	renderer := &concurrentRenderer{
		recordingRenderer: recordingRenderer{},
		delay:             5 * time.Millisecond,
	}
	storage := &recordingStorage{}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{})
	if err == nil {
		t.Fatalf("expected render timeout error, got nil")
	}
	if result == nil {
		t.Fatalf("expected build result despite error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[0].Err == nil {
		t.Fatalf("expected diagnostic error for timeout")
	}
	if len(result.Errors) == 0 {
		t.Fatalf("expected recorded error slice")
	}
}

func TestBuildDryRunDiagnostics(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 4, 2, 18, 5, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}
	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
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
	if result.PagesSkipped != 0 {
		t.Fatalf("expected no skipped pages in dry-run, got %d", result.PagesSkipped)
	}
	if result.AssetsSkipped != 0 {
		t.Fatalf("expected no skipped assets in dry-run, got %d", result.AssetsSkipped)
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
	writeCalls := 0
	for _, call := range storage.ExecCalls() {
		if call.Query == storageOpWrite {
			writeCalls++
		}
	}
	if writeCalls != 0 {
		t.Fatalf("expected no storage writes for dry-run, got %d", writeCalls)
	}
}

func TestBuildGeneratesSitemapAndRobots(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)
	fixtures.Config.GenerateSitemap = true
	fixtures.Config.GenerateRobots = true

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if result.PagesSkipped != 0 {
		t.Fatalf("expected no skipped pages, got %d", result.PagesSkipped)
	}
	if result.AssetsSkipped != 0 {
		t.Fatalf("expected no skipped assets, got %d", result.AssetsSkipped)
	}

	var sitemapWritten, robotsWritten bool
	expectedSitemap := path.Join(fixtures.Config.OutputDir, "sitemap.xml")
	expectedRobots := path.Join(fixtures.Config.OutputDir, "robots.txt")
	for _, call := range storage.ExecCalls() {
		if call.Query != storageOpWrite {
			continue
		}
		if len(call.Args) == 0 {
			continue
		}
		target, ok := call.Args[0].(string)
		if !ok {
			continue
		}
		switch target {
		case expectedSitemap:
			sitemapWritten = true
		case expectedRobots:
			robotsWritten = true
		}
	}
	if !sitemapWritten {
		t.Fatalf("expected sitemap write to %s", expectedSitemap)
	}
	if !robotsWritten {
		t.Fatalf("expected robots write to %s", expectedRobots)
	}
}

func TestBuildSkipsSitemapAndFeedsWhenDisabled(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 5, 12, 18, 30, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)
	fixtures.Config.GenerateSitemap = false
	fixtures.Config.GenerateRobots = false
	fixtures.Config.GenerateFeeds = false

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if result.FeedsBuilt != 0 {
		t.Fatalf("expected no feeds built when disabled, got %d", result.FeedsBuilt)
	}
	if result.Metrics.SitemapDuration != 0 {
		t.Fatalf("expected zero sitemap duration when disabled, got %s", result.Metrics.SitemapDuration)
	}
	if result.Metrics.FeedDuration != 0 {
		t.Fatalf("expected zero feed duration when disabled, got %s", result.Metrics.FeedDuration)
	}

	for _, call := range storage.ExecCalls() {
		if call.Query != storageOpWrite {
			continue
		}
		if len(call.Args) < 4 {
			continue
		}
		category, _ := call.Args[3].(string)
		if category == string(categorySitemap) || category == string(categoryFeed) {
			t.Fatalf("unexpected artifact write with category %s while disabled", category)
		}
	}
}

func TestBuildCopiesThemeAssets(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 6, 10, 11, 30, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}
	assetResolver := newStubAssetResolver()

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
		Assets:   assetResolver,
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if result.AssetsBuilt != 2 {
		t.Fatalf("expected 2 assets copied, got %d", result.AssetsBuilt)
	}
	if result.AssetsSkipped != 0 {
		t.Fatalf("expected no skipped assets, got %d", result.AssetsSkipped)
	}
	expectedAssets := map[string]struct{}{
		path.Join(fixtures.Config.OutputDir, "assets/public/css/site.css"): {},
		path.Join(fixtures.Config.OutputDir, "assets/public/js/app.js"):    {},
	}
	for _, call := range storage.ExecCalls() {
		if call.Query != storageOpWrite {
			continue
		}
		if len(call.Args) < 4 {
			continue
		}
		target, ok := call.Args[0].(string)
		if !ok {
			continue
		}
		category, _ := call.Args[3].(string)
		if _, exists := expectedAssets[target]; exists {
			if category != string(categoryAsset) {
				t.Fatalf("expected asset category for %s, got %s", target, category)
			}
			delete(expectedAssets, target)
		}
	}
	if len(expectedAssets) != 0 {
		t.Fatalf("missing asset writes: %v", expectedAssets)
	}
}

func TestBuildReturnsErrorWhenStorageWriteFails(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 6, 15, 8, 0, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)

	renderer := &recordingRenderer{}
	storage := &failingStorage{failOnWrite: true}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{})
	if err == nil {
		t.Fatalf("expected storage write error")
	}
	if result == nil || len(result.Errors) == 0 {
		t.Fatalf("expected errors recorded in result")
	}
	if !strings.Contains(err.Error(), "storage write failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildFailsWhenThemesDisabled(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 6, 22, 10, 45, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   themes.NewNoOpService(),
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{})
	if err == nil {
		t.Fatalf("expected error when themes disabled")
	}
	if !errors.Is(err, errTemplateRequired) {
		t.Fatalf("expected errTemplateRequired, got %v", err)
	}
	if len(result.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for missing templates")
	}
	for _, diag := range result.Diagnostics {
		if diag.Err == nil {
			t.Fatalf("expected diagnostic error for missing template")
		}
	}
	for _, call := range storage.ExecCalls() {
		if call.Query == storageOpWrite {
			t.Fatalf("expected no artifact writes when template resolution fails")
		}
	}
}

func TestBuildSkipsPagesWithManifest(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 7, 15, 12, 0, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)
	fixtures.Config.Incremental = true

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	initialResult, err := svc.Build(ctx, BuildOptions{})
	if err != nil {
		t.Fatalf("initial build: %v", err)
	}
	if len(initialResult.Rendered) != fixtures.LocalizedCount() {
		t.Fatalf("expected %d rendered pages, got %d", fixtures.LocalizedCount(), len(initialResult.Rendered))
	}
	manifestTarget := joinOutputPath(strings.Trim(strings.TrimSpace(fixtures.Config.OutputDir), "/"), manifestFileName)
	if _, ok := storage.files[manifestTarget]; !ok {
		t.Fatalf("expected manifest written to %s", manifestTarget)
	}
	storedManifest, err := parseManifest(storage.files[manifestTarget])
	if err != nil {
		t.Fatalf("parse stored manifest: %v", err)
	}
	buildCtx, err := svc.loadContext(ctx, BuildOptions{})
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if len(storedManifest.Pages) != len(buildCtx.Pages) {
		t.Fatalf("expected manifest to contain %d pages, got %d", len(buildCtx.Pages), len(storedManifest.Pages))
	}
	for _, page := range buildCtx.Pages {
		route := safeTranslationPath(page.Translation)
		destRel := buildOutputPath(route, page.Locale.Code, buildCtx.DefaultLocale)
		output := joinOutputPath(strings.Trim(strings.TrimSpace(fixtures.Config.OutputDir), "/"), destRel)
		if !storedManifest.shouldSkipPage(page.Page.ID, page.Locale.Code, page.Metadata.Hash, output) {
			t.Fatalf("manifest mismatch for %s/%s", page.Page.ID, page.Locale.Code)
		}
	}

	expectedLocalized := fixtures.LocalizedCount()
	renderer.assertCalls(t, expectedLocalized)

	initialExecs := len(storage.ExecCalls())

	renderer2 := &recordingRenderer{}
	svc2 := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer2,
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)
	svc2.now = func() time.Time { return now.Add(30 * time.Minute) }

	result, err := svc2.Build(ctx, BuildOptions{})
	if err != nil {
		t.Fatalf("incremental build: %v", err)
	}

	if result.PagesBuilt != 0 {
		t.Fatalf("expected no rebuilt pages, got %d", result.PagesBuilt)
	}
	if result.PagesSkipped != expectedLocalized {
		t.Fatalf("expected %d skipped pages, got %d", expectedLocalized, result.PagesSkipped)
	}
	if len(result.Rendered) != 0 {
		t.Fatalf("expected no rendered outputs when skipping, got %d", len(result.Rendered))
	}
	if len(result.Diagnostics) != expectedLocalized {
		t.Fatalf("expected %d diagnostics, got %d", expectedLocalized, len(result.Diagnostics))
	}
	if result.AssetsBuilt != 0 {
		t.Fatalf("expected no assets copied, got %d", result.AssetsBuilt)
	}
	if result.AssetsSkipped != 0 {
		t.Fatalf("expected no skipped assets (no resolver), got %d", result.AssetsSkipped)
	}
	renderer2.assertCalls(t, 0)

	additionalPageWrites := 0
	execCalls := storage.ExecCalls()
	for _, call := range execCalls[initialExecs:] {
		if call.Query != storageOpWrite {
			continue
		}
		if len(call.Args) < 4 {
			continue
		}
		if category, _ := call.Args[3].(string); category == string(categoryPage) {
			additionalPageWrites++
		}
	}
	if additionalPageWrites != 0 {
		t.Fatalf("expected no additional page writes, got %d", additionalPageWrites)
	}
}

func TestBuildPageForcesRender(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 8, 4, 15, 0, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)
	fixtures.Config.Incremental = true

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	if _, err := svc.Build(ctx, BuildOptions{}); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	renderer.assertCalls(t, fixtures.LocalizedCount())

	initialExecs := len(storage.ExecCalls())
	targetPage := fixtures.PageIDs[0]
	locales := fixtures.Config.Locales

	renderer2 := &recordingRenderer{}
	svc2 := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer2,
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)
	svc2.now = func() time.Time { return now.Add(5 * time.Minute) }

	if err := svc2.BuildPage(ctx, targetPage, locales[0]); err != nil {
		t.Fatalf("build page: %v", err)
	}
	renderer2.assertCalls(t, 1)

	newCalls := storage.ExecCalls()[initialExecs:]
	pageWrites := 0
	for _, call := range newCalls {
		if call.Query != storageOpWrite {
			continue
		}
		if len(call.Args) < 4 {
			continue
		}
		category, _ := call.Args[3].(string)
		if category == string(categoryPage) {
			pageWrites++
		}
	}
	if pageWrites != 1 {
		t.Fatalf("expected 1 page rewrite, got %d", pageWrites)
	}
}

func TestBuildAssetsForcesCopy(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 9, 10, 9, 30, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)
	fixtures.Config.Incremental = true

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}
	assetResolver := newStubAssetResolver()

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Assets:   assetResolver,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	if _, err := svc.Build(ctx, BuildOptions{}); err != nil {
		t.Fatalf("initial build: %v", err)
	}
	initialCalls := len(storage.ExecCalls())

	renderer2 := &recordingRenderer{}
	svc2 := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer2,
		Storage:  storage,
		Assets:   assetResolver,
		Logger:   logging.NoOp(),
	}).(*service)
	svc2.now = func() time.Time { return now.Add(10 * time.Minute) }

	if err := svc2.BuildAssets(ctx); err != nil {
		t.Fatalf("build assets: %v", err)
	}
	newCalls := storage.ExecCalls()[initialCalls:]
	assetWrites := 0
	for _, call := range newCalls {
		if call.Query != storageOpWrite {
			continue
		}
		if len(call.Args) < 4 {
			continue
		}
		if category, _ := call.Args[3].(string); category == string(categoryAsset) {
			assetWrites++
		}
	}
	if assetWrites != 2 {
		t.Fatalf("expected 2 asset rewrites, got %d", assetWrites)
	}
}

func TestBuildFailsOnAssetCopyTimeout(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 9, 20, 10, 0, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)
	fixtures.Config.AssetCopyTimeout = time.Millisecond

	renderer := &recordingRenderer{}
	storage := &recordingStorage{}
	assetResolver := &delayedAssetResolver{
		base:  newStubAssetResolver(),
		delay: 5 * time.Millisecond,
	}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: renderer,
		Storage:  storage,
		Assets:   assetResolver,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	result, err := svc.Build(ctx, BuildOptions{})
	if err == nil {
		t.Fatalf("expected asset copy timeout error")
	}
	if result == nil || len(result.Errors) == 0 {
		t.Fatalf("expected errors recorded for asset timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if result.AssetsBuilt != 0 {
		t.Fatalf("expected no assets built on timeout, got %d", result.AssetsBuilt)
	}
}

func TestCleanInvokesStorageRemove(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 10, 1, 8, 0, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)
	storage := &recordingStorage{}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: &recordingRenderer{},
		Storage:  storage,
		Logger:   logging.NoOp(),
	}).(*service)

	if err := svc.Clean(ctx); err != nil {
		t.Fatalf("clean: %v", err)
	}
	found := false
	for _, call := range storage.ExecCalls() {
		if call.Query != storageOpRemove {
			continue
		}
		if len(call.Args) == 0 {
			continue
		}
		if target, _ := call.Args[0].(string); target == strings.Trim(strings.TrimSpace(fixtures.Config.OutputDir), "/") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected remove call for output directory")
	}
}

func TestGeneratorHooksInvoked(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 11, 5, 13, 15, 0, 0, time.UTC)
	fixtures := newRenderFixtures(now)
	storage := &recordingStorage{}
	assetResolver := newStubAssetResolver()

	type recorder struct {
		mu          sync.Mutex
		beforeBuild int
		afterBuild  int
		afterPage   int
		beforeClean int
		afterClean  int
	}
	rec := &recorder{}
	hooks := Hooks{
		BeforeBuild: func(context.Context, BuildOptions) error {
			rec.mu.Lock()
			rec.beforeBuild++
			rec.mu.Unlock()
			return nil
		},
		AfterBuild: func(context.Context, BuildOptions, *BuildResult) error {
			rec.mu.Lock()
			rec.afterBuild++
			rec.mu.Unlock()
			return nil
		},
		AfterPage: func(context.Context, RenderedPage) error {
			rec.mu.Lock()
			rec.afterPage++
			rec.mu.Unlock()
			return nil
		},
		BeforeClean: func(context.Context, string) error {
			rec.mu.Lock()
			rec.beforeClean++
			rec.mu.Unlock()
			return nil
		},
		AfterClean: func(context.Context, string) error {
			rec.mu.Lock()
			rec.afterClean++
			rec.mu.Unlock()
			return nil
		},
	}

	svc := NewService(fixtures.Config, Dependencies{
		Pages:    fixtures.Pages,
		Content:  fixtures.Content,
		Menus:    fixtures.Menus,
		Themes:   fixtures.Themes,
		Locales:  fixtures.Locales,
		Renderer: &recordingRenderer{},
		Storage:  storage,
		Assets:   assetResolver,
		Hooks:    hooks,
		Logger:   logging.NoOp(),
	}).(*service)
	svc.now = func() time.Time { return now }

	if _, err := svc.Build(ctx, BuildOptions{}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := svc.BuildAssets(ctx); err != nil {
		t.Fatalf("build assets: %v", err)
	}
	if err := svc.Clean(ctx); err != nil {
		t.Fatalf("clean: %v", err)
	}
	if err := svc.BuildPage(ctx, fixtures.PageIDs[0], ""); err != nil {
		t.Fatalf("build page: %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.beforeBuild != 3 {
		t.Fatalf("expected beforeBuild invoked 3 times, got %d", rec.beforeBuild)
	}
	if rec.afterBuild != 3 {
		t.Fatalf("expected afterBuild invoked 3 times, got %d", rec.afterBuild)
	}
	if rec.afterPage == 0 {
		t.Fatalf("expected afterPage hook to run")
	}
	if rec.beforeClean != 1 || rec.afterClean != 1 {
		t.Fatalf("expected clean hooks to run once, got %d/%d", rec.beforeClean, rec.afterClean)
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

type storageCall struct {
	Query string
	Args  []any
}

type recordingStorage struct {
	mu    sync.Mutex
	execs []storageCall
	files map[string][]byte
}

func (s *recordingStorage) Exec(_ context.Context, query string, args ...any) (interfaces.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if query == storageOpWrite && len(args) >= 2 {
		if target, ok := args[0].(string); ok {
			if reader, ok := args[1].(io.Reader); ok && reader != nil {
				data, err := io.ReadAll(reader)
				if err == nil {
					if s.files == nil {
						s.files = map[string][]byte{}
					}
					s.files[target] = append([]byte(nil), data...)
				}
			}
		}
	}
	if query == storageOpRemove && len(args) >= 1 {
		if target, ok := args[0].(string); ok {
			if s.files != nil {
				for path := range s.files {
					if path == target || strings.HasPrefix(path, strings.TrimRight(target, "/")+"/") {
						delete(s.files, path)
					}
				}
			}
		}
	}
	copied := append([]any(nil), args...)
	s.execs = append(s.execs, storageCall{
		Query: query,
		Args:  copied,
	})
	return noopResult{}, nil
}

func (s *recordingStorage) Query(_ context.Context, query string, args ...any) (interfaces.Rows, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := append([]any(nil), args...)
	s.execs = append(s.execs, storageCall{
		Query: query,
		Args:  copied,
	})
	if query == storageOpRead && len(args) > 0 {
		if target, ok := args[0].(string); ok {
			if data, ok := s.files[target]; ok {
				return &bufferedRows{
					data: [][]byte{append([]byte(nil), data...)},
				}, nil
			}
		}
	}
	return &bufferedRows{}, nil
}

func (s *recordingStorage) Transaction(_ context.Context, fn func(tx interfaces.Transaction) error) error {
	if fn == nil {
		return nil
	}
	return fn(&recordingTx{storage: s})
}

func (s *recordingStorage) ExecCalls() []storageCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	calls := make([]storageCall, len(s.execs))
	copy(calls, s.execs)
	return calls
}

type failingStorage struct {
	recordingStorage
	failOnWrite bool
}

func (s *failingStorage) Exec(ctx context.Context, query string, args ...any) (interfaces.Result, error) {
	if query == storageOpWrite && s.failOnWrite {
		return nil, fmt.Errorf("storage write failed")
	}
	return s.recordingStorage.Exec(ctx, query, args...)
}

type recordingTx struct {
	storage *recordingStorage
}

func (tx *recordingTx) Exec(ctx context.Context, query string, args ...any) (interfaces.Result, error) {
	return tx.storage.Exec(ctx, query, args...)
}

func (tx *recordingTx) Query(ctx context.Context, query string, args ...any) (interfaces.Rows, error) {
	return tx.storage.Query(ctx, query, args...)
}

func (recordingTx) Transaction(context.Context, func(interfaces.Transaction) error) error {
	return fmt.Errorf("nested transactions not supported")
}

func (recordingTx) Commit() error   { return nil }
func (recordingTx) Rollback() error { return nil }

type noopResult struct{}

func (noopResult) RowsAffected() (int64, error) { return 0, nil }
func (noopResult) LastInsertId() (int64, error) { return 0, nil }

type bufferedRows struct {
	data  [][]byte
	index int
}

func (r *bufferedRows) Next() bool {
	if r.index >= len(r.data) {
		return false
	}
	r.index++
	return true
}

func (r *bufferedRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.data) {
		return fmt.Errorf("buffered rows: scan without next")
	}
	if len(dest) == 0 {
		return fmt.Errorf("buffered rows: missing destination")
	}
	value := r.data[r.index-1]
	switch target := dest[0].(type) {
	case *[]byte:
		*target = append((*target)[:0], value...)
		return nil
	case *string:
		*target = string(value)
		return nil
	default:
		return fmt.Errorf("buffered rows: unsupported scan type %T", dest[0])
	}
}

func (r *bufferedRows) Close() error { return nil }

type stubAssetResolver struct {
	assets map[string][]byte
}

func newStubAssetResolver() *stubAssetResolver {
	return &stubAssetResolver{
		assets: map[string][]byte{
			"public/css/site.css": []byte("body {}"),
			"public/js/app.js":    []byte("console.log('ok')"),
		},
	}
}

func (r *stubAssetResolver) Open(_ context.Context, _ *themes.Theme, asset string) (io.ReadCloser, error) {
	data, ok := r.assets[asset]
	if !ok {
		return nil, fmt.Errorf("asset %s not found", asset)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (r *stubAssetResolver) ResolvePath(_ *themes.Theme, asset string) (string, error) {
	if _, ok := r.assets[asset]; !ok {
		return "", fmt.Errorf("asset %s not found", asset)
	}
	return asset, nil
}

type delayedAssetResolver struct {
	base  *stubAssetResolver
	delay time.Duration
}

func (r *delayedAssetResolver) Open(ctx context.Context, theme *themes.Theme, asset string) (io.ReadCloser, error) {
	if r.base == nil {
		return nil, fmt.Errorf("nil base resolver")
	}
	select {
	case <-time.After(r.delay):
		return r.base.Open(ctx, theme, asset)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *delayedAssetResolver) ResolvePath(theme *themes.Theme, asset string) (string, error) {
	if r.base == nil {
		return "", fmt.Errorf("nil base resolver")
	}
	return r.base.ResolvePath(theme, asset)
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

	basePath := "public"
	themeRecord := &themes.Theme{
		ID:        themeID,
		Name:      "aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
		Templates: []*themes.Template{templateRecord},
		Config: themes.ThemeConfig{
			Assets: &themes.ThemeAssets{
				BasePath: &basePath,
				Styles:   []string{"css/site.css"},
				Scripts:  []string{"js/app.js"},
			},
		},
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

func newGeneratorShortcodeService(tb testing.TB) interfaces.ShortcodeService {
	validator := shortcodepkg.NewValidator()
	registry := shortcodepkg.NewRegistry(validator)
	if err := shortcodepkg.RegisterBuiltIns(registry, nil); err != nil {
		tb.Fatalf("RegisterBuiltIns: %v", err)
	}
	renderer := shortcodepkg.NewRenderer(registry, validator)
	return shortcodepkg.NewService(registry, renderer)
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
