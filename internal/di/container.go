package di

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/goliatone/go-cms/internal/adapters/noop"
	storageadapter "github.com/goliatone/go-cms/internal/adapters/storage"
	adminstorage "github.com/goliatone/go-cms/internal/admin/storage"
	"github.com/goliatone/go-cms/internal/blocks"
	cmscommands "github.com/goliatone/go-cms/internal/commands"
	auditcmd "github.com/goliatone/go-cms/internal/commands/audit"
	blockscmd "github.com/goliatone/go-cms/internal/commands/blocks"
	contentcmd "github.com/goliatone/go-cms/internal/commands/content"
	markdowncmd "github.com/goliatone/go-cms/internal/commands/markdown"
	mediacmd "github.com/goliatone/go-cms/internal/commands/media"
	menuscmd "github.com/goliatone/go-cms/internal/commands/menus"
	pagescmd "github.com/goliatone/go-cms/internal/commands/pages"
	staticcmd "github.com/goliatone/go-cms/internal/commands/static"
	widgetscmd "github.com/goliatone/go-cms/internal/commands/widgets"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/i18n"
	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/logging/console"
	"github.com/goliatone/go-cms/internal/logging/gologger"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	cmsscheduler "github.com/goliatone/go-cms/internal/scheduler"
	"github.com/goliatone/go-cms/internal/storageconfig"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/internal/workflow"
	workflowsimple "github.com/goliatone/go-cms/internal/workflow/simple"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/storage"
	command "github.com/goliatone/go-command"
	repocache "github.com/goliatone/go-repository-cache/cache"
	urlkit "github.com/goliatone/go-urlkit"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

// ErrWorkflowEngineNotProvided is returned when a custom workflow provider is configured without supplying an engine.
var ErrWorkflowEngineNotProvided = errors.New("di: workflow engine required for custom workflow provider")

// Container wires module dependencies. Phase 1 only returns no-op services.
type Container struct {
	Config runtimeconfig.Config

	storage          interfaces.StorageProvider
	storageRepo      storageconfig.Repository
	storageFactories map[string]StorageFactory
	storageProfiles  map[string]storage.Profile
	storageMu        sync.RWMutex
	storageHandle    *storageHandle
	storageCancel    context.CancelFunc
	storageLogger    interfaces.Logger
	storageAdminSvc  *adminstorage.Service
	activeProfile    string

	cache     interfaces.CacheProvider
	template  interfaces.TemplateRenderer
	media     interfaces.MediaProvider
	auth      interfaces.AuthService
	scheduler interfaces.Scheduler

	bunDB         *bun.DB
	cacheTTL      time.Duration
	cacheService  repocache.CacheService
	keySerializer repocache.KeySerializer

	memoryContentRepo     *content.MemoryContentRepository
	memoryContentTypeRepo *content.MemoryContentTypeRepository
	memoryLocaleRepo      *content.MemoryLocaleRepository

	contentRepo     *contentRepositoryProxy
	contentTypeRepo *contentTypeRepositoryProxy
	localeRepo      *localeRepositoryProxy

	memoryPageRepo *pages.MemoryPageRepository
	pageRepo       *pageRepositoryProxy

	memoryBlockDefinitionRepo  blocks.DefinitionRepository
	memoryBlockRepo            blocks.InstanceRepository
	memoryBlockTranslationRepo blocks.TranslationRepository
	memoryBlockVersionRepo     blocks.InstanceVersionRepository

	blockRepo            *blockInstanceRepositoryProxy
	blockDefinitionRepo  *blockDefinitionRepositoryProxy
	blockTranslationRepo *blockTranslationRepositoryProxy
	blockVersionRepo     *blockVersionRepositoryProxy

	memoryMenuRepo              menus.MenuRepository
	memoryMenuItemRepo          menus.MenuItemRepository
	memoryMenuTranslationRepo   menus.MenuItemTranslationRepository
	memoryWidgetDefinitionRepo  widgets.DefinitionRepository
	memoryWidgetInstanceRepo    widgets.InstanceRepository
	memoryWidgetTranslationRepo widgets.TranslationRepository
	memoryWidgetAreaRepo        widgets.AreaDefinitionRepository
	memoryWidgetPlacementRepo   widgets.AreaPlacementRepository
	memoryThemeRepo             themes.ThemeRepository
	memoryTemplateRepo          themes.TemplateRepository

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

	contentSvc     content.Service
	pageSvc        pages.Service
	blockSvc       blocks.Service
	i18nSvc        i18n.Service
	menuSvc        menus.Service
	widgetSvc      widgets.Service
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
	generatorHooks         generator.Hooks

	auditRecorder jobs.AuditRecorder
	jobWorker     *jobs.Worker

	workflowEngine          interfaces.WorkflowEngine
	workflowDefinitionStore interfaces.WorkflowDefinitionStore
}

