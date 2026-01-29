package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/schema"
)

// AdminAPI registers admin endpoints for content types and blocks.
type AdminAPI struct {
	basePath        string
	contentTypes    content.ContentTypeService
	blocks          blocks.Service
	overlayResolver schema.OverlayResolver
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

	api.registerContentTypeRoutes(mux, base)
	api.registerSchemaRoutes(mux, base)
	api.registerBlockRoutes(mux, base)

	return nil
}
