package http

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/goliatone/go-cms/internal/promotions"
	"github.com/google/uuid"
)

type promoteEnvironmentPayload struct {
	Scope            promotions.PromoteScope   `json:"scope,omitempty"`
	ContentTypeIDs   []uuid.UUID               `json:"content_type_ids,omitempty"`
	ContentTypeSlugs []string                  `json:"content_type_slugs,omitempty"`
	ContentIDs       []uuid.UUID               `json:"content_ids,omitempty"`
	ContentSlugs     []string                  `json:"content_slugs,omitempty"`
	Options          promotions.PromoteOptions `json:"options,omitempty"`
}

type promoteContentTypePayload struct {
	TargetEnvironment   string                    `json:"target_environment,omitempty"`
	TargetEnvironmentID *uuid.UUID                `json:"target_environment_id,omitempty"`
	Options             promotions.PromoteOptions `json:"options,omitempty"`
}

type promoteContentPayload struct {
	TargetEnvironment   string                    `json:"target_environment,omitempty"`
	TargetEnvironmentID *uuid.UUID                `json:"target_environment_id,omitempty"`
	Options             promotions.PromoteOptions `json:"options,omitempty"`
}

func (api *AdminAPI) registerPromotionRoutes(mux *http.ServeMux, base string) {
	if mux == nil {
		return
	}
	envRoot := joinPath(base, "environments")
	mux.HandleFunc("POST "+envRoot+"/{source}/promote/{target}", api.handlePromoteEnvironment)
	contentTypeRoot := joinPath(base, "content-types")
	mux.HandleFunc("POST "+contentTypeRoot+"/{id}/promote", api.handlePromoteContentType)
	contentRoot := joinPath(base, "content")
	mux.HandleFunc("POST "+contentRoot+"/{id}/promote", api.handlePromoteContentEntry)
}

func (api *AdminAPI) handlePromoteEnvironment(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.promotions == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	source := strings.TrimSpace(r.PathValue("source"))
	target := strings.TrimSpace(r.PathValue("target"))
	if source == "" || target == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "source and target required"})
		return
	}
	var payload promoteEnvironmentPayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	req := promotions.PromoteEnvironmentRequest{
		SourceEnvironment: source,
		TargetEnvironment: target,
		Scope:             payload.Scope,
		ContentTypeIDs:    payload.ContentTypeIDs,
		ContentTypeSlugs:  payload.ContentTypeSlugs,
		ContentIDs:        payload.ContentIDs,
		ContentSlugs:      payload.ContentSlugs,
		Options:           payload.Options,
	}
	result, err := api.promotions.PromoteEnvironment(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *AdminAPI) handlePromoteContentType(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.promotions == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload promoteContentTypePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	targetEnv := strings.TrimSpace(r.URL.Query().Get("to"))
	if targetEnv == "" {
		key, err := api.resolveEnvironmentKey(r, payload.TargetEnvironment, payload.TargetEnvironmentID)
		if err != nil {
			writeError(w, err)
			return
		}
		targetEnv = key
	}
	if strings.TrimSpace(targetEnv) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "target_environment required"})
		return
	}
	req := promotions.PromoteContentTypeRequest{
		ContentTypeID:     id,
		TargetEnvironment: targetEnv,
		Options:           payload.Options,
	}
	result, err := api.promotions.PromoteContentType(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (api *AdminAPI) handlePromoteContentEntry(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.promotions == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload promoteContentPayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	targetEnv := strings.TrimSpace(r.URL.Query().Get("to"))
	if targetEnv == "" {
		key, err := api.resolveEnvironmentKey(r, payload.TargetEnvironment, payload.TargetEnvironmentID)
		if err != nil {
			writeError(w, err)
			return
		}
		targetEnv = key
	}
	if strings.TrimSpace(targetEnv) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "target_environment required"})
		return
	}
	req := promotions.PromoteContentEntryRequest{
		ContentID:         id,
		TargetEnvironment: targetEnv,
		Options:           payload.Options,
	}
	result, err := api.promotions.PromoteContentEntry(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
