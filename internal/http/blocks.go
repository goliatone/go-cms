package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/permissions"
	"github.com/goliatone/go-cms/internal/schema"
)

type blockCreatePayload struct {
	Name             string         `json:"name"`
	Description      *string        `json:"description,omitempty"`
	Icon             *string        `json:"icon,omitempty"`
	Schema           map[string]any `json:"schema"`
	Defaults         map[string]any `json:"defaults,omitempty"`
	EditorStyleURL   *string        `json:"editor_style_url,omitempty"`
	FrontendStyleURL *string        `json:"frontend_style_url,omitempty"`
}

type blockUpdatePayload struct {
	Name             *string        `json:"name,omitempty"`
	Description      *string        `json:"description,omitempty"`
	Icon             *string        `json:"icon,omitempty"`
	Schema           map[string]any `json:"schema,omitempty"`
	Defaults         map[string]any `json:"defaults,omitempty"`
	EditorStyleURL   *string        `json:"editor_style_url,omitempty"`
	FrontendStyleURL *string        `json:"frontend_style_url,omitempty"`
}

type blockDefinitionResponse struct {
	*blocks.Definition
	Permissions permissions.PermissionSet `json:"permissions,omitempty"`
}

func buildBlockDefinitionResponse(definition *blocks.Definition) blockDefinitionResponse {
	if definition == nil {
		return blockDefinitionResponse{}
	}
	return blockDefinitionResponse{
		Definition:  definition,
		Permissions: permissions.BlockLibraryPermissions(),
	}
}

func buildBlockDefinitionResponses(definitions []*blocks.Definition) []blockDefinitionResponse {
	if len(definitions) == 0 {
		return nil
	}
	out := make([]blockDefinitionResponse, 0, len(definitions))
	for _, definition := range definitions {
		if definition == nil {
			continue
		}
		out = append(out, buildBlockDefinitionResponse(definition))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (api *AdminAPI) registerBlockRoutes(mux *http.ServeMux, base string) {
	if mux == nil {
		return
	}
	root := joinPath(base, "blocks")
	mux.HandleFunc("GET "+root, api.handleBlockList)
	mux.HandleFunc("POST "+root, api.handleBlockCreate)
	mux.HandleFunc("GET "+root+"/{id}", api.handleBlockGet)
	mux.HandleFunc("PUT "+root+"/{id}", api.handleBlockUpdate)
	mux.HandleFunc("DELETE "+root+"/{id}", api.handleBlockDelete)
}

func (api *AdminAPI) handleBlockList(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.blocks == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.BlocksRead) {
		return
	}
	list, err := api.blocks.ListDefinitions(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, buildBlockDefinitionResponses(list))
}

func (api *AdminAPI) handleBlockGet(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.blocks == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.BlocksRead) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	record, err := api.blocks.GetDefinition(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, buildBlockDefinitionResponse(record))
}

func (api *AdminAPI) handleBlockCreate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.blocks == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.BlocksCreate) {
		return
	}
	var payload blockCreatePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	req := blocks.RegisterDefinitionInput{
		Name:             payload.Name,
		Description:      payload.Description,
		Icon:             payload.Icon,
		Schema:           payload.Schema,
		Defaults:         payload.Defaults,
		EditorStyleURL:   payload.EditorStyleURL,
		FrontendStyleURL: payload.FrontendStyleURL,
	}
	created, err := api.blocks.RegisterDefinition(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, buildBlockDefinitionResponse(created))
}

func (api *AdminAPI) handleBlockUpdate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.blocks == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.BlocksUpdate) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload blockUpdatePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	req := blocks.UpdateDefinitionInput{
		ID:               id,
		Name:             payload.Name,
		Description:      payload.Description,
		Icon:             payload.Icon,
		Schema:           payload.Schema,
		Defaults:         payload.Defaults,
		EditorStyleURL:   payload.EditorStyleURL,
		FrontendStyleURL: payload.FrontendStyleURL,
	}
	updated, err := api.blocks.UpdateDefinition(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, buildBlockDefinitionResponse(updated))
}

func (api *AdminAPI) handleBlockDelete(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.blocks == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.BlocksDelete) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	hardDelete := parseBoolQuery(r.URL.Query().Get("hard_delete"), true)
	req := blocks.DeleteDefinitionRequest{
		ID:         id,
		HardDelete: hardDelete,
	}
	if err := api.blocks.DeleteDefinition(r.Context(), req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (api *AdminAPI) collectBlockSchemas(ctx context.Context, record *content.ContentType) ([]schema.BlockSchema, error) {
	if api == nil || api.blocks == nil {
		return nil, nil
	}
	definitions, err := api.blocks.ListDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	meta := schema.ExtractMetadata(record.Schema)
	out := make([]schema.BlockSchema, 0, len(definitions))
	for _, def := range definitions {
		if def == nil || def.Schema == nil {
			continue
		}
		name := strings.TrimSpace(def.Name)
		if name == "" {
			continue
		}
		if !meta.BlockAvailability.Empty() && !meta.BlockAvailability.Allows(name) {
			continue
		}
		out = append(out, schema.BlockSchema{
			Name:   name,
			Schema: def.Schema,
		})
	}
	return out, nil
}