// Option mutates the container before it is finalised.
type Option func(*Container)

// StorageFactory constructs storage providers and their backing handles for a profile.
type StorageFactory func(ctx context.Context, profile storage.Profile) (StorageFactoryResult, error)

// StorageFactoryResult captures the outcome of a storage factory invocation.
type StorageFactoryResult struct {
	Provider interfaces.StorageProvider
	BunDB    *bun.DB
	Closer   func(context.Context) error
}

type storageHandle struct {
	profile  storage.Profile
	provider interfaces.StorageProvider
	bunDB    *bun.DB
	closer   func(context.Context) error
}

func (h *storageHandle) Close(ctx context.Context) {
	if h == nil || h.closer == nil {
		return
	}
	_ = h.closer(ctx)
}

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
		if sp != nil {
			c.storageHandle = &storageHandle{
				profile: storage.Profile{
					Name:     "manual",
					Provider: "manual",
				},
				provider: sp,
			}
		}
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

// WithWorkflowEngine overrides the workflow engine used by the module.
func WithWorkflowEngine(engine interfaces.WorkflowEngine) Option {
	return func(c *Container) {
		c.workflowEngine = engine
	}
}

// WithWorkflowDefinitionStore registers an external source for workflow definitions.
func WithWorkflowDefinitionStore(store interfaces.WorkflowDefinitionStore) Option {
	return func(c *Container) {
		c.workflowDefinitionStore = store
	}
}

