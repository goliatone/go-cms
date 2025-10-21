package di

import (
	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

type Container struct {
	Config   cms.Config
	storage  interfaces.StorageProvider
	cache    interfaces.CacheProvider
	template interfaces.TemplateRenderer
	media    interfaces.MediaProvider
	auth     interfaces.AuthProvider
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

func WithAuth(ap interfaces.AuthProvider) Option {
	return func(c *Container) {
		c.auth = ap
	}
}

func NewContainer(cfg cms.Config, opts ...Option) *Container {
	c := &Container{Config: cfg}

	for _, opt := range opts {
		opt(c)
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

func (c *Container) AuthProvider() interfaces.AuthProvider {
	return c.auth
}
