package content_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
)

type stubPageResolver struct {
	pageIDs []uuid.UUID
}

func (s stubPageResolver) PageIDsForContent(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return s.pageIDs, nil
}

func TestContentCreateSyncsEmbeddedBlocks(t *testing.T) {
	ctx := context.Background()

	contentRepo := content.NewMemoryContentRepository()
	contentTypeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	typeID := uuid.New()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"blocks": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		},
		"additionalProperties": true,
	}
	if err := contentTypeRepo.Put(&content.ContentType{
		ID:     typeID,
		Name:   "Page",
		Slug:   "page",
		Schema: schema,
	}); err != nil {
		t.Fatalf("seed content type: %v", err)
	}

	defRepo := blocks.NewMemoryDefinitionRepository()
	instRepo := blocks.NewMemoryInstanceRepository()
	trRepo := blocks.NewMemoryTranslationRepository()
	blockSvc := blocks.NewService(defRepo, instRepo, trRepo)
	def, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	pageID := uuid.New()
	bridge := blocks.NewEmbeddedBlockBridge(blockSvc, localeRepo, stubPageResolver{pageIDs: []uuid.UUID{pageID}})
	svc := content.NewService(contentRepo, contentTypeRepo, localeRepo, content.WithEmbeddedBlocksResolver(bridge))

	actorID := uuid.New()
	_, err = svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "home",
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Home",
			Blocks: []map[string]any{{
				content.EmbeddedBlockTypeKey: "hero",
				"title":                      "Hello",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	instances, err := blockSvc.ListPageInstances(ctx, pageID)
	if err != nil {
		t.Fatalf("list page instances: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 block instance, got %d", len(instances))
	}
	instance := instances[0]
	if instance.DefinitionID != def.ID {
		t.Fatalf("expected definition %s, got %s", def.ID, instance.DefinitionID)
	}
	if instance.Region != "blocks" {
		t.Fatalf("expected region blocks, got %s", instance.Region)
	}
	if instance.PageID == nil || *instance.PageID != pageID {
		t.Fatalf("expected page id %s, got %v", pageID, instance.PageID)
	}
	if len(instance.Translations) != 1 {
		t.Fatalf("expected 1 translation, got %d", len(instance.Translations))
	}
	if title, ok := instance.Translations[0].Content["title"].(string); !ok || title != "Hello" {
		t.Fatalf("expected translation title Hello, got %v", instance.Translations[0].Content["title"])
	}
}

func TestContentCreateRejectsUnavailableBlock(t *testing.T) {
	ctx := context.Background()

	contentRepo := content.NewMemoryContentRepository()
	contentTypeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	typeID := uuid.New()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"blocks": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		},
		"additionalProperties": true,
		"metadata": map[string]any{
			"block_availability": map[string]any{
				"allow": []any{"hero"},
			},
		},
	}
	if err := contentTypeRepo.Put(&content.ContentType{
		ID:     typeID,
		Name:   "Page",
		Slug:   "page",
		Schema: schema,
	}); err != nil {
		t.Fatalf("seed content type: %v", err)
	}

	defRepo := blocks.NewMemoryDefinitionRepository()
	instRepo := blocks.NewMemoryInstanceRepository()
	trRepo := blocks.NewMemoryTranslationRepository()
	blockSvc := blocks.NewService(defRepo, instRepo, trRepo)
	if _, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	}); err != nil {
		t.Fatalf("register hero definition: %v", err)
	}
	if _, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "promo",
		Schema: map[string]any{"fields": []any{"title"}},
	}); err != nil {
		t.Fatalf("register promo definition: %v", err)
	}

	pageID := uuid.New()
	bridge := blocks.NewEmbeddedBlockBridge(blockSvc, localeRepo, stubPageResolver{pageIDs: []uuid.UUID{pageID}})
	svc := content.NewService(contentRepo, contentTypeRepo, localeRepo, content.WithEmbeddedBlocksResolver(bridge))

	actorID := uuid.New()
	_, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "home",
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Home",
			Blocks: []map[string]any{{
				content.EmbeddedBlockTypeKey: "promo",
				"title":                      "Not allowed",
			}},
		}},
	})
	if err == nil {
		t.Fatal("expected content create to fail for unavailable block")
	}
	if !errors.Is(err, content.ErrContentSchemaInvalid) {
		t.Fatalf("expected ErrContentSchemaInvalid, got %v", err)
	}
	if !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("expected availability message, got %v", err)
	}
}

func TestContentUpdateSyncsEmbeddedBlocks(t *testing.T) {
	ctx := context.Background()

	contentRepo := content.NewMemoryContentRepository()
	contentTypeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	typeID := uuid.New()
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"blocks": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		},
		"additionalProperties": true,
	}
	if err := contentTypeRepo.Put(&content.ContentType{
		ID:     typeID,
		Name:   "Page",
		Slug:   "page",
		Schema: schema,
	}); err != nil {
		t.Fatalf("seed content type: %v", err)
	}

	defRepo := blocks.NewMemoryDefinitionRepository()
	instRepo := blocks.NewMemoryInstanceRepository()
	trRepo := blocks.NewMemoryTranslationRepository()
	blockSvc := blocks.NewService(defRepo, instRepo, trRepo)
	blockSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"_type": map[string]any{"const": "hero"},
			"title": map[string]any{"type": "string"},
		},
		"required": []string{"_type", "title"},
		"metadata": map[string]any{
			"schema_version": "hero@v1.0.0",
		},
	}
	if _, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: blockSchema,
	}); err != nil {
		t.Fatalf("register definition: %v", err)
	}

	pageID := uuid.New()
	bridge := blocks.NewEmbeddedBlockBridge(blockSvc, localeRepo, stubPageResolver{pageIDs: []uuid.UUID{pageID}})
	svc := content.NewService(contentRepo, contentTypeRepo, localeRepo, content.WithEmbeddedBlocksResolver(bridge))

	actorID := uuid.New()
	created, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "home",
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Home",
			Blocks: []map[string]any{{
				content.EmbeddedBlockTypeKey: "hero",
				"title":                      "Hello",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	instances, err := blockSvc.ListPageInstances(ctx, pageID)
	if err != nil {
		t.Fatalf("list page instances: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 block instance, got %d", len(instances))
	}
	if title, ok := instances[0].Translations[0].Content["title"].(string); !ok || title != "Hello" {
		t.Fatalf("expected initial title Hello, got %v", instances[0].Translations[0].Content["title"])
	}

	_, err = svc.Update(ctx, content.UpdateContentRequest{
		ID:        created.ID,
		UpdatedBy: actorID,
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Home Updated",
			Blocks: []map[string]any{{
				content.EmbeddedBlockTypeKey: "hero",
				"title":                      "Updated",
			}},
		}},
	})
	if err != nil {
		t.Fatalf("update content: %v", err)
	}

	instances, err = blockSvc.ListPageInstances(ctx, pageID)
	if err != nil {
		t.Fatalf("list page instances after update: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("expected 1 block instance after update, got %d", len(instances))
	}
	if title, ok := instances[0].Translations[0].Content["title"].(string); !ok || title != "Updated" {
		t.Fatalf("expected updated title Updated, got %v", instances[0].Translations[0].Content["title"])
	}
}
