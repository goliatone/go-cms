package http

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/permissions"
	"github.com/google/uuid"
)

type menuCreatePayload struct {
	Code          string     `json:"code"`
	Location      string     `json:"location,omitempty"`
	Description   *string    `json:"description,omitempty"`
	Environment   string     `json:"environment,omitempty"`
	EnvironmentID *uuid.UUID `json:"environment_id,omitempty"`
	CreatedBy     *uuid.UUID `json:"created_by,omitempty"`
	UpdatedBy     *uuid.UUID `json:"updated_by,omitempty"`
	ActorID       *uuid.UUID `json:"actor_id,omitempty"`
}

type menuUpdatePayload struct {
	Location      *string    `json:"location,omitempty"`
	Description   *string    `json:"description,omitempty"`
	Environment   *string    `json:"environment,omitempty"`
	EnvironmentID *uuid.UUID `json:"environment_id,omitempty"`
	UpdatedBy     *uuid.UUID `json:"updated_by,omitempty"`
	ActorID       *uuid.UUID `json:"actor_id,omitempty"`
}

type menuDeletePayload struct {
	Force     bool       `json:"force,omitempty"`
	DeletedBy *uuid.UUID `json:"deleted_by,omitempty"`
	ActorID   *uuid.UUID `json:"actor_id,omitempty"`
}

func (api *AdminAPI) registerMenuRoutes(mux *http.ServeMux, base string) {
	if mux == nil {
		return
	}
	root := joinPath(base, "menus")
	mux.HandleFunc("GET "+root, api.handleMenuList)
	mux.HandleFunc("POST "+root, api.handleMenuCreate)
	mux.HandleFunc("GET "+root+"/{id}", api.handleMenuGet)
	mux.HandleFunc("PUT "+root+"/{id}", api.handleMenuUpdate)
	mux.HandleFunc("DELETE "+root+"/{id}", api.handleMenuDelete)
}

func (api *AdminAPI) handleMenuList(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.menus == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if r == nil || r.URL == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "request missing"})
		return
	}
	query := r.URL.Query()
	code := strings.TrimSpace(query.Get("code"))
	location := strings.TrimSpace(query.Get("location"))
	if code == "" && location == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "code or location required"})
		return
	}
	envKey, err := api.resolveEnvironmentKeyWithDefault(r, "", nil, false)
	if err != nil {
		writeError(w, err)
		return
	}
	if !requirePermissionWithEnv(w, r, permissions.MenusRead, envKey) {
		return
	}
	var record *menus.Menu
	if code != "" {
		if strings.TrimSpace(envKey) == "" {
			record, err = api.menus.GetMenuByCode(r.Context(), code)
		} else {
			record, err = api.menus.GetMenuByCode(r.Context(), code, envKey)
		}
	} else {
		if strings.TrimSpace(envKey) == "" {
			record, err = api.menus.GetMenuByLocation(r.Context(), location)
		} else {
			record, err = api.menus.GetMenuByLocation(r.Context(), location, envKey)
		}
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (api *AdminAPI) handleMenuGet(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.menus == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	record, err := api.menus.GetMenu(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	envKey, err := api.environmentKeyForID(r.Context(), record.EnvironmentID)
	if err != nil {
		writeError(w, err)
		return
	}
	if !requirePermissionWithEnv(w, r, permissions.MenusRead, envKey) {
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (api *AdminAPI) handleMenuCreate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.menus == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	var payload menuCreatePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	envKey, err := api.resolveEnvironmentKeyWithDefault(r, payload.Environment, payload.EnvironmentID, api.requireExplicit)
	if err != nil {
		writeError(w, err)
		return
	}
	if !requirePermissionWithEnv(w, r, permissions.MenusCreate, envKey) {
		return
	}
	actor := resolveActorID(payload.CreatedBy, payload.ActorID)
	updatedBy := resolveActorID(payload.UpdatedBy, payload.ActorID)
	if actor != uuid.Nil && updatedBy == uuid.Nil {
		updatedBy = actor
	}
	req := menus.CreateMenuInput{
		Code:           payload.Code,
		Location:       payload.Location,
		Description:    payload.Description,
		CreatedBy:      actor,
		UpdatedBy:      updatedBy,
		EnvironmentKey: envKey,
	}
	created, err := api.menus.CreateMenu(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (api *AdminAPI) handleMenuUpdate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.menus == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload menuUpdatePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	existing, err := api.menus.GetMenu(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	location := existing.Location
	if payload.Location != nil {
		location = *payload.Location
	}
	description := existing.Description
	if payload.Description != nil {
		description = payload.Description
	}
	var envKey string
	if payload.Environment != nil || payload.EnvironmentID != nil || api.requireExplicit {
		keyVal := ""
		if payload.Environment != nil {
			keyVal = *payload.Environment
		}
		envKey, err = api.resolveEnvironmentKeyWithDefault(r, keyVal, payload.EnvironmentID, api.requireExplicit)
		if err != nil {
			writeError(w, err)
			return
		}
	} else {
		envKey, err = api.environmentKeyForID(r.Context(), existing.EnvironmentID)
		if err != nil {
			writeError(w, err)
			return
		}
	}
	if !requirePermissionWithEnv(w, r, permissions.MenusUpdate, envKey) {
		return
	}
	actor := resolveActorID(payload.UpdatedBy, payload.ActorID)
	req := menus.UpsertMenuInput{
		Code:           existing.Code,
		Location:       location,
		Description:    description,
		Actor:          actor,
		EnvironmentKey: envKey,
	}
	updated, err := api.menus.UpsertMenu(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (api *AdminAPI) handleMenuDelete(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.menus == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload menuDeletePayload
	decodeErr := decodeJSON(r, &payload)
	if decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: decodeErr.Error()})
		return
	}
	existing, err := api.menus.GetMenu(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	envKey, err := api.environmentKeyForID(r.Context(), existing.EnvironmentID)
	if err != nil {
		writeError(w, err)
		return
	}
	if !requirePermissionWithEnv(w, r, permissions.MenusDelete, envKey) {
		return
	}
	force := payload.Force
	if !force {
		force = parseBoolQuery(r.URL.Query().Get("force"), false)
	}
	actor := resolveActorID(payload.DeletedBy, payload.ActorID)
	req := menus.DeleteMenuRequest{
		MenuID:    id,
		DeletedBy: actor,
		Force:     force,
	}
	if err := api.menus.DeleteMenu(r.Context(), req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
