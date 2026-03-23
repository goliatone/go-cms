package content_test

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/goliatone/go-cms/content"
	"github.com/google/uuid"
)

func TestNormalizeContentTypeCapabilitiesMigratesLegacyDeliveryMenu(t *testing.T) {
	normalized, validation := content.NormalizeContentTypeCapabilities(map[string]any{
		"delivery": map[string]any{
			"enabled": true,
			"kind":    "page",
			"menu": map[string]any{
				"location": "site.main",
			},
		},
	})
	if len(validation) != 0 {
		t.Fatalf("expected no validation errors, got %#v", validation)
	}
	delivery, _ := normalized["delivery"].(map[string]any)
	if _, exists := delivery["menu"]; exists {
		t.Fatalf("expected legacy delivery.menu to be migrated out of canonical payload")
	}
	navigation, _ := normalized["navigation"].(map[string]any)
	if navigation == nil {
		t.Fatalf("expected navigation capability")
	}
	defaults, _ := navigation["default_locations"].([]string)
	if len(defaults) != 1 || defaults[0] != "site.main" {
		t.Fatalf("expected default_locations [site.main], got %#v", navigation["default_locations"])
	}
}

func TestValidateAndNormalizeContentTypeCapabilitiesRejectsInvalidNavigationSubset(t *testing.T) {
	_, err := content.ValidateAndNormalizeContentTypeCapabilities(map[string]any{
		"navigation": map[string]any{
			"enabled":            true,
			"eligible_locations": []any{"site.main"},
			"default_locations":  []any{"site.footer"},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !errors.Is(err, content.ErrContentTypeCapabilitiesInvalid) {
		t.Fatalf("expected ErrContentTypeCapabilitiesInvalid, got %v", err)
	}
	var validationErr *content.ContentTypeCapabilityValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ContentTypeCapabilityValidationError, got %T", err)
	}
	if validationErr.Fields["capabilities.navigation.default_locations"] == "" {
		t.Fatalf("expected default_locations validation failure, got %#v", validationErr.Fields)
	}
}

func TestValidateAndNormalizeContentTypeCapabilitiesRejectsInvalidBooleanFields(t *testing.T) {
	_, err := content.ValidateAndNormalizeContentTypeCapabilities(map[string]any{
		"navigation": map[string]any{
			"allow_instance_override": "banana",
		},
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	var validationErr *content.ContentTypeCapabilityValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ContentTypeCapabilityValidationError, got %T", err)
	}
	if validationErr.Fields["capabilities.navigation.allow_instance_override"] == "" {
		t.Fatalf("expected allow_instance_override validation failure, got %#v", validationErr.Fields)
	}
}

func TestNormalizeContentTypeCapabilitiesSupportsSearchPassThrough(t *testing.T) {
	normalized, validation := content.NormalizeContentTypeCapabilities(map[string]any{
		"search": map[string]any{
			"enabled":    true,
			"collection": "legacy_articles",
			"fields": map[string]any{
				"title": map[string]any{"weight": 10},
			},
		},
	})
	if len(validation) != 0 {
		t.Fatalf("expected no validation errors, got %#v", validation)
	}
	search, _ := normalized["search"].(map[string]any)
	if search == nil {
		t.Fatalf("expected search capability")
	}
	if search["index"] != "legacy_articles" {
		t.Fatalf("expected search.index to be canonicalized, got %#v", search["index"])
	}
	if _, ok := search["collection"]; ok {
		t.Fatalf("expected canonical search payload to omit collection, got %#v", search)
	}
	fields, _ := search["fields"].(map[string]any)
	if fields == nil {
		t.Fatalf("expected search.fields object")
	}
	title, _ := fields["title"].(map[string]any)
	if title == nil {
		t.Fatalf("expected title field config")
	}
	if title["weight"] != 10 {
		t.Fatalf("expected weight 10, got %#v", title["weight"])
	}
}

func TestNormalizeContentTypeCapabilitiesRemovesLegacyAliasKeys(t *testing.T) {
	normalized, validation := content.NormalizeContentTypeCapabilities(map[string]any{
		"navigation_enabled": true,
		"navigation_eligible_locations": []any{
			"site.main",
		},
		"delivery_enabled": true,
		"search_enabled":   true,
	})
	if len(validation) != 0 {
		t.Fatalf("expected no validation errors, got %#v", validation)
	}

	if _, ok := normalized["navigation_enabled"]; ok {
		t.Fatalf("expected navigation_enabled alias to be removed")
	}
	if _, ok := normalized["navigation_eligible_locations"]; ok {
		t.Fatalf("expected navigation_eligible_locations alias to be removed")
	}
	if _, ok := normalized["delivery_enabled"]; ok {
		t.Fatalf("expected delivery_enabled alias to be removed")
	}
	if _, ok := normalized["search_enabled"]; ok {
		t.Fatalf("expected search_enabled alias to be removed")
	}
	if _, ok := normalized["search_index"]; ok {
		t.Fatalf("expected search_index alias to be removed")
	}

	navigation, _ := normalized["navigation"].(map[string]any)
	if navigation == nil || navigation["enabled"] != true {
		t.Fatalf("expected canonical navigation.enabled=true, got %#v", navigation)
	}
}

func TestNormalizeContentTypeCapabilitiesCanonicalizesSearchIndexAliases(t *testing.T) {
	normalized, validation := content.NormalizeContentTypeCapabilities(map[string]any{
		"search_collection": "articles",
		"search_index":      "posts",
	})
	if len(validation) != 0 {
		t.Fatalf("expected no validation errors, got %#v", validation)
	}
	search, _ := normalized["search"].(map[string]any)
	if search == nil {
		t.Fatalf("expected search capability")
	}
	if search["index"] != "posts" {
		t.Fatalf("expected search.index=posts, got %#v", search["index"])
	}
	if _, ok := search["collection"]; ok {
		t.Fatalf("expected collection alias to be removed, got %#v", search)
	}
}

func TestBackfillContentTypeNavigationDefaultsMigratesLegacyCapabilities(t *testing.T) {
	idLegacy := uuid.New()
	idCanonical := uuid.New()
	service := &stubContentTypeService{
		records: map[uuid.UUID]*content.ContentType{
			idLegacy: {
				ID:   idLegacy,
				Name: "Page",
				Slug: "page",
				Capabilities: map[string]any{
					"delivery": map[string]any{
						"menu": map[string]any{
							"location": "site.main",
						},
					},
				},
			},
			idCanonical: {
				ID:   idCanonical,
				Name: "Post",
				Slug: "post",
				Capabilities: map[string]any{
					"navigation": map[string]any{
						"enabled":            true,
						"eligible_locations": []string{"site.main"},
						"default_locations":  []string{"site.main"},
					},
				},
			},
		},
	}

	updated, err := content.BackfillContentTypeNavigationDefaults(context.Background(), service)
	if err != nil {
		t.Fatalf("backfill capabilities: %v", err)
	}
	if updated != 2 {
		t.Fatalf("expected 2 updated records, got %d", updated)
	}
	legacyRecord := service.records[idLegacy]
	navigation, _ := legacyRecord.Capabilities["navigation"].(map[string]any)
	if navigation == nil {
		t.Fatalf("expected migrated navigation capability")
	}
}

type stubContentTypeService struct {
	records map[uuid.UUID]*content.ContentType
}

func (s *stubContentTypeService) Create(context.Context, content.CreateContentTypeRequest) (*content.ContentType, error) {
	return nil, errors.New("not implemented")
}

func (s *stubContentTypeService) Update(_ context.Context, req content.UpdateContentTypeRequest) (*content.ContentType, error) {
	record := s.records[req.ID]
	if record == nil {
		return nil, errors.New("record not found")
	}
	record.Capabilities = cloneAnyMap(req.Capabilities)
	return record, nil
}

func (s *stubContentTypeService) Delete(context.Context, content.DeleteContentTypeRequest) error {
	return errors.New("not implemented")
}

func (s *stubContentTypeService) Get(_ context.Context, id uuid.UUID) (*content.ContentType, error) {
	return s.records[id], nil
}

func (s *stubContentTypeService) GetBySlug(_ context.Context, slug string, _ ...string) (*content.ContentType, error) {
	for _, record := range s.records {
		if record != nil && record.Slug == slug {
			return record, nil
		}
	}
	return nil, nil
}

func (s *stubContentTypeService) List(_ context.Context, _ ...string) ([]*content.ContentType, error) {
	out := make([]*content.ContentType, 0, len(s.records))
	for _, record := range s.records {
		out = append(out, record)
	}
	return out, nil
}

func (s *stubContentTypeService) Find(context.Context, string, ...string) ([]*content.ContentType, error) {
	return nil, errors.New("not implemented")
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	maps.Copy(out, src)
	return out
}
