package content_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
)

func TestContentEntryMetadataPersistsStructuralFields(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	seedContentType(t, typeStore, &content.ContentType{
		ID:   contentTypeID,
		Name: "page",
		Slug: "page",
		Schema: map[string]any{
			"fields": []any{map[string]any{"name": "body"}},
		},
		Capabilities: map[string]any{
			"navigation": map[string]any{
				"enabled":                 true,
				"eligible_locations":      []any{"site.main", "site.footer"},
				"default_locations":       []any{"site.main"},
				"default_visible":         true,
				"allow_instance_override": true,
			},
		},
	})

	localeStore.Put(&content.Locale{
		ID:      uuid.New(),
		Code:    "en",
		Display: "English",
	})

	svc := content.NewService(contentStore, typeStore, localeStore)
	ctx := context.Background()
	parentID := uuid.New()
	templateID := uuid.New()
	authorID := uuid.New()

	created, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "about",
		Status:        "draft",
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
		Metadata: map[string]any{
			"parent_id":   parentID,
			"template_id": templateID.String(),
			"path":        " /about ",
			"order":       7,
			"_navigation": map[string]any{
				"site.main":   " show ",
				"site.footer": "hide",
			},
			"legacy": "keep",
		},
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "About",
				Content: map[string]any{"body": "content"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	assertEntryMetadata(t, created.Metadata, parentID, templateID, "/about", 7)
	if got := requireString(t, created.Metadata["legacy"]); got != "keep" {
		t.Fatalf("expected legacy metadata %q got %q", "keep", got)
	}
	if _, ok := created.Metadata["order"]; ok {
		t.Fatalf("expected order to be normalized to sort_order")
	}
	assertNavigationMetadata(t, created.Metadata, map[string]string{
		"site.main":   "show",
		"site.footer": "hide",
	})
	assertStringSlice(t, created.Metadata["effective_menu_locations"], []string{"site.main"})
	assertNavigationVisibilityMetadata(t, created.Metadata, map[string]bool{
		"site.main":   true,
		"site.footer": false,
	})

	if _, err := svc.UpdateTranslation(ctx, content.UpdateContentTranslationRequest{
		ContentID: created.ID,
		Locale:    "en",
		Title:     "About Updated",
		Content:   map[string]any{"body": "updated"},
		UpdatedBy: authorID,
	}); err != nil {
		t.Fatalf("update translation: %v", err)
	}

	updated, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	assertEntryMetadata(t, updated.Metadata, parentID, templateID, "/about", 7)
	if got := requireString(t, updated.Metadata["legacy"]); got != "keep" {
		t.Fatalf("expected legacy metadata to persist, got %q", got)
	}

	updatedMeta := map[string]any{
		"parent_id":   parentID.String(),
		"template_id": templateID.String(),
		"path":        "/about-updated",
		"sort_order":  12,
		"_navigation": map[string]any{
			"site.main": "inherit",
		},
		"effective_menu_locations": []any{"site.footer", "site.main", "site.footer"},
	}
	updatedContent, err := svc.Update(ctx, content.UpdateContentRequest{
		ID:        created.ID,
		UpdatedBy: authorID,
		Metadata:  updatedMeta,
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "About Updated",
				Content: map[string]any{"body": "updated"},
			},
		},
	})
	if err != nil {
		t.Fatalf("update content metadata: %v", err)
	}
	assertEntryMetadata(t, updatedContent.Metadata, parentID, templateID, "/about-updated", 12)
	if _, ok := updatedContent.Metadata["legacy"]; ok {
		t.Fatalf("expected metadata to be replaced on update")
	}
	assertNavigationMetadata(t, updatedContent.Metadata, map[string]string{
		"site.main": "inherit",
	})
	assertStringSlice(t, updatedContent.Metadata["effective_menu_locations"], []string{"site.main"})
	assertNavigationVisibilityMetadata(t, updatedContent.Metadata, map[string]bool{
		"site.main":   true,
		"site.footer": false,
	})
}

func TestContentEntryMetadataRejectsInvalidNavigationState(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	seedContentType(t, typeStore, &content.ContentType{
		ID:   contentTypeID,
		Name: "page",
		Slug: "page",
		Schema: map[string]any{
			"fields": []any{map[string]any{"name": "body"}},
		},
	})
	localeStore.Put(&content.Locale{
		ID:      uuid.New(),
		Code:    "en",
		Display: "English",
	})

	svc := content.NewService(contentStore, typeStore, localeStore)
	_, err := svc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "invalid-navigation",
		Status:        "draft",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Metadata: map[string]any{
			"_navigation": map[string]any{
				"site.main": "invalid",
			},
		},
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "About",
				Content: map[string]any{"body": "content"},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected metadata validation error")
	}
}

