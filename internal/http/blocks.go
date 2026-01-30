package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/permissions"
	"github.com/goliatone/go-cms/internal/schema"
	"github.com/google/uuid"
)

type blockCreatePayload struct {
	Name             string         `json:"name"`
	Slug             string         `json:"slug,omitempty"`
	Description      *string        `json:"description,omitempty"`
	Icon             *string        `json:"icon,omitempty"`
	Category         *string        `json:"category,omitempty"`
	Status           *string        `json:"status,omitempty"`
	Schema           map[string]any `json:"schema"`
	UISchema         map[string]any `json:"ui_schema,omitempty"`
	Defaults         map[string]any `json:"defaults,omitempty"`
	EditorStyleURL   *string        `json:"editor_style_url,omitempty"`
	FrontendStyleURL *string        `json:"frontend_style_url,omitempty"`
	Environment      string         `json:"environment,omitempty"`
	EnvironmentID    *uuid.UUID     `json:"environment_id,omitempty"`
}

type blockUpdatePayload struct {
	Name             *string        `json:"name,omitempty"`
	Slug             *string        `json:"slug,omitempty"`
	Description      *string        `json:"description,omitempty"`
	Icon             *string        `json:"icon,omitempty"`
	Category         *string        `json:"category,omitempty"`
	Status           *string        `json:"status,omitempty"`
	Schema           map[string]any `json:"schema,omitempty"`
	UISchema         map[string]any `json:"ui_schema,omitempty"`
	Defaults         map[string]any `json:"defaults,omitempty"`
	EditorStyleURL   *string        `json:"editor_style_url,omitempty"`
	FrontendStyleURL *string        `json:"frontend_style_url,omitempty"`
	Environment      *string        `json:"environment,omitempty"`
	EnvironmentID    *uuid.UUID     `json:"environment_id,omitempty"`
}

type blockDefinitionResponse struct {
	*blocks.Definition
	Permissions permissions.PermissionSet `json:"permissions,omitempty"`
}

type blockDefinitionVersionResponse struct {
	Version         string         `json:"version"`
	Schema          map[string]any `json:"schema"`
	Defaults        map[string]any `json:"defaults,omitempty"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at,omitempty"`
	MigrationStatus string         `json:"migration_status,omitempty"`
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
	mux.HandleFunc("GET "+root+"/{id}/versions", api.handleBlockDefinitionVersions)
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
	envKey, err := api.resolveEnvironmentKey(r, "", nil)
	if err != nil {
		writeError(w, err)
		return
	}
	var list []*blocks.Definition
	if strings.TrimSpace(envKey) == "" {
		list, err = api.blocks.ListDefinitions(r.Context())
	} else {
		list, err = api.blocks.ListDefinitions(r.Context(), envKey)
	}
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

func (api *AdminAPI) handleBlockDefinitionVersions(w http.ResponseWriter, r *http.Request) {
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
	versions, err := api.blocks.ListDefinitionVersions(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]blockDefinitionVersionResponse, 0, len(versions))
	for _, version := range versions {
		if version == nil {
			continue
		}
		createdAt := version.CreatedAt
		if createdAt.IsZero() {
			createdAt = version.UpdatedAt
		}
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		updatedAt := ""
		if !version.UpdatedAt.IsZero() {
			updatedAt = version.UpdatedAt.UTC().Format(time.RFC3339)
		}
		resp = append(resp, blockDefinitionVersionResponse{
			Version:         strings.TrimSpace(version.SchemaVersion),
			Schema:          version.Schema,
			Defaults:        version.Defaults,
			CreatedAt:       createdAt.UTC().Format(time.RFC3339),
			UpdatedAt:       updatedAt,
			MigrationStatus: blocks.ResolveDefinitionMigrationStatus(version.Schema, version.SchemaVersion),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": resp})
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
	envKey, err := api.resolveEnvironmentKey(r, payload.Environment, payload.EnvironmentID)
	if err != nil {
		writeError(w, err)
		return
	}
	status := ""
	if payload.Status != nil {
		status = strings.TrimSpace(*payload.Status)
	}
	req := blocks.RegisterDefinitionInput{
		Name:             payload.Name,
		Slug:             strings.TrimSpace(payload.Slug),
		Description:      payload.Description,
		Icon:             payload.Icon,
		Category:         payload.Category,
		Status:           status,
		Schema:           payload.Schema,
		UISchema:         payload.UISchema,
		Defaults:         payload.Defaults,
		EditorStyleURL:   payload.EditorStyleURL,
		FrontendStyleURL: payload.FrontendStyleURL,
		EnvironmentKey:   envKey,
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
	var envKey *string
	if payload.Environment != nil || payload.EnvironmentID != nil {
		keyVal := ""
		if payload.Environment != nil {
			keyVal = *payload.Environment
		}
		resolved, err := api.resolveEnvironmentKey(r, keyVal, payload.EnvironmentID)
		if err != nil {
			writeError(w, err)
			return
		}
		if strings.TrimSpace(resolved) != "" {
			envKey = &resolved
		}
	}
	req := blocks.UpdateDefinitionInput{
		ID:               id,
		Name:             payload.Name,
		Slug:             payload.Slug,
		Description:      payload.Description,
		Icon:             payload.Icon,
		Category:         payload.Category,
		Status:           payload.Status,
		Schema:           payload.Schema,
		UISchema:         payload.UISchema,
		Defaults:         payload.Defaults,
		EditorStyleURL:   payload.EditorStyleURL,
		FrontendStyleURL: payload.FrontendStyleURL,
		EnvironmentKey:   envKey,
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
	var (
		definitions []*blocks.Definition
		err         error
	)
	if record != nil && record.EnvironmentID != uuid.Nil {
		envKey, resolveErr := api.environmentKeyForID(ctx, record.EnvironmentID)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if strings.TrimSpace(envKey) != "" {
			definitions, err = api.blocks.ListDefinitions(ctx, envKey)
		} else {
			definitions, err = api.blocks.ListDefinitions(ctx)
		}
	} else {
		definitions, err = api.blocks.ListDefinitions(ctx)
	}
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
