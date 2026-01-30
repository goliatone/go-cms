package http

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
)

type contentTranslationPayload struct {
	Locale  string           `json:"locale"`
	Title   string           `json:"title"`
	Summary *string          `json:"summary,omitempty"`
	Content map[string]any   `json:"content,omitempty"`
	Blocks  []map[string]any `json:"blocks,omitempty"`
}

type contentCreatePayload struct {
	ContentTypeID            uuid.UUID                   `json:"content_type_id"`
	Slug                     string                      `json:"slug,omitempty"`
	Status                   string                      `json:"status,omitempty"`
	Environment              string                      `json:"environment,omitempty"`
	EnvironmentID            *uuid.UUID                  `json:"environment_id,omitempty"`
	Translations             []contentTranslationPayload `json:"translations"`
	AllowMissingTranslations bool                        `json:"allow_missing_translations,omitempty"`
	CreatedBy                *uuid.UUID                  `json:"created_by,omitempty"`
	UpdatedBy                *uuid.UUID                  `json:"updated_by,omitempty"`
	ActorID                  *uuid.UUID                  `json:"actor_id,omitempty"`
}

type contentUpdatePayload struct {
	Status                   string                      `json:"status,omitempty"`
	Environment              *string                     `json:"environment,omitempty"`
	EnvironmentID            *uuid.UUID                  `json:"environment_id,omitempty"`
	Translations             []contentTranslationPayload `json:"translations"`
	Metadata                 map[string]any              `json:"metadata,omitempty"`
	AllowMissingTranslations bool                        `json:"allow_missing_translations,omitempty"`
	UpdatedBy                *uuid.UUID                  `json:"updated_by,omitempty"`
	ActorID                  *uuid.UUID                  `json:"actor_id,omitempty"`
}

type contentDeletePayload struct {
	HardDelete bool       `json:"hard_delete,omitempty"`
	DeletedBy  *uuid.UUID `json:"deleted_by,omitempty"`
	ActorID    *uuid.UUID `json:"actor_id,omitempty"`
}

func (api *AdminAPI) registerContentRoutes(mux *http.ServeMux, base string) {
	if mux == nil {
		return
	}
	root := joinPath(base, "content")
	mux.HandleFunc("GET "+root, api.handleContentList)
	mux.HandleFunc("POST "+root, api.handleContentCreate)
	mux.HandleFunc("GET "+root+"/{id}", api.handleContentGet)
	mux.HandleFunc("PUT "+root+"/{id}", api.handleContentUpdate)
	mux.HandleFunc("DELETE "+root+"/{id}", api.handleContentDelete)
}

func (api *AdminAPI) handleContentList(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	envKey, err := api.resolveEnvironmentKey(r, "", nil)
	if err != nil {
		writeError(w, err)
		return
	}
	var (
		list []*content.Content
	)
	if strings.TrimSpace(envKey) == "" {
		list, err = api.content.List(r.Context())
	} else {
		list, err = api.content.List(r.Context(), envKey)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (api *AdminAPI) handleContentGet(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	record, err := api.content.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (api *AdminAPI) handleContentCreate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	var payload contentCreatePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	envKey := ""
	if strings.TrimSpace(payload.Environment) != "" || payload.EnvironmentID != nil || api.requireExplicit {
		resolved, err := api.resolveEnvironmentKeyWithDefault(r, payload.Environment, payload.EnvironmentID, api.requireExplicit)
		if err != nil {
			writeError(w, err)
			return
		}
		envKey = resolved
	}
	translations := make([]content.ContentTranslationInput, 0, len(payload.Translations))
	for _, tr := range payload.Translations {
		translations = append(translations, content.ContentTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Content: tr.Content,
			Blocks:  tr.Blocks,
		})
	}
	actor := resolveActorID(payload.CreatedBy, payload.ActorID)
	updatedBy := resolveActorID(payload.UpdatedBy, payload.ActorID)
	if actor != uuid.Nil && updatedBy == uuid.Nil {
		updatedBy = actor
	}
	req := content.CreateContentRequest{
		ContentTypeID:            payload.ContentTypeID,
		Slug:                     payload.Slug,
		Status:                   payload.Status,
		EnvironmentKey:           envKey,
		CreatedBy:                actor,
		UpdatedBy:                updatedBy,
		Translations:             translations,
		AllowMissingTranslations: payload.AllowMissingTranslations,
	}
	created, err := api.content.Create(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (api *AdminAPI) handleContentUpdate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload contentUpdatePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	envKey := ""
	if payload.Environment != nil || payload.EnvironmentID != nil || api.requireExplicit {
		keyVal := ""
		if payload.Environment != nil {
			keyVal = *payload.Environment
		}
		resolved, err := api.resolveEnvironmentKeyWithDefault(r, keyVal, payload.EnvironmentID, api.requireExplicit)
		if err != nil {
			writeError(w, err)
			return
		}
		envKey = resolved
	}
	translations := make([]content.ContentTranslationInput, 0, len(payload.Translations))
	for _, tr := range payload.Translations {
		translations = append(translations, content.ContentTranslationInput{
			Locale:  tr.Locale,
			Title:   tr.Title,
			Summary: tr.Summary,
			Content: tr.Content,
			Blocks:  tr.Blocks,
		})
	}
	actor := resolveActorID(payload.UpdatedBy, payload.ActorID)
	req := content.UpdateContentRequest{
		ID:                       id,
		Status:                   payload.Status,
		EnvironmentKey:           envKey,
		UpdatedBy:                actor,
		Translations:             translations,
		Metadata:                 payload.Metadata,
		AllowMissingTranslations: payload.AllowMissingTranslations,
	}
	updated, err := api.content.Update(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (api *AdminAPI) handleContentDelete(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.content == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload contentDeletePayload
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
	req := content.DeleteContentRequest{
		ID:         id,
		DeletedBy:  actor,
		HardDelete: hardDelete,
	}
	if err := api.content.Delete(r.Context(), req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
