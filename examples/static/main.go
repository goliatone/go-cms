package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/examples/genutil"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/google/uuid"
)

var (
	authorID  = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	enLocale  = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	esLocale  = uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	pageType  = uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	outputDir = filepath.ToSlash("dist/static-demo")
)

func main() {
	ctx := context.Background()

	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("resolve working directory: %v", err)
	}

	themeDir := filepath.Join(projectRoot, "theme")
	templateDir := filepath.Join(themeDir, "templates")

	renderer, err := genutil.NewGoTemplateRenderer(templateDir)
	if err != nil {
		log.Fatalf("configure template renderer: %v", err)
	}
	storage := genutil.NewFilesystemStorage(filepath.Join(projectRoot, outputDir), outputDir)
	assets := genutil.NewThemeAssetResolver()

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}
	cfg.Features.Themes = true
	cfg.Features.Widgets = false
	cfg.Themes.DefaultTheme = "demo-theme"
	cfg.Themes.DefaultVariant = ""
	cfg.Themes.PartialFallbacks = map[string]string{
		"layout.header": "templates/layout.tmpl",
		"layout.footer": "templates/layout.tmpl",
	}
	cfg.Generator.Enabled = true
	cfg.Generator.OutputDir = outputDir
	cfg.Generator.BaseURL = ""
	cfg.Generator.CopyAssets = true
	cfg.Generator.GenerateSitemap = true
	cfg.Generator.CleanBuild = true
	cfg.Generator.Menus = map[string]string{
		"primary": "main",
	}

	module, err := cms.New(cfg,
		di.WithTemplate(renderer),
		di.WithGeneratorStorage(storage),
		di.WithGeneratorAssetResolver(assets),
	)
	if err != nil {
		log.Fatalf("bootstrap module: %v", err)
	}

	container := module.Container()
	if err := seedLocales(container.LocaleRepository()); err != nil {
		log.Fatalf("seed locales: %v", err)
	}

	templateID, err := seedTheme(ctx, container.ThemeService(), themeDir)
	if err != nil {
		log.Fatalf("seed theme: %v", err)
	}

	contentBySlug, err := seedContent(ctx, module.Content(), container.ContentTypeRepository())
	if err != nil {
		log.Fatalf("seed content: %v", err)
	}

	pagesBySlug, err := seedPages(ctx, module.Pages(), contentBySlug, templateID)
	if err != nil {
		log.Fatalf("seed pages: %v", err)
	}

	if err := seedMenus(ctx, container.MenuService(), pagesBySlug); err != nil {
		log.Fatalf("seed menus: %v", err)
	}

	_ = os.RemoveAll(filepath.Join(projectRoot, outputDir))

	start := time.Now()
	result, err := module.Generator().Build(ctx, generator.BuildOptions{
		Locales: []string{"en", "es"},
	})
	if err != nil {
		log.Fatalf("static build failed: %v", err)
	}

	log.Printf("static build complete: pages=%d assets=%d duration=%s", result.PagesBuilt, result.AssetsBuilt, time.Since(start).Truncate(time.Millisecond))
	log.Printf("output written to %s", outputDir)
}

func seedLocales(repo content.LocaleRepository) error {
	seeder, ok := repo.(interface{ Put(*content.Locale) })
	if !ok {
		return nil
	}
	now := time.Now().UTC()

	seeder.Put(&content.Locale{
		ID:        enLocale,
		Code:      "en",
		Display:   "English",
		IsActive:  true,
		IsDefault: true,
		CreatedAt: now,
	})
	seeder.Put(&content.Locale{
		ID:        esLocale,
		Code:      "es",
		Display:   "Español",
		IsActive:  true,
		IsDefault: false,
		CreatedAt: now,
	})
	return nil
}

func seedTheme(ctx context.Context, svc themes.Service, themeDir string) (uuid.UUID, error) {
	if svc == nil {
		return uuid.Nil, nil
	}

	theme, err := svc.RegisterTheme(ctx, themes.RegisterThemeInput{
		Name:      "demo-theme",
		Version:   "1.0.0",
		ThemePath: filepath.ToSlash(themeDir),
		Config: themes.ThemeConfig{
			Assets: &themes.ThemeAssets{
				BasePath: stringPtr("assets"),
				Styles:   []string{"theme.css"},
				Images:   []string{"logo.svg"},
			},
			MenuLocations: []themes.ThemeMenuLocation{
				{Code: "primary", Name: "Primary Navigation"},
			},
		},
	})
	if err != nil {
		return uuid.Nil, err
	}

	template, err := svc.RegisterTemplate(ctx, themes.RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "Standard Page",
		Slug:         "standard-page",
		TemplatePath: "page.tmpl",
		Regions: map[string]themes.TemplateRegion{
			"main": {Name: "Main", AcceptsBlocks: true},
		},
	})
	if err != nil {
		return uuid.Nil, err
	}

	if _, err := svc.ActivateTheme(ctx, theme.ID); err != nil {
		return uuid.Nil, err
	}

	return template.ID, nil
}

