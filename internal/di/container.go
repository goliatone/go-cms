package di

import (
	"strings"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/adapters/noop"
	"github.com/goliatone/go-cms/internal/adapters/storage"
	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/i18n"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
	repocache "github.com/goliatone/go-repository-cache/cache"
	urlkit "github.com/goliatone/go-urlkit"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Container wires module dependencies. Phase 1 only returns no-op services.
type Container struct {
	Config cms.Config

	storage  interfaces.StorageProvider
	cache    interfaces.CacheProvider
	template interfaces.TemplateRenderer
	media    interfaces.MediaProvider
	auth     interfaces.AuthService

	bunDB         *bun.DB
	cacheTTL      time.Duration
	cacheService  repocache.CacheService
	keySerializer repocache.KeySerializer

	contentRepo     content.ContentRepository
	contentTypeRepo content.ContentTypeRepository

	localeRepo content.LocaleRepository

	pageRepo pages.PageRepository

	blockRepo            blocks.InstanceRepository
	blockDefinitionRepo  blocks.DefinitionRepository
	blockTranslationRepo blocks.TranslationRepository

	menuRepo            menus.MenuRepository
	menuItemRepo        menus.MenuItemRepository
	menuTranslationRepo menus.MenuItemTranslationRepository
	menuURLResolver     menus.URLResolver
	routeManager        *urlkit.RouteManager

	widgetDefinitionRepo  widgets.DefinitionRepository
	widgetInstanceRepo    widgets.InstanceRepository
	widgetTranslationRepo widgets.TranslationRepository
	widgetAreaRepo        widgets.AreaDefinitionRepository
	widgetPlacementRepo   widgets.AreaPlacementRepository

	themeRepo    themes.ThemeRepository
	templateRepo themes.TemplateRepository

	memoryLocaleRepo *content.MemoryLocaleRepository

	contentSvc content.Service
	pageSvc    pages.Service
	blockSvc   blocks.Service
	i18nSvc    i18n.Service
	menuSvc    menus.Service
	widgetSvg  widgets.Service
	themeSvc   themes.Service
}

// Option mutates the container before it is finalised.
type Option func(*Container)

// WithStorage overrides the default storage provider.
func WithStorage(sp interfaces.StorageProvider) Option {
	return func(c *Container) {
		c.storage = sp
	}
}

// WithCache overrides the default cache provider.
func WithCache(service repocache.CacheService, serializer repocache.KeySerializer) Option {
	return func(c *Container) {
		c.cacheService = service
		c.keySerializer = serializer
	}
}

// WithTemplate overrides the default template renderer.
func WithTemplate(tr interfaces.TemplateRenderer) Option {
	return func(c *Container) {
		c.template = tr
	}
}

// WithMedia overrides the default media provider.
func WithMedia(mp interfaces.MediaProvider) Option {
	return func(c *Container) {
		c.media = mp
	}
}

// WithAuth overrides the default auth provider.
func WithAuth(ap interfaces.AuthService) Option {
	return func(c *Container) {
		c.auth = ap
	}
}

func WithBunDB(db *bun.DB) Option {
	return func(c *Container) {
		c.bunDB = db
	}
}

// WithContentService overrides the default content service binding.
func WithContentService(svc content.Service) Option {
	return func(c *Container) {
		c.contentSvc = svc
	}
}

// WithPageService overrides the default page service binding.
func WithPageService(svc pages.Service) Option {
	return func(c *Container) {
		c.pageSvc = svc
	}
}

// WithBlockService overrides the default page service binding.
func WithBlockService(svc blocks.Service) Option {
	return func(c *Container) {
		c.blockSvc = svc
	}
}

func WithWidgetService(svc widgets.Service) Option {
	return func(c *Container) {
		c.widgetSvg = svc
	}
}

func WithThemeService(svc themes.Service) Option {
	return func(c *Container) {
		c.themeSvc = svc
	}
}

func WithMenuService(svc menus.Service) Option {
	return func(c *Container) {
		c.menuSvc = svc
	}
}

// WithI18nService overrides the default i18n service binding.
func WithI18nService(svc i18n.Service) Option {
	return func(c *Container) {
		c.i18nSvc = svc
	}
}

// NewContainer creates a container with the provided configuration.
func NewContainer(cfg cms.Config, opts ...Option) *Container {
	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	cacheTTL := cfg.Cache.DefaultTTL
	if cacheTTL <= 0 {
		cacheTTL = time.Minute
	}

	memoryContentRepo := content.NewMemoryContentRepository()
	memoryContentTypeRepo := content.NewMemoryContentTypeRepository()
	memoryLocaleRepo := content.NewMemoryLocaleRepository()
	memoryPageRepo := pages.NewMemoryPageRepository()

	memoryBlockDefRepo := blocks.NewMemoryDefinitionRepository()
	memoryBlockRepo := blocks.NewMemoryInstanceRepository()
	memoryBlockTranslationRepo := blocks.NewMemoryTranslationRepository()

	memoryMenuRepo := menus.NewMemoryMenuRepository()
	memoryMenuItemRepo := menus.NewMemoryMenuItemRepository()
	memoryMenuTranslationRepo := menus.NewMemoryMenuItemTranslationRepository()

	memoryWidgetDefinitionRepo := widgets.NewMemoryDefinitionRepository()
	memoryWidgetInstanceRepo := widgets.NewMemoryInstanceRepository()
	memoryWidgetTranslationRepo := widgets.NewMemoryTranslationRepository()
	memoryWidgetAreaRepo := widgets.NewMemoryAreaDefinitionRepository()
	memoryWidgetPlacementRepo := widgets.NewMemoryAreaPlacementRepository()

	memoryThemeRepo := themes.NewMemoryThemeRepository()
	memoryTemplateRepo := themes.NewMemoryTemplateRepository()

	c := &Container{
		Config:                cfg,
		storage:               storage.NewNoOpProvider(),
		template:              noop.Template(),
		media:                 noop.Media(),
		auth:                  noop.Auth(),
		cacheTTL:              cacheTTL,
		contentRepo:           memoryContentRepo,
		contentTypeRepo:       memoryContentTypeRepo,
		localeRepo:            memoryLocaleRepo,
		pageRepo:              memoryPageRepo,
		blockDefinitionRepo:   memoryBlockDefRepo,
		blockRepo:             memoryBlockRepo,
		blockTranslationRepo:  memoryBlockTranslationRepo,
		menuRepo:              memoryMenuRepo,
		menuItemRepo:          memoryMenuItemRepo,
		menuTranslationRepo:   memoryMenuTranslationRepo,
		widgetDefinitionRepo:  memoryWidgetDefinitionRepo,
		widgetInstanceRepo:    memoryWidgetInstanceRepo,
		widgetTranslationRepo: memoryWidgetTranslationRepo,
		widgetAreaRepo:        memoryWidgetAreaRepo,
		widgetPlacementRepo:   memoryWidgetPlacementRepo,
		themeRepo:             memoryThemeRepo,
		templateRepo:          memoryTemplateRepo,
		memoryLocaleRepo:      memoryLocaleRepo,
	}

	c.seedLocales()

	for _, opt := range opts {
		opt(c)
	}

	c.configureCacheDefaults()
	c.configureRepositories()
	c.configureNavigation()

	if c.contentSvc == nil {
		c.contentSvc = content.NewService(c.contentRepo, c.contentTypeRepo, c.localeRepo)
	}

	if c.blockSvc == nil {
		c.blockSvc = blocks.NewService(
			c.blockDefinitionRepo,
			c.blockRepo,
			c.blockTranslationRepo,
		)
	}

	if c.themeSvc == nil {
		if !c.Config.Features.Themes {
			c.themeSvc = themes.NewNoOpService()
		} else {
			c.themeSvc = themes.NewService(c.themeRepo, c.templateRepo)
		}
	}

	if c.pageSvc == nil {
		pageOpts := []pages.ServiceOption{}
		if c.blockSvc != nil {
			pageOpts = append(pageOpts, pages.WithBlockService(c.blockSvc))
		}
		c.pageSvc = pages.NewService(c.pageRepo, c.contentRepo, c.localeRepo, pageOpts...)
	}

	if c.menuSvc == nil {
		menuOpts := []menus.ServiceOption{}
		if c.pageRepo != nil {
			menuOpts = append(menuOpts, menus.WithPageRepository(c.pageRepo))
		}
		if c.menuURLResolver != nil {
			menuOpts = append(menuOpts, menus.WithURLResolver(c.menuURLResolver))
		}
		c.menuSvc = menus.NewService(
			c.menuRepo,
			c.menuItemRepo,
			c.menuTranslationRepo,
			c.localeRepo,
			menuOpts...,
		)
	}

	if c.widgetSvg == nil {
		if !c.Config.Features.Widgets {
			c.widgetSvg = widgets.NewNoOpService()
		} else {
			registry := widgets.NewRegistry()
			registerBuiltInWidgets(registry)

			serviceOptions := []widgets.ServiceOption{
				widgets.WithRegistry(registry),
			}
			if c.widgetAreaRepo != nil {
				serviceOptions = append(serviceOptions, widgets.WithAreaDefinitionRepository(c.widgetAreaRepo))
			}
			if c.widgetPlacementRepo != nil {
				serviceOptions = append(serviceOptions, widgets.WithAreaPlacementRepository(c.widgetPlacementRepo))
			}

			c.widgetSvg = widgets.NewService(
				c.widgetDefinitionRepo,
				c.widgetInstanceRepo,
				c.widgetTranslationRepo,
				serviceOptions...,
			)
		}
	}

	if c.pageSvc == nil {
		pageOpts := []pages.ServiceOption{}
		if c.blockSvc != nil {
			pageOpts = append(pageOpts, pages.WithBlockService(c.blockSvc))
		}
		if c.widgetSvg != nil {
			pageOpts = append(pageOpts, pages.WithWidgetService(c.widgetSvg))
		}
		if c.themeSvc != nil {
			pageOpts = append(pageOpts, pages.WithThemeService(c.themeSvc))
		}
		c.pageSvc = pages.NewService(c.pageRepo, c.contentRepo, c.localeRepo, pageOpts...)
	}

	if c.menuSvc == nil {
		menuOpcs := []menus.ServiceOption{}
		if c.pageRepo != nil {
			menuOpcs = append(menuOpcs, menus.WithPageRepository(c.pageRepo))
		}
		if c.menuURLResolver != nil {
			menuOpcs = append(menuOpcs, menus.WithURLResolver(c.menuURLResolver))
		}
		c.menuSvc = menus.NewService(
			c.menuRepo,
			c.menuItemRepo,
			c.menuTranslationRepo,
			c.localeRepo,
			menuOpcs...,
		)
	}

	return c
}

func (c *Container) configureCacheDefaults() {
	if !c.Config.Cache.Enabled {
		return
	}

	if c.cacheService == nil {
		cfg := repocache.DefaultConfig()
		if c.cacheTTL > 0 {
			cfg.TTL = c.cacheTTL
		}
		service, err := repocache.NewCacheService(cfg)
		if err == nil {
			c.cacheService = service
		}
	}

	if c.cacheService != nil && c.keySerializer == nil {
		c.keySerializer = repocache.NewDefaultKeySerializer()
	}
}

func (c *Container) configureRepositories() {
	if c.bunDB != nil {
		c.contentRepo = content.NewBunContentRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)
		c.contentTypeRepo = content.NewBunContentTypeRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)
		c.localeRepo = content.NewBunLocaleRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)

		c.pageRepo = pages.NewBunPageRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)

		c.blockDefinitionRepo = blocks.NewBunDefinitionRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)
		c.blockRepo = blocks.NewBunInstanceRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)
		c.blockTranslationRepo = blocks.NewBunTranslationRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)

		c.menuRepo = menus.NewBunMenuRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)
		c.menuItemRepo = menus.NewBunMenuItemRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)
		c.menuTranslationRepo = menus.NewBunMenuItemTranslationRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)

		c.widgetDefinitionRepo = widgets.NewBunDefinitionRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)
		c.widgetInstanceRepo = widgets.NewBunInstanceRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)
		c.widgetTranslationRepo = widgets.NewBunTranslationRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)
		c.widgetAreaRepo = widgets.NewBunAreaDefinitionRepository(c.bunDB)
		c.widgetPlacementRepo = widgets.NewBunAreaPlacementRepository(c.bunDB)

		c.themeRepo = themes.NewBunThemeRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)
		c.templateRepo = themes.NewBunTemplateRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer)

		c.memoryLocaleRepo = nil
	}
}

