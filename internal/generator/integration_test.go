package generator_test

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	ditesting "github.com/goliatone/go-cms/internal/di/testing"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestIntegrationBuildWithMemoryRepositories(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 12, 1, 9, 0, 0, 0, time.UTC)

	cfg := runtimeconfig.DefaultConfig()
	cfg.Features.Themes = true
	cfg.Generator.Enabled = true
	cfg.Generator.OutputDir = "dist"
	cfg.Generator.BaseURL = "https://example.test"
	cfg.Generator.GenerateSitemap = true
	cfg.Generator.GenerateRobots = true
	cfg.Generator.GenerateFeeds = true
	cfg.Generator.Menus = map[string]string{}
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}

	renderer := &integrationRenderer{}
	container, memStorage, err := ditesting.NewGeneratorContainer(cfg, di.WithTemplate(renderer))
	if err != nil {
		t.Fatalf("build container: %v", err)
	}

	themeSvc := container.ThemeService().(themes.Service)
	template, _ := registerThemeFixtures(t, ctx, themeSvc)

	contentRepo := container.ContentRepository()
	localeRepo := container.LocaleRepository()
	pageRepo := container.PageRepository()

	enLocale, err := localeRepo.GetByCode(ctx, "en")
	if err != nil {
		t.Fatalf("lookup en locale: %v", err)
	}
	esLocale, err := localeRepo.GetByCode(ctx, "es")
	if err != nil {
		t.Fatalf("lookup es locale: %v", err)
	}

	contentID := uuid.New()
	contentRecord := &content.Content{
		ID:             contentID,
		ContentTypeID:  uuid.New(),
		CurrentVersion: 1,
		Status:         "published",
		Slug:           "company",
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		CreatedAt:      now,
		UpdatedAt:      now,
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  enLocale.ID,
				Title:     "Company",
				Content:   map[string]any{"body": "english"},
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  esLocale.ID,
				Title:     "Empresa",
				Content:   map[string]any{"body": "espanol"},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	if _, err := contentRepo.Create(ctx, contentRecord); err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageID := uuid.New()
	pageRecord := &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: template.ID,
		Slug:       "company",
		Status:     "published",
		IsVisible:  true,
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
		CreatedAt:  now,
		UpdatedAt:  now,
		Translations: []*pages.PageTranslation{
			{
				ID:        uuid.New(),
				PageID:    pageID,
				LocaleID:  enLocale.ID,
				Title:     "Company",
				Path:      "/company",
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        uuid.New(),
				PageID:    pageID,
				LocaleID:  esLocale.ID,
				Title:     "Empresa",
				Path:      "/es/empresa",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	if _, err := pageRepo.Create(ctx, pageRecord); err != nil {
		t.Fatalf("create page: %v", err)
	}

	svc := container.GeneratorService()
	result, err := svc.Build(ctx, generator.BuildOptions{})
	if err != nil {
		t.Fatalf("integration build: %v", err)
	}
	if result == nil {
		t.Fatalf("expected result")
	}
	if result.PagesBuilt != 2 {
		t.Fatalf("expected 2 pages built, got %d", result.PagesBuilt)
	}
	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected diagnostics for two pages, got %d", len(result.Diagnostics))
	}
	if result.Duration == 0 {
		t.Fatalf("expected non-zero duration")
	}
	if result.Metrics.RenderDuration == 0 {
		t.Fatalf("expected render metrics recorded")
	}
	if result.FeedsBuilt == 0 {
		t.Fatalf("expected feed artifacts to be generated")
	}

	writes := memStorage.ExecCalls()
	pageWrites := 0
	expectedFeeds := map[string]struct{}{
		path.Join(cfg.Generator.OutputDir, "feeds/en.rss.xml"):  {},
		path.Join(cfg.Generator.OutputDir, "feeds/en.atom.xml"): {},
		path.Join(cfg.Generator.OutputDir, "feeds/es.rss.xml"):  {},
		path.Join(cfg.Generator.OutputDir, "feeds/es.atom.xml"): {},
		path.Join(cfg.Generator.OutputDir, "feed.xml"):          {},
		path.Join(cfg.Generator.OutputDir, "feed.atom.xml"):     {},
	}
	var sitemapWritten bool
	for _, call := range writes {
		if call.Query != "generator.write" {
			continue
		}
		if len(call.Args) == 0 {
			continue
		}
		target, ok := call.Args[0].(string)
		if !ok {
			continue
		}
		if strings.HasSuffix(target, "index.html") {
			pageWrites++
		}
		category, _ := call.Args[3].(string)
		if category == "sitemap" && strings.HasSuffix(target, "sitemap.xml") {
			sitemapWritten = true
		}
		if category == "feed" {
			if _, exists := expectedFeeds[target]; exists {
				delete(expectedFeeds, target)
			}
		}
	}
	if pageWrites != 2 {
		t.Fatalf("expected 2 page writes, got %d", pageWrites)
	}
	if !sitemapWritten {
		t.Fatalf("expected sitemap artifact to be written")
	}
	if len(expectedFeeds) != 0 {
		t.Fatalf("missing feed writes: %v", expectedFeeds)
	}
}

func TestIntegrationBuildFeedsIncrementalWithSQLite(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2024, 11, 20, 12, 0, 0, 0, time.UTC)

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)
	registerGeneratorModels(t, bunDB)

	cfg := runtimeconfig.DefaultConfig()
	cfg.Features.Themes = true
	cfg.Generator.Enabled = true
	cfg.Generator.OutputDir = "dist"
	cfg.Generator.BaseURL = "https://example.test"
	cfg.Generator.GenerateSitemap = true
	cfg.Generator.GenerateFeeds = true
	cfg.Generator.GenerateRobots = false
	cfg.Generator.CopyAssets = false
	cfg.Generator.Incremental = true
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}

	renderer := &integrationRenderer{}
	container, memStorage, err := ditesting.NewGeneratorContainer(cfg, di.WithBunDB(bunDB), di.WithTemplate(renderer))
	if err != nil {
		t.Fatalf("build container: %v", err)
	}

	enLocaleID := uuid.New()
	esLocaleID := uuid.New()

	enLocale := &content.Locale{
		ID:        enLocaleID,
		Code:      "en",
		Display:   "English",
		IsActive:  true,
		IsDefault: true,
		CreatedAt: now,
	}
	esLocale := &content.Locale{
		ID:        esLocaleID,
		Code:      "es",
		Display:   "Spanish",
		IsActive:  true,
		IsDefault: false,
		CreatedAt: now,
	}
	if _, err := bunDB.NewInsert().Model(enLocale).Exec(ctx); err != nil {
		t.Fatalf("insert en locale: %v", err)
	}
	if _, err := bunDB.NewInsert().Model(esLocale).Exec(ctx); err != nil {
		t.Fatalf("insert es locale: %v", err)
	}

	contentTypeID := uuid.New()
	contentType := &content.ContentType{
		ID:        contentTypeID,
		Name:      "page",
		Schema:    map[string]any{"fields": []map[string]any{{"name": "body", "type": "richtext"}}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := bunDB.NewInsert().Model(contentType).Exec(ctx); err != nil {
		t.Fatalf("insert content type: %v", err)
	}

	themeSvc := container.ThemeService().(themes.Service)
	template, _ := registerThemeFixtures(t, ctx, themeSvc)

	contentRepo := container.ContentRepository()
	pageRepo := container.PageRepository()

	publishedAt := now.Add(-6 * time.Hour)
	authorID := uuid.New()
	contentID := uuid.New()
	contentRecord := &content.Content{
		ID:             contentID,
		ContentTypeID:  contentTypeID,
		CurrentVersion: 1,
		Status:         "published",
		Slug:           "news",
		PublishedAt:    &publishedAt,
		CreatedBy:      authorID,
		UpdatedBy:      authorID,
		CreatedAt:      now,
		UpdatedAt:      now,
		Translations: []*content.ContentTranslation{
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  enLocaleID,
				Title:     "News",
				Summary:   strPtr("Latest company news"),
				Content:   map[string]any{"body": "english body"},
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        uuid.New(),
				ContentID: contentID,
				LocaleID:  esLocaleID,
				Title:     "Noticias",
				Summary:   strPtr("Últimas noticias"),
				Content:   map[string]any{"body": "spanish body"},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	if _, err := contentRepo.Create(ctx, contentRecord); err != nil {
		t.Fatalf("create content: %v", err)
	}

	pageID := uuid.New()
	pageRecord := &pages.Page{
		ID:          pageID,
		ContentID:   contentID,
		TemplateID:  template.ID,
		Slug:        "news",
		Status:      "published",
		PublishedAt: &publishedAt,
		IsVisible:   true,
		CreatedBy:   authorID,
		UpdatedBy:   authorID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Translations: []*pages.PageTranslation{
			{
				ID:        uuid.New(),
				PageID:    pageID,
				LocaleID:  enLocaleID,
				Title:     "News",
				Path:      "/news",
				Summary:   strPtr("English summary"),
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        uuid.New(),
				PageID:    pageID,
				LocaleID:  esLocaleID,
				Title:     "Noticias",
				Path:      "/es/noticias",
				Summary:   strPtr("Resumen español"),
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	if _, err := pageRepo.Create(ctx, pageRecord); err != nil {
		t.Fatalf("create page: %v", err)
	}

	svc := container.GeneratorService()

	firstResult, err := svc.Build(ctx, generator.BuildOptions{})
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	if firstResult.FeedsBuilt == 0 {
		t.Fatalf("expected feeds generated on first build")
	}

	initialCalls := len(memStorage.ExecCalls())

	secondResult, err := svc.Build(ctx, generator.BuildOptions{})
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if secondResult.FeedsBuilt == 0 {
		t.Fatalf("expected feeds generated on incremental build")
	}

	newCalls := memStorage.ExecCalls()[initialCalls:]
	pageWrites := 0
	feedWrites := 0
	sitemapWrites := 0
	for _, call := range newCalls {
		if call.Query != "generator.write" || len(call.Args) < 4 {
			continue
		}
		target, _ := call.Args[0].(string)
		category, _ := call.Args[3].(string)
		if category == "page" && strings.HasSuffix(target, "index.html") {
			pageWrites++
		}
		if category == "feed" {
			feedWrites++
		}
		if category == "sitemap" {
			sitemapWrites++
		}
	}
	if pageWrites != 0 {
		t.Fatalf("expected no page writes on incremental build, got %d", pageWrites)
	}
	if feedWrites == 0 {
		t.Fatalf("expected feed rewrites on incremental build")
	}
	if sitemapWrites == 0 {
		t.Fatalf("expected sitemap rewrite on incremental build")
	}
}

func registerThemeFixtures(t *testing.T, ctx context.Context, svc themes.Service) (*themes.Template, *themes.Theme) {
	t.Helper()
	theme, err := svc.RegisterTheme(ctx, themes.RegisterThemeInput{
		Name:      "aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
	})
	if err != nil {
		t.Fatalf("register theme: %v", err)
	}
	tmpl, err := svc.RegisterTemplate(ctx, themes.RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "page",
		Slug:         "page",
		TemplatePath: "themes/aurora/page.html",
		Regions: map[string]themes.TemplateRegion{
			"main": {
				Name:          "Main",
				AcceptsBlocks: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("register template: %v", err)
	}
	return tmpl, theme
}

func registerGeneratorModels(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()

	models := []any{
		(*content.Locale)(nil),
		(*content.ContentType)(nil),
		(*content.Content)(nil),
		(*content.ContentTranslation)(nil),
		(*content.ContentVersion)(nil),
		(*pages.Page)(nil),
		(*pages.PageTranslation)(nil),
		(*pages.PageVersion)(nil),
		(*themes.Theme)(nil),
		(*themes.Template)(nil),
	}

	for _, model := range models {
		if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			t.Fatalf("create table %T: %v", model, err)
		}
	}
}

func strPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v := value
	return &v
}

type integrationRenderer struct{}

func (integrationRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	return integrationRenderer{}.RenderTemplate(name, data, out...)
}

func (integrationRenderer) RenderTemplate(name string, data any, out ...io.Writer) (string, error) {
	ctx, ok := data.(generator.TemplateContext)
	if !ok {
		return "", fmt.Errorf("unexpected template context %T", data)
	}
	return fmt.Sprintf("<html><body>%s-%s</body></html>", name, ctx.Page.Locale.Code), nil
}

func (integrationRenderer) RenderString(templateContent string, data any, out ...io.Writer) (string, error) {
	return integrationRenderer{}.RenderTemplate(templateContent, data, out...)
}

func (integrationRenderer) RegisterFilter(string, func(any, any) (any, error)) error {
	return nil
}

func (integrationRenderer) GlobalContext(any) error { return nil }
