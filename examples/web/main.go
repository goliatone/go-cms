package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/adapters/noop"
	"github.com/goliatone/go-cms/internal/blocks"
	markdowncmd "github.com/goliatone/go-cms/internal/commands/markdown"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/markdown"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
	router "github.com/goliatone/go-router"
	urlkit "github.com/goliatone/go-urlkit"
	"github.com/google/uuid"
)

var (
	demoAuthorID           = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	demoPageContentTypeID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	demoBlogContentTypeID  = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	demoProductContentType = uuid.MustParse("33333333-3333-3333-3333-333333333333")
)

// cmsContentServiceAdapter adapts the cms.ContentService to the interfaces.ContentService contract
// expected by the Markdown importer. It provides a minimal Create/List/GetBySlug implementation.
type cmsContentServiceAdapter struct {
	service cms.ContentService
}

func newCMSContentServiceAdapter(service cms.ContentService) interfaces.ContentService {
	if service == nil {
		return nil
	}
	return &cmsContentServiceAdapter{service: service}
}

func (a *cmsContentServiceAdapter) Create(ctx context.Context, req interfaces.ContentCreateRequest) (*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}

	translations := make([]content.ContentTranslationInput, 0, len(req.Translations))
	for _, tr := range req.Translations {
		translations = append(translations, content.ContentTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Content: cloneFieldMap(tr.Fields),
		})
	}

	result, err := a.service.Create(ctx, content.CreateContentRequest{
		ContentTypeID: req.ContentTypeID,
		Slug:          req.Slug,
		Status:        req.Status,
		CreatedBy:     req.CreatedBy,
		UpdatedBy:     req.UpdatedBy,
		Translations:  translations,
	})
	if err != nil {
		return nil, err
	}
	return toContentRecord(result), nil
}

func (a *cmsContentServiceAdapter) Update(ctx context.Context, req interfaces.ContentUpdateRequest) (*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}

	translations := make([]content.ContentTranslationInput, 0, len(req.Translations))
	for _, tr := range req.Translations {
		translations = append(translations, content.ContentTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Content: cloneFieldMap(tr.Fields),
		})
	}

	record, err := a.service.Update(ctx, content.UpdateContentRequest{
		ID:           req.ID,
		Status:       req.Status,
		UpdatedBy:    req.UpdatedBy,
		Translations: translations,
		Metadata:     cloneFieldMap(req.Metadata),
	})
	if err != nil {
		return nil, err
	}
	return toContentRecord(record), nil
}

func (a *cmsContentServiceAdapter) GetBySlug(ctx context.Context, slug string) (*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}
	records, err := a.service.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record != nil && strings.EqualFold(record.Slug, slug) {
			return toContentRecord(record), nil
		}
	}
	return nil, nil
}

func (a *cmsContentServiceAdapter) List(ctx context.Context) ([]*interfaces.ContentRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("content service unavailable")
	}
	records, err := a.service.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*interfaces.ContentRecord, 0, len(records))
	for _, record := range records {
		out = append(out, toContentRecord(record))
	}
	return out, nil
}

func (a *cmsContentServiceAdapter) Delete(ctx context.Context, req interfaces.ContentDeleteRequest) error {
	if a == nil || a.service == nil {
		return errors.New("content service unavailable")
	}
	return a.service.Delete(ctx, content.DeleteContentRequest{
		ID:         req.ID,
		DeletedBy:  req.DeletedBy,
		HardDelete: req.HardDelete,
	})
}

func toContentRecord(record *content.Content) *interfaces.ContentRecord {
	if record == nil {
		return nil
	}

	translations := make([]interfaces.ContentTranslation, 0, len(record.Translations))
	for _, tr := range record.Translations {
		if tr == nil {
			continue
		}
		locale := ""
		if tr.Locale != nil {
			locale = tr.Locale.Code
		}
		translations = append(translations, interfaces.ContentTranslation{
			ID:      tr.ID,
			Locale:  locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Fields:  cloneFieldMap(tr.Content),
		})
	}

	return &interfaces.ContentRecord{
		ID:           record.ID,
		ContentType:  record.ContentTypeID,
		Slug:         record.Slug,
		Status:       record.Status,
		Translations: translations,
		Metadata:     nil,
	}
}

func cloneFieldMap(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		out[k] = v
	}
	return out
}

// cmsPageServiceAdapter adapts the CMS page service to the importer interface, using a
// delete-and-recreate strategy for updates while keeping auxiliary metadata in-memory.
type cmsPageServiceAdapter struct {
	service pages.Service

	mu              sync.RWMutex
	translationMeta map[uuid.UUID]map[uuid.UUID]storedPageTranslation
	pageMetadata    map[uuid.UUID]map[string]any
}

type storedPageTranslation struct {
	Locale string
	Fields map[string]any
}

func newCMSPageServiceAdapter(service pages.Service) interfaces.PageService {
	if service == nil {
		return nil
	}
	return &cmsPageServiceAdapter{
		service:         service,
		translationMeta: map[uuid.UUID]map[uuid.UUID]storedPageTranslation{},
		pageMetadata:    map[uuid.UUID]map[string]any{},
	}
}

