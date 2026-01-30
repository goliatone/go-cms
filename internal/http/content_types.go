package http

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/permissions"
	"github.com/goliatone/go-cms/internal/schema"
	"github.com/goliatone/go-cms/internal/validation"
	"github.com/goliatone/go-slug"
	"github.com/google/uuid"
)

type contentTypeCreatePayload struct {
	Name          string         `json:"name"`
	Slug          string         `json:"slug,omitempty"`
	Description   *string        `json:"description,omitempty"`
	Schema        map[string]any `json:"schema"`
	UISchema      map[string]any `json:"ui_schema,omitempty"`
	Capabilities  map[string]any `json:"capabilities,omitempty"`
	Icon          *string        `json:"icon,omitempty"`
	Status        string         `json:"status,omitempty"`
	Environment   string         `json:"environment,omitempty"`
	EnvironmentID *uuid.UUID     `json:"environment_id,omitempty"`
	CreatedBy     *uuid.UUID     `json:"created_by,omitempty"`
	UpdatedBy     *uuid.UUID     `json:"updated_by,omitempty"`
	ActorID       *uuid.UUID     `json:"actor_id,omitempty"`
}

type contentTypeUpdatePayload struct {
	Name                 *string        `json:"name,omitempty"`
	Slug                 *string        `json:"slug,omitempty"`
	Description          *string        `json:"description,omitempty"`
	Schema               map[string]any `json:"schema,omitempty"`
	UISchema             map[string]any `json:"ui_schema,omitempty"`
	Capabilities         map[string]any `json:"capabilities,omitempty"`
	Icon                 *string        `json:"icon,omitempty"`
	Status               *string        `json:"status,omitempty"`
	AllowBreakingChanges bool           `json:"allow_breaking_changes,omitempty"`
	Environment          *string        `json:"environment,omitempty"`
	EnvironmentID        *uuid.UUID     `json:"environment_id,omitempty"`
	UpdatedBy            *uuid.UUID     `json:"updated_by,omitempty"`
	ActorID              *uuid.UUID     `json:"actor_id,omitempty"`
}

type contentTypePublishPayload struct {
	AllowBreakingChanges bool       `json:"allow_breaking_changes,omitempty"`
	UpdatedBy            *uuid.UUID `json:"updated_by,omitempty"`
	ActorID              *uuid.UUID `json:"actor_id,omitempty"`
}

type contentTypeClonePayload struct {
	Name          *string        `json:"name,omitempty"`
	Slug          *string        `json:"slug,omitempty"`
	Description   *string        `json:"description,omitempty"`
	Schema        map[string]any `json:"schema,omitempty"`
	UISchema      map[string]any `json:"ui_schema,omitempty"`
	Capabilities  map[string]any `json:"capabilities,omitempty"`
	Icon          *string        `json:"icon,omitempty"`
	Status        *string        `json:"status,omitempty"`
	Environment   string         `json:"environment,omitempty"`
	EnvironmentID *uuid.UUID     `json:"environment_id,omitempty"`
	CreatedBy     *uuid.UUID     `json:"created_by,omitempty"`
	UpdatedBy     *uuid.UUID     `json:"updated_by,omitempty"`
	ActorID       *uuid.UUID     `json:"actor_id,omitempty"`
}

type contentTypeDeletePayload struct {
	HardDelete bool       `json:"hard_delete,omitempty"`
	DeletedBy  *uuid.UUID `json:"deleted_by,omitempty"`
	ActorID    *uuid.UUID `json:"actor_id,omitempty"`
}

type schemaValidatePayload struct {
	Schema            map[string]any `json:"schema"`
	FailOnUnsupported bool           `json:"fail_on_unsupported,omitempty"`
}

type schemaPreviewPayload struct {
	Schema            map[string]any           `json:"schema"`
	UISchema          map[string]any           `json:"ui_schema,omitempty"`
	Overlays          []schema.OverlayDocument `json:"overlays,omitempty"`
	Slug              string                   `json:"slug,omitempty"`
	FailOnUnsupported bool                     `json:"fail_on_unsupported,omitempty"`
}

type schemaPreviewResponse struct {
	Schema   map[string]any           `json:"schema"`
	UISchema map[string]any           `json:"ui_schema,omitempty"`
	Overlays []schema.OverlayDocument `json:"overlays,omitempty"`
	Version  string                   `json:"version"`
	Metadata schema.Metadata          `json:"metadata"`
}

type contentTypeResponse struct {
	*content.ContentType
	Permissions        permissions.PermissionSet `json:"permissions,omitempty"`
	ContentPermissions permissions.PermissionSet `json:"content_permissions,omitempty"`
}

