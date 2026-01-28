package content_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
)

func TestPreviewDraftMigratesEmbeddedBlocksWithoutPersistence(t *testing.T) {
	ctx := context.Background()

	contentRepo := content.NewMemoryContentRepository()
	contentTypeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentTypeID := uuid.New()
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
		ID:     contentTypeID,
		Name:   "Page",
		Slug:   "page",
		Schema: schema,
	}); err != nil {
		t.Fatalf("seed content type: %v", err)
	}

	migrator := blocks.NewMigrator()
	if err := migrator.Register("hero", "hero@v1.0.0", "hero@v2.0.0", func(payload map[string]any) (map[string]any, error) {
		out := map[string]any{}
		for k, v := range payload {
			out[k] = v
		}
		if headline, ok := out["headline"]; ok {
			out["title"] = headline
			delete(out, "headline")
		}
		return out, nil
	}); err != nil {
		t.Fatalf("register migration: %v", err)
	}

	defRepo := blocks.NewMemoryDefinitionRepository()
	defVersionRepo := blocks.NewMemoryDefinitionVersionRepository()
	instRepo := blocks.NewMemoryInstanceRepository()
	trRepo := blocks.NewMemoryTranslationRepository()
	blockSvc := blocks.NewService(
		defRepo,
		instRepo,
		trRepo,
		blocks.WithDefinitionVersionRepository(defVersionRepo),
		blocks.WithSchemaMigrator(migrator),
	)

	schemaV2 := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"_type": map[string]any{"const": "hero"},
			"title": map[string]any{"type": "string"},
		},
		"required": []string{"_type", "title"},
		"metadata": map[string]any{
			"schema_version": "hero@v2.0.0",
		},
	}
	definition, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: schemaV2,
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	schemaV1 := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"_type":    map[string]any{"const": "hero"},
			"headline": map[string]any{"type": "string"},
		},
		"required": []string{"_type", "headline"},
		"metadata": map[string]any{
			"schema_version": "hero@v1.0.0",
		},
	}
	if _, err := blockSvc.CreateDefinitionVersion(ctx, blocks.CreateDefinitionVersionInput{
		DefinitionID: definition.ID,
		Schema:       schemaV1,
	}); err != nil {
		t.Fatalf("register definition version: %v", err)
	}

	bridge := blocks.NewEmbeddedBlockBridge(blockSvc, localeRepo, stubPageResolver{pageIDs: []uuid.UUID{uuid.New()}})
	svc := content.NewService(
		contentRepo,
		contentTypeRepo,
		localeRepo,
		content.WithEmbeddedBlocksResolver(bridge),
		content.WithVersioningEnabled(true),
	)

	actorID := uuid.New()
	base, err := svc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "preview",
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Preview",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	snapshot := content.ContentVersionSnapshot{
		Translations: []content.ContentVersionTranslationSnapshot{{
			Locale: "en",
			Title:  "Draft EN",
			Content: map[string]any{
				content.EmbeddedBlocksKey: []map[string]any{{
					content.EmbeddedBlockTypeKey:   "hero",
					content.EmbeddedBlockSchemaKey: "hero@v1.0.0",
					"headline":                     "Hello",
				}},
			},
		}},
	}

	draft, err := svc.CreateDraft(ctx, content.CreateContentDraftRequest{
		ContentID: base.ID,
		Snapshot:  snapshot,
		CreatedBy: actorID,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	preview, err := svc.PreviewDraft(ctx, content.PreviewContentDraftRequest{
		ContentID: base.ID,
		Version:   draft.Version,
	})
	if err != nil {
		t.Fatalf("preview draft: %v", err)
	}
	if preview == nil || preview.Version == nil {
		t.Fatalf("expected preview result")
	}

	blocksPayload, ok := content.ExtractEmbeddedBlocks(preview.Version.Snapshot.Translations[0].Content)
	if !ok || len(blocksPayload) != 1 {
		t.Fatalf("expected migrated embedded blocks")
	}
	block := blocksPayload[0]
	if schemaVersion, _ := block[content.EmbeddedBlockSchemaKey].(string); schemaVersion != "hero@v2.0.0" {
		t.Fatalf("expected schema hero@v2.0.0 got %v", block[content.EmbeddedBlockSchemaKey])
	}
	if title, _ := block["title"].(string); title != "Hello" {
		t.Fatalf("expected migrated title Hello got %v", block["title"])
	}

	if preview.Content == nil || len(preview.Content.Translations) != 1 {
		t.Fatalf("expected preview content translations")
	}
	previewBlocks, ok := content.ExtractEmbeddedBlocks(preview.Content.Translations[0].Content)
	if !ok || len(previewBlocks) != 1 {
		t.Fatalf("expected preview content blocks")
	}
	if _, ok := previewBlocks[0]["headline"]; ok {
		t.Fatalf("expected preview content to use migrated fields")
	}

	stored, err := contentRepo.GetVersion(ctx, base.ID, draft.Version)
	if err != nil {
		t.Fatalf("fetch stored version: %v", err)
	}
	storedBlocks, ok := content.ExtractEmbeddedBlocks(stored.Snapshot.Translations[0].Content)
	if !ok || len(storedBlocks) != 1 {
		t.Fatalf("expected stored embedded blocks")
	}
	if schemaVersion, _ := storedBlocks[0][content.EmbeddedBlockSchemaKey].(string); schemaVersion != "hero@v1.0.0" {
		t.Fatalf("expected stored schema hero@v1.0.0 got %v", storedBlocks[0][content.EmbeddedBlockSchemaKey])
	}
	if headline, _ := storedBlocks[0]["headline"].(string); headline != "Hello" {
		t.Fatalf("expected stored headline Hello got %v", storedBlocks[0]["headline"])
	}
}