// WithGeneratorHooks registers lifecycle hooks for generator operations.
func WithGeneratorHooks(hooks generator.Hooks) Option {
	return func(c *Container) {
		c.generatorHooks = hooks
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
		c.widgetSvc = svc
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

// WithStorageRepository overrides the storage profile repository used for runtime configuration.
func WithStorageRepository(repo storageconfig.Repository) Option {
	return func(c *Container) {
		if repo != nil {
			c.storageRepo = repo
		}
	}
}

// WithStorageFactory registers a storage provider factory under the given kind.
func WithStorageFactory(kind string, factory StorageFactory) Option {
	return func(c *Container) {
		trimmed := strings.ToLower(strings.TrimSpace(kind))
		if trimmed == "" || factory == nil {
			return
		}
		if c.storageFactories == nil {
			c.storageFactories = map[string]StorageFactory{}
		}
		c.storageFactories[trimmed] = factory
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
	memoryBlockVersionRepo := blocks.NewMemoryInstanceVersionRepository()

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
		Config:           cfg,
		storage:          storageadapter.NewNoOpProvider(),
		storageRepo:      storageconfig.NewMemoryRepository(),
		storageFactories: map[string]StorageFactory{},
		storageProfiles:  map[string]storage.Profile{},
		template:         noop.Template(),
		media:            noop.Media(),
		auth:             noop.Auth(),
		cache:            noop.Cache(),
		cacheTTL:         cacheTTL,

		memoryContentRepo:     memoryContentRepo,
		memoryContentTypeRepo: memoryContentTypeRepo,
		memoryLocaleRepo:      memoryLocaleRepo,
		memoryPageRepo:        memoryPageRepo,

		contentRepo:     newContentRepositoryProxy(memoryContentRepo),
		contentTypeRepo: newContentTypeRepositoryProxy(memoryContentTypeRepo),
		localeRepo:      newLocaleRepositoryProxy(memoryLocaleRepo),
		pageRepo:        newPageRepositoryProxy(memoryPageRepo),

		memoryBlockDefinitionRepo:  memoryBlockDefRepo,
		memoryBlockRepo:            memoryBlockRepo,
		memoryBlockTranslationRepo: memoryBlockTranslationRepo,
		memoryBlockVersionRepo:     memoryBlockVersionRepo,

		memoryMenuRepo:              memoryMenuRepo,
		memoryMenuItemRepo:          memoryMenuItemRepo,
		memoryMenuTranslationRepo:   memoryMenuTranslationRepo,
		memoryWidgetDefinitionRepo:  memoryWidgetDefinitionRepo,
		memoryWidgetInstanceRepo:    memoryWidgetInstanceRepo,
		memoryWidgetTranslationRepo: memoryWidgetTranslationRepo,
		memoryWidgetAreaRepo:        memoryWidgetAreaRepo,
		memoryWidgetPlacementRepo:   memoryWidgetPlacementRepo,
		memoryThemeRepo:             memoryThemeRepo,
		memoryTemplateRepo:          memoryTemplateRepo,

		blockDefinitionRepo:  newBlockDefinitionRepositoryProxy(memoryBlockDefRepo),
		blockRepo:            newBlockInstanceRepositoryProxy(memoryBlockRepo),
		blockTranslationRepo: newBlockTranslationRepositoryProxy(memoryBlockTranslationRepo),
		blockVersionRepo:     newBlockVersionRepositoryProxy(memoryBlockVersionRepo),

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

		mediaSvc:         media.NewNoOpService(),
		generatorStorage: storageadapter.NewNoOpProvider(),
	}

	c.seedLocales()

	for _, opt := range opts {
		opt(c)
	}

	if err := c.configureLoggerProvider(); err != nil {
		return nil, err
	}
	c.storageLogger = logging.StorageLogger(c.loggerProvider)
	c.configureCacheDefaults()
	if err := c.initializeStorage(context.Background()); err != nil {
		return nil, err
	}
	c.configureNavigation()
	c.configureScheduler()
	c.configureMediaService()
	if err := c.configureWorkflowEngine(); err != nil {
		return nil, err
	}
	c.configureStorageAdminService()

	requireTranslations := c.Config.I18N.RequireTranslations
	translationsEnabled := c.Config.I18N.Enabled
	if !translationsEnabled && requireTranslations {
		requireTranslations = false
	}

	if c.contentSvc == nil {
		contentOpts := []content.ServiceOption{
			content.WithVersioningEnabled(c.Config.Features.Versioning),
			content.WithVersionRetentionLimit(c.Config.Retention.Content),
			content.WithScheduler(c.scheduler),
			content.WithSchedulingEnabled(c.Config.Features.Scheduling),
			content.WithLogger(logging.ContentLogger(c.loggerProvider)),
		}
		contentOpts = append(contentOpts,
			content.WithRequireTranslations(requireTranslations),
			content.WithTranslationsEnabled(translationsEnabled),
		)
		c.contentSvc = content.NewService(c.contentRepo, c.contentTypeRepo, c.localeRepo, contentOpts...)
	}

	if c.blockSvc == nil {
		blockOpts := []blocks.ServiceOption{
			blocks.WithMediaService(c.mediaSvc),
			blocks.WithVersioningEnabled(c.Config.Features.Versioning),
			blocks.WithVersionRetentionLimit(c.Config.Retention.Blocks),
		}
		if c.blockVersionRepo != nil {
			blockOpts = append(blockOpts, blocks.WithInstanceVersionRepository(c.blockVersionRepo))
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

	if c.widgetSvc == nil {
		if !c.Config.Features.Widgets {
			c.widgetSvc = widgets.NewNoOpService()
		} else {
			registry := widgets.NewRegistry()
			applyConfiguredWidgetDefinitions(registry, c.Config.Widgets.Definitions)

			serviceOptions := []widgets.ServiceOption{
				widgets.WithRegistry(registry),
			}
			if c.widgetAreaRepo != nil {
				serviceOptions = append(serviceOptions, widgets.WithAreaDefinitionRepository(c.widgetAreaRepo))
			}
			if c.widgetPlacementRepo != nil {
				serviceOptions = append(serviceOptions, widgets.WithAreaPlacementRepository(c.widgetPlacementRepo))
			}

			c.widgetSvc = widgets.NewService(
				c.widgetDefinitionRepo,
				c.widgetInstanceRepo,
				c.widgetTranslationRepo,
				serviceOptions...,
			)
		}
	}

	if c.pageSvc == nil {
		pageOpts := []pages.ServiceOption{
			pages.WithMediaService(c.mediaSvc),
			pages.WithPageVersioningEnabled(c.Config.Features.Versioning),
			pages.WithPageVersionRetentionLimit(c.Config.Retention.Pages),
			pages.WithSchedulingEnabled(c.Config.Features.Scheduling),
			pages.WithScheduler(c.scheduler),
			pages.WithLogger(logging.PagesLogger(c.loggerProvider)),
			pages.WithWorkflowEngine(c.workflowEngine),
		}
		pageOpts = append(pageOpts,
			pages.WithRequireTranslations(requireTranslations),
			pages.WithTranslationsEnabled(translationsEnabled),
		)
		if c.blockSvc != nil {
			pageOpts = append(pageOpts, pages.WithBlockService(c.blockSvc))
		}
		if c.widgetSvc != nil {
			pageOpts = append(pageOpts, pages.WithWidgetService(c.widgetSvc))
		}
		if c.themeSvc != nil {
			pageOpts = append(pageOpts, pages.WithThemeService(c.themeSvc))
		}
		c.pageSvc = pages.NewService(c.pageRepo, c.contentRepo, c.localeRepo, pageOpts...)
	}

	if c.menuSvc == nil {
		menuOpts := []menus.ServiceOption{}
		menuOpts = append(menuOpts,
			menus.WithRequireTranslations(requireTranslations),
			menus.WithTranslationsEnabled(translationsEnabled),
		)
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
				OutputDir:        c.Config.Generator.OutputDir,
				BaseURL:          c.Config.Generator.BaseURL,
				CleanBuild:       c.Config.Generator.CleanBuild,
				Incremental:      c.Config.Generator.Incremental,
				CopyAssets:       c.Config.Generator.CopyAssets,
				GenerateSitemap:  c.Config.Generator.GenerateSitemap,
				GenerateRobots:   c.Config.Generator.GenerateRobots,
				GenerateFeeds:    c.Config.Generator.GenerateFeeds,
				Workers:          c.Config.Generator.Workers,
				DefaultLocale:    c.Config.DefaultLocale,
				Locales:          append([]string{}, c.Config.I18N.Locales...),
				Menus:            maps.Clone(c.Config.Generator.Menus),
				RenderTimeout:    c.Config.Generator.RenderTimeout,
				AssetCopyTimeout: c.Config.Generator.AssetCopyTimeout,
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
				Hooks:    c.generatorHooks,
				Logger:   logging.GeneratorLogger(c.loggerProvider),
			}
			c.generatorSvc = generator.NewService(genCfg, genDeps)
		}
	}

	c.subscribeStorageEvents()

	return c, nil
}

func (c *Container) configureLoggerProvider() error {
	if c.loggerProvider != nil || !c.Config.Features.Logger {
		return nil
	}

	provider := strings.ToLower(strings.TrimSpace(c.Config.Logging.Provider))
	switch provider {
	case "", "console":
		options := console.Options{}
		if lvl, ok := parseConsoleLevel(c.Config.Logging.Level); ok {
			options.MinLevel = &lvl
		}
		c.loggerProvider = console.NewProvider(options)
	case "gologger":
		provider, err := gologger.NewProvider(gologger.Config{
			Level:     c.Config.Logging.Level,
			Format:    c.Config.Logging.Format,
			AddSource: c.Config.Logging.AddSource,
			Focus:     append([]string{}, c.Config.Logging.Focus...),
		})
		if err != nil {
			return err
		}
		c.loggerProvider = provider
	default:
		c.loggerProvider = console.NewProvider(console.Options{})
	}

	return nil
}

func parseConsoleLevel(level string) (console.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "trace":
		lvl := console.LevelTrace
		return lvl, true
	case "debug":
		lvl := console.LevelDebug
		return lvl, true
	case "info", "":
		lvl := console.LevelInfo
		return lvl, true
	case "warn", "warning":
		lvl := console.LevelWarn
		return lvl, true
	case "error":
		lvl := console.LevelError
		return lvl, true
	case "fatal":
		lvl := console.LevelFatal
		return lvl, true
	default:
		return 0, false
	}
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
		if c.contentRepo != nil {
			c.contentRepo.swap(content.NewBunContentRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer))
		}
		if c.contentTypeRepo != nil {
			c.contentTypeRepo.swap(content.NewBunContentTypeRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer))
		}
		if c.localeRepo != nil {
			c.localeRepo.swap(content.NewBunLocaleRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer))
		}
		if c.pageRepo != nil {
			c.pageRepo.swap(pages.NewBunPageRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer))
		}
		if c.blockDefinitionRepo != nil {
			c.blockDefinitionRepo.swap(blocks.NewBunDefinitionRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer))
		}
		if c.blockRepo != nil {
			c.blockRepo.swap(blocks.NewBunInstanceRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer))
		}
		if c.blockTranslationRepo != nil {
			c.blockTranslationRepo.swap(blocks.NewBunTranslationRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer))
		}
		if c.blockVersionRepo != nil {
			c.blockVersionRepo.swap(blocks.NewBunInstanceVersionRepositoryWithCache(c.bunDB, c.cacheService, c.keySerializer))
		}

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

		return
	}

	if c.contentRepo != nil && c.memoryContentRepo != nil {
		c.contentRepo.swap(c.memoryContentRepo)
	}
	if c.contentTypeRepo != nil && c.memoryContentTypeRepo != nil {
		c.contentTypeRepo.swap(c.memoryContentTypeRepo)
	}
	if c.localeRepo != nil && c.memoryLocaleRepo != nil {
		c.localeRepo.swap(c.memoryLocaleRepo)
	}
	if c.pageRepo != nil && c.memoryPageRepo != nil {
		c.pageRepo.swap(c.memoryPageRepo)
	}
	if c.blockDefinitionRepo != nil && c.memoryBlockDefinitionRepo != nil {
		c.blockDefinitionRepo.swap(c.memoryBlockDefinitionRepo)
	}
	if c.blockRepo != nil && c.memoryBlockRepo != nil {
		c.blockRepo.swap(c.memoryBlockRepo)
	}
	if c.blockTranslationRepo != nil && c.memoryBlockTranslationRepo != nil {
		c.blockTranslationRepo.swap(c.memoryBlockTranslationRepo)
	}
	if c.blockVersionRepo != nil && c.memoryBlockVersionRepo != nil {
		c.blockVersionRepo.swap(c.memoryBlockVersionRepo)
	}

	if c.memoryMenuRepo != nil {
		c.menuRepo = c.memoryMenuRepo
	}
	if c.memoryMenuItemRepo != nil {
		c.menuItemRepo = c.memoryMenuItemRepo
	}
	if c.memoryMenuTranslationRepo != nil {
		c.menuTranslationRepo = c.memoryMenuTranslationRepo
	}
	if c.memoryWidgetDefinitionRepo != nil {
		c.widgetDefinitionRepo = c.memoryWidgetDefinitionRepo
	}
	if c.memoryWidgetInstanceRepo != nil {
		c.widgetInstanceRepo = c.memoryWidgetInstanceRepo
	}
	if c.memoryWidgetTranslationRepo != nil {
		c.widgetTranslationRepo = c.memoryWidgetTranslationRepo
	}
	if c.memoryWidgetAreaRepo != nil {
		c.widgetAreaRepo = c.memoryWidgetAreaRepo
	}
	if c.memoryWidgetPlacementRepo != nil {
		c.widgetPlacementRepo = c.memoryWidgetPlacementRepo
	}
	if c.memoryThemeRepo != nil {
		c.themeRepo = c.memoryThemeRepo
	}
	if c.memoryTemplateRepo != nil {
		c.templateRepo = c.memoryTemplateRepo
	}
}