func seedContent(ctx context.Context, svc content.Service, repo content.ContentTypeRepository) (map[string]*content.Content, error) {
	if svc == nil {
		return nil, nil
	}

	if seeder, ok := repo.(interface {
		Put(*content.ContentType) error
	}); ok {
		if err := seeder.Put(&content.ContentType{
			ID:   pageType,
			Name: "Demo Page",
			Slug: "demo-page",
			Schema: map[string]any{
				"fields": map[string]string{"body": "string"},
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}); err != nil {
			return nil, err
		}
	}

	pages := []struct {
		Slug         string
		Translations map[string]struct {
			Title string
			Path  string
			Body  string
		}
	}{
		{
			Slug: "home",
			Translations: map[string]struct {
				Title string
				Path  string
				Body  string
			}{
				"en": {Title: "Home", Path: "/", Body: "Welcome to the static Go CMS demo."},
				"es": {Title: "Inicio", Path: "/es", Body: "Bienvenido a la demostración estática de Go CMS."},
			},
		},
		{
			Slug: "company",
			Translations: map[string]struct {
				Title string
				Path  string
				Body  string
			}{
				"en": {Title: "Company", Path: "/company", Body: "Learn more about the team behind your content experience."},
				"es": {Title: "Empresa", Path: "/es/empresa", Body: "Conoce al equipo detrás de tu experiencia de contenido."},
			},
		},
		{
			Slug: "services",
			Translations: map[string]struct {
				Title string
				Path  string
				Body  string
			}{
				"en": {Title: "Services", Path: "/services", Body: "Discover our consulting, integration, and support services."},
				"es": {Title: "Servicios", Path: "/es/servicios", Body: "Descubre nuestros servicios de consultoría, integración y soporte."},
			},
		},
		{
			Slug: "blog",
			Translations: map[string]struct {
				Title string
				Path  string
				Body  string
			}{
				"en": {Title: "Blog", Path: "/blog", Body: "Read implementation stories, product updates, and how-to guides."},
				"es": {Title: "Blog", Path: "/es/blog", Body: "Lee historias de implementación, novedades del producto y guías prácticas."},
			},
		},
		{
			Slug: "contact",
			Translations: map[string]struct {
				Title string
				Path  string
				Body  string
			}{
				"en": {Title: "Contact", Path: "/contact", Body: "Get in touch for demos, pricing, or partnership opportunities."},
				"es": {Title: "Contacto", Path: "/es/contacto", Body: "Ponte en contacto para demos, precios u oportunidades de colaboración."},
			},
		},
	}

	result := make(map[string]*content.Content, len(pages))

	for _, page := range pages {
		translations := make([]content.ContentTranslationInput, 0, len(page.Translations))
		for locale, tr := range page.Translations {
			translations = append(translations, content.ContentTranslationInput{
				Locale:  locale,
				Title:   tr.Title,
				Content: map[string]any{"body": tr.Body},
			})
		}
		record, err := svc.Create(ctx, content.CreateContentRequest{
			ContentTypeID: pageType,
			Slug:          page.Slug,
			Status:        "published",
			CreatedBy:     authorID,
			UpdatedBy:     authorID,
			Translations:  translations,
		})
		if err != nil {
			return nil, err
		}
		result[page.Slug] = record
	}

	return result, nil
}

func seedPages(ctx context.Context, svc pages.Service, contents map[string]*content.Content, templateID uuid.UUID) (map[string]*pages.Page, error) {
	if svc == nil {
		return nil, nil
	}

	result := make(map[string]*pages.Page, len(contents))

	for slug, contentRecord := range contents {
		var translations []pages.PageTranslationInput
		switch slug {
		case "home":
			translations = []pages.PageTranslationInput{
				{Locale: "en", Title: "Home", Path: "/"},
				{Locale: "es", Title: "Inicio", Path: "/es"},
			}
		case "company":
			translations = []pages.PageTranslationInput{
				{Locale: "en", Title: "Company", Path: "/company"},
				{Locale: "es", Title: "Empresa", Path: "/es/empresa"},
			}
		case "services":
			translations = []pages.PageTranslationInput{
				{Locale: "en", Title: "Services", Path: "/services"},
				{Locale: "es", Title: "Servicios", Path: "/es/servicios"},
			}
		case "blog":
			translations = []pages.PageTranslationInput{
				{Locale: "en", Title: "Blog", Path: "/blog"},
				{Locale: "es", Title: "Blog", Path: "/es/blog"},
			}
		case "contact":
			translations = []pages.PageTranslationInput{
				{Locale: "en", Title: "Contact", Path: "/contact"},
				{Locale: "es", Title: "Contacto", Path: "/es/contacto"},
			}
		}

		record, err := svc.Create(ctx, pages.CreatePageRequest{
			ContentID:    contentRecord.ID,
			TemplateID:   templateID,
			Slug:         slug,
			Status:       "published",
			CreatedBy:    authorID,
			UpdatedBy:    authorID,
			Translations: translations,
		})
		if err != nil {
			return nil, err
		}
		result[slug] = record
	}
	return result, nil
}

func seedMenus(ctx context.Context, svc menus.Service, pages map[string]*pages.Page) error {
	if svc == nil {
		return nil
	}

	menu, err := svc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "main",
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		return err
	}

	add := func(position int, slug, labelEN, labelES string) error {
		target := map[string]any{
			"type":    "page",
			"slug":    slug,
			"page_id": pages[slug].ID.String(),
		}
		_, err := svc.AddMenuItem(ctx, menus.AddMenuItemInput{
			MenuID:    menu.ID,
			Position:  position,
			Target:    target,
			CreatedBy: authorID,
			UpdatedBy: authorID,
			Translations: []menus.MenuItemTranslationInput{
				{Locale: "en", Label: labelEN},
				{Locale: "es", Label: labelES},
			},
		})
		return err
	}

	if err := add(0, "home", "Home", "Inicio"); err != nil {
		return err
	}
	if err := add(1, "company", "Company", "Empresa"); err != nil {
		return err
	}
	if err := add(2, "services", "Services", "Servicios"); err != nil {
		return err
	}
	if err := add(3, "blog", "Blog", "Blog"); err != nil {
		return err
	}
	if err := add(4, "contact", "Contact", "Contacto"); err != nil {
		return err
	}
	return nil
}

func stringPtr(value string) *string {
	return &value
}