func buildContentTypeResponse(record *content.ContentType) contentTypeResponse {
	if record == nil {
		return contentTypeResponse{}
	}
	return contentTypeResponse{
		ContentType:        record,
		Permissions:        permissions.BuilderContentTypePermissions(),
		ContentPermissions: permissions.ContentTypePermissions(record.Slug),
	}
}

func buildContentTypeResponses(records []*content.ContentType) []contentTypeResponse {
	if len(records) == 0 {
		return nil
	}
	out := make([]contentTypeResponse, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		out = append(out, buildContentTypeResponse(record))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (api *AdminAPI) registerContentTypeRoutes(mux *http.ServeMux, base string) {
	if mux == nil {
		return
	}
	root := joinPath(base, "content-types")
	mux.HandleFunc("GET "+root, api.handleContentTypeList)
	mux.HandleFunc("POST "+root, api.handleContentTypeCreate)
	mux.HandleFunc("GET "+root+"/{id}", api.handleContentTypeGet)
	mux.HandleFunc("PUT "+root+"/{id}", api.handleContentTypeUpdate)
	mux.HandleFunc("DELETE "+root+"/{id}", api.handleContentTypeDelete)
	mux.HandleFunc("POST "+root+"/{id}/publish", api.handleContentTypePublish)
	mux.HandleFunc("POST "+root+"/{id}/clone", api.handleContentTypeClone)
}

func (api *AdminAPI) registerSchemaRoutes(mux *http.ServeMux, base string) {
	if mux == nil {
		return
	}
	root := joinPath(base, "content-types")
	mux.HandleFunc("POST "+root+"/validate", api.handleSchemaValidate)
	mux.HandleFunc("POST "+root+"/preview", api.handleSchemaPreview)
	mux.HandleFunc("GET "+root+"/{id}/schema", api.handleSchemaExport)
	mux.HandleFunc("GET "+root+"/{id}/openapi", api.handleSchemaOpenAPI)
}

func (api *AdminAPI) handleContentTypeList(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.contentTypes == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		query = strings.TrimSpace(r.URL.Query().Get("search"))
	}
	var (
		list []*content.ContentType
		err  error
	)
	envKey, err := api.resolveEnvironmentKeyWithDefault(r, "", nil, false)
	if err != nil {
		writeError(w, err)
		return
	}
	if !requirePermissionWithEnv(w, r, permissions.ContentTypesRead, envKey) {
		return
	}
	if query == "" {
		list, err = api.contentTypes.List(r.Context(), envKey)
	} else {
		list, err = api.contentTypes.Search(r.Context(), query, envKey)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, buildContentTypeResponses(list))
}

func (api *AdminAPI) handleContentTypeGet(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.contentTypes == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	record, err := api.contentTypes.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	envKey, err := api.environmentKeyForID(r.Context(), record.EnvironmentID)
	if err != nil {
		writeError(w, err)
		return
	}
	if !requirePermissionWithEnv(w, r, permissions.ContentTypesRead, envKey) {
		return
	}
	writeJSON(w, http.StatusOK, buildContentTypeResponse(record))
}

func (api *AdminAPI) handleContentTypeCreate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.contentTypes == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	var payload contentTypeCreatePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	envKey, err := api.resolveEnvironmentKeyWithDefault(r, payload.Environment, payload.EnvironmentID, api.requireExplicit)
	if err != nil {
		writeError(w, err)
		return
	}
	if !requirePermissionWithEnv(w, r, permissions.ContentTypesCreate, envKey) {
		return
	}
	if strings.TrimSpace(payload.Slug) == "" {
		payload.Slug = content.DeriveContentTypeSlug(&content.ContentType{
			Name:   payload.Name,
			Schema: payload.Schema,
		})
	}
	if normalized, err := slug.Normalize(payload.Slug); err == nil && normalized != "" {
		payload.Slug = normalized
	}
	if payload.Schema != nil {
		normalized, err := ensureSchemaVersion(payload.Schema, payload.Slug)
		if err != nil {
			writeError(w, err)
			return
		}
		payload.Schema = normalized
	}
	actor := resolveActorID(payload.CreatedBy, payload.ActorID)
	updatedBy := resolveActorID(payload.UpdatedBy, payload.ActorID)
	if actor != uuid.Nil && updatedBy == uuid.Nil {
		updatedBy = actor
	}
	req := content.CreateContentTypeRequest{
		Name:           payload.Name,
		Slug:           payload.Slug,
		Description:    payload.Description,
		Schema:         payload.Schema,
		UISchema:       payload.UISchema,
		Capabilities:   payload.Capabilities,
		Icon:           payload.Icon,
		Status:         payload.Status,
		EnvironmentKey: envKey,
		CreatedBy:      actor,
		UpdatedBy:      updatedBy,
	}
	created, err := api.contentTypes.Create(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, buildContentTypeResponse(created))
}

