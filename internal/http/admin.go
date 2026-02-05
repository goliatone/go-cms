package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/promotions"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/internal/schema"
)

// AdminAPI registers admin endpoints for content types, blocks, and environment-aware operations.
type AdminAPI struct {
	basePath        string
	contentTypes    content.ContentTypeService
	content         content.Service
	menus           menus.Service
	blocks          blocks.Service
	environments    cmsenv.Service
	promotions      promotions.Service
	overlayResolver schema.OverlayResolver
	defaultEnvKey   string
	requireExplicit bool
}

// AdminOption mutates the AdminAPI configuration.
type AdminOption func(*AdminAPI)

// NewAdminAPI constructs an AdminAPI instance.
func NewAdminAPI(opts ...AdminOption) *AdminAPI {
	api := &AdminAPI{
		basePath: "/admin/api",
	}
	for _, opt := range opts {
		if opt != nil {
			opt(api)
		}
	}
	return api
}

// WithBasePath overrides the base API path (defaults to "/admin/api").
func WithBasePath(path string) AdminOption {
	return func(api *AdminAPI) {
		if api == nil {
			return
		}
		if trimmed := strings.TrimSpace(path); trimmed != "" {
			api.basePath = trimmed
		}
	}
}

// WithEnvironmentConfig wires environment defaults and requirements for the admin API.
func WithEnvironmentConfig(cfg runtimeconfig.EnvironmentsConfig) AdminOption {
	return func(api *AdminAPI) {
		if api == nil {
			return
		}
		api.requireExplicit = cfg.RequireExplicit
		api.defaultEnvKey = strings.TrimSpace(cfg.DefaultKey)
		if api.defaultEnvKey == "" {
			api.defaultEnvKey = cmsenv.DefaultKey
		}
	}
}

// WithContentTypeService wires the content type service.
func WithContentTypeService(service content.ContentTypeService) AdminOption {
	return func(api *AdminAPI) {
		if api != nil {
			api.contentTypes = service
		}
	}
}

// WithBlockService wires the block service.
func WithBlockService(service blocks.Service) AdminOption {
	return func(api *AdminAPI) {
		if api != nil {
			api.blocks = service
		}
	}
}

// WithContentService wires the content entry service.
func WithContentService(service content.Service) AdminOption {
	return func(api *AdminAPI) {
		if api != nil {
			api.content = service
		}
	}
}

// WithMenuService wires the menu service.
func WithMenuService(service menus.Service) AdminOption {
	return func(api *AdminAPI) {
		if api != nil {
			api.menus = service
		}
	}
}

// WithEnvironmentService wires the environment service.
func WithEnvironmentService(service cmsenv.Service) AdminOption {
	return func(api *AdminAPI) {
		if api != nil {
			api.environments = service
		}
	}
}

// WithPromotionService wires the promotion orchestration service.
func WithPromotionService(service promotions.Service) AdminOption {
	return func(api *AdminAPI) {
		if api != nil {
			api.promotions = service
		}
	}
}

// WithOverlayResolver sets the overlay resolver used by schema preview/export utilities.
func WithOverlayResolver(resolver schema.OverlayResolver) AdminOption {
	return func(api *AdminAPI) {
		if api != nil {
			api.overlayResolver = resolver
		}
	}
}

// Register attaches the admin endpoints to the provided mux.
func (api *AdminAPI) Register(mux *http.ServeMux) error {
	if mux == nil {
		return fmt.Errorf("http: mux is required")
	}
	if api == nil {
		return fmt.Errorf("http: admin api is nil")
	}

	base := joinPath(api.basePath, "")

	api.registerEnvironmentRoutes(mux, base)
	api.registerContentTypeRoutes(mux, base)
	api.registerSchemaRoutes(mux, base)
	api.registerContentRoutes(mux, base)
	api.registerMenuRoutes(mux, base)
	api.registerBlockRoutes(mux, base)
	api.registerPromotionRoutes(mux, base)

	return nil
}
