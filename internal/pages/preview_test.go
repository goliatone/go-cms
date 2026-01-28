package pages_test

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

type stubContentPageResolver struct {
	pages map[uuid.UUID][]uuid.UUID
}

func (s stubContentPageResolver) PageIDsForContent(_ context.Context, contentID uuid.UUID) ([]uuid.UUID, error) {
	if s.pages == nil {
		return nil, nil
	}
	return s.pages[contentID], nil
}

func TestPreviewDraftResolvesEmbeddedBlocks(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(0, 0).UTC()

	contentRepo := content.NewMemoryContentRepository()
	localeRepo := content.NewMemoryLocaleRepository()
	pageRepo := pages.NewMemoryPageRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

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

	contentID := uuid.New()
	contentRecord := &content.Content{
		ID:            contentID,
		ContentTypeID: uuid.New(),
		Status:        "draft",
		Slug:          "home",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		CreatedAt:     now,
		UpdatedAt:     now,
		Translations: []*content.ContentTranslation{{
			ID:        uuid.New(),
			ContentID: contentID,
			LocaleID:  localeID,
			Locale:    &content.Locale{ID: localeID, Code: "en", Display: "English"},
			Title:     "Home",
			Content: map[string]any{
				content.EmbeddedBlocksKey: []map[string]any{{
					content.EmbeddedBlockTypeKey:   "hero",
					content.EmbeddedBlockSchemaKey: "hero@v1.0.0",
					"headline":                     "Hello",
				}},
			},
			CreatedAt: now,
			UpdatedAt: now,
		}},
	}
	if _, err := contentRepo.Create(ctx, contentRecord); err != nil {
		t.Fatalf("seed content: %v", err)
	}

	pageID := uuid.New()
	pageRecord := &pages.Page{
		ID:         pageID,
		ContentID:  contentID,
		TemplateID: uuid.New(),
		Slug:       "home",
		Status:     "draft",
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
		CreatedAt:  now,
		UpdatedAt:  now,
		Translations: []*pages.PageTranslation{{
			ID:        uuid.New(),
			PageID:    pageID,
			LocaleID:  localeID,
			Title:     "Home",
			Path:      "/home",
			CreatedAt: now,
			UpdatedAt: now,
		}},
	}
	if _, err := pageRepo.Create(ctx, pageRecord); err != nil {
		t.Fatalf("seed page: %v", err)
	}

	if _, err := pageRepo.CreateVersion(ctx, &pages.PageVersion{
		ID:        uuid.New(),
		PageID:    pageID,
		Version:   1,
		Status:    domain.StatusDraft,
		Snapshot:  pages.PageVersionSnapshot{},
		CreatedBy: uuid.New(),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("seed page version: %v", err)
	}

	bridge := blocks.NewEmbeddedBlockBridge(
		blockSvc,
		localeRepo,
		stubContentPageResolver{pages: map[uuid.UUID][]uuid.UUID{contentID: {pageID}}},
	)

	pageSvc := pages.NewService(
		pageRepo,
		contentRepo,
		localeRepo,
		pages.WithEmbeddedBlockBridge(bridge),
		pages.WithBlockService(blockSvc),
		pages.WithPageVersioningEnabled(true),
		pages.WithPageClock(func() time.Time { return now }),
	)

	preview, err := pageSvc.PreviewDraft(ctx, pages.PreviewPageDraftRequest{
		PageID:  pageID,
		Version: 1,
	})
	if err != nil {
		t.Fatalf("preview draft: %v", err)
	}
	if preview == nil || preview.Page == nil {
		t.Fatalf("expected preview page")
	}
	if len(preview.Page.Blocks) != 1 {
		t.Fatalf("expected 1 preview block, got %d", len(preview.Page.Blocks))
	}
	if len(preview.Page.Blocks[0].Translations) != 1 {
		t.Fatalf("expected 1 block translation")
	}
	fields := preview.Page.Blocks[0].Translations[0].Content
	if fields["title"] != "Hello" {
		t.Fatalf("expected migrated title Hello got %v", fields["title"])
	}
	if _, ok := fields["headline"]; ok {
		t.Fatalf("expected headline to be removed in preview")
	}

	stored, err := contentRepo.GetByID(ctx, contentID)
	if err != nil {
		t.Fatalf("fetch stored content: %v", err)
	}
	blocksPayload, ok := content.ExtractEmbeddedBlocks(stored.Translations[0].Content)
	if !ok || len(blocksPayload) != 1 {
		t.Fatalf("expected stored embedded blocks")
	}
	if blocksPayload[0]["headline"] != "Hello" {
		t.Fatalf("expected stored headline Hello got %v", blocksPayload[0]["headline"])
	}
}
