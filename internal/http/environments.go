package http

import (
	"errors"
	"io"
	"net/http"

	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/permissions"
	"github.com/google/uuid"
)

type environmentCreatePayload struct {
	Key         string  `json:"key"`
	Name        string  `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
	IsDefault   bool    `json:"is_default,omitempty"`
}

type environmentUpdatePayload struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
	IsDefault   *bool   `json:"is_default,omitempty"`
}

func (api *AdminAPI) registerEnvironmentRoutes(mux *http.ServeMux, base string) {
	if mux == nil {
		return
	}
	root := joinPath(base, "environments")
	mux.HandleFunc("GET "+root, api.handleEnvironmentList)
	mux.HandleFunc("POST "+root, api.handleEnvironmentCreate)
	mux.HandleFunc("GET "+root+"/{id}", api.handleEnvironmentGet)
	mux.HandleFunc("PUT "+root+"/{id}", api.handleEnvironmentUpdate)
	mux.HandleFunc("DELETE "+root+"/{id}", api.handleEnvironmentDelete)
}

func (api *AdminAPI) handleEnvironmentList(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.environments == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.EnvironmentsRead) {
		return
	}
	if r == nil || r.URL == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "request missing"})
		return
	}
	activeOnly := parseBoolQuery(r.URL.Query().Get("active"), false)
	var (
		list []*cmsenv.Environment
		err  error
	)
	if activeOnly {
		list, err = api.environments.ListActiveEnvironments(r.Context())
	} else {
		list, err = api.environments.ListEnvironments(r.Context())
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (api *AdminAPI) handleEnvironmentCreate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.environments == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.EnvironmentsCreate) {
		return
	}
	var payload environmentCreatePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	req := cmsenv.CreateEnvironmentInput{
		Key:         payload.Key,
		Name:        payload.Name,
		Description: payload.Description,
		IsActive:    payload.IsActive,
		IsDefault:   payload.IsDefault,
	}
	created, err := api.environments.CreateEnvironment(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (api *AdminAPI) handleEnvironmentGet(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.environments == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.EnvironmentsRead) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	record, err := api.environments.GetEnvironment(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (api *AdminAPI) handleEnvironmentUpdate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.environments == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.EnvironmentsUpdate) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload environmentUpdatePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	req := cmsenv.UpdateEnvironmentInput{
		ID:          id,
		Name:        payload.Name,
		Description: payload.Description,
		IsActive:    payload.IsActive,
		IsDefault:   payload.IsDefault,
	}
	updated, err := api.environments.UpdateEnvironment(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (api *AdminAPI) handleEnvironmentDelete(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.environments == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.EnvironmentsDelete) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	if err := api.environments.DeleteEnvironment(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func envIDPtr(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
