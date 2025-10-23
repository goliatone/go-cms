package di

import (
	"strings"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/adapters/noop"
	"github.com/goliatone/go-cms/internal/adapters/storage"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/i18n"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/interfaces"
	repocache "github.com/goliatone/go-repository-cache/cache"
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
	localeRepo      content.LocaleRepository
	pageRepo        pages.PageRepository

	memoryLocaleRepo *content.MemoryLocaleRepository

	contentSvc content.Service
	pageSvc    pages.Service
	i18nSvc    i18n.Service
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

// WithI18nService overrides the default i18n service binding.
func WithI18nService(svc i18n.Service) Option {
	return func(c *Container) {
		c.i18nSvc = svc
	}
}

// NewContainer creates a container with the provided configuration.
func NewContainer(cfg cms.Config, opts ...Option) *Container {
	cacheTTL := cfg.Cache.DefaultTTL
	if cacheTTL <= 0 {
		cacheTTL = time.Minute
	}

	memoryContentRepo := content.NewMemoryContentRepository()
	memoryContentTypeRepo := content.NewMemoryContentTypeRepository()
	memoryLocaleRepo := content.NewMemoryLocaleRepository()
	memoryPageRepo := pages.NewMemoryPageRepository()

	c := &Container{
		Config:           cfg,
		storage:          storage.NewNoOpProvider(),
		template:         noop.Template(),
		media:            noop.Media(),
		auth:             noop.Auth(),
		cacheTTL:         cacheTTL,
		contentRepo:      memoryContentRepo,
		contentTypeRepo:  memoryContentTypeRepo,
		localeRepo:       memoryLocaleRepo,
		pageRepo:         memoryPageRepo,
		memoryLocaleRepo: memoryLocaleRepo,
	}

	c.seedLocales()

	for _, opt := range opts {
		opt(c)
	}

	c.configureCacheDefaults()
	c.configureRepositories()

	if c.contentSvc == nil {
		c.contentSvc = content.NewService(c.contentRepo, c.contentTypeRepo, c.localeRepo)
	}

	if c.pageSvc == nil {
		c.pageSvc = pages.NewService(c.pageRepo, c.contentRepo, c.localeRepo)
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
		c.memoryLocaleRepo = nil
	}
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
