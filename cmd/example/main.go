package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/adapters/noop"
	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/markdown"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/internal/pages"
	shortcodepkg "github.com/goliatone/go-cms/internal/shortcode"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
	urlkit "github.com/goliatone/go-urlkit"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	if len(os.Args) > 1 && os.Args[1] == "shortcodes" {
		if err := runShortcodeSmoke(ctx); err != nil {
			log.Fatalf("shortcode smoke: %v", err)
		}
		return
	}

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.Locales = []string{"en", "es"}
	cfg.Features.Widgets = true
	cfg.Features.Themes = true
	cfg.Features.Versioning = true
	cfg.Features.Scheduling = true
	cfg.Features.MediaLibrary = true
	cfg.Features.AdvancedCache = true
	cfg.Themes.DefaultTheme = "aurora"

	cfg.Navigation.RouteConfig = &urlkit.Config{
		Groups: []urlkit.GroupConfig{
			{
				Name:    "frontend",
				BaseURL: "https://example.com",
				Paths: map[string]string{
					"page": "/pages/:slug",
				},
				Groups: []urlkit.GroupConfig{
					{
						Name: "es",
						Path: "/es",
						Paths: map[string]string{
							"page": "/paginas/:slug",
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

	allowMissingTranslations := strings.EqualFold(os.Getenv("CMS_ALLOW_MISSING_TRANSLATIONS"), "true")

	module, err := cms.New(cfg, di.WithCacheProvider(noop.Cache()))
	if err != nil {
		log.Fatalf("initialise cms: %v", err)
	}
	container := module.Container()
	blockSvc := module.Blocks()
	menuSvc := module.Menus()
	widgetSvc := module.Widgets()
	themeSvc := module.Themes()
	pageSvc := module.Pages()
	contentSvc := module.Content()
	mediaSvc := module.Media()
	scheduler := module.Scheduler()
	authorID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	theme, err := themeSvc.RegisterTheme(ctx, themes.RegisterThemeInput{
		Name:      "Aurora",
		Version:   "1.0.0",
		ThemePath: filepath.Join(cfg.Themes.BasePath, "aurora"),
		Config: themes.ThemeConfig{
			WidgetAreas: []themes.ThemeWidgetArea{
				{Code: "hero", Name: "Hero Banner"},
				{Code: "sidebar", Name: "Sidebar"},
			},
		},
	})
	if err != nil {
		if errors.Is(err, themes.ErrThemeExists) {
			theme, err = themeSvc.GetThemeByName(ctx, "Aurora")
			if err != nil {
				log.Fatalf("resolve theme: %v", err)
			}
		} else if !errors.Is(err, themes.ErrFeatureDisabled) {
			log.Fatalf("register theme: %v", err)
		}
	}

	var landingTemplate *themes.Template
	if theme != nil {
		landingTemplate, err = themeSvc.RegisterTemplate(ctx, themes.RegisterTemplateInput{
			ThemeID:      theme.ID,
			Name:         "Landing Page",
			Slug:         "landing",
			TemplatePath: filepath.Join("templates", "landing.html.tmpl"),
			Regions: map[string]themes.TemplateRegion{
				"hero": {
					Name:          "Hero Banner",
					AcceptsBlocks: true,
				},
				"sidebar": {
					Name:           "Sidebar Widgets",
					AcceptsWidgets: true,
				},
			},
		})
		if err != nil {
			if errors.Is(err, themes.ErrTemplateSlugConflict) {
				templates, listErr := themeSvc.ListTemplates(ctx, theme.ID)
				if listErr != nil {
					log.Fatalf("list templates: %v", listErr)
				}
				for _, tpl := range templates {
					if tpl != nil && tpl.Slug == "landing" {
						landingTemplate = tpl
						break
					}
				}
				if landingTemplate == nil {
					log.Fatalf("template 'landing' not found after conflict")
				}
			} else if !errors.Is(err, themes.ErrFeatureDisabled) {
				log.Fatalf("register template: %v", err)
			}
		}

		if _, err := themeSvc.ActivateTheme(ctx, theme.ID); err != nil && !errors.Is(err, themes.ErrFeatureDisabled) {
			log.Fatalf("activate theme: %v", err)
		}
	}

	var widgetDefinition *widgets.Definition
	var heroWidget *widgets.Instance
	if landingTemplate != nil {
		if err := ensureWidgetArea(ctx, widgetSvc, widgets.RegisterAreaDefinitionInput{
			Code:       "hero",
			Name:       "Hero Banner",
			Scope:      widgets.AreaScopeTemplate,
			TemplateID: &landingTemplate.ID,
		}); err != nil {
			log.Fatalf("ensure hero widget area: %v", err)
		}
		if err := ensureWidgetArea(ctx, widgetSvc, widgets.RegisterAreaDefinitionInput{
			Code:       "sidebar",
			Name:       "Sidebar",
			Scope:      widgets.AreaScopeTemplate,
			TemplateID: &landingTemplate.ID,
		}); err != nil {
			log.Fatalf("ensure sidebar widget area: %v", err)
		}

		widgetDefinition, err = widgetSvc.RegisterDefinition(ctx, widgets.RegisterDefinitionInput{
			Name: "announcement",
			Schema: map[string]any{
				"fields": []any{"message"},
			},
			Defaults: map[string]any{
				"message": "Welcome to Aurora",
			},
		})
		if err != nil {
			if errors.Is(err, widgets.ErrDefinitionExists) {
				defs, listErr := widgetSvc.ListDefinitions(ctx)
				if listErr != nil {
					log.Fatalf("list widget definitions: %v", listErr)
				}
				for _, def := range defs {
					if def != nil && def.Name == "announcement" {
						widgetDefinition = def
						break
					}
				}
				if widgetDefinition == nil {
					log.Fatalf("widget definition 'announcement' not found after conflict")
				}
			} else {
				log.Fatalf("register widget definition: %v", err)
			}
		}

		if widgetDefinition != nil {
			heroWidget, err = widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
				DefinitionID: widgetDefinition.ID,
				Configuration: map[string]any{
					"message": "Introducing the Aurora theme",
				},
				CreatedBy: authorID,
				UpdatedBy: authorID,
			})
			if err != nil {
				if !errors.Is(err, widgets.ErrFeatureDisabled) {
					log.Fatalf("create widget instance: %v", err)
				}
			} else {
				if _, assignErr := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
					AreaCode:   "hero",
					InstanceID: heroWidget.ID,
				}); assignErr != nil && !errors.Is(assignErr, widgets.ErrAreaPlacementExists) {
					log.Fatalf("assign widget to area: %v", assignErr)
				}
			}
		}
	}

	typeID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	maybeSeedContentType(ctx, container, &content.ContentType{
		ID:   typeID,
		Name: "page",
		Slug: "page",
		Schema: map[string]any{
			"fields": []map[string]any{
				{
					"name": "body",
					"type": "richtext",
				},
			},
		},
	})

	contentReq := content.CreateContentRequest{
		ContentTypeID:            typeID,
		Slug:                     "company-overview",
		Status:                   "published",
		CreatedBy:                authorID,
		UpdatedBy:                authorID,
		AllowMissingTranslations: allowMissingTranslations,
	}

	enSummary := "Who we are"
	esSummary := "Quienes somos"

	contentReq.Translations = []content.ContentTranslationInput{
		{
			Locale:  "en",
			Title:   "Company Overview",
			Summary: &enSummary,
			Content: map[string]any{
				"body": "Welcome to our company. We build modular CMS components in Go.",
			},
		},
		{
			Locale:  "es",
			Title:   "Resumen de la empresa",
			Summary: &esSummary,
			Content: map[string]any{
				"body": "Bienvenido a nuestra empresa. Construimos componentes CMS modulares en Go.",
			},
		},
	}

	contentRecord, err := contentSvc.Create(ctx, contentReq)
	if err != nil {
		log.Fatalf("create content: %v", err)
	}

	if cfg.Features.Versioning {
		draft, err := contentSvc.CreateDraft(ctx, content.CreateContentDraftRequest{
			ContentID: contentRecord.ID,
			Snapshot: content.ContentVersionSnapshot{
				Fields: map[string]any{"headline": "Aurora Launch"},
				Translations: []content.ContentVersionTranslationSnapshot{
					{
						Locale:  "en",
						Title:   "Aurora Launch Draft",
						Content: map[string]any{"body": "Draft body for the Aurora release."},
					},
				},
			},
			CreatedBy: authorID,
			UpdatedBy: authorID,
		})
		if err != nil {
			log.Printf("create content draft: %v", err)
		} else {
			if _, err := contentSvc.PublishDraft(ctx, content.PublishContentDraftRequest{
				ContentID:   contentRecord.ID,
				Version:     draft.Version,
				PublishedBy: authorID,
			}); err != nil {
				log.Printf("publish content draft: %v", err)
			}
		}
	}

	if cfg.Features.Scheduling {
		publishAt := time.Now().Add(-2 * time.Minute)
		if _, err := contentSvc.Schedule(ctx, content.ScheduleContentRequest{
			ContentID:   contentRecord.ID,
			PublishAt:   &publishAt,
			ScheduledBy: authorID,
		}); err != nil {
			log.Printf("schedule content publish: %v", err)
		} else {
			worker := jobs.NewWorker(scheduler, container.ContentRepository())
			if err := worker.Process(ctx); err != nil {
				log.Printf("process scheduled jobs: %v", err)
			} else if updated, err := contentSvc.Get(ctx, contentRecord.ID); err == nil {
				log.Printf("content %s status after scheduling: %s", updated.ID, updated.Status)
			}
		}
	}

	if cfg.Features.MediaLibrary {
		bindings := media.BindingSet{
			"hero": {
				{
					Slot: "hero",
					Reference: interfaces.MediaReference{
						ID: "demo-hero",
					},
					Required: []string{"thumb"},
				},
			},
		}
		if resolved, err := mediaSvc.ResolveBindings(ctx, bindings, media.ResolveOptions{}); err != nil {
			log.Printf("resolve media bindings: %v", err)
		} else {
			prettyPrint("media_bindings.json", resolved)
		}
	}

	definition, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name: "hero",
		Schema: map[string]any{
			"fields": []any{"title", "body"},
		},
	})
	if err != nil {
		if errors.Is(err, blocks.ErrDefinitionExists) {
			defs, listErr := blockSvc.ListDefinitions(ctx)
			if listErr != nil {
				log.Fatalf("list block definitions: %v", listErr)
			}
			for _, existing := range defs {
				if existing.Name == "hero" {
					definition = existing
					break
				}
			}
			if definition == nil {
				log.Fatalf("block definition 'hero' not found after conflict")
			}
		} else {
			log.Fatalf("register block definition: %v", err)
		}
	}

	templateID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	if landingTemplate != nil {
		templateID = landingTemplate.ID
	}
	pageReq := pages.CreatePageRequest{
		ContentID:                contentRecord.ID,
		TemplateID:               templateID,
		Slug:                     "company-overview",
		Status:                   "published",
		CreatedBy:                authorID,
		UpdatedBy:                authorID,
		AllowMissingTranslations: allowMissingTranslations,
		Translations: []pages.PageTranslationInput{
			{
				Locale: "en",
				Title:  "Company Overview",
				Path:   "/company",
			},
			{
				Locale: "es",
				Title:  "Resumen corporativo",
				Path:   "/es/empresa",
			},
		},
	}

	pageRecord, err := pageSvc.Create(ctx, pageReq)
	if err != nil {
		log.Fatalf("create page: %v", err)
	}

	localeRepo := container.LocaleRepository()
	locale, err := localeRepo.GetByCode(ctx, "en")
	if err != nil {
		log.Fatalf("resolve locale: %v", err)
	}

	blockInstance, err := blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: definition.ID,
		PageID:       &pageRecord.ID,
		Region:       "hero",
		Position:     0,
		Configuration: map[string]any{
			"layout": "full",
		},
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		log.Fatalf("create block instance: %v", err)
	}

	if _, err := blockSvc.AddTranslation(ctx, blocks.AddTranslationInput{
		BlockInstanceID: blockInstance.ID,
		LocaleID:        locale.ID,
		Content: map[string]any{
			"title": "Hero Title",
			"body":  "Reusable block content",
		},
	}); err != nil {
		log.Fatalf("add block translation: %v", err)
	}

	if err := widgets.Bootstrap(ctx, widgetSvc, widgets.BootstrapConfig{
		Areas: []widgets.RegisterAreaDefinitionInput{
			{
				Code:  "sidebar.primary",
				Name:  "Primary Sidebar",
				Scope: widgets.AreaScopeGlobal,
			},
		},
	}); err != nil && !errors.Is(err, widgets.ErrFeatureDisabled) {
		log.Fatalf("bootstrap widgets: %v", err)
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
		Category: strPtr("marketing"),
		Icon:     strPtr("sparkles"),
	})
	if err != nil {
		log.Fatalf("ensure widget definition: %v", err)
	}

	guestWidget, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
		DefinitionID: promoDefinition.ID,
		Configuration: map[string]any{
			"headline": "Save 20% on annual plans",
			"cta_text": "Get started",
		},
		VisibilityRules: map[string]any{
			"audience": []any{"guest"},
		},
		Position:  0,
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		log.Fatalf("create guest widget: %v", err)
	}

	memberWidget, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
		DefinitionID: promoDefinition.ID,
		Configuration: map[string]any{
			"headline": "Thanks for being a member",
			"cta_text": "See your dashboard",
		},
		VisibilityRules: map[string]any{
			"audience": []any{"member"},
		},
		Position:  1,
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		log.Fatalf("create member widget: %v", err)
	}

	if _, err := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   "sidebar.primary",
		InstanceID: guestWidget.ID,
		Position:   intPtr(0),
	}); err != nil {
		log.Fatalf("assign guest widget: %v", err)
	}

	if _, err := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   "sidebar.primary",
		InstanceID: memberWidget.ID,
		Position:   intPtr(1),
	}); err != nil {
		log.Fatalf("assign member widget: %v", err)
	}

	fetched, err := pageSvc.Get(ctx, pageRecord.ID)
	if err != nil {
		log.Fatalf("fetch page: %v", err)
	}

	localeIndex, err := resolveLocaleCodes(ctx, container, cfg.I18N.Locales)
	if err != nil {
		log.Fatalf("resolve locales: %v", err)
	}

	list, err := pageSvc.List(ctx)
	if err != nil {
		log.Fatalf("list pages: %v", err)
	}

	menuCode := "primary"
	if err := cms.SeedMenu(ctx, cms.SeedMenuOptions{
		Menus:    menuSvc,
		MenuCode: menuCode,
		Locale:   "en",
		Actor:    authorID,
		Items: []cms.SeedMenuItem{
			{
				Path:     "primary.company",
				Position: intPtr(0),
				Type:     "item",
				Target: map[string]any{
					"type": "page",
					"slug": pageRecord.Slug,
				},
				AllowMissingTranslations: allowMissingTranslations,
				Translations: []cms.MenuItemTranslationInput{
					{Locale: "en", Label: "Company"},
					{Locale: "es", Label: "Empresa"},
				},
			},
		},
	}); err != nil {
		log.Fatalf("seed menu: %v", err)
	}

	navigationEN, err := menuSvc.ResolveNavigation(ctx, menuCode, "en")
	if err != nil {
		log.Fatalf("resolve navigation en: %v", err)
	}
	navigationES, err := menuSvc.ResolveNavigation(ctx, menuCode, "es")
	if err != nil {
		log.Fatalf("resolve navigation es: %v", err)
	}

	nowWidgets := time.Now().UTC()
	guestSidebar, err := widgetSvc.ResolveArea(ctx, widgets.ResolveAreaInput{
		AreaCode: "sidebar.primary",
		Audience: []string{"guest"},
		Now:      nowWidgets,
	})
	if err != nil {
		log.Fatalf("resolve guest widgets: %v", err)
	}

	memberSidebar, err := widgetSvc.ResolveArea(ctx, widgets.ResolveAreaInput{
		AreaCode: "sidebar.primary",
		Audience: []string{"member"},
		Now:      nowWidgets,
	})
	if err != nil {
		log.Fatalf("resolve member widgets: %v", err)
	}

	payload := map[string]any{
		"content_id": contentRecord.ID,
		"page_id":    pageRecord.ID,
		"page":       stripRuntimeFields(fetched, localeIndex),
		"pages":      buildPageSummaries(list),
		"menus": map[string]any{
			"code": menuCode,
			"navigation": map[string]any{
				"en": navigationEN,
				"es": navigationES,
			},
		},
		"widgets": map[string]any{
			"areas": map[string]any{
				"sidebar.primary": map[string]any{
					"guest":  summarizeResolvedWidgets(guestSidebar),
					"member": summarizeResolvedWidgets(memberSidebar),
				},
			},
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		log.Fatalf("encode output: %v", err)
	}
}

