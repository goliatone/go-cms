package di

import (
	"strings"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/adapters/noop"
	"github.com/goliatone/go-cms/internal/adapters/storage"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type Container struct {
	Config   cms.Config
	storage  interfaces.StorageProvider
	cache    interfaces.CacheProvider
	template interfaces.TemplateRenderer
	media    interfaces.MediaProvider
	auth     interfaces.AuthService

	contentRepo     *content.MemoryContentRepository
	contentTypeRepo *content.MemoryContentTypeRepository
	localeRepo      *content.MemoryLocaleRepository
	pagesRepo       *pages.MemoryPageRepository

	contentSvc content.Service
	pageSvc    pages.Service
}

type Option func(*Container)

func WithStorage(sp interfaces.StorageProvider) Option {
	return func(c *Container) {
		c.storage = sp
	}
}

func WithCache(cp interfaces.CacheProvider) Option {
	return func(c *Container) {
		c.cache = cp
	}
}

func WithTemplate(tr interfaces.TemplateRenderer) Option {
	return func(c *Container) {
		c.template = tr
	}
}

func WithMedia(mp interfaces.MediaProvider) Option {
	return func(c *Container) {
		c.media = mp
	}
}

func WithAuth(ap interfaces.AuthService) Option {
	return func(c *Container) {
		c.auth = ap
	}
}

func WithContentService(svc content.Service) Option {
	return func(c *Container) {
		c.contentSvc = svc
	}
}

func WithPageService(svc pages.Service) Option {
	return func(c *Container) {
		c.pageSvc = svc
	}
}

func NewContainer(cfg cms.Config, opts ...Option) *Container {
	c := &Container{
		Config:          cfg,
		storage:         storage.NewNoOpProvider(),
		cache:           noop.Cache(),
		template:        noop.Template(),
		media:           noop.Media(),
		auth:            noop.Auth(),
		contentRepo:     content.NewMemoryContentRepository(),
		contentTypeRepo: content.NewMemoryContentTypeRepository(),
		localeRepo:      content.NewMemoryLocaleRepository(),
		pagesRepo:       pages.NewMemoryPageRepository(),
	}

	c.seedLocales()

	for _, opt := range opts {
		opt(c)
	}

	if c.contentSvc == nil {
		c.contentSvc = content.NewService(c.contentRepo, c.contentTypeRepo, c.localeRepo)
	}

	if c.pageSvc == nil {
		c.pageSvc = pages.NewService(c.pagesRepo, c.contentRepo, c.localeRepo)
	}

	return c
}

func (c *Container) API() *cms.API {
	return cms.Module()
}

func (c *Container) StorageProvider() interfaces.StorageProvider {
	return c.storage
}

func (c *Container) CacheProvider() interfaces.CacheProvider {
	return c.cache
}

func (c *Container) TemplateRenderer() interfaces.TemplateRenderer {
	return c.template
}

func (c *Container) MediaProvider() interfaces.MediaProvider {
	return c.media
}

// AuthService exposes the configured auth service.
func (c *Container) AuthService() interfaces.AuthService {
	return c.auth
}

// ContentRepository exposes the in-memory content repository (scaffolding helper).
func (c *Container) ContentRepository() *content.MemoryContentRepository {
	return c.contentRepo
}

// LocaleRepository exposes the locale repository.
func (c *Container) LocaleRepository() *content.MemoryLocaleRepository {
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

func (c *Container) seedLocales() {
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
		c.localeRepo.Put(&content.Locale{
			ID:        uuid.New(),
			Code:      lower,
			Display:   normalized,
			IsActive:  true,
			IsDefault: strings.EqualFold(normalized, c.Config.DefaultLocale),
		})
	}
}