func (a *cmsPageServiceAdapter) Create(ctx context.Context, req interfaces.PageCreateRequest) (*interfaces.PageRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}

	translations := make([]pages.PageTranslationInput, 0, len(req.Translations))
	for _, tr := range req.Translations {
		translations = append(translations, pages.PageTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Path:    tr.Path,
			Summary: tr.Summary,
		})
	}

	record, err := a.service.Create(ctx, pages.CreatePageRequest{
		ContentID:    req.ContentID,
		TemplateID:   req.TemplateID,
		ParentID:     req.ParentID,
		Slug:         req.Slug,
		Status:       req.Status,
		CreatedBy:    req.CreatedBy,
		UpdatedBy:    req.UpdatedBy,
		Translations: translations,
	})
	if err != nil {
		return nil, err
	}

	a.setPageMetadata(record.ID, req.Metadata)
	a.storePageMeta(record, req.Translations)
	return a.toPageRecord(record), nil
}

func (a *cmsPageServiceAdapter) Update(ctx context.Context, req interfaces.PageUpdateRequest) (*interfaces.PageRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}

	if req.ID == uuid.Nil {
		return nil, errors.New("page id required")
	}

	existing, err := a.service.Get(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("page %s not found", req.ID)
	}

	templateID := existing.TemplateID
	if req.TemplateID != nil && *req.TemplateID != uuid.Nil {
		templateID = *req.TemplateID
	}

	if err := a.service.Delete(ctx, pages.DeletePageRequest{
		ID:         req.ID,
		DeletedBy:  req.UpdatedBy,
		HardDelete: true,
	}); err != nil {
		return nil, err
	}
	a.clearPageMeta(req.ID)

	translations := make([]pages.PageTranslationInput, 0, len(req.Translations))
	for _, tr := range req.Translations {
		translations = append(translations, pages.PageTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Path:    tr.Path,
			Summary: tr.Summary,
		})
	}

	created, err := a.service.Create(ctx, pages.CreatePageRequest{
		ContentID:    existing.ContentID,
		TemplateID:   templateID,
		ParentID:     existing.ParentID,
		Slug:         existing.Slug,
		Status:       req.Status,
		CreatedBy:    req.UpdatedBy,
		UpdatedBy:    req.UpdatedBy,
		Translations: translations,
	})
	if err != nil {
		return nil, err
	}

	a.setPageMetadata(created.ID, req.Metadata)
	a.storePageMeta(created, req.Translations)
	return a.toPageRecord(created), nil
}

func (a *cmsPageServiceAdapter) GetBySlug(ctx context.Context, slug string) (*interfaces.PageRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}
	records, err := a.service.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record != nil && strings.EqualFold(record.Slug, slug) {
			return a.toPageRecord(record), nil
		}
	}
	return nil, nil
}

func (a *cmsPageServiceAdapter) List(ctx context.Context) ([]*interfaces.PageRecord, error) {
	if a == nil || a.service == nil {
		return nil, errors.New("page service unavailable")
	}
	records, err := a.service.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*interfaces.PageRecord, 0, len(records))
	for _, record := range records {
		out = append(out, a.toPageRecord(record))
	}
	return out, nil
}

func (a *cmsPageServiceAdapter) Delete(ctx context.Context, req interfaces.PageDeleteRequest) error {
	if a == nil || a.service == nil {
		return errors.New("page service unavailable")
	}
	if err := a.service.Delete(ctx, pages.DeletePageRequest{
		ID:         req.ID,
		DeletedBy:  req.DeletedBy,
		HardDelete: req.HardDelete,
	}); err != nil {
		return err
	}
	a.clearPageMeta(req.ID)
	return nil
}

func (a *cmsPageServiceAdapter) toPageRecord(record *pages.Page) *interfaces.PageRecord {
	if record == nil {
		return nil
	}
	meta := a.metaForPage(record.ID)
	translations := make([]interfaces.PageTranslation, 0, len(record.Translations))
	for _, tr := range record.Translations {
		if tr == nil {
			continue
		}
		info := meta[tr.ID]
		translations = append(translations, interfaces.PageTranslation{
			ID:      tr.ID,
			Locale:  info.Locale,
			Title:   tr.Title,
			Path:    tr.Path,
			Summary: tr.Summary,
			Fields:  cloneFieldMap(info.Fields),
		})
	}
	return &interfaces.PageRecord{
		ID:           record.ID,
		ContentID:    record.ContentID,
		TemplateID:   record.TemplateID,
		Slug:         record.Slug,
		Status:       record.Status,
		Translations: translations,
		Metadata:     a.pageMetaClone(record.ID),
	}
}

func (a *cmsPageServiceAdapter) storePageMeta(record *pages.Page, inputs []interfaces.PageTranslationInput) {
	if record == nil || len(record.Translations) == 0 {
		return
	}
	index := make(map[string]interfaces.PageTranslationInput, len(inputs))
	for _, in := range inputs {
		key := strings.ToLower(strings.TrimSpace(in.Path))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(in.Locale))
		}
		index[key] = in
	}
	meta := make(map[uuid.UUID]storedPageTranslation, len(record.Translations))
	for _, tr := range record.Translations {
		if tr == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(tr.Path))
		in := index[key]
		meta[tr.ID] = storedPageTranslation{
			Locale: strings.TrimSpace(in.Locale),
			Fields: cloneFieldMap(in.Fields),
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.translationMeta[record.ID] = meta
}