func stripRuntimeFields(page *pages.Page, locales map[uuid.UUID]string) map[string]any {
	output := map[string]any{
		"id":        page.ID,
		"slug":      page.Slug,
		"status":    page.Status,
		"contentID": page.ContentID,
		"template":  page.TemplateID,
		"translations": func() []map[string]any {
			results := make([]map[string]any, 0, len(page.Translations))
			for _, tr := range page.Translations {
				results = append(results, map[string]any{
					"locale": locales[tr.LocaleID],
					"title":  tr.Title,
					"path":   tr.Path,
				})
			}
			return results
		}(),
	}
	if len(page.Widgets) > 0 {
		areas := make(map[string][]map[string]any, len(page.Widgets))
		for code, resolved := range page.Widgets {
			areas[code] = summarizeResolvedWidgets(resolved)
		}
		output["widgets"] = areas
	}
	return output
}

func runShortcodeSmoke(ctx context.Context) error {
	registry := shortcodepkg.NewRegistry(shortcodepkg.NewValidator())
	if err := shortcodepkg.RegisterBuiltIns(registry, nil); err != nil {
		return fmt.Errorf("shortcode smoke: register built-ins: %w", err)
	}

	renderer := shortcodepkg.NewRenderer(registry, shortcodepkg.NewValidator())
	service := shortcodepkg.NewService(registry, renderer, shortcodepkg.WithWordPressSyntax(true))

	markdownInput := strings.TrimSpace(`
# Shortcode Smoke Test

Welcome to the shortcode smoke test. Hugo-style shortcodes render inline:

{{< alert type="info" >}}Stay hydrated and keep shipping!{{< /alert >}}

WordPress syntax is also supported via the preprocessor:

[youtube id="dQw4w9WgXcQ"]
`)

	rendered, err := service.Process(ctx, markdownInput, interfaces.ShortcodeProcessOptions{
		EnableWordPress: true,
	})
	if err != nil {
		return fmt.Errorf("shortcode smoke: process shortcodes: %w", err)
	}

	parser := markdown.NewGoldmarkParser(interfaces.ParseOptions{})
	html, err := parser.Parse([]byte(rendered))
	if err != nil {
		return fmt.Errorf("shortcode smoke: render markdown: %w", err)
	}

	fmt.Println("=== Shortcode Markdown ===")
	fmt.Println(markdownInput)
	fmt.Println()
	fmt.Println("=== Rendered HTML ===")
	fmt.Println(string(html))

	return nil
}

