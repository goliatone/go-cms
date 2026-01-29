// Package http provides optional HTTP adapters for CMS admin APIs.
//
// Routes follow the Content Type Builder design spec and mount under /admin/api:
//   - Content types: /content-types, /content-types/{id}, /content-types/{id}/publish, /content-types/{id}/clone
//   - Schema utilities: /content-types/validate, /content-types/preview,
//     /content-types/{id}/schema, /content-types/{id}/openapi
//   - Block library: /blocks, /blocks/{id}
//
// Host applications can register handlers on their own mux/router as needed.
package http
