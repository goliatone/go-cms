package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/google/uuid"
)

func TestPublishDraftMigratesEmbeddedBlocks(t *testing.T) {
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
	def, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
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
		DefinitionID: def.ID,
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
		Slug:          "home",
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Home",
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

	published, err := svc.PublishDraft(ctx, content.PublishContentDraftRequest{
		ContentID:   base.ID,
		Version:     draft.Version,
		PublishedBy: uuid.New(),
	})
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}

	if len(published.Snapshot.Translations) != 1 {
		t.Fatalf("expected 1 translation, got %d", len(published.Snapshot.Translations))
	}
	blocksPayload, ok := content.ExtractEmbeddedBlocks(published.Snapshot.Translations[0].Content)
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
	if _, ok := block["headline"]; ok {
		t.Fatalf("expected headline to be removed")
	}
}

func TestDraftAllowsMissingBlockRequiredButPublishFails(t *testing.T) {
	ctx := context.Background()

	contentRepo := content.NewMemoryContentRepository()
	contentTypeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentTypeID := uuid.New()
	if err := contentTypeRepo.Put(&content.ContentType{
		ID:     contentTypeID,
		Name:   "Article",
		Slug:   "article",
		Schema: map[string]any{},
	}); err != nil {
		t.Fatalf("seed content type: %v", err)
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
	)

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
		Slug:          "relaxed-draft",
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Draft",
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
					content.EmbeddedBlockTypeKey: "hero",
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

	_, err = svc.PublishDraft(ctx, content.PublishContentDraftRequest{
		ContentID:   base.ID,
		Version:     draft.Version,
		PublishedBy: uuid.New(),
	})
	if err == nil {
		t.Fatal("expected publish to fail for missing required block fields")
	}
	if !errors.Is(err, content.ErrContentSchemaInvalid) {
		t.Fatalf("expected schema invalid error, got %v", err)
	}
}

func TestDraftRejectsImmutableBlockTypeChange(t *testing.T) {
	ctx := context.Background()

	contentRepo := content.NewMemoryContentRepository()
	contentTypeRepo := content.NewMemoryContentTypeRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentTypeID := uuid.New()
	if err := contentTypeRepo.Put(&content.ContentType{
		ID:     contentTypeID,
		Name:   "Page",
		Slug:   "page",
		Schema: map[string]any{},
	}); err != nil {
		t.Fatalf("seed content type: %v", err)
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
	)

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
	def, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: blockSchema,
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}
	promoSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"_type": map[string]any{"const": "promo"},
			"title": map[string]any{"type": "string"},
		},
		"required": []string{"_type", "title"},
		"metadata": map[string]any{
			"schema_version": "promo@v1.0.0",
		},
	}
	if _, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "promo",
		Schema: promoSchema,
	}); err != nil {
		t.Fatalf("register promo definition: %v", err)
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
		Slug:          "immutable-block",
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Draft",
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
					content.EmbeddedBlockTypeKey: "promo",
					"title":                      "Hello",
					content.EmbeddedBlockMetaKey: map[string]any{
						"definition_id": def.ID.String(),
					},
				}},
			},
		}},
	}

	_, err = svc.CreateDraft(ctx, content.CreateContentDraftRequest{
		ContentID: base.ID,
		Snapshot:  snapshot,
		CreatedBy: actorID,
	})
	if err == nil {
		t.Fatal("expected draft creation to fail for immutable _type change")
	}
	if !errors.Is(err, content.ErrContentSchemaInvalid) {
		t.Fatalf("expected schema invalid error, got %v", err)
	}
}
