package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/adapters/noop"
	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	router "github.com/goliatone/go-router"
	urlkit "github.com/goliatone/go-urlkit"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	// Configure CMS
	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}
	cfg.Features.Widgets = true
	cfg.Features.Themes = true

	// Configure URL routing for menus
	cfg.Navigation.RouteConfig = &urlkit.Config{
		Groups: []urlkit.GroupConfig{
			{
				Name:    "frontend",
				BaseURL: "http://localhost:3000",
				Paths: map[string]string{
					"home":  "/",
					"page":  "/pages/:slug",
					"about": "/about",
				},
				Groups: []urlkit.GroupConfig{
					{
						Name: "es",
						Path: "/es",
						Paths: map[string]string{
							"home":  "/",
							"page":  "/paginas/:slug",
							"about": "/acerca-de",
						},
					},
				},
			},
		},
	}
	cfg.Navigation.URLKit = cms.URLKitResolverConfig{
		DefaultGroup: "frontend",
		LocaleGroups: map[string]string{
			"es": "frontend.es",
		},
		DefaultRoute: "page",
		SlugParam:    "slug",
	}

	// Initialize CMS
	module, err := cms.New(cfg, di.WithCacheProvider(noop.Cache()))
	if err != nil {
		log.Fatalf("initialize cms: %v", err)
	}

	// Setup demo data
	if err := setupDemoData(ctx, module, &cfg); err != nil {
		log.Fatalf("setup demo data: %v", err)
	}

	// Create go-router Fiber server with built-in template engine
	viewsDir := "./views"
	viewCfg := router.NewSimpleViewConfig(viewsDir).
		WithReload(true)

	viewEngine, err := router.InitializeViewEngine(viewCfg)
	if err != nil {
		log.Fatalf("initialize view engine: %v", err)
	}

	server := router.NewFiberAdapter(func(a *fiber.App) *fiber.App {
		return fiber.New(fiber.Config{
			AppName:           "go-cms Web Example",
			EnablePrintRoutes: true,
			PassLocalsToViews: true,
			Views:             viewEngine,
		})
	})
	r := server.Router()

	// Setup routes
	setupRoutes(r, module, &cfg)

	// Serve static files
	r.Static("/static", "./static")

	// Start server with graceful shutdown
	addr := ":3000"
	log.Printf("Starting web server on %s", addr)
	log.Printf("Open http://localhost:3000 in your browser")

	// Handle graceful shutdown
	go func() {
		if err := server.Serve(addr); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func setupRoutes(r router.Router[*fiber.App], module *cms.Module, cfg *cms.Config) {
	pageSvc := module.Pages()
	menuSvc := module.Menus()
	widgetSvc := module.Widgets()
	contentSvc := module.Content()
	container := module.Container()

	// Home page
	r.Get("/", func(ctx router.Context) error {
		locale := ctx.Query("locale", "en")

		// Get primary menu
		navigation, err := menuSvc.ResolveNavigation(ctx.Context(), "primary", locale)
		if err != nil && !errors.Is(err, menus.ErrMenuNotFound) {
			return err
		}

		// Get all pages
		pagesList, err := pageSvc.List(ctx.Context())
		if err != nil {
			return err
		}

		// Transform pages to locale-specific display format
		pagesDisplay := make([]map[string]any, 0, len(pagesList))
		for _, page := range pagesList {
			pagesDisplay = append(pagesDisplay, map[string]any{
				"slug":   page.Slug,
				"status": page.Status,
				"title":  getPageTitle(page, locale, container),
			})
		}

		// Render template
		return ctx.Render("index", map[string]any{
			"title":      "Welcome to go-cms",
			"locale":     locale,
			"menu":       navigation,
			"pages":      pagesDisplay,
			"currentURL": "/",
		})
	})

	// Page by slug
	r.Get("/pages/:slug", func(ctx router.Context) error {
		slug := ctx.Param("slug")
		locale := ctx.Query("locale", "en")

		// Find page by slug
		pagesList, err := pageSvc.List(ctx.Context())
		if err != nil {
			return err
		}

		var page *pages.Page
		for _, p := range pagesList {
			if p.Slug == slug {
				page = p
				break
			}
		}

		if page == nil {
			return ctx.Render("error", map[string]any{
				"error_code":    404,
				"error_title":   "Page Not Found",
				"error_message": fmt.Sprintf("The page '%s' does not exist.", slug),
			})
		}

		// Get full page with blocks
		page, err = pageSvc.Get(ctx.Context(), page.ID)
		if err != nil {
			return err
		}

		// Get content
		var contentData *content.Content
		if page.ContentID != uuid.Nil {
			contentData, err = contentSvc.Get(ctx.Context(), page.ContentID)
			if err != nil {
				return err
			}
		}

		// Get menu
		navigation, err := menuSvc.ResolveNavigation(ctx.Context(), "primary", locale)
		if err != nil && !errors.Is(err, menus.ErrMenuNotFound) {
			return err
		}

		// Get widgets for sidebar if feature enabled
		var sidebarWidgets []*widgets.ResolvedWidget
		if cfg.Features.Widgets {
			localeID := getLocaleIDByCode(container, locale)
			resolveInput := widgets.ResolveAreaInput{
				AreaCode: "sidebar.primary",
				Audience: []string{"guest"},
				Now:      time.Now().UTC(),
			}
			if localeID != uuid.Nil {
				resolveInput.LocaleID = &localeID
			}
			sidebarWidgets, err = widgetSvc.ResolveArea(ctx.Context(), resolveInput)
			if err != nil {
				log.Printf("Could not resolve widgets: %v", err)
			} else {
				log.Printf("Resolved %d widgets for sidebar.primary (locale: %s, ID: %s)", len(sidebarWidgets), locale, localeID)
				for i, w := range sidebarWidgets {
					log.Printf("  Widget %d: ID=%s, Config=%+v", i, w.Instance.ID, w.Instance.Configuration)
				}
			}
		} else {
			log.Printf("Widgets feature is disabled")
		}

		// Prepare locale-specific data for template
		contentTranslation := getContentTranslation(contentData, locale, container)
		pageTranslation := getPageTranslation(page, locale, container)
		blockTranslations := getBlockTranslations(page.Blocks, locale, container)

		log.Printf("Widgets count for template: %d", len(sidebarWidgets))

		// Render template
		return ctx.Render("page", map[string]any{
			"title":       getPageTitle(page, locale, container),
			"locale":      locale,
			"menu":        navigation,
			"page":        toMap(page),
			"translation": toMap(pageTranslation),
			"content":     toMap(contentTranslation),
			"blocks":      blockTranslations,
			"widgets":     sidebarWidgets,
		})
	})

	// Locale switcher
	r.Get("/switch-locale/:locale", func(ctx router.Context) error {
		locale := ctx.Param("locale")
		referer := ctx.Referer()
		if referer == "" {
			referer = "/"
		}
		return ctx.Redirect(referer + "?locale=" + locale)
	})

	// API: List all pages
	r.Get("/api/pages", func(ctx router.Context) error {
		pagesList, err := pageSvc.List(ctx.Context())
		if err != nil {
			return err
		}
		return ctx.JSON(200, pagesList)
	})

	// API: Get page by ID
	r.Get("/api/pages/:id", func(ctx router.Context) error {
		id := ctx.Param("id")
		pageID, err := uuid.Parse(id)
		if err != nil {
			return ctx.JSON(400, map[string]string{"error": "invalid page id"})
		}

		page, err := pageSvc.Get(ctx.Context(), pageID)
		if err != nil {
			return ctx.JSON(404, map[string]string{"error": "page not found"})
		}

		return ctx.JSON(200, page)
	})

	// API: Get menu
	r.Get("/api/menus/:code", func(ctx router.Context) error {
		code := ctx.Param("code")
		locale := ctx.Query("locale", "en")

		navigation, err := menuSvc.ResolveNavigation(ctx.Context(), code, locale)
		if err != nil {
			if errors.Is(err, menus.ErrMenuNotFound) {
				return ctx.JSON(404, map[string]string{"error": "menu not found"})
			}
			return err
		}

		return ctx.JSON(200, navigation)
	})
}

func setupDemoData(ctx context.Context, module *cms.Module, cfg *cms.Config) error {
	blockSvc := module.Blocks()
	menuSvc := module.Menus()
	widgetSvc := module.Widgets()
	themeSvc := module.Themes()
	pageSvc := module.Pages()
	contentSvc := module.Content()
	container := module.Container()

	authorID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	// Register theme
	theme, err := themeSvc.RegisterTheme(ctx, themes.RegisterThemeInput{
		Name:      "Default",
		Version:   "1.0.0",
		ThemePath: "./themes/default",
		Config: themes.ThemeConfig{
			WidgetAreas: []themes.ThemeWidgetArea{
				{Code: "hero", Name: "Hero Banner"},
				{Code: "sidebar", Name: "Sidebar"},
			},
		},
	})
	if err != nil && !errors.Is(err, themes.ErrThemeExists) {
		return fmt.Errorf("register theme: %w", err)
	}

	var templateID uuid.UUID
	if theme != nil {
		template, err := themeSvc.RegisterTemplate(ctx, themes.RegisterTemplateInput{
			ThemeID:      theme.ID,
			Name:         "Page Template",
			Slug:         "page",
			TemplatePath: filepath.Join("templates", "page.html"),
			Regions: map[string]themes.TemplateRegion{
				"content": {
					Name:          "Main Content",
					AcceptsBlocks: true,
				},
				"sidebar": {
					Name:           "Sidebar",
					AcceptsWidgets: true,
				},
			},
		})
		if err != nil && !errors.Is(err, themes.ErrTemplateSlugConflict) {
			return fmt.Errorf("register template: %w", err)
		}
		if template != nil {
			templateID = template.ID
		}
	}

	if templateID == uuid.Nil {
		templateID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	}

	// Create content type
	typeID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	maybeSeedContentType(ctx, container, &content.ContentType{
		ID:   typeID,
		Name: "page",
		Schema: map[string]any{
			"fields": []map[string]any{
				{
					"name": "body",
					"type": "richtext",
				},
			},
		},
	})

	// Create sample content
	enSummary := "Learn about our CMS"
	esSummary := "Conozca nuestro CMS"

	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "about",
		Status:        "published",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "About go-cms",
				Summary: &enSummary,
				Content: map[string]any{
					"body": "<p>go-cms is a modular headless CMS library written in Go.</p><p>It provides content management capabilities including pages, blocks, widgets, menus, and internationalization.</p>",
				},
			},
			{
				Locale:  "es",
				Title:   "Acerca de go-cms",
				Summary: &esSummary,
				Content: map[string]any{
					"body": "<p>go-cms es una biblioteca CMS modular sin cabeza escrita en Go.</p><p>Proporciona capacidades de gestión de contenido que incluyen páginas, bloques, widgets, menús e internacionalización.</p>",
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create content: %w", err)
	}

	// Create page
	pageRecord, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: templateID,
		Slug:       "about",
		Status:     "published",
		CreatedBy:  authorID,
		UpdatedBy:  authorID,
		Translations: []pages.PageTranslationInput{
			{
				Locale: "en",
				Title:  "About",
				Path:   "/about",
			},
			{
				Locale: "es",
				Title:  "Acerca de",
				Path:   "/es/acerca-de",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create page: %w", err)
	}

	// Register block definition
	definition, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name: "hero",
		Schema: map[string]any{
			"fields": []any{"title", "body"},
		},
	})
	if err != nil && !errors.Is(err, blocks.ErrDefinitionExists) {
		return fmt.Errorf("register block definition: %w", err)
	}

	// Create block instance
	if definition != nil {
		locale, err := container.LocaleRepository().GetByCode(ctx, "en")
		if err != nil {
			return fmt.Errorf("get locale: %w", err)
		}

		blockInstance, err := blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
			DefinitionID: definition.ID,
			PageID:       &pageRecord.ID,
			Region:       "content",
			Position:     0,
			Configuration: map[string]any{
				"layout": "full",
			},
			CreatedBy: authorID,
			UpdatedBy: authorID,
		})
		if err != nil {
			return fmt.Errorf("create block instance: %w", err)
		}

		if _, err := blockSvc.AddTranslation(ctx, blocks.AddTranslationInput{
			BlockInstanceID: blockInstance.ID,
			LocaleID:        locale.ID,
			Content: map[string]any{
				"title": "Welcome to go-cms",
				"body":  "A modular, headless CMS library for Go applications",
			},
		}); err != nil {
			return fmt.Errorf("add block translation: %w", err)
		}
	}

	// Create menu
	menuRecord, err := menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		return fmt.Errorf("create menu: %w", err)
	}

	// Add menu items
	if _, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menuRecord.ID,
		Position: 0,
		Target: map[string]any{
			"type": "url",
			"url":  "/",
		},
		CreatedBy: authorID,
		UpdatedBy: authorID,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Home"},
			{Locale: "es", Label: "Inicio"},
		},
	}); err != nil {
		return fmt.Errorf("add home menu item: %w", err)
	}

	if _, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menuRecord.ID,
		Position: 1,
		Target: map[string]any{
			"type": "page",
			"slug": pageRecord.Slug,
		},
		CreatedBy: authorID,
		UpdatedBy: authorID,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "About"},
			{Locale: "es", Label: "Acerca de"},
		},
	}); err != nil {
		return fmt.Errorf("add about menu item: %w", err)
	}

	// Setup widgets if enabled
	if cfg.Features.Widgets {
		if err := widgets.Bootstrap(ctx, widgetSvc, widgets.BootstrapConfig{
			Areas: []widgets.RegisterAreaDefinitionInput{
				{
					Code:  "sidebar.primary",
					Name:  "Primary Sidebar",
					Scope: widgets.AreaScopeGlobal,
				},
			},
		}); err != nil && !errors.Is(err, widgets.ErrFeatureDisabled) {
			return fmt.Errorf("bootstrap widgets: %w", err)
		}

		promoDefinition, err := ensureWidgetDefinition(ctx, widgetSvc, widgets.RegisterDefinitionInput{
			Name: "promo_banner",
			Schema: map[string]any{
				"fields": []any{
					map[string]any{"name": "headline"},
					map[string]any{"name": "cta_text"},
				},
			},
			Defaults: map[string]any{
				"cta_text": "Learn more",
			},
		})
		if err != nil {
			return fmt.Errorf("ensure widget definition: %w", err)
		}

		guestWidget, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
			DefinitionID: promoDefinition.ID,
			Configuration: map[string]any{
				"headline": "Explore go-cms Features",
				"cta_text": "Get Started",
			},
			VisibilityRules: map[string]any{
				"audience": []any{"guest"},
			},
			Position:  0,
			CreatedBy: authorID,
			UpdatedBy: authorID,
		})
		if err != nil {
			return fmt.Errorf("create widget: %w", err)
		}

		// Assign widget to area for each configured locale
		for _, localeCode := range cfg.I18N.Locales {
			locale, err := container.LocaleRepository().GetByCode(ctx, localeCode)
			if err != nil {
				log.Printf("Warning: could not get locale %s: %v", localeCode, err)
				continue
			}

			placement, err := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
				AreaCode:   "sidebar.primary",
				LocaleID:   &locale.ID,
				InstanceID: guestWidget.ID,
				Position:   intPtr(0),
			})
			if err != nil {
				return fmt.Errorf("assign widget to area for locale %s: %w", localeCode, err)
			}
			log.Printf("Widget assigned to area for locale %s: %d placements", localeCode, len(placement))
		}
	}

	log.Println("Demo data setup complete")
	return nil
}