func (c *Container) configureNavigation() {
	if c.menuURLResolver != nil {
		return
	}

	navCfg := c.Config.Navigation
	if navCfg.RouteConfig == nil {
		return
	}

	manager := urlkit.NewRouteManager(navCfg.RouteConfig)
	c.routeManager = manager

	resolver := menus.NewURLKitResolver(menus.URLKitResolverOptions{
		Manager:       manager,
		DefaultGroup:  strings.TrimSpace(navCfg.URLKit.DefaultGroup),
		LocaleGroups:  navCfg.URLKit.LocaleGroups,
		DefaultRoute:  strings.TrimSpace(navCfg.URLKit.DefaultRoute),
		SlugParam:     navCfg.URLKit.SlugParam,
		LocaleParam:   strings.TrimSpace(navCfg.URLKit.LocaleParam),
		LocaleIDParam: strings.TrimSpace(navCfg.URLKit.LocaleIDParam),
		RouteField:    strings.TrimSpace(navCfg.URLKit.RouteField),
		ParamsField:   strings.TrimSpace(navCfg.URLKit.ParamsField),
		QueryField:    strings.TrimSpace(navCfg.URLKit.QueryField),
	})

	c.menuURLResolver = resolver
}

// API constructs the top-level CMS API façade.
func (c *Container) API() *cms.API {
	return cms.Module()
}