func (a *cmsPageServiceAdapter) clearPageMeta(pageID uuid.UUID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.translationMeta, pageID)
	delete(a.pageMetadata, pageID)
}

func (a *cmsPageServiceAdapter) metaForPage(pageID uuid.UUID) map[uuid.UUID]storedPageTranslation {
	a.mu.RLock()
	defer a.mu.RUnlock()
	meta := a.translationMeta[pageID]
	if len(meta) == 0 {
		return map[uuid.UUID]storedPageTranslation{}
	}
	out := make(map[uuid.UUID]storedPageTranslation, len(meta))
	for id, data := range meta {
		out[id] = storedPageTranslation{
			Locale: data.Locale,
			Fields: cloneFieldMap(data.Fields),
		}
	}
	return out
}

func (a *cmsPageServiceAdapter) setPageMetadata(pageID uuid.UUID, metadata map[string]any) {
	if pageID == uuid.Nil {
		return
	}
	if len(metadata) == 0 {
		a.mu.Lock()
		delete(a.pageMetadata, pageID)
		a.mu.Unlock()
		return
	}
	a.mu.Lock()
	a.pageMetadata[pageID] = cloneFieldMap(metadata)
	a.mu.Unlock()
}

func (a *cmsPageServiceAdapter) pageMetaClone(pageID uuid.UUID) map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return cloneFieldMap(a.pageMetadata[pageID])
}