func getPageTitle(page *pages.Page, localeCode string, container *di.Container) string {
	localeID := getLocaleIDByCode(container, localeCode)
	for _, tr := range page.Translations {
		if tr.LocaleID == localeID && tr.Title != "" {
			return tr.Title
		}
	}
	// Fallback to first available translation
	for _, tr := range page.Translations {
		if tr.Title != "" {
			return tr.Title
		}
	}
	return "Page"
}

// getPageTranslation returns the translation for the given locale
func getPageTranslation(page *pages.Page, localeCode string, container *di.Container) *pages.PageTranslation {
	localeID := getLocaleIDByCode(container, localeCode)
	for _, tr := range page.Translations {
		if tr.LocaleID == localeID {
			return tr
		}
	}
	// Fallback to first translation
	if len(page.Translations) > 0 {
		return page.Translations[0]
	}
	return nil
}

// getContentTranslation returns the translation for the given locale
func getContentTranslation(contentData *content.Content, localeCode string, container *di.Container) *content.ContentTranslation {
	if contentData == nil {
		return nil
	}
	localeID := getLocaleIDByCode(container, localeCode)
	for _, tr := range contentData.Translations {
		if tr.LocaleID == localeID {
			return tr
		}
	}
	// Fallback to first translation
	if len(contentData.Translations) > 0 {
		return contentData.Translations[0]
	}
	return nil
}

