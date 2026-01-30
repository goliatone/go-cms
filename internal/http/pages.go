package http

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

type pageTranslationPayload struct {
	Locale        string           `json:"locale"`
	Title         string           `json:"title"`
	Path          string           `json:"path"`
	Summary       *string          `json:"summary,omitempty"`
	MediaBindings media.BindingSet `json:"media_bindings,omitempty"`
}

type pageCreatePayload struct {
	ContentID                uuid.UUID                `json:"content_id"`
	TemplateID               uuid.UUID                `json:"template_id"`
	ParentID                 *uuid.UUID               `json:"parent_id,omitempty"`
	Slug                     string                   `json:"slug,omitempty"`
	Status                   string                   `json:"status,omitempty"`
	Translations             []pageTranslationPayload `json:"translations"`
	AllowMissingTranslations bool                     `json:"allow_missing_translations,omitempty"`
	CreatedBy                *uuid.UUID               `json:"created_by,omitempty"`
	UpdatedBy                *uuid.UUID               `json:"updated_by,omitempty"`
	ActorID                  *uuid.UUID               `json:"actor_id,omitempty"`
}

type pageUpdatePayload struct {
	TemplateID               *uuid.UUID               `json:"template_id,omitempty"`
	Status                   string                   `json:"status,omitempty"`
	Translations             []pageTranslationPayload `json:"translations"`
	AllowMissingTranslations bool                     `json:"allow_missing_translations,omitempty"`
	UpdatedBy                *uuid.UUID               `json:"updated_by,omitempty"`
	ActorID                  *uuid.UUID               `json:"actor_id,omitempty"`
}

type pageDeletePayload struct {
	HardDelete bool       `json:"hard_delete,omitempty"`
	DeletedBy  *uuid.UUID `json:"deleted_by,omitempty"`
	ActorID    *uuid.UUID `json:"actor_id,omitempty"`
}

func (api *AdminAPI) registerPageRoutes(mux *http.ServeMux, base string) {
	if mux == nil {
		return
	}
	root := joinPath(base, "pages")
	mux.HandleFunc("GET "+root, api.handlePageList)
	mux.HandleFunc("POST "+root, api.handlePageCreate)
	mux.HandleFunc("GET "+root+"/{id}", api.handlePageGet)
	mux.HandleFunc("PUT "+root+"/{id}", api.handlePageUpdate)
	mux.HandleFunc("DELETE "+root+"/{id}", api.handlePageDelete)
}

func (api *AdminAPI) handlePageList(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.pages == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	envKey, err := api.resolveEnvironmentKey(r, "", nil)
	if err != nil {
		writeError(w, err)
		return
	}
	var list []*pages.Page
	if strings.TrimSpace(envKey) == "" {
		list, err = api.pages.List(r.Context())
	} else {
		list, err = api.pages.List(r.Context(), envKey)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (api *AdminAPI) handlePageGet(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.pages == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	record, err := api.pages.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (api *AdminAPI) handlePageCreate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.pages == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	var payload pageCreatePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	translations := make([]pages.PageTranslationInput, 0, len(payload.Translations))
	for _, tr := range payload.Translations {
		translations = append(translations, pages.PageTranslationInput{
			Locale:        tr.Locale,
			Title:         tr.Title,
			Path:          tr.Path,
			Summary:       tr.Summary,
			MediaBindings: tr.MediaBindings,
		})
	}
	actor := resolveActorID(payload.CreatedBy, payload.ActorID)
	updatedBy := resolveActorID(payload.UpdatedBy, payload.ActorID)
	if actor != uuid.Nil && updatedBy == uuid.Nil {
		updatedBy = actor
	}
	req := pages.CreatePageRequest{
		ContentID:                payload.ContentID,
		TemplateID:               payload.TemplateID,
		ParentID:                 payload.ParentID,
		Slug:                     payload.Slug,
		Status:                   payload.Status,
		CreatedBy:                actor,
		UpdatedBy:                updatedBy,
		Translations:             translations,
		AllowMissingTranslations: payload.AllowMissingTranslations,
	}
	created, err := api.pages.Create(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (api *AdminAPI) handlePageUpdate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.pages == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload pageUpdatePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	translations := make([]pages.PageTranslationInput, 0, len(payload.Translations))
	for _, tr := range payload.Translations {
		translations = append(translations, pages.PageTranslationInput{
			Locale:        tr.Locale,
			Title:         tr.Title,
			Path:          tr.Path,
			Summary:       tr.Summary,
			MediaBindings: tr.MediaBindings,
		})
	}
	actor := resolveActorID(payload.UpdatedBy, payload.ActorID)
	req := pages.UpdatePageRequest{
		ID:                       id,
		TemplateID:               payload.TemplateID,
		Status:                   payload.Status,
		UpdatedBy:                actor,
		Translations:             translations,
		AllowMissingTranslations: payload.AllowMissingTranslations,
	}
	updated, err := api.pages.Update(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (api *AdminAPI) handlePageDelete(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.pages == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload pageDeletePayload
	decodeErr := decodeJSON(r, &payload)
	if decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: decodeErr.Error()})
		return
	}
	hardDelete := payload.HardDelete
	if !hardDelete {
		hardDelete = parseBoolQuery(r.URL.Query().Get("hard_delete"), false)
	}
	actor := resolveActorID(payload.DeletedBy, payload.ActorID)
	req := pages.DeletePageRequest{
		ID:         id,
		DeletedBy:  actor,
		HardDelete: hardDelete,
	}
	if err := api.pages.Delete(r.Context(), req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