func (c *Container) registerDefaultStorageFactories() {
	if c.storageFactories == nil {
		c.storageFactories = map[string]StorageFactory{}
	}
	if _, ok := c.storageFactories["bun"]; !ok {
		c.storageFactories["bun"] = c.bunStorageFactory
	}
}

func (c *Container) bunStorageFactory(ctx context.Context, profile storage.Profile) (StorageFactoryResult, error) {
	cfg := profile.Config
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	dsn := strings.TrimSpace(cfg.DSN)
	if driver == "" {
		return StorageFactoryResult{}, errors.New("di: storage profile driver required for bun provider")
	}
	if dsn == "" {
		return StorageFactoryResult{}, errors.New("di: storage profile DSN required for bun provider")
	}

	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return StorageFactoryResult{}, fmt.Errorf("di: open storage driver %s: %w", driver, err)
	}

	var bunDB *bun.DB
	switch driver {
	case "sqlite3", "sqlite":
		bunDB = bun.NewDB(sqlDB, sqlitedialect.New())
	case "postgres", "pgx", "pg":
		bunDB = bun.NewDB(sqlDB, pgdialect.New())
	default:
		_ = sqlDB.Close()
		return StorageFactoryResult{}, fmt.Errorf("di: unsupported bun driver %q", driver)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return StorageFactoryResult{}, fmt.Errorf("di: ping storage driver %s: %w", driver, err)
	}

	provider := storageadapter.NewBunStorageAdapter(sqlDB)

	return StorageFactoryResult{
		Provider: provider,
		BunDB:    bunDB,
		Closer: func(context.Context) error {
			return sqlDB.Close()
		},
	}, nil
}