// getBlockTranslations returns translations for the given locale
func getBlockTranslations(blockInstances []*blocks.Instance, localeCode string, container *di.Container) []map[string]any {
	localeID := getLocaleIDByCode(container, localeCode)
	var result []map[string]any
	for _, block := range blockInstances {
		for _, tr := range block.Translations {
			if tr.LocaleID == localeID {
				result = append(result, map[string]any{
					"region":  block.Region,
					"content": tr.Content,
				})
				break
			}
		}
	}
	return result
}

// getLocaleIDByCode looks up a locale UUID by its code
func getLocaleIDByCode(container *di.Container, code string) uuid.UUID {
	ctx := context.Background()
	locale, err := container.LocaleRepository().GetByCode(ctx, code)
	if err != nil || locale == nil {
		return uuid.Nil
	}
	return locale.ID
}

func intPtr(v int) *int {
	return &v
}

func ensureWidgetDefinition(ctx context.Context, svc widgets.Service, input widgets.RegisterDefinitionInput) (*widgets.Definition, error) {
	definition, err := svc.RegisterDefinition(ctx, input)
	if err == nil {
		return definition, nil
	}
	if errors.Is(err, widgets.ErrDefinitionExists) {
		definitions, listErr := svc.ListDefinitions(ctx)
		if listErr != nil {
			return nil, listErr
		}
		for _, existing := range definitions {
			if existing != nil && existing.Name == input.Name {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("widget definition %q not found after conflict", input.Name)
	}
	return nil, err
}

func maybeSeedContentType(_ context.Context, container *di.Container, ct *content.ContentType) {
	repo := container.ContentTypeRepository()
	if repo == nil {
		return
	}

	if seeder, ok := repo.(interface{ Put(*content.ContentType) }); ok {
		seeder.Put(ct)
	}
}

// toMap converts a struct or slice to a template-friendly format via JSON marshaling
// This makes the data accessible in Django templates which need lowercase field names
func toMap(v any) any {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	// Try to unmarshal as a slice first
	var sliceResult []any
	if err := json.Unmarshal(data, &sliceResult); err == nil {
		return sliceResult
	}

	// Otherwise unmarshal as a map
	var mapResult map[string]any
	if err := json.Unmarshal(data, &mapResult); err != nil {
		return nil
	}
	return mapResult
}