func TestContentEntryMetadataIgnoresOverridesWhenInstanceOverridesDisabled(t *testing.T) {
	contentStore := content.NewMemoryContentRepository()
	typeStore := content.NewMemoryContentTypeRepository()
	localeStore := content.NewMemoryLocaleRepository()

	contentTypeID := uuid.New()
	seedContentType(t, typeStore, &content.ContentType{
		ID:   contentTypeID,
		Name: "page",
		Slug: "page",
		Schema: map[string]any{
			"fields": []any{map[string]any{"name": "body"}},
		},
		Capabilities: map[string]any{
			"navigation": map[string]any{
				"enabled":                 true,
				"eligible_locations":      []any{"site.main", "site.footer"},
				"default_locations":       []any{"site.main"},
				"default_visible":         true,
				"allow_instance_override": false,
			},
		},
	})
	localeStore.Put(&content.Locale{
		ID:      uuid.New(),
		Code:    "en",
		Display: "English",
	})

	svc := content.NewService(contentStore, typeStore, localeStore)
	created, err := svc.Create(context.Background(), content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "override-disabled",
		Status:        "draft",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Metadata: map[string]any{
			"_navigation": map[string]any{
				"site.main":   "hide",
				"site.footer": "show",
			},
		},
		Translations: []content.ContentTranslationInput{
			{
				Locale:  "en",
				Title:   "Override Disabled",
				Content: map[string]any{"body": "content"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	assertStringSlice(t, created.Metadata["effective_menu_locations"], []string{"site.main"})
	assertNavigationVisibilityMetadata(t, created.Metadata, map[string]bool{
		"site.main":   true,
		"site.footer": false,
	})
}

func assertEntryMetadata(t *testing.T, metadata map[string]any, parentID, templateID uuid.UUID, path string, sortOrder int) {
	t.Helper()

	if metadata == nil {
		t.Fatalf("expected metadata to be set")
	}

	if got := requireString(t, metadata["parent_id"]); got != parentID.String() {
		t.Fatalf("expected parent_id %s got %s", parentID, got)
	}
	if got := requireString(t, metadata["template_id"]); got != templateID.String() {
		t.Fatalf("expected template_id %s got %s", templateID, got)
	}
	if got := requireString(t, metadata["path"]); got != path {
		t.Fatalf("expected path %q got %q", path, got)
	}
	if got := requireInt(t, metadata["sort_order"]); got != sortOrder {
		t.Fatalf("expected sort_order %d got %d", sortOrder, got)
	}
}

func requireString(t *testing.T, value any) string {
	t.Helper()
	typed, ok := value.(string)
	if !ok {
		t.Fatalf("expected string value, got %T", value)
	}
	return typed
}

func requireInt(t *testing.T, value any) int {
	t.Helper()
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		t.Fatalf("expected integer value, got %T", value)
	}
	return 0
}

func assertNavigationMetadata(t *testing.T, metadata map[string]any, expected map[string]string) {
	t.Helper()
	raw, ok := metadata["_navigation"].(map[string]string)
	if !ok {
		t.Fatalf("expected _navigation map[string]string, got %T", metadata["_navigation"])
	}
	if len(raw) != len(expected) {
		t.Fatalf("expected %d _navigation entries, got %d", len(expected), len(raw))
	}
	for key, value := range expected {
		if raw[key] != value {
			t.Fatalf("expected _navigation[%q]=%q got %q", key, value, raw[key])
		}
	}
}

func assertStringSlice(t *testing.T, value any, expected []string) {
	t.Helper()
	actual, ok := value.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", value)
	}
	if len(actual) != len(expected) {
		t.Fatalf("expected slice len %d got %d (%#v)", len(expected), len(actual), actual)
	}
	for idx := range expected {
		if actual[idx] != expected[idx] {
			t.Fatalf("expected[%d]=%q got %q", idx, expected[idx], actual[idx])
		}
	}
}

func assertNavigationVisibilityMetadata(t *testing.T, metadata map[string]any, expected map[string]bool) {
	t.Helper()
	raw, ok := metadata["effective_navigation_visibility"].(map[string]bool)
	if !ok {
		t.Fatalf("expected effective_navigation_visibility map[string]bool, got %T", metadata["effective_navigation_visibility"])
	}
	if len(raw) != len(expected) {
		t.Fatalf("expected %d visibility entries, got %d", len(expected), len(raw))
	}
	for key, value := range expected {
		if raw[key] != value {
			t.Fatalf("expected effective_navigation_visibility[%q]=%t got %t", key, value, raw[key])
		}
	}
}