func (c *Container) initializeStorage(ctx context.Context) error {
	c.registerDefaultStorageFactories()

	c.storageMu.Lock()
	if c.storageHandle != nil && c.storageHandle.provider != nil {
		c.storageMu.Unlock()
		c.configureRepositories()
		return nil
	}
	if len(c.storageProfiles) == 0 && c.storageRepo != nil {
		if stored, err := c.storageRepo.List(ctx); err == nil {
			for _, profile := range stored {
				cloned := cloneStorageProfile(profile)
				c.storageProfiles[strings.TrimSpace(cloned.Name)] = cloned
			}
		} else {
			c.storageLog().Error("storage.profile_list_failed", "error", err)
		}
	}
	if len(c.storageProfiles) == 0 && len(c.Config.Storage.Profiles) > 0 {
		for _, profile := range c.Config.Storage.Profiles {
			cloned := cloneStorageProfile(profile)
			if c.storageRepo != nil {
				if _, err := c.storageRepo.Upsert(ctx, cloned); err != nil {
					c.storageMu.Unlock()
					return fmt.Errorf("di: upsert storage profile %s: %w", cloned.Name, err)
				}
			}
			c.storageProfiles[strings.TrimSpace(cloned.Name)] = cloned
		}
	}

	activeProfile := strings.TrimSpace(c.activeProfile)
	if activeProfile == "" {
		for name, profile := range c.storageProfiles {
			if profile.Default {
				activeProfile = name
				break
			}
		}
	}
	if activeProfile == "" {
		if _, ok := c.storageProfiles[strings.TrimSpace(c.Config.Storage.Provider)]; ok {
			activeProfile = strings.TrimSpace(c.Config.Storage.Provider)
		}
	}
	if activeProfile == "" && len(c.storageProfiles) > 0 {
		for name := range c.storageProfiles {
			activeProfile = name
			break
		}
	}
	c.storageMu.Unlock()

	if profile, ok := c.storageProfiles[activeProfile]; ok {
		if err := c.activateStorageProfile(ctx, profile); err != nil {
			c.storageLog().Error("storage.profile_activate_failed", "profile", profile.Name, "error", err)
			return err
		}
		return nil
	}

	if c.bunDB != nil {
		c.storageMu.Lock()
		c.storage = storageadapter.NewBunStorageAdapter(c.bunDB.DB)
		c.storageHandle = &storageHandle{
			profile: storage.Profile{
				Name:     strings.TrimSpace(c.Config.Storage.Provider),
				Provider: "bun",
			},
			provider: c.storage,
			bunDB:    c.bunDB,
		}
		c.storageMu.Unlock()
		c.configureRepositories()
		return nil
	}

	c.configureRepositories()
	return nil
}