func (api *AdminAPI) handleContentTypeUpdate(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.contentTypes == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload contentTypeUpdatePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
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
	}
	if !requirePermissionWithEnv(w, r, permissions.ContentTypesUpdate, envKey) {
		return
	}
	actor := resolveActorID(payload.UpdatedBy, payload.ActorID)
	req := content.UpdateContentTypeRequest{
		ID:                   id,
		Name:                 payload.Name,
		Slug:                 payload.Slug,
		Description:          payload.Description,
		Schema:               payload.Schema,
		UISchema:             payload.UISchema,
		Capabilities:         payload.Capabilities,
		Icon:                 payload.Icon,
		Status:               payload.Status,
		EnvironmentKey:       envKey,
		UpdatedBy:            actor,
		AllowBreakingChanges: payload.AllowBreakingChanges,
	}
	updated, err := api.contentTypes.Update(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, buildContentTypeResponse(updated))
}

func (api *AdminAPI) handleContentTypeDelete(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.contentTypes == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.ContentTypesDelete) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	var payload contentTypeDeletePayload
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
	req := content.DeleteContentTypeRequest{
		ID:         id,
		DeletedBy:  actor,
		HardDelete: hardDelete,
	}
	if err := api.contentTypes.Delete(r.Context(), req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (api *AdminAPI) handleContentTypePublish(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.contentTypes == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	record, err := api.contentTypes.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	envKey, err := api.environmentKeyForID(r.Context(), record.EnvironmentID)
	if err != nil {
		writeError(w, err)
		return
	}
	if !requirePermissionWithEnv(w, r, permissions.ContentTypesPublish, envKey) {
		return
	}
	var payload contentTypePublishPayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	actor := resolveActorID(payload.UpdatedBy, payload.ActorID)
	status := content.ContentTypeStatusActive
	req := content.UpdateContentTypeRequest{
		ID:                   id,
		Status:               &status,
		EnvironmentKey:       envKey,
		UpdatedBy:            actor,
		AllowBreakingChanges: payload.AllowBreakingChanges,
	}
	updated, err := api.contentTypes.Update(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, buildContentTypeResponse(updated))
}

func (api *AdminAPI) handleContentTypeClone(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.contentTypes == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	source, err := api.contentTypes.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	var payload contentTypeClonePayload
	if err := decodeJSON(r, &payload); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	envKey, err := api.resolveEnvironmentKeyWithDefault(r, payload.Environment, payload.EnvironmentID, api.requireExplicit)
	if err != nil {
		writeError(w, err)
		return
	}
	if !requirePermissionWithEnv(w, r, permissions.ContentTypesCreate, envKey) {
		return
	}

	name := source.Name
	if payload.Name != nil {
		name = *payload.Name
	}
	slugValue := source.Slug
	if payload.Slug != nil {
		slugValue = *payload.Slug
	}
	description := source.Description
	if payload.Description != nil {
		description = payload.Description
	}
	icon := source.Icon
	if payload.Icon != nil {
		icon = payload.Icon
	}
	schemaPayload := cloneMap(source.Schema)
	if payload.Schema != nil {
		schemaPayload = payload.Schema
	}
	uiSchema := cloneMap(source.UISchema)
	if payload.UISchema != nil {
		uiSchema = payload.UISchema
	}
	capabilities := cloneMap(source.Capabilities)
	if payload.Capabilities != nil {
		capabilities = payload.Capabilities
	}
	status := content.ContentTypeStatusDraft
	if payload.Status != nil {
		status = *payload.Status
	}

	if strings.TrimSpace(slugValue) == "" {
		slugValue = content.DeriveContentTypeSlug(&content.ContentType{
			Name:   name,
			Schema: schemaPayload,
		})
	}
	if normalized, err := slug.Normalize(slugValue); err == nil && normalized != "" {
		slugValue = normalized
	}
	if schemaPayload != nil {
		normalized, err := ensureSchemaVersion(schemaPayload, slugValue)
		if err != nil {
			writeError(w, err)
			return
		}
		schemaPayload = normalized
	}

	actor := resolveActorID(payload.CreatedBy, payload.ActorID)
	updatedBy := resolveActorID(payload.UpdatedBy, payload.ActorID)
	if actor != uuid.Nil && updatedBy == uuid.Nil {
		updatedBy = actor
	}
	if strings.TrimSpace(envKey) == "" {
		envKey, err = api.environmentKeyForID(r.Context(), source.EnvironmentID)
		if err != nil {
			writeError(w, err)
			return
		}
	}

	req := content.CreateContentTypeRequest{
		Name:           name,
		Slug:           slugValue,
		Description:    description,
		Schema:         schemaPayload,
		UISchema:       uiSchema,
		Capabilities:   capabilities,
		Icon:           icon,
		Status:         status,
		EnvironmentKey: envKey,
		CreatedBy:      actor,
		UpdatedBy:      updatedBy,
	}
	created, err := api.contentTypes.Create(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, buildContentTypeResponse(created))
}

func (api *AdminAPI) handleSchemaValidate(w http.ResponseWriter, r *http.Request) {
	if !requirePermission(w, r, permissions.ContentTypesRead) {
		return
	}
	var payload schemaValidatePayload
	if err := decodeJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	if payload.Schema == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "schema required"})
		return
	}
	if payload.FailOnUnsupported {
		if err := schema.ValidateSchemaSubset(payload.Schema); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{
				Error:   "validation_failed",
				Message: err.Error(),
				Issues:  validation.Issues(err),
			})
			return
		}
	}
	if err := validation.ValidateSchema(payload.Schema); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, errorResponse{
			Error:   "validation_failed",
			Message: err.Error(),
			Issues:  validation.Issues(err),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}

