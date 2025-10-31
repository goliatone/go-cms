package generator_test

import (
	"context"
	"fmt"
	"io"
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
	"github.com/google/uuid"
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

	writes := memStorage.ExecCalls()
	pageWrites := 0
	for _, call := range writes {
		if call.Query != "generator.write" {
			continue
		}
		if len(call.Args) == 0 {
			continue
		}
		if path, ok := call.Args[0].(string); ok && strings.HasSuffix(path, "index.html") {
			pageWrites++
		}
	}
	if pageWrites != 2 {
		t.Fatalf("expected 2 page writes, got %d", pageWrites)
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
