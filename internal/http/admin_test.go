package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	cmsenv "github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/promotions"
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

func TestAdminAPI_EnvironmentCRUD(t *testing.T) {
	mux, services := setupAdminAPI(t)
	if services.envSvc == nil {
		t.Fatalf("expected environment service wired")
	}

	createBody := map[string]any{
		"key":         "dev",
		"name":        "Development",
		"description": "Development environment",
		"is_default":  true,
	}
	createResp := doJSONRequest(t, mux, http.MethodPost, "/admin/api/environments", createBody, http.StatusCreated)

	var created cmsenv.Environment
	decodeJSONBody(t, createResp, &created)
	if created.ID == uuid.Nil {
		t.Fatalf("expected created environment id")
	}
	if created.Key != "dev" {
		t.Fatalf("expected key dev got %q", created.Key)
	}
	if !created.IsDefault {
		t.Fatalf("expected dev to be default")
	}

	listResp := doJSONRequest(t, mux, http.MethodGet, "/admin/api/environments", nil, http.StatusOK)
	var list []*cmsenv.Environment
	decodeJSONBody(t, listResp, &list)
	if len(list) < 2 {
		t.Fatalf("expected at least 2 environments got %d", len(list))
	}

	getResp := doJSONRequest(t, mux, http.MethodGet, "/admin/api/environments/"+created.ID.String(), nil, http.StatusOK)
	var fetched cmsenv.Environment
	decodeJSONBody(t, getResp, &fetched)
	if fetched.ID != created.ID {
		t.Fatalf("expected fetched id %s got %s", created.ID, fetched.ID)
	}

	updateBody := map[string]any{
		"description": "Updated description",
		"is_active":   false,
	}
	updateResp := doJSONRequest(t, mux, http.MethodPut, "/admin/api/environments/"+created.ID.String(), updateBody, http.StatusOK)
	var updated cmsenv.Environment
	decodeJSONBody(t, updateResp, &updated)
	if updated.Description == nil || *updated.Description != "Updated description" {
		t.Fatalf("expected updated description")
	}
	if updated.IsActive {
		t.Fatalf("expected environment to be inactive")
	}

	doJSONRequest(t, mux, http.MethodDelete, "/admin/api/environments/"+created.ID.String(), nil, http.StatusNoContent)
	doJSONRequest(t, mux, http.MethodGet, "/admin/api/environments/"+created.ID.String(), nil, http.StatusNotFound)
}

func TestAdminAPI_PromotionEndpoints(t *testing.T) {
	promo := &promotionStub{
		envResult: &promotions.PromoteEnvironmentResult{
			SourceEnv: promotions.EnvironmentRef{ID: uuid.New(), Key: "dev"},
			TargetEnv: promotions.EnvironmentRef{ID: uuid.New(), Key: "prod"},
			Summary: promotions.PromoteSummary{
				ContentTypes:   promotions.PromoteSummaryCounts{Created: 1},
				ContentEntries: promotions.PromoteSummaryCounts{Created: 2},
			},
			Items: []promotions.PromoteItem{
				{
					Kind:     "block_definition",
					SourceID: uuid.New(),
					TargetID: uuid.New(),
					Status:   "created",
				},
			},
		},
		itemResult: &promotions.PromoteItem{
			Kind:     "content_type",
			SourceID: uuid.New(),
			TargetID: uuid.New(),
			Status:   "created",
		},
	}

	mux, _ := setupAdminAPI(t, WithPromotionService(promo))

	bulkPayload := map[string]any{
		"scope": "all",
		"options": map[string]any{
			"promote_as_active": true,
		},
	}
	bulkResp := doJSONRequest(t, mux, http.MethodPost, "/admin/api/environments/dev/promote/prod", bulkPayload, http.StatusOK)
	var bulkResult promotions.PromoteEnvironmentResult
	decodeJSONBody(t, bulkResp, &bulkResult)
	if bulkResult.SourceEnv.Key != "dev" || bulkResult.TargetEnv.Key != "prod" {
		t.Fatalf("expected source dev and target prod")
	}
	if len(bulkResult.Items) != 1 || bulkResult.Items[0].Kind != "block_definition" {
		t.Fatalf("expected block_definition item in response")
	}
	if promo.lastEnv == nil || promo.lastEnv.SourceEnvironment != "dev" || promo.lastEnv.TargetEnvironment != "prod" {
		t.Fatalf("expected promotion service to receive source and target env")
	}

	typeID := uuid.New()
	typeResp := doJSONRequest(t, mux, http.MethodPost, "/admin/api/content-types/"+typeID.String()+"/promote?to=prod", map[string]any{}, http.StatusOK)
	var typeResult promotions.PromoteItem
	decodeJSONBody(t, typeResp, &typeResult)
	if promo.lastType == nil || promo.lastType.ContentTypeID != typeID || promo.lastType.TargetEnvironment != "prod" {
		t.Fatalf("expected content type promotion request to target prod")
	}

	contentID := uuid.New()
	contentResp := doJSONRequest(t, mux, http.MethodPost, "/admin/api/content/"+contentID.String()+"/promote", map[string]any{
		"target_environment": "staging",
	}, http.StatusOK)
	var contentResult promotions.PromoteItem
	decodeJSONBody(t, contentResp, &contentResult)
	if promo.lastContent == nil || promo.lastContent.ContentID != contentID || promo.lastContent.TargetEnvironment != "staging" {
		t.Fatalf("expected content promotion request to target staging")
	}
}

