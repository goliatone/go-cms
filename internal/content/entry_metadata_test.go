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
			"legacy":      "keep",
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