func (c *Container) storagePreviewer() adminstorage.PreviewFunc {
	if len(c.storageFactories) == 0 {
		return nil
	}
	return func(ctx context.Context, profile storage.Profile) (adminstorage.PreviewResult, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		cloned := cloneStorageProfile(profile)
		kind := strings.ToLower(strings.TrimSpace(cloned.Provider))
		if kind == "" {
			return adminstorage.PreviewResult{}, fmt.Errorf("di: storage preview provider required")
		}
		factory, ok := c.storageFactories[kind]
		if !ok {
			return adminstorage.PreviewResult{}, fmt.Errorf("di: no storage factory registered for provider %q", cloned.Provider)
		}
		result, err := factory(ctx, cloned)
		if err != nil {
			return adminstorage.PreviewResult{}, err
		}
		if result.Closer != nil {
			defer result.Closer(ctx)
		}
		diagnostics := map[string]any{
			"provider": cloned.Provider,
			"verified": true,
		}
		if driver := strings.TrimSpace(cloned.Config.Driver); driver != "" {
			diagnostics["driver"] = driver
		}
		if result.BunDB != nil {
			diagnostics["dialect"] = result.BunDB.Dialect().Name()
		}
		preview := adminstorage.PreviewResult{
			Profile:     cloneStorageProfile(cloned),
			Diagnostics: diagnostics,
		}
		if reporter, ok := any(result.Provider).(storage.CapabilityReporter); ok {
			preview.Capabilities = reporter.Capabilities()
		}
		if _, ok := any(result.Provider).(storage.Reloadable); ok && !preview.Capabilities.SupportsReload {
			preview.Capabilities.SupportsReload = true
		}
		return preview, nil
	}
}

func (c *Container) activateStorageProfile(ctx context.Context, profile storage.Profile) error {
	kind := strings.ToLower(strings.TrimSpace(profile.Provider))
	factory, ok := c.storageFactories[kind]
	if !ok {
		return fmt.Errorf("di: no storage factory registered for provider %q", profile.Provider)
	}

	result, err := factory(ctx, profile)
	if err != nil {
		return err
	}

	handle := &storageHandle{
		profile:  cloneStorageProfile(profile),
		provider: result.Provider,
		bunDB:    result.BunDB,
		closer:   result.Closer,
	}

	c.swapStorageHandle(ctx, handle)
	c.storageMu.Lock()
	c.activeProfile = handle.profile.Name
	c.storageMu.Unlock()
	c.storageLog().Info("storage.profile_activated", "profile", handle.profile.Name, "provider", handle.profile.Provider)
	return nil
}