// StorageProvider exposes the configured storage implementation.
func (c *Container) StorageProvider() interfaces.StorageProvider {
	return c.storage
}

// TemplateRenderer exposes the configured template renderer.
func (c *Container) TemplateRenderer() interfaces.TemplateRenderer {
	return c.template
}

// MediaProvider exposes the configured media provider.
func (c *Container) MediaProvider() interfaces.MediaProvider {
	return c.media
}

// AuthService exposes the configured auth service.
func (c *Container) AuthService() interfaces.AuthService {
	return c.auth
}

// ContentRepository exposes the configured content repository.
func (c *Container) ContentRepository() content.ContentRepository {
	return c.contentRepo
}

// ContentTypeRepository exposes the configured content-type repository.
func (c *Container) ContentTypeRepository() content.ContentTypeRepository {
	return c.contentTypeRepo
}

// LocaleRepository exposes the configured locale repository.
func (c *Container) LocaleRepository() content.LocaleRepository {
	return c.localeRepo
}

// ContentService returns the configured content service.
func (c *Container) ContentService() content.Service {
	return c.contentSvc
}

// PageService returns the configured page service.
func (c *Container) PageService() pages.Service {
	return c.pageSvc
}

// BlockService returns the configured block service.
func (c *Container) BlockService() blocks.Service {
	return c.blockSvc
}