func buildPageSummaries(pagesList []*pages.Page) []map[string]any {
	summaries := make([]map[string]any, 0, len(pagesList))
	for _, entry := range pagesList {
		summaries = append(summaries, map[string]any{
			"id":     entry.ID,
			"slug":   entry.Slug,
			"status": entry.Status,
		})
	}
	return summaries
}

func summarizeResolvedWidgets(items []*widgets.ResolvedWidget) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	summaries := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item == nil || item.Instance == nil {
			continue
		}
		summary := map[string]any{
			"id":            item.Instance.ID,
			"definitionID":  item.Instance.DefinitionID,
			"configuration": item.Instance.Configuration,
		}
		if item.Placement != nil {
			summary["position"] = item.Placement.Position
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func ensureWidgetArea(ctx context.Context, svc widgets.Service, input widgets.RegisterAreaDefinitionInput) error {
	if svc == nil {
		return nil
	}
	if _, err := svc.RegisterAreaDefinition(ctx, input); err != nil {
		if errors.Is(err, widgets.ErrAreaDefinitionExists) || errors.Is(err, widgets.ErrFeatureDisabled) || errors.Is(err, widgets.ErrAreaFeatureDisabled) {
			return nil
		}
		return err
	}
	return nil
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

func prettyPrint(label string, payload any) {
	fmt.Printf("\n%s:\n", label)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		log.Printf("pretty print %s: %v", label, err)
	}
}

func intPtr(v int) *int {
	return &v
}

func strPtr(value string) *string {
	return &value
}

func resolveLocaleCodes(ctx context.Context, container *di.Container, codes []string) (map[uuid.UUID]string, error) {
	index := make(map[uuid.UUID]string, len(codes))
	for _, code := range codes {
		loc, err := container.LocaleRepository().GetByCode(ctx, code)
		if err != nil {
			return nil, err
		}
		index[loc.ID] = code
	}
	return index, nil
}

func maybeSeedContentType(_ context.Context, container *di.Container, ct *content.ContentType) {
	repo := container.ContentTypeRepository()
	if repo == nil {
		return
	}

	if seeder, ok := repo.(interface {
		Put(*content.ContentType) error
	}); ok {
		if err := seeder.Put(ct); err != nil {
			log.Printf("seed content type %s: %v", ct.Slug, err)
		}
	}
}