func (c *Container) swapStorageHandle(ctx context.Context, handle *storageHandle) {
	c.storageMu.Lock()
	prev := c.storageHandle
	c.storageHandle = handle
	if handle != nil {
		c.storage = handle.provider
		c.bunDB = handle.bunDB
	} else {
		c.storage = storageadapter.NewNoOpProvider()
		c.bunDB = nil
	}
	c.storageMu.Unlock()

	c.configureRepositories()

	if prev != nil && prev != handle {
		prev.Close(ctx)
	}
}

func (c *Container) selectFallbackProfile(ctx context.Context) {
	c.storageMu.Lock()
	var fallback storage.Profile
	for _, profile := range c.storageProfiles {
		fallback = profile
		if profile.Default {
			break
		}
	}
	c.storageMu.Unlock()
	if fallback.Name == "" {
		c.swapStorageHandle(ctx, nil)
		c.storageLog().Warn("storage.profile_cleared")
		return
	}
	if err := c.activateStorageProfile(ctx, fallback); err != nil {
		c.storageLog().Error("storage.profile_activate_failed", "profile", fallback.Name, "error", err)
		c.swapStorageHandle(ctx, nil)
	}
}

func (c *Container) subscribeStorageEvents() {
	if c.storageRepo == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	events, err := c.storageRepo.Subscribe(ctx)
	if err != nil {
		cancel()
		c.storageLog().Error("storage.subscription_failed", "error", err)
		return
	}
	c.storageCancel = cancel
	go func() {
		for evt := range events {
			c.handleStorageEvent(ctx, evt)
		}
	}()
}

func (c *Container) handleStorageEvent(ctx context.Context, evt storageconfig.ChangeEvent) {
	name := strings.TrimSpace(evt.Profile.Name)
	switch evt.Type {
	case storageconfig.ChangeCreated, storageconfig.ChangeUpdated:
		if name == "" {
			return
		}
		cloned := cloneStorageProfile(evt.Profile)
		c.storageMu.Lock()
		c.storageProfiles[name] = cloned
		c.storageMu.Unlock()
		if cloned.Default || name == c.activeProfile {
			if err := c.activateStorageProfile(ctx, cloned); err != nil {
				c.storageLog().Error("storage.profile_activate_failed", "profile", cloned.Name, "error", err)
			}
		}
	case storageconfig.ChangeDeleted:
		c.storageMu.Lock()
		delete(c.storageProfiles, name)
		shouldFallback := name == c.activeProfile
		if shouldFallback {
			c.storageMu.Unlock()
			c.selectFallbackProfile(ctx)
			return
		}
		c.storageMu.Unlock()
	}
}

func cloneStorageProfile(profile storage.Profile) storage.Profile {
	cloned := profile
	if profile.Config.Options != nil {
		cloned.Config.Options = maps.Clone(profile.Config.Options)
	}
	if profile.Fallbacks != nil {
		cloned.Fallbacks = append([]string(nil), profile.Fallbacks...)
	}
	if profile.Labels != nil {
		cloned.Labels = maps.Clone(profile.Labels)
	}
	return cloned
}