// MenuService returns the configured menu service.
func (c *Container) MenuService() menus.Service {
	return c.menuSvc
}

// WidgetService returns the configured widget service.
func (c *Container) WidgetService() widgets.Service {
	return c.widgetSvg
}

func (c *Container) ThemeService() themes.Service {
	return c.themeSvc
}

// I18nService returns the configured i18n service (lazy).
func (c *Container) I18nService() i18n.Service {
	if c.i18nSvc != nil {
		return c.i18nSvc
	}

	if !c.Config.I18N.Enabled {
		c.i18nSvc = i18n.NewNoOpService()
		return c.i18nSvc
	}

	cfg := i18n.FromModuleConfig(c.Config.DefaultLocale, c.Config.I18N.Locales)

	fixture, err := i18n.DefaultFixture()
	if err != nil {
		c.i18nSvc = i18n.NewNoOpService()
		return c.i18nSvc
	}

	for _, loc := range fixture.Config.Locales {
		cfg.WithFallbacks(loc.Code, loc.Fallbacks...)
	}

	if len(cfg.Locales) == 0 && len(fixture.Config.Locales) > 0 {
		cfg = fixture.Config
	}

	service, err := i18n.NewInMemoryService(cfg, fixture.Translations)
	if err != nil {
		c.i18nSvc = i18n.NewNoOpService()
		return c.i18nSvc
	}

	c.i18nSvc = service
	return c.i18nSvc
}

func (c *Container) seedLocales() {
	if c.memoryLocaleRepo == nil {
		return
	}

	locales := c.Config.I18N.Locales
	if len(locales) == 0 {
		locales = []string{c.Config.DefaultLocale}
	}

	seen := map[string]struct{}{}
	for _, code := range locales {
		normalized := strings.TrimSpace(code)
		if normalized == "" {
			continue
		}
		lower := strings.ToLower(normalized)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		c.memoryLocaleRepo.Put(&content.Locale{
			ID:        uuid.New(),
			Code:      lower,
			Display:   normalized,
			IsActive:  true,
			IsDefault: strings.EqualFold(normalized, c.Config.DefaultLocale),
		})
	}
}

func registerBuiltInWidgets(registry *widgets.Registry) {
	if registry == nil {
		return
	}
	// TODO: Do now hardcode this, take from config
	description := strPtr("Captures visitor email addresses with a simple form")
	category := strPtr("marketing")
	icon := strPtr("envelope-open")

	registry.Register(widgets.RegisterDefinitionInput{
		Name:        "newsletter_signup",
		Description: description,
		Schema: map[string]any{
			"fields": []any{
				map[string]any{"name": "headline", "type": "text"},
				map[string]any{"name": "subheadline", "type": "text"},
				map[string]any{"name": "cta_text", "type": "text"},
				map[string]any{"name": "success_message", "type": "text"},
			},
		},
		Defaults: map[string]any{
			"cta_text":        "Subscribe",
			"success_message": "Thanks for subscribing!",
		},
		Category: category,
		Icon:     icon,
	})
}

func strPtr(value string) *string {
	return &value
}