type testServices struct {
	contentSvc content.ContentTypeService
	blockSvc   blocks.Service
	envSvc     cmsenv.Service
}

func setupAdminAPI(t *testing.T, opts ...AdminOption) (*http.ServeMux, testServices) {
	t.Helper()

	envRepo := cmsenv.NewMemoryRepository()
	envSvc := cmsenv.NewService(envRepo)
	if _, err := envSvc.CreateEnvironment(context.Background(), cmsenv.CreateEnvironmentInput{
		Key:       cmsenv.DefaultKey,
		Name:      "Default",
		IsDefault: true,
	}); err != nil {
		t.Fatalf("seed default environment: %v", err)
	}

	typeRepo := content.NewMemoryContentTypeRepository()
	contentSvc := content.NewContentTypeService(typeRepo, content.WithContentTypeEnvironmentService(envSvc))

	blockSvc := blocks.NewService(
		blocks.NewMemoryDefinitionRepository(),
		blocks.NewMemoryInstanceRepository(),
		blocks.NewMemoryTranslationRepository(),
		blocks.WithEnvironmentService(envSvc),
	)

	apiOpts := []AdminOption{
		WithContentTypeService(contentSvc),
		WithBlockService(blockSvc),
		WithEnvironmentService(envSvc),
	}
	apiOpts = append(apiOpts, opts...)
	api := NewAdminAPI(apiOpts...)
	mux := http.NewServeMux()
	if err := api.Register(mux); err != nil {
		t.Fatalf("register api: %v", err)
	}
	return mux, testServices{contentSvc: contentSvc, blockSvc: blockSvc, envSvc: envSvc}
}

type promotionStub struct {
	lastEnv     *promotions.PromoteEnvironmentRequest
	lastType    *promotions.PromoteContentTypeRequest
	lastContent *promotions.PromoteContentEntryRequest

	envResult  *promotions.PromoteEnvironmentResult
	itemResult *promotions.PromoteItem
}

func (s *promotionStub) PromoteEnvironment(ctx context.Context, req promotions.PromoteEnvironmentRequest) (*promotions.PromoteEnvironmentResult, error) {
	s.lastEnv = &req
	return s.envResult, nil
}

func (s *promotionStub) PromoteContentType(ctx context.Context, req promotions.PromoteContentTypeRequest) (*promotions.PromoteItem, error) {
	s.lastType = &req
	if s.itemResult == nil {
		return &promotions.PromoteItem{Kind: "content_type", SourceID: req.ContentTypeID, TargetID: uuid.New(), Status: "created"}, nil
	}
	return s.itemResult, nil
}

func (s *promotionStub) PromoteContentEntry(ctx context.Context, req promotions.PromoteContentEntryRequest) (*promotions.PromoteItem, error) {
	s.lastContent = &req
	if s.itemResult == nil {
		return &promotions.PromoteItem{Kind: "content_entry", SourceID: req.ContentID, TargetID: uuid.New(), Status: "created"}, nil
	}
	return s.itemResult, nil
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
