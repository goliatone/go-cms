// Package http provides optional HTTP adapters for CMS admin APIs.
//
// Routes follow the Content Type Builder design spec and mount under /admin/api:
//   - Environments: /environments, /environments/{id}
//   - Content types: /content-types, /content-types/{id}, /content-types/{id}/publish, /content-types/{id}/clone
//   - Schema utilities: /content-types/validate, /content-types/preview,
//     /content-types/{id}/schema, /content-types/{id}/openapi
//   - Content entries: /content, /content/{id}
//   - Pages: /pages, /pages/{id}
//   - Menus: /menus, /menus/{id}
//   - Block library: /blocks, /blocks/{id}
//   - Promotions: /environments/{source}/promote/{target},
//     /content-types/{id}/promote, /content/{id}/promote
//
// Host applications can register handlers on their own mux/router as needed.
package http
