package di

import (
	"errors"
	"maps"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/adapters/noop"
	"github.com/goliatone/go-cms/internal/adapters/storage"
	"github.com/goliatone/go-cms/internal/blocks"
	cmscommands "github.com/goliatone/go-cms/internal/commands"
	auditcmd "github.com/goliatone/go-cms/internal/commands/audit"
	blockscmd "github.com/goliatone/go-cms/internal/commands/blocks"
	contentcmd "github.com/goliatone/go-cms/internal/commands/content"
	markdowncmd "github.com/goliatone/go-cms/internal/commands/markdown"
	mediacmd "github.com/goliatone/go-cms/internal/commands/media"
	menuscmd "github.com/goliatone/go-cms/internal/commands/menus"
	pagescmd "github.com/goliatone/go-cms/internal/commands/pages"
	widgetscmd "github.com/goliatone/go-cms/internal/commands/widgets"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/i18n"
	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	cmsscheduler "github.com/goliatone/go-cms/internal/scheduler"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
	repocache "github.com/goliatone/go-repository-cache/cache"
	urlkit "github.com/goliatone/go-urlkit"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Container wires module dependencies. Phase 1 only returns no-op services.
type Container struct {
	Config runtimeconfig.Config

	storage   interfaces.StorageProvider
	cache     interfaces.CacheProvider
	template  interfaces.TemplateRenderer
	media     interfaces.MediaProvider
	auth      interfaces.AuthService
	scheduler interfaces.Scheduler

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

	contentSvc     content.Service
	pageSvc        pages.Service
	blockSvc       blocks.Service
	i18nSvc        i18n.Service
	menuSvc        menus.Service
	widgetSvg      widgets.Service
	themeSvc       themes.Service
	mediaSvc       media.Service
	markdownSvc    interfaces.MarkdownService
	loggerProvider interfaces.LoggerProvider

	commandRegistry      CommandRegistry
	commandDispatcher    CommandDispatcher
	cronRegistrar        CronRegistrar
	commandSubscriptions []CommandSubscription
	commandHandlers      []any

	generatorSvc           generator.Service
	generatorStorage       interfaces.StorageProvider
	generatorAssetResolver generator.AssetResolver

	auditRecorder jobs.AuditRecorder
	jobWorker     *jobs.Worker
}

// Option mutates the container before it is finalised.
type Option func(*Container)

// CommandRegistry records command handlers so hosts can expose them via CLI or cron.
type CommandRegistry interface {
	RegisterCommand(handler any) error
}

// CommandDispatcher subscribes command handlers to a dispatcher implementation.
type CommandDispatcher interface {
	RegisterCommand(handler any) (CommandSubscription, error)
}

// CommandSubscription allows hosts to tear down dispatcher subscriptions.
type CommandSubscription interface {
	Unsubscribe()
}

// CronRegistrar registers command handlers with a cron scheduler.
type CronRegistrar func(command.HandlerConfig, any) error

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

func WithCacheProvider(provider interfaces.CacheProvider) Option {
	return func(c *Container) {
		c.cache = provider
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

func WithMarkdownService(svc interfaces.MarkdownService) Option {
	return func(c *Container) {
		c.markdownSvc = svc
	}
}

func WithScheduler(s interfaces.Scheduler) Option {
	return func(c *Container) {
		c.scheduler = s
	}
}

// WithGeneratorOutput overrides the generator output directory.
func WithGeneratorOutput(output string) Option {
	return func(c *Container) {
		c.Config.Generator.OutputDir = output
	}
}

// WithGeneratorStorage overrides the storage provider used by the generator.
func WithGeneratorStorage(sp interfaces.StorageProvider) Option {
	return func(c *Container) {
		c.generatorStorage = sp
	}
}

// WithGeneratorAssetResolver overrides the asset resolver used during static builds.
func WithGeneratorAssetResolver(resolver generator.AssetResolver) Option {
	return func(c *Container) {
		c.generatorAssetResolver = resolver
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

// WithLoggerProvider overrides the logger provider used to construct command loggers.
func WithLoggerProvider(provider interfaces.LoggerProvider) Option {
	return func(c *Container) {
		c.loggerProvider = provider
	}
}

// WithCommandRegistry wires an external command registry for automatic command registration.
func WithCommandRegistry(reg CommandRegistry) Option {
	return func(c *Container) {
		c.commandRegistry = reg
	}
}

// WithCommandDispatcher wires an external dispatcher registrar used for auto-subscription.
func WithCommandDispatcher(dispatcher CommandDispatcher) Option {
	return func(c *Container) {
		c.commandDispatcher = dispatcher
	}
}

// WithCronRegistrar registers a cron scheduler function used when commands expose cron handlers.
func WithCronRegistrar(registrar CronRegistrar) Option {
	return func(c *Container) {
		c.cronRegistrar = registrar
	}
}

// WithAuditRecorder overrides the audit recorder used by audit commands and worker instrumentation.
func WithAuditRecorder(recorder jobs.AuditRecorder) Option {
	return func(c *Container) {
		c.auditRecorder = recorder
	}
}

// WithCommandWorker overrides the worker invoked by audit command handlers.
func WithCommandWorker(worker *jobs.Worker) Option {
	return func(c *Container) {
		c.jobWorker = worker
	}
}

// NewContainer creates a container with the provided configuration.
func NewContainer(cfg runtimeconfig.Config, opts ...Option) (*Container, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
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
		cache:                 noop.Cache(),
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
		mediaSvc:              media.NewNoOpService(),
		generatorStorage:      storage.NewNoOpProvider(),
	}

	c.seedLocales()

	for _, opt := range opts {
		opt(c)
	}

	c.configureCacheDefaults()
	c.configureRepositories()
	c.configureNavigation()
	c.configureScheduler()
	c.configureMediaService()

	if c.contentSvc == nil {
		contentOpts := []content.ServiceOption{
			content.WithVersioningEnabled(c.Config.Features.Versioning),
			content.WithScheduler(c.scheduler),
			content.WithSchedulingEnabled(c.Config.Features.Scheduling),
		}
		c.contentSvc = content.NewService(c.contentRepo, c.contentTypeRepo, c.localeRepo, contentOpts...)
	}

	if c.blockSvc == nil {
		blockOpts := []blocks.ServiceOption{
			blocks.WithMediaService(c.mediaSvc),
			blocks.WithVersioningEnabled(c.Config.Features.Versioning),
		}
		c.blockSvc = blocks.NewService(
			c.blockDefinitionRepo,
			c.blockRepo,
			c.blockTranslationRepo,
			blockOpts...,
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
		pageOpts = append(pageOpts, pages.WithMediaService(c.mediaSvc))
		pageOpts = append(pageOpts,
			pages.WithPageVersioningEnabled(c.Config.Features.Versioning),
			pages.WithSchedulingEnabled(c.Config.Features.Scheduling),
			pages.WithScheduler(c.scheduler),
		)
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
		pageOpts = append(pageOpts, pages.WithMediaService(c.mediaSvc))
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

	if c.auditRecorder == nil {
		c.auditRecorder = jobs.NewInMemoryAuditRecorder()
	}
	if c.jobWorker == nil {
		c.jobWorker = jobs.NewWorker(c.scheduler, c.contentRepo, c.pageRepo, jobs.WithAuditRecorder(c.auditRecorder))
	}

	c.applyCronRegistrar()

	if err := c.registerCommandHandlers(); err != nil {
		return nil, err
	}

	if c.generatorSvc == nil {
		if !c.Config.Generator.Enabled {
			c.generatorSvc = generator.NewDisabledService()
		} else {
			genCfg := generator.Config{
				OutputDir:       c.Config.Generator.OutputDir,
				BaseURL:         c.Config.Generator.BaseURL,
				CleanBuild:      c.Config.Generator.CleanBuild,
				Incremental:     c.Config.Generator.Incremental,
				CopyAssets:      c.Config.Generator.CopyAssets,
				GenerateSitemap: c.Config.Generator.GenerateSitemap,
				GenerateRobots:  c.Config.Generator.GenerateRobots,
				GenerateFeeds:   c.Config.Generator.GenerateFeeds,
				Workers:         c.Config.Generator.Workers,
				DefaultLocale:   c.Config.DefaultLocale,
				Locales:         append([]string{}, c.Config.I18N.Locales...),
				Menus:           maps.Clone(c.Config.Generator.Menus),
			}
			genDeps := generator.Dependencies{
				Pages:    c.PageService(),
				Content:  c.ContentService(),
				Blocks:   c.BlockService(),
				Widgets:  c.WidgetService(),
				Menus:    c.MenuService(),
				Themes:   c.ThemeService(),
				I18N:     c.I18nService(),
				Renderer: c.template,
				Storage:  c.generatorStorage,
				Locales:  c.localeRepo,
				Assets:   c.generatorAssetResolver,
			}
			c.generatorSvc = generator.NewService(genCfg, genDeps)
		}
	}

	return c, nil
}

func (c *Container) configureCacheDefaults() {
	if !c.Config.Cache.Enabled || !c.Config.Features.AdvancedCache {
		c.cacheService = nil
		c.keySerializer = nil
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

func (c *Container) configureScheduler() {
	if !c.Config.Features.Scheduling {
		c.scheduler = cmsscheduler.NewNoOp()
		return
	}
	if c.scheduler == nil {
		c.scheduler = cmsscheduler.NewInMemory()
	}
}

func (c *Container) configureMediaService() {
	if !c.Config.Features.MediaLibrary || c.media == nil {
		c.mediaSvc = media.NewNoOpService()
		return
	}
	options := []media.ServiceOption{}
	if c.Config.Features.AdvancedCache && c.cache != nil && c.cacheTTL > 0 {
		options = append(options, media.WithCache(c.cache, c.cacheTTL))
	}
	c.mediaSvc = media.NewService(c.media, options...)
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

func (c *Container) PageRepository() pages.PageRepository {
	return c.pageRepo
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

// MediaService returns the configured media helper service.
func (c *Container) MediaService() media.Service {
	return c.mediaSvc
}

// MarkdownService returns the configured markdown service.
func (c *Container) MarkdownService() interfaces.MarkdownService {
	return c.markdownSvc
}

func (c *Container) Scheduler() interfaces.Scheduler {
	return c.scheduler
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

// CommandHandlers returns copies of the registered command handlers.
func (c *Container) CommandHandlers() []any {
	if len(c.commandHandlers) == 0 {
		return nil
	}
	handlers := make([]any, len(c.commandHandlers))
	copy(handlers, c.commandHandlers)
	return handlers
}

// GeneratorService returns the configured static site generator service.
func (c *Container) GeneratorService() generator.Service {
	if c == nil {
		return generator.NewDisabledService()
	}
	if c.generatorSvc == nil {
		return generator.NewDisabledService()
	}
	return c.generatorSvc
}

func (c *Container) applyCronRegistrar() {
	if c.commandRegistry == nil || c.cronRegistrar == nil {
		return
	}
	if reg, ok := c.commandRegistry.(interface {
		SetCronRegister(func(command.HandlerConfig, any) error) *command.Registry
	}); ok && reg != nil {
		reg.SetCronRegister(c.cronRegistrar)
	}
}

func (c *Container) shouldBuildCommands() bool {
	if !c.Config.Commands.Enabled {
		return false
	}
	if c.commandRegistry != nil {
		return true
	}
	if c.commandDispatcher != nil {
		return true
	}
	if c.cronRegistrar != nil {
		return true
	}
	return false
}

func (c *Container) registerCommandHandlers() error {
	if !c.shouldBuildCommands() {
		return nil
	}

	var errs error

	register := func(handler any) {
		if handler == nil {
			return
		}
		c.commandHandlers = append(c.commandHandlers, handler)

		if c.commandRegistry != nil {
			if err := c.commandRegistry.RegisterCommand(handler); err != nil {
				errs = errors.Join(errs, err)
			}
		}
		if c.Config.Commands.AutoRegisterDispatcher && c.commandDispatcher != nil {
			subscription, err := c.commandDispatcher.RegisterCommand(handler)
			if err != nil {
				errs = errors.Join(errs, err)
			} else if subscription != nil {
				c.commandSubscriptions = append(c.commandSubscriptions, subscription)
			}
		}
		if c.Config.Commands.AutoRegisterCron && c.cronRegistrar != nil {
			if cronCmd, ok := handler.(command.CronCommand); ok {
				if err := c.cronRegistrar(cronCmd.CronOptions(), cronCmd.CronHandler()); err != nil {
					errs = errors.Join(errs, err)
				}
			}
		}
	}

	loggerFor := func(module string) interfaces.Logger {
		return cmscommands.CommandLogger(c.loggerProvider, module)
	}

	// Content commands.
	if c.contentSvc != nil {
		gates := contentcmd.FeatureGates{
			VersioningEnabled: func() bool { return c.Config.Features.Versioning },
			SchedulingEnabled: func() bool { return c.Config.Features.Scheduling },
		}
		if c.Config.Features.Versioning {
			contentLogger := loggerFor("content")
			register(contentcmd.NewPublishContentHandler(c.contentSvc, contentLogger, gates))
			register(contentcmd.NewRestoreContentVersionHandler(c.contentSvc, contentLogger, gates))
		}
		if c.Config.Features.Scheduling {
			register(contentcmd.NewScheduleContentHandler(c.contentSvc, loggerFor("content"), gates))
		}
	}

	// Page commands.
	if c.pageSvc != nil {
		gates := pagescmd.FeatureGates{
			VersioningEnabled: func() bool { return c.Config.Features.Versioning },
			SchedulingEnabled: func() bool { return c.Config.Features.Scheduling },
		}
		if c.Config.Features.Versioning {
			pagesLogger := loggerFor("pages")
			register(pagescmd.NewPublishPageHandler(c.pageSvc, pagesLogger, gates))
			register(pagescmd.NewRestorePageVersionHandler(c.pageSvc, pagesLogger, gates))
		}
		if c.Config.Features.Scheduling {
			register(pagescmd.NewSchedulePageHandler(c.pageSvc, loggerFor("pages"), gates))
		}
	}

	// Media commands.
	if c.mediaSvc != nil && c.Config.Features.MediaLibrary {
		gates := mediacmd.FeatureGates{
			MediaLibraryEnabled: func() bool { return c.Config.Features.MediaLibrary },
		}
		mediaLogger := loggerFor("media")
		register(mediacmd.NewImportAssetsHandler(c.mediaSvc, mediaLogger, gates))
		register(mediacmd.NewCleanupAssetsHandler(c.mediaSvc, mediaLogger, gates))
	}

	// Markdown commands.
	if c.markdownSvc != nil && c.Config.Features.Markdown {
		gates := markdowncmd.FeatureGates{
			MarkdownEnabled: func() bool { return c.Config.Features.Markdown },
		}
		handlerSet, err := markdowncmd.RegisterMarkdownCommands(nil, c.markdownSvc, c.loggerProvider, gates)
		if err != nil {
			errs = errors.Join(errs, err)
		} else if handlerSet != nil {
			register(handlerSet.Import)
			register(handlerSet.Sync)
		}
	}

	// Menu commands.
	if c.menuSvc != nil {
		gates := menuscmd.FeatureGates{
			MenusEnabled: func() bool { return c.menuSvc != nil },
		}
		register(menuscmd.NewInvalidateMenuCacheHandler(c.menuSvc, loggerFor("menus"), gates))
	}

	// Blocks commands.
	if c.blockSvc != nil {
		gates := blockscmd.FeatureGates{
			BlocksEnabled: func() bool { return c.blockSvc != nil },
		}
		register(blockscmd.NewSyncBlockRegistryHandler(c.blockSvc, loggerFor("blocks"), gates))
	}

	// Widget commands.
	if c.widgetSvg != nil && c.Config.Features.Widgets {
		gates := widgetscmd.FeatureGates{
			WidgetsEnabled: func() bool { return c.Config.Features.Widgets },
		}
		register(widgetscmd.NewSyncWidgetRegistryHandler(c.widgetSvg, loggerFor("widgets"), gates))
	}

	// Audit commands.
	if c.Config.Features.Scheduling && c.auditRecorder != nil {
		auditLogger := loggerFor("audit")
		if c.jobWorker != nil {
			register(auditcmd.NewReplayAuditHandler(c.jobWorker, auditLogger))
		}
		register(auditcmd.NewExportAuditHandler(c.auditRecorder, auditLogger))
		cleanupOpts := []auditcmd.CleanupHandlerOption{}
		if expr := strings.TrimSpace(c.Config.Commands.CleanupAuditCron); expr != "" {
			cleanupOpts = append(cleanupOpts, auditcmd.CleanupWithCronExpression(expr))
		}
		register(auditcmd.NewCleanupAuditHandler(c.auditRecorder, auditLogger, cleanupOpts...))
	}

	return errs
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