func (c *Container) storageLog() interfaces.Logger {
	if c.storageLogger != nil {
		return c.storageLogger
	}
	return logging.NoOp()
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
	logger := logging.SchedulerLogger(c.loggerProvider)
	if !c.Config.Features.Scheduling {
		c.scheduler = cmsscheduler.NewNoOp()
		logger.Debug("scheduler.feature_disabled", "provider", "noop")
		return
	}
	if c.scheduler == nil {
		c.scheduler = cmsscheduler.NewInMemory()
		logger.Info("scheduler.configured", "provider", "in-memory")
	} else {
		logger.Debug("scheduler.provider_supplied", "provider", fmt.Sprintf("%T", c.scheduler))
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

func (c *Container) configureStorageAdminService() {
	if c.storageRepo == nil || c.storageAdminSvc != nil {
		return
	}
	options := []adminstorage.Option{}
	if previewer := c.storagePreviewer(); previewer != nil {
		options = append(options, adminstorage.WithPreviewer(previewer))
	}
	c.storageAdminSvc = adminstorage.NewService(c.storageRepo, c.auditRecorder, options...)
}

func (c *Container) configureWorkflowEngine() error {
	if !c.Config.Workflow.Enabled {
		c.workflowEngine = nil
		return nil
	}

	provider := strings.ToLower(strings.TrimSpace(c.Config.Workflow.Provider))
	switch provider {
	case "", "simple":
		if c.workflowEngine == nil {
			c.workflowEngine = workflowsimple.New()
		}
	case "custom":
		if c.workflowEngine == nil {
			return ErrWorkflowEngineNotProvided
		}
	default:
		if c.workflowEngine == nil {
			c.workflowEngine = workflowsimple.New()
		}
	}

	if c.workflowEngine == nil {
		return nil
	}

	ctx := context.Background()

	configuredDefinitions, err := workflow.CompileDefinitionConfigs(c.Config.Workflow.Definitions)
	if err != nil {
		return err
	}

	definitions := make([]interfaces.WorkflowDefinition, 0, len(configuredDefinitions))
	definitions = append(definitions, configuredDefinitions...)

	if c.workflowDefinitionStore != nil {
		storeDefinitions, err := c.workflowDefinitionStore.ListWorkflowDefinitions(ctx)
		if err != nil {
			return fmt.Errorf("load workflow definitions: %w", err)
		}
		if len(storeDefinitions) > 0 {
			definitions = append(definitions, storeDefinitions...)
		}
	}

	for _, definition := range definitions {
		if err := c.workflowEngine.RegisterWorkflow(ctx, definition); err != nil {
			return fmt.Errorf("register workflow definition %s: %w", definition.EntityType, err)
		}
	}

	return nil
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

// TranslationsEnabled reports whether translation handling is globally enabled.
func (c *Container) TranslationsEnabled() bool {
	if c == nil {
		return false
	}
	return c.Config.I18N.Enabled
}

// TranslationsRequired reports whether translations must be provided when enabled.
func (c *Container) TranslationsRequired() bool {
	if c == nil {
		return false
	}
	if !c.Config.I18N.Enabled {
		return false
	}
	return c.Config.I18N.RequireTranslations
}

// ContentService returns the configured content service.
func (c *Container) ContentService() content.Service {
	return c.contentSvc
}

// StorageAdminService exposes the storage admin helper.
func (c *Container) StorageAdminService() *adminstorage.Service {
	return c.storageAdminSvc
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
	return c.widgetSvc
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

// WorkflowEngine returns the configured workflow engine (may be nil when disabled).
func (c *Container) WorkflowEngine() interfaces.WorkflowEngine {
	return c.workflowEngine
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

	// Static generator commands.
	if c.generatorSvc != nil && c.Config.Generator.Enabled {
		gates := staticcmd.FeatureGates{
			GeneratorEnabled: func() bool { return c.Config.Generator.Enabled },
			SitemapEnabled:   func() bool { return c.Config.Generator.GenerateSitemap },
		}
		staticLogger := loggerFor("static")
		register(staticcmd.NewBuildSiteHandler(c.generatorSvc, staticLogger, gates))
		register(staticcmd.NewDiffSiteHandler(c.generatorSvc, staticLogger, gates))
		register(staticcmd.NewCleanSiteHandler(c.generatorSvc, staticLogger, gates))
		register(staticcmd.NewBuildSitemapHandler(c.generatorSvc, staticLogger, gates))
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
	if c.widgetSvc != nil && c.Config.Features.Widgets {
		gates := widgetscmd.FeatureGates{
			WidgetsEnabled: func() bool { return c.Config.Features.Widgets },
		}
		register(widgetscmd.NewSyncWidgetRegistryHandler(c.widgetSvc, loggerFor("widgets"), gates))
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

func applyConfiguredWidgetDefinitions(registry *widgets.Registry, definitions []runtimeconfig.WidgetDefinitionConfig) {
	if registry == nil || len(definitions) == 0 {
		return
	}
	for _, definition := range definitions {
		input := buildWidgetDefinitionInput(definition)
		if input.Name == "" || len(input.Schema) == 0 {
			continue
		}
		registry.Register(input)
	}
}

func buildWidgetDefinitionInput(definition runtimeconfig.WidgetDefinitionConfig) widgets.RegisterDefinitionInput {
	name := strings.TrimSpace(definition.Name)
	input := widgets.RegisterDefinitionInput{
		Name:     name,
		Schema:   definition.Schema,
		Defaults: definition.Defaults,
	}
	if description := strings.TrimSpace(definition.Description); description != "" {
		input.Description = strPtr(description)
	}
	if category := strings.TrimSpace(definition.Category); category != "" {
		input.Category = strPtr(category)
	}
	if icon := strings.TrimSpace(definition.Icon); icon != "" {
		input.Icon = strPtr(icon)
	}
	return input
}

func strPtr(value string) *string {
	return &value
}
