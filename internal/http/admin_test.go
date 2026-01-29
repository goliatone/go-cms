package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
)

func TestAdminAPI_ContentTypeLifecycle(t *testing.T) {
	mux, _ := setupAdminAPI(t)

	actor := uuid.New()
	createBody := map[string]any{
		"name":      "Press Release",
		"schema":    basicSchema(),
		"ui_schema": map[string]any{"layout": map[string]any{"type": "tabs"}},
		"actor_id":  actor.String(),
	}
	createResp := doJSONRequest(t, mux, http.MethodPost, "/admin/api/content-types", createBody, http.StatusCreated)

	var created content.ContentType
	decodeJSONBody(t, createResp, &created)
	if created.ID == uuid.Nil {
		t.Fatalf("expected created content type id")
	}
	if created.Slug != "press-release" {
		t.Fatalf("expected slug press-release got %q", created.Slug)
	}
	if created.SchemaVersion == "" {
		t.Fatalf("expected schema_version to be set")
	}

	listResp := doJSONRequest(t, mux, http.MethodGet, "/admin/api/content-types", nil, http.StatusOK)
	var list []*content.ContentType
	decodeJSONBody(t, listResp, &list)
	if len(list) != 1 {
		t.Fatalf("expected 1 content type got %d", len(list))
	}

	getPath := "/admin/api/content-types/" + created.ID.String()
	getResp := doJSONRequest(t, mux, http.MethodGet, getPath, nil, http.StatusOK)
	var fetched content.ContentType
	decodeJSONBody(t, getResp, &fetched)
	if fetched.ID != created.ID {
		t.Fatalf("expected fetched id %s got %s", created.ID, fetched.ID)
	}

	updateBody := map[string]any{
		"description": "Updated description",
		"actor_id":    actor.String(),
	}
	updateResp := doJSONRequest(t, mux, http.MethodPut, getPath, updateBody, http.StatusOK)
	var updated content.ContentType
	decodeJSONBody(t, updateResp, &updated)
	if updated.Description == nil || *updated.Description != "Updated description" {
		t.Fatalf("expected updated description")
	}

	publishResp := doJSONRequest(t, mux, http.MethodPost, getPath+"/publish", map[string]any{"actor_id": actor.String()}, http.StatusOK)
	var published content.ContentType
	decodeJSONBody(t, publishResp, &published)
	if published.Status != content.ContentTypeStatusActive {
		t.Fatalf("expected status active got %q", published.Status)
	}

	cloneBody := map[string]any{
		"name":     "Press Release Copy",
		"slug":     "press-release-copy",
		"actor_id": actor.String(),
	}
	cloneResp := doJSONRequest(t, mux, http.MethodPost, getPath+"/clone", cloneBody, http.StatusCreated)
	var cloned content.ContentType
	decodeJSONBody(t, cloneResp, &cloned)
	if cloned.ID == created.ID {
		t.Fatalf("expected clone id to differ")
	}
	if cloned.Slug != "press-release-copy" {
		t.Fatalf("expected cloned slug press-release-copy got %q", cloned.Slug)
	}
	if cloned.Status != content.ContentTypeStatusDraft {
		t.Fatalf("expected cloned status draft got %q", cloned.Status)
	}

	deletePath := "/admin/api/content-types/" + cloned.ID.String()
	doJSONRequest(t, mux, http.MethodDelete, deletePath+"?hard_delete=true", nil, http.StatusNoContent)

	doJSONRequest(t, mux, http.MethodGet, deletePath, nil, http.StatusNotFound)
}

func TestAdminAPI_SchemaUtilities(t *testing.T) {
	mux, _ := setupAdminAPI(t)

	validResp := doJSONRequest(t, mux, http.MethodPost, "/admin/api/content-types/validate", map[string]any{
		"schema": basicSchema(),
	}, http.StatusOK)
	var validResult map[string]any
	decodeJSONBody(t, validResp, &validResult)
	if ok, _ := validResult["valid"].(bool); !ok {
		t.Fatalf("expected valid response")
	}

	invalidResp := doJSONRequest(t, mux, http.MethodPost, "/admin/api/content-types/validate", map[string]any{
		"schema": map[string]any{"type": 123},
	}, http.StatusUnprocessableEntity)
	var invalidResult map[string]any
	decodeJSONBody(t, invalidResp, &invalidResult)
	if invalidResult["error"] != "validation_failed" {
		t.Fatalf("expected validation_failed error")
	}

	previewResp := doJSONRequest(t, mux, http.MethodPost, "/admin/api/content-types/preview", map[string]any{
		"schema": basicSchema(),
		"slug":   "preview",
	}, http.StatusOK)
	var preview schemaPreviewResponse
	decodeJSONBody(t, previewResp, &preview)
	if preview.Version == "" {
		t.Fatalf("expected preview version")
	}
	if preview.Metadata.Slug != "preview" {
		t.Fatalf("expected metadata slug preview got %q", preview.Metadata.Slug)
	}
}

func TestAdminAPI_BlockCRUDAndOpenAPI(t *testing.T) {
	mux, services := setupAdminAPI(t)

	blockBody := map[string]any{
		"name":   "Hero",
		"schema": basicSchema(),
	}
	blockResp := doJSONRequest(t, mux, http.MethodPost, "/admin/api/blocks", blockBody, http.StatusCreated)
	var blockDef blocks.Definition
	decodeJSONBody(t, blockResp, &blockDef)
	if blockDef.ID == uuid.Nil {
		t.Fatalf("expected block definition id")
	}

	ctBody := map[string]any{
		"name":   "Article",
		"schema": basicSchema(),
	}
	ctResp := doJSONRequest(t, mux, http.MethodPost, "/admin/api/content-types", ctBody, http.StatusCreated)
	var ct content.ContentType
	decodeJSONBody(t, ctResp, &ct)

	openapiPath := "/admin/api/content-types/" + ct.ID.String() + "/openapi"
	openapiResp := doJSONRequest(t, mux, http.MethodGet, openapiPath, nil, http.StatusOK)
	var doc map[string]any
	decodeJSONBody(t, openapiResp, &doc)
	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	if schemas == nil || schemas["article"] == nil {
		t.Fatalf("expected content type schema in openapi components")
	}
	if schemas["hero"] == nil {
		t.Fatalf("expected block schema in openapi components")
	}

	if services.blockSvc == nil {
		t.Fatalf("expected block service wired")
	}
}

type testServices struct {
	contentSvc content.ContentTypeService
	blockSvc   blocks.Service
}

func setupAdminAPI(t *testing.T) (*http.ServeMux, testServices) {
	t.Helper()

	typeRepo := content.NewMemoryContentTypeRepository()
	contentSvc := content.NewContentTypeService(typeRepo)

	blockSvc := blocks.NewService(
		blocks.NewMemoryDefinitionRepository(),
		blocks.NewMemoryInstanceRepository(),
		blocks.NewMemoryTranslationRepository(),
	)

	api := NewAdminAPI(
		WithContentTypeService(contentSvc),
		WithBlockService(blockSvc),
	)
	mux := http.NewServeMux()
	if err := api.Register(mux); err != nil {
		t.Fatalf("register api: %v", err)
	}
	return mux, testServices{contentSvc: contentSvc, blockSvc: blockSvc}
}

func doJSONRequest(t *testing.T, mux *http.ServeMux, method, path string, body any, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("expected status %d got %d (%s)", wantStatus, rec.Code, rec.Body.String())
	}
	return rec
}

func decodeJSONBody(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func basicSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type": "string",
			},
		},
	}
}