func (api *AdminAPI) handleSchemaPreview(w http.ResponseWriter, r *http.Request) {
	if !requirePermission(w, r, permissions.ContentTypesRead) {
		return
	}
	var payload schemaPreviewPayload
	if err := decodeJSON(r, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: err.Error()})
		return
	}
	if payload.Schema == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "schema required"})
		return
	}
	opts := schema.NormalizeOptions{
		Slug:              strings.TrimSpace(payload.Slug),
		OverlayResolver:   api.overlayResolver,
		OverlayDocuments:  payload.Overlays,
		FailOnUnsupported: payload.FailOnUnsupported,
	}
	preview, err := schema.BuildDeliveryPayload(r.Context(), payload.Schema, opts)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := schemaPreviewResponse{
		Schema:   preview.Schema,
		UISchema: payload.UISchema,
		Overlays: preview.Overlays,
		Version:  preview.Version,
		Metadata: preview.Metadata,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (api *AdminAPI) handleSchemaExport(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.contentTypes == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.ContentTypesRead) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	record, err := api.contentTypes.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record.Schema)
}

func (api *AdminAPI) handleSchemaOpenAPI(w http.ResponseWriter, r *http.Request) {
	if api == nil || api.contentTypes == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "service_unavailable"})
		return
	}
	if !requirePermission(w, r, permissions.ContentTypesRead) {
		return
	}
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "bad_request", Message: "invalid id"})
		return
	}
	record, err := api.contentTypes.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	version, err := resolveSchemaVersion(record.Schema, record.Slug, record.SchemaVersion)
	if err != nil {
		writeError(w, err)
		return
	}
	blockSchemas, err := api.collectBlockSchemas(r.Context(), record)
	if err != nil {
		writeError(w, err)
		return
	}
	projection, err := schema.ProjectToOpenAPI(record.Slug, record.Name, record.Schema, version, blockSchemas)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projection.Document.AsMap())
}

func resolveSchemaVersion(payload map[string]any, slug, stored string) (schema.Version, error) {
	trimmed := strings.TrimSpace(stored)
	if trimmed != "" {
		version, err := schema.ParseVersion(trimmed)
		if err != nil {
			return schema.Version{}, err
		}
		if normalized := strings.TrimSpace(slug); normalized != "" {
			version.Slug = normalized
		}
		return version, nil
	}
	_, version, err := schema.EnsureSchemaVersion(payload, slug)
	if err != nil {
		return schema.Version{}, err
	}
	return version, nil
}

func ensureSchemaVersion(payload map[string]any, slug string) (map[string]any, error) {
	if payload == nil {
		return nil, nil
	}
	cloned := cloneMap(payload)
	if metaRaw, ok := cloned["metadata"].(map[string]any); ok && metaRaw != nil {
		if versionRaw, ok := metaRaw["schema_version"].(string); ok && strings.TrimSpace(versionRaw) != "" {
			if parsed, err := schema.ParseVersion(versionRaw); err == nil {
				if trimmed := strings.TrimSpace(slug); trimmed != "" && parsed.Slug != trimmed {
					delete(metaRaw, "schema_version")
				}
			}
		}
		if trimmed := strings.TrimSpace(slug); trimmed != "" {
			metaRaw["slug"] = trimmed
		}
		cloned["metadata"] = metaRaw
	}
	normalized, _, err := schema.EnsureSchemaVersion(cloned, slug)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	copied := make(map[string]any, len(input))
	for key, value := range input {
		copied[key] = value
	}
	return copied
}