func main() {
	ctx := context.Background()

	// Configure CMS
	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}
	cfg.Features.Widgets = true
	cfg.Features.Themes = true
	cfg.Features.Markdown = true
	cfg.Markdown.Enabled = true
	cfg.Markdown.ContentDir = "./content"
	cfg.Markdown.DefaultLocale = "en"
	cfg.Markdown.Locales = []string{"en", "es"}
	cfg.Markdown.LocalePatterns = map[string]string{
		"en": "en/**/*.md",
		"es": "es/**/*.md",
	}

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
	pageTemplateID, err := setupDemoData(ctx, module, &cfg)
	if err != nil {
		log.Fatalf("setup demo data: %v", err)
	}

	if err := bootstrapMarkdownDemo(ctx, module, &cfg, pageTemplateID); err != nil {
		log.Printf("bootstrap markdown demo: %v", err)
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

	pageRouteHandler := func(ctx router.Context) error {
		slug := ctx.Param("slug")
		locale := ctx.Query("locale", "en")

		pagesList, err := pageSvc.List(ctx.Context())
		if err != nil {
			return err
		}

		var page *pages.Page
		for _, candidate := range pagesList {
			if candidate.Slug == slug {
				page = candidate
				break
			}
		}

		if page == nil {
			requestPath := ctx.Path()
			for _, candidate := range pagesList {
				for _, tr := range candidate.Translations {
					if tr.Path == requestPath {
						page = candidate
						break
					}
				}
				if page != nil {
					break
				}
			}
		}

		if page == nil {
			return ctx.Render("error", map[string]any{
				"error_code":    404,
				"error_title":   "Page Not Found",
				"error_message": fmt.Sprintf("The page '%s' does not exist.", slug),
			})
		}

		page, err = pageSvc.Get(ctx.Context(), page.ID)
		if err != nil {
			return err
		}

		var contentData *content.Content
		if page.ContentID != uuid.Nil {
			contentData, err = contentSvc.Get(ctx.Context(), page.ContentID)
			if err != nil {
				return err
			}
		}

		navigation, err := menuSvc.ResolveNavigation(ctx.Context(), "primary", locale)
		if err != nil && !errors.Is(err, menus.ErrMenuNotFound) {
			return err
		}

		var sidebarWidgets []map[string]any
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
			resolvedWidgets, err := widgetSvc.ResolveArea(ctx.Context(), resolveInput)
			if err != nil {
				log.Printf("Could not resolve widgets: %v", err)
			} else {
				log.Printf("Resolved %d widgets for sidebar.primary (locale: %s, ID: %s)", len(resolvedWidgets), locale, localeID)
				sidebarWidgets = prepareWidgetsForTemplate(resolvedWidgets, localeID, widgetSvc, ctx.Context())
			}
		} else {
			log.Printf("Widgets feature is disabled")
		}

		contentTranslation := getContentTranslation(contentData, locale, container)
		pageTranslation := getPageTranslation(page, locale, container)
		blockTranslations := getBlockTranslations(page.Blocks, locale, container)

		log.Printf("Widgets count for template: %d", len(sidebarWidgets))

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
	}

	// Page routes
	r.Get("/pages/:slug", pageRouteHandler)
	r.Get("/blog/:slug", pageRouteHandler)

	// Locale switcher
	r.Get("/switch-locale/:locale", func(ctx router.Context) error {
		locale := ctx.Param("locale")
		referer := ctx.Referer()
		if referer == "" {
			referer = "/"
		}

		// Parse the referer URL to properly handle existing query parameters
		parsedURL, err := url.Parse(referer)
		if err != nil {
			// If parsing fails, fall back to simple path
			return ctx.Redirect("/?locale=" + locale)
		}

		// Get the path without query parameters and add the new locale
		redirectURL := parsedURL.Path
		if redirectURL == "" {
			redirectURL = "/"
		}
		return ctx.Redirect(redirectURL + "?locale=" + locale)
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

func bootstrapMarkdownDemo(ctx context.Context, module *cms.Module, cfg *cms.Config, pageTemplateID uuid.UUID) error {
	if !cfg.Features.Markdown || !cfg.Markdown.Enabled {
		return nil
	}

	var service *markdown.Service
	logger := logging.MarkdownLogger(nil)
	mdCfg := markdown.Config{
		BasePath:       strings.TrimSpace(cfg.Markdown.ContentDir),
		DefaultLocale:  cfg.Markdown.DefaultLocale,
		Locales:        append([]string(nil), cfg.Markdown.Locales...),
		LocalePatterns: cfg.Markdown.LocalePatterns,
		Pattern:        cfg.Markdown.Pattern,
		Recursive:      cfg.Markdown.Recursive,
		Parser: interfaces.ParseOptions{
			Extensions: append([]string(nil), cfg.Markdown.Parser.Extensions...),
			Sanitize:   cfg.Markdown.Parser.Sanitize,
			HardWraps:  cfg.Markdown.Parser.HardWraps,
			SafeMode:   cfg.Markdown.Parser.SafeMode,
		},
	}
	options := []markdown.ServiceOption{
		markdown.WithLogger(logger),
	}
	if adapter := newCMSContentServiceAdapter(module.Content()); adapter != nil {
		options = append(options, markdown.WithContentService(adapter))
	}
	if pageAdapter := newCMSPageServiceAdapter(module.Pages()); pageAdapter != nil {
		options = append(options, markdown.WithPageService(pageAdapter))
	}
	mdService, err := markdown.NewService(
		mdCfg,
		nil,
		options...,
	)
	if err != nil {
		return fmt.Errorf("initialise markdown service: %w", err)
	}
	service = mdService

	var templatePtr *uuid.UUID
	if pageTemplateID != uuid.Nil {
		templateID := pageTemplateID
		templatePtr = &templateID
	}
	createPages := templatePtr != nil
	if createPages {
		log.Printf("markdown demo: page import enabled; updates will delete and recreate pages as needed")
	} else {
		log.Printf("markdown demo: page import disabled; markdown sync will only import content")
	}

	syncOpts := interfaces.SyncOptions{
		ImportOptions: interfaces.ImportOptions{
			ContentTypeID: demoBlogContentTypeID,
			AuthorID:      demoAuthorID,
			CreatePages:   createPages,
			TemplateID:    templatePtr,
		},
		UpdateExisting: true,
	}

	result, err := service.Sync(ctx, ".", syncOpts)
	if err != nil {
		return fmt.Errorf("sync markdown demo content: %w", err)
	}

	if result != nil {
		log.Printf("markdown demo sync completed: created=%d updated=%d deleted=%d skipped=%d errors=%d",
			result.Created, result.Updated, result.Deleted, result.Skipped, len(result.Errors))
		for _, syncErr := range result.Errors {
			log.Printf("markdown demo sync error: %v", syncErr)
		}
	}

	registry := &demoCommandRegistry{}
	gates := markdowncmd.FeatureGates{
		MarkdownEnabled: func() bool { return cfg.Features.Markdown && cfg.Markdown.Enabled },
	}

	handlerSet, err := markdowncmd.RegisterMarkdownCommands(registry, service, nil, gates)
	if err != nil {
		return fmt.Errorf("register markdown command handlers: %w", err)
	}
	log.Printf("markdown demo registered %d command handlers", len(registry.handlers))

	cron := &demoCronScheduler{}
	cronJobName := "markdown-sync"
	cronConfig := command.HandlerConfig{
		Expression: "@every 5m",
	}
	syncMsg := markdowncmd.SyncDirectoryCommand{
		Directory:      ".",
		ContentTypeID:  demoBlogContentTypeID,
		AuthorID:       demoAuthorID,
		CreatePages:    createPages,
		UpdateExisting: true,
	}
	if templatePtr != nil {
		syncMsg.TemplateID = templatePtr
	}
	if err := markdowncmd.RegisterMarkdownCron(cron.Register, handlerSet.Sync, cronConfig, syncMsg); err != nil {
		return fmt.Errorf("register markdown cron: %w", err)
	}
	log.Printf("markdown demo scheduled cron job %q with expression %q", cronJobName, cronConfig.Expression)

	if len(cron.jobs) > 0 {
		log.Printf("markdown demo executing initial cron sync run")
		if err := cron.jobs[0](); err != nil {
			log.Printf("markdown demo cron execution error: %v", err)
		}
	}

	return nil
}

type demoCommandRegistry struct {
	handlers []any
}

func (r *demoCommandRegistry) RegisterCommand(handler any) error {
	r.handlers = append(r.handlers, handler)
	log.Printf("markdown demo: registered command handler %T", handler)
	return nil
}

type demoCronScheduler struct {
	registrations []command.HandlerConfig
	jobs          []func() error
}

func (s *demoCronScheduler) Register(cfg command.HandlerConfig, handler any) error {
	s.registrations = append(s.registrations, cfg)
	if fn, ok := handler.(func() error); ok {
		s.jobs = append(s.jobs, fn)
	}
	log.Printf("markdown demo: cron registration stored (expression=%s)", cfg.Expression)
	return nil
}

func setupDemoData(ctx context.Context, module *cms.Module, cfg *cms.Config) (uuid.UUID, error) {
	blockSvc := module.Blocks()
	menuSvc := module.Menus()
	widgetSvc := module.Widgets()
	themeSvc := module.Themes()
	pageSvc := module.Pages()
	contentSvc := module.Content()
	container := module.Container()

	authorID := demoAuthorID

	// Register theme
	themeInput := themes.RegisterThemeInput{
		Name:      "Default",
		Version:   "1.0.0",
		ThemePath: "./themes/default",
		Config: themes.ThemeConfig{
			WidgetAreas: []themes.ThemeWidgetArea{
				{Code: "hero", Name: "Hero Banner"},
				{Code: "sidebar", Name: "Sidebar"},
			},
		},
	}
	theme, err := themeSvc.RegisterTheme(ctx, themeInput)
	if err != nil {
		if errors.Is(err, themes.ErrThemeExists) {
			existing, lookupErr := themeSvc.GetThemeByName(ctx, themeInput.Name)
			if lookupErr != nil {
				return uuid.Nil, fmt.Errorf("lookup existing theme: %w", lookupErr)
			}
			theme = existing
		} else {
			return uuid.Nil, fmt.Errorf("register theme: %w", err)
		}
	}

	var templateID uuid.UUID
	templateInput := themes.RegisterTemplateInput{
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
	}
	template, err := themeSvc.RegisterTemplate(ctx, templateInput)
	if err != nil {
		if errors.Is(err, themes.ErrTemplateSlugConflict) {
			templates, listErr := themeSvc.ListTemplates(ctx, theme.ID)
			if listErr != nil {
				return uuid.Nil, fmt.Errorf("lookup existing template: %w", listErr)
			}
			for _, existing := range templates {
				if existing.Slug == templateInput.Slug {
					templateID = existing.ID
					break
				}
			}
			if templateID == uuid.Nil {
				return uuid.Nil, fmt.Errorf("lookup existing template: slug %s not found", templateInput.Slug)
			}
		} else {
			return uuid.Nil, fmt.Errorf("register template: %w", err)
		}
	} else if template != nil {
		templateID = template.ID
	}

	// Create content types
	pageTypeID := demoPageContentTypeID
	maybeSeedContentType(ctx, container, &content.ContentType{
		ID:   pageTypeID,
		Name: "page",
		Schema: map[string]any{
			"fields": []map[string]any{
				{"name": "body", "type": "richtext", "required": true},
			},
		},
	})

	// Blog post content type
	blogTypeID := demoBlogContentTypeID
	maybeSeedContentType(ctx, container, &content.ContentType{
		ID:   blogTypeID,
		Name: "blog_post",
		Schema: map[string]any{
			"fields": []map[string]any{
				{"name": "body", "type": "richtext", "required": true},
				{"name": "excerpt", "type": "text", "required": false},
				{"name": "author", "type": "string", "required": true},
				{"name": "tags", "type": "array", "required": false},
				{"name": "featured_image", "type": "string", "required": false},
			},
		},
	})

	// Product content type
	productTypeID := demoProductContentType
	maybeSeedContentType(ctx, container, &content.ContentType{
		ID:   productTypeID,
		Name: "product",
		Schema: map[string]any{
			"fields": []map[string]any{
				{"name": "description", "type": "richtext", "required": true},
				{"name": "price", "type": "number", "required": true},
				{"name": "features", "type": "array", "required": false},
				{"name": "specs", "type": "object", "required": false},
			},
		},
	})

	// Register block definitions
	heroBlockDef, err := ensureBlockDefinition(ctx, blockSvc, blocks.RegisterDefinitionInput{
		Name: "hero",
		Schema: map[string]any{
			"fields": []any{"title", "subtitle", "cta_text", "cta_url", "background_image"},
		},
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("register hero block: %w", err)
	}

	featuresBlockDef, err := ensureBlockDefinition(ctx, blockSvc, blocks.RegisterDefinitionInput{
		Name: "features_grid",
		Schema: map[string]any{
			"fields": []any{"title", "features"},
		},
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("register features block: %w", err)
	}

	ctaBlockDef, err := ensureBlockDefinition(ctx, blockSvc, blocks.RegisterDefinitionInput{
		Name: "call_to_action",
		Schema: map[string]any{
			"fields": []any{"headline", "description", "button_text", "button_url"},
		},
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("register cta block: %w", err)
	}

	// Create About page with content type
	enAboutSummary := "Learn about our CMS"
	esAboutSummary := "Conozca nuestro CMS"

	aboutContent, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: pageTypeID,
		Slug:          "about",
		Status:        "published",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "About go-cms",
				Summary: &enAboutSummary,
				Content: map[string]any{
					"body": "<p>go-cms is a modular headless CMS library written in Go.</p><p>It provides content management capabilities including pages, blocks, widgets, menus, and internationalization.</p>",
				},
			},
			{
				Locale:  "es",
				Title:   "Acerca de go-cms",
				Summary: &esAboutSummary,
				Content: map[string]any{
					"body": "<p>go-cms es una biblioteca CMS modular sin cabeza escrita en Go.</p><p>Proporciona capacidades de gestión de contenido que incluyen páginas, bloques, widgets, menús e internacionalización.</p>",
				},
			},
		},
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create about content: %w", err)
	}

	aboutPage, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  aboutContent.ID,
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
		return uuid.Nil, fmt.Errorf("create about page: %w", err)
	}

	// Add hero block to about page
	if err := createLocalizedBlock(ctx, blockSvc, container, heroBlockDef, &aboutPage.ID, "content", 0, map[string]map[string]any{
		"en": {
			"title":            "Welcome to go-cms",
			"subtitle":         "A modular, headless CMS library for Go applications",
			"cta_text":         "Get Started",
			"cta_url":          "/docs",
			"background_image": "/static/images/hero-bg.jpg",
		},
		"es": {
			"title":            "Bienvenido a go-cms",
			"subtitle":         "Una biblioteca CMS modular sin cabeza para aplicaciones Go",
			"cta_text":         "Comenzar",
			"cta_url":          "/docs",
			"background_image": "/static/images/hero-bg.jpg",
		},
	}); err != nil {
		return uuid.Nil, fmt.Errorf("create hero block: %w", err)
	}

	// Add features block to about page
	if err := createLocalizedBlock(ctx, blockSvc, container, featuresBlockDef, &aboutPage.ID, "content", 1, map[string]map[string]any{
		"en": {
			"title": "Key Features",
			"features": []map[string]any{
				{"icon": "📄", "title": "Pages", "description": "Hierarchical page management"},
				{"icon": "🧩", "title": "Blocks", "description": "Reusable content components"},
				{"icon": "🎨", "title": "Widgets", "description": "Dynamic behavioral components"},
				{"icon": "🗂️", "title": "Content Types", "description": "Flexible content schemas"},
			},
		},
		"es": {
			"title": "Características Principales",
			"features": []map[string]any{
				{"icon": "📄", "title": "Páginas", "description": "Gestión jerárquica de páginas"},
				{"icon": "🧩", "title": "Bloques", "description": "Componentes de contenido reutilizables"},
				{"icon": "🎨", "title": "Widgets", "description": "Componentes dinámicos de comportamiento"},
				{"icon": "🗂️", "title": "Tipos de Contenido", "description": "Esquemas de contenido flexibles"},
			},
		},
	}); err != nil {
		return uuid.Nil, fmt.Errorf("create features block: %w", err)
	}

	// Create blog posts
	enExcerpt1 := "Discover the core concepts of go-cms"
	esExcerpt1 := "Descubre los conceptos básicos de go-cms"

	blogPost1, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: blogTypeID,
		Slug:          "getting-started",
		Status:        "published",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Getting Started with go-cms",
				Summary: &enExcerpt1,
				Content: map[string]any{
					"body":           "<p>This guide will walk you through the basics of using go-cms in your Go application.</p><p>You'll learn about content types, pages, blocks, and widgets.</p>",
					"excerpt":        "Discover the core concepts of go-cms",
					"author":         "John Doe",
					"tags":           []string{"tutorial", "getting-started", "go"},
					"featured_image": "/static/images/blog1.jpg",
				},
			},
			{
				Locale:  "es",
				Title:   "Comenzando con go-cms",
				Summary: &esExcerpt1,
				Content: map[string]any{
					"body":           "<p>Esta guía le mostrará los conceptos básicos del uso de go-cms en su aplicación Go.</p><p>Aprenderá sobre tipos de contenido, páginas, bloques y widgets.</p>",
					"excerpt":        "Descubre los conceptos básicos de go-cms",
					"author":         "Juan Pérez",
					"tags":           []string{"tutorial", "primeros-pasos", "go"},
					"featured_image": "/static/images/blog1.jpg",
				},
			},
		},
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create blog post 1: %w", err)
	}

	blogPage1, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  blogPost1.ID,
		TemplateID: templateID,
		Slug:       "getting-started",
		Status:     "published",
		CreatedBy:  authorID,
		UpdatedBy:  authorID,
		Translations: []pages.PageTranslationInput{
			{
				Locale: "en",
				Title:  "Getting Started with go-cms",
				Path:   "/blog/getting-started",
			},
			{
				Locale: "es",
				Title:  "Comenzando con go-cms",
				Path:   "/es/blog/comenzando",
			},
		},
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create blog page 1: %w", err)
	}

	// Add CTA block to blog post
	if err := createLocalizedBlock(ctx, blockSvc, container, ctaBlockDef, &blogPage1.ID, "content", 0, map[string]map[string]any{
		"en": {
			"headline":    "Ready to Build?",
			"description": "Start using go-cms in your next project today.",
			"button_text": "View Documentation",
			"button_url":  "/docs",
		},
		"es": {
			"headline":    "¿Listo para Construir?",
			"description": "Comience a usar go-cms en su próximo proyecto hoy.",
			"button_text": "Ver Documentación",
			"button_url":  "/docs",
		},
	}); err != nil {
		return uuid.Nil, fmt.Errorf("create cta block: %w", err)
	}

	// Create menu
	menuRecord, err := menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create menu: %w", err)
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
		return uuid.Nil, fmt.Errorf("add home menu item: %w", err)
	}

	if _, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menuRecord.ID,
		Position: 1,
		Target: map[string]any{
			"type": "page",
			"slug": aboutPage.Slug,
		},
		CreatedBy: authorID,
		UpdatedBy: authorID,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "About"},
			{Locale: "es", Label: "Acerca de"},
		},
	}); err != nil {
		return uuid.Nil, fmt.Errorf("add about menu item: %w", err)
	}

	if _, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menuRecord.ID,
		Position: 2,
		Target: map[string]any{
			"type": "page",
			"slug": blogPage1.Slug,
		},
		CreatedBy: authorID,
		UpdatedBy: authorID,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Blog"},
			{Locale: "es", Label: "Blog"},
		},
	}); err != nil {
		return uuid.Nil, fmt.Errorf("add blog menu item: %w", err)
	}

	if _, err := menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
		MenuID:   menuRecord.ID,
		Position: 3,
		Target: map[string]any{
			"type": "url",
			"url":  "/blog/markdown-sync-demo",
		},
		CreatedBy: authorID,
		UpdatedBy: authorID,
		Translations: []menus.MenuItemTranslationInput{
			{Locale: "en", Label: "Markdown Demo"},
			{Locale: "es", Label: "Demo Markdown"},
		},
	}); err != nil {
		return uuid.Nil, fmt.Errorf("add markdown demo menu item: %w", err)
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
			return uuid.Nil, fmt.Errorf("bootstrap widgets: %w", err)
		}

		// Newsletter widget
		newsletterDef, err := ensureWidgetDefinition(ctx, widgetSvc, widgets.RegisterDefinitionInput{
			Name: "newsletter",
			Schema: map[string]any{
				"fields": []any{
					map[string]any{"name": "headline"},
					map[string]any{"name": "description"},
					map[string]any{"name": "button_text"},
				},
			},
			Defaults: map[string]any{
				"button_text": "Subscribe",
			},
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("ensure newsletter widget definition: %w", err)
		}

		newsletterWidget, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
			DefinitionID: newsletterDef.ID,
			Configuration: map[string]any{
				"headline":    "Stay Updated",
				"description": "Get the latest news and updates delivered to your inbox.",
				"button_text": "Subscribe Now",
			},
			VisibilityRules: map[string]any{
				"audience": []any{"guest"},
			},
			Position:  0,
			CreatedBy: authorID,
			UpdatedBy: authorID,
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("create newsletter widget: %w", err)
		}

		// Add translations for newsletter widget
		for _, localeCode := range cfg.I18N.Locales {
			locale, err := container.LocaleRepository().GetByCode(ctx, localeCode)
			if err != nil {
				log.Printf("Warning: could not get locale %s: %v", localeCode, err)
				continue
			}

			var content map[string]any
			if localeCode == "es" {
				content = map[string]any{
					"headline":    "Mantente Actualizado",
					"description": "Recibe las últimas noticias y actualizaciones en tu bandeja de entrada.",
					"button_text": "Suscríbete Ahora",
				}
			} else {
				content = map[string]any{
					"headline":    "Stay Updated",
					"description": "Get the latest news and updates delivered to your inbox.",
					"button_text": "Subscribe Now",
				}
			}

			if _, err := widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
				InstanceID: newsletterWidget.ID,
				LocaleID:   locale.ID,
				Content:    content,
			}); err != nil {
				return uuid.Nil, fmt.Errorf("add newsletter widget translation for locale %s: %w", localeCode, err)
			}
		}

		// Promo widget
		promoDef, err := ensureWidgetDefinition(ctx, widgetSvc, widgets.RegisterDefinitionInput{
			Name: "promo_banner",
			Schema: map[string]any{
				"fields": []any{
					map[string]any{"name": "headline"},
					map[string]any{"name": "offer"},
					map[string]any{"name": "cta_text"},
					map[string]any{"name": "badge"},
				},
			},
			Defaults: map[string]any{
				"cta_text": "Learn more",
			},
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("ensure promo widget definition: %w", err)
		}

		promoWidget, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
			DefinitionID: promoDef.ID,
			Configuration: map[string]any{
				"headline": "Limited Time Offer",
				"offer":    "Get 50% off your first project",
				"cta_text": "Claim Offer",
				"badge":    "NEW",
			},
			VisibilityRules: map[string]any{
				"audience": []any{"guest", "user"},
			},
			UnpublishOn: timePtr(time.Now().Add(30 * 24 * time.Hour)),
			Position:    1,
			CreatedBy:   authorID,
			UpdatedBy:   authorID,
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("create promo widget: %w", err)
		}

		// Add translations for promo widget
		for _, localeCode := range cfg.I18N.Locales {
			locale, err := container.LocaleRepository().GetByCode(ctx, localeCode)
			if err != nil {
				log.Printf("Warning: could not get locale %s: %v", localeCode, err)
				continue
			}

			var content map[string]any
			if localeCode == "es" {
				content = map[string]any{
					"headline": "Oferta por Tiempo Limitado",
					"offer":    "Obtén 50% de descuento en tu primer proyecto",
					"cta_text": "Reclamar Oferta",
					"badge":    "NUEVO",
				}
			} else {
				content = map[string]any{
					"headline": "Limited Time Offer",
					"offer":    "Get 50% off your first project",
					"cta_text": "Claim Offer",
					"badge":    "NEW",
				}
			}

			if _, err := widgetSvc.AddTranslation(ctx, widgets.AddTranslationInput{
				InstanceID: promoWidget.ID,
				LocaleID:   locale.ID,
				Content:    content,
			}); err != nil {
				return uuid.Nil, fmt.Errorf("add promo widget translation for locale %s: %w", localeCode, err)
			}
		}

		// Assign widgets to area for each configured locale
		for _, localeCode := range cfg.I18N.Locales {
			locale, err := container.LocaleRepository().GetByCode(ctx, localeCode)
			if err != nil {
				log.Printf("Warning: could not get locale %s: %v", localeCode, err)
				continue
			}

			// Assign newsletter widget
			if _, err := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
				AreaCode:   "sidebar.primary",
				LocaleID:   &locale.ID,
				InstanceID: newsletterWidget.ID,
				Position:   intPtr(0),
			}); err != nil {
				return uuid.Nil, fmt.Errorf("assign newsletter widget for locale %s: %w", localeCode, err)
			}

			// Assign promo widget
			if _, err := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
				AreaCode:   "sidebar.primary",
				LocaleID:   &locale.ID,
				InstanceID: promoWidget.ID,
				Position:   intPtr(1),
			}); err != nil {
				return uuid.Nil, fmt.Errorf("assign promo widget for locale %s: %w", localeCode, err)
			}

			log.Printf("Widgets assigned to area for locale %s", localeCode)
		}
	}

	log.Println("Demo data setup complete")
	log.Printf("  - Content Types: 3 (page, blog_post, product)")
	log.Printf("  - Block Definitions: 3 (hero, features_grid, call_to_action)")
	log.Printf("  - Pages: 3 (about, getting-started blog, markdown-sync-demo)")
	log.Printf("  - Widgets: 2 (newsletter, promo_banner)")
	return templateID, nil
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

func timePtr(t time.Time) *time.Time {
	return &t
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

func ensureBlockDefinition(ctx context.Context, svc blocks.Service, input blocks.RegisterDefinitionInput) (*blocks.Definition, error) {
	definition, err := svc.RegisterDefinition(ctx, input)
	if err == nil {
		return definition, nil
	}
	if errors.Is(err, blocks.ErrDefinitionExists) {
		definitions, listErr := svc.ListDefinitions(ctx)
		if listErr != nil {
			return nil, listErr
		}
		for _, existing := range definitions {
			if existing != nil && existing.Name == input.Name {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("block definition %q not found after conflict", input.Name)
	}
	return nil, err
}

// createLocalizedBlock creates a block instance with translations for multiple locales
func createLocalizedBlock(
	ctx context.Context,
	blockSvc blocks.Service,
	container *di.Container,
	definition *blocks.Definition,
	pageID *uuid.UUID,
	region string,
	position int,
	translations map[string]map[string]any,
) error {
	authorID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	blockInstance, err := blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: definition.ID,
		PageID:       pageID,
		Region:       region,
		Position:     position,
		CreatedBy:    authorID,
		UpdatedBy:    authorID,
	})
	if err != nil {
		return fmt.Errorf("create block instance: %w", err)
	}

	for localeCode, content := range translations {
		locale, err := container.LocaleRepository().GetByCode(ctx, localeCode)
		if err != nil {
			return fmt.Errorf("get locale %s: %w", localeCode, err)
		}

		if _, err := blockSvc.AddTranslation(ctx, blocks.AddTranslationInput{
			BlockInstanceID: blockInstance.ID,
			LocaleID:        locale.ID,
			Content:         content,
		}); err != nil {
			return fmt.Errorf("add block translation for locale %s: %w", localeCode, err)
		}
	}

	return nil
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

// prepareWidgetsForTemplate prepares widgets with their translations merged into configuration
func prepareWidgetsForTemplate(
	resolvedWidgets []*widgets.ResolvedWidget,
	localeID uuid.UUID,
	widgetSvc widgets.Service,
	ctx context.Context,
) []map[string]any {
	result := make([]map[string]any, 0, len(resolvedWidgets))

	for _, resolved := range resolvedWidgets {
		if resolved.Instance == nil {
			continue
		}

		// Start with the base configuration
		mergedConfig := make(map[string]any)
		for k, v := range resolved.Instance.Configuration {
			mergedConfig[k] = v
		}

		// Look for translation in the instance's Translations that were already loaded
		if localeID != uuid.Nil && resolved.Instance.Translations != nil {
			for _, translation := range resolved.Instance.Translations {
				if translation.LocaleID == localeID {
					// Translation content overrides base configuration
					for k, v := range translation.Content {
						mergedConfig[k] = v
					}
					break
				}
			}
		}

		// Load definition name
		definitionName := ""
		if resolved.Instance.DefinitionID != uuid.Nil {
			definition, err := widgetSvc.GetDefinition(ctx, resolved.Instance.DefinitionID)
			if err == nil && definition != nil {
				definitionName = definition.Name
			}
		}

		widgetData := map[string]any{
			"definition": map[string]any{
				"name": definitionName,
			},
			"instance": map[string]any{
				"id":            resolved.Instance.ID,
				"configuration": mergedConfig,
			},
			"placement": resolved.Placement,
		}

		result = append(result, widgetData)
	}

	return result
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
