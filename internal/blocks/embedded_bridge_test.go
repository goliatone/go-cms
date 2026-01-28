package blocks_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	cmsschema "github.com/goliatone/go-cms/internal/schema"
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

func TestEmbeddedBlockBridgeBackfillFromLegacy(t *testing.T) {
	ctx := context.Background()

	contentRepo := content.NewMemoryContentRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentID := uuid.New()
	now := time.Unix(0, 0).UTC()
	record := &content.Content{
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
			Title:     "Home",
			Content: map[string]any{
				"body": "Welcome",
			},
			CreatedAt: now,
			UpdatedAt: now,
		}},
	}
	if _, err := contentRepo.Create(ctx, record); err != nil {
		t.Fatalf("seed content: %v", err)
	}

	blockSvc := newBlockService()
	def, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	pageID := uuid.New()
	instance, err := blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: def.ID,
		PageID:       &pageID,
		Region:       "blocks",
		Position:     0,
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if _, err := blockSvc.AddTranslation(ctx, blocks.AddTranslationInput{
		BlockInstanceID: instance.ID,
		LocaleID:        localeID,
		Content: map[string]any{
			"title": "Legacy Title",
		},
	}); err != nil {
		t.Fatalf("add translation: %v", err)
	}

	bridge := blocks.NewEmbeddedBlockBridge(
		blockSvc,
		localeRepo,
		stubContentPageResolver{pages: map[uuid.UUID][]uuid.UUID{contentID: {pageID}}},
		blocks.WithEmbeddedBlocksContentRepository(contentRepo),
	)

	report, err := bridge.BackfillFromLegacy(ctx, blocks.BackfillOptions{})
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if report.ContentCount != 1 {
		t.Fatalf("expected 1 content backfilled, got %d", report.ContentCount)
	}
	if len(report.Errors) > 0 {
		t.Fatalf("unexpected backfill errors: %v", report.Errors)
	}

	stored, err := contentRepo.GetByID(ctx, contentID)
	if err != nil {
		t.Fatalf("fetch content: %v", err)
	}
	if len(stored.Translations) != 1 {
		t.Fatalf("expected 1 translation, got %d", len(stored.Translations))
	}
	blocksPayload, ok := content.ExtractEmbeddedBlocks(stored.Translations[0].Content)
	if !ok || len(blocksPayload) != 1 {
		t.Fatalf("expected embedded blocks to be backfilled")
	}
	block := blocksPayload[0]
	if blockType, ok := block[content.EmbeddedBlockTypeKey].(string); !ok || blockType != "hero" {
		t.Fatalf("expected embedded block type hero, got %v", block[content.EmbeddedBlockTypeKey])
	}
	if title, ok := block["title"].(string); !ok || title != "Legacy Title" {
		t.Fatalf("expected embedded block title Legacy Title, got %v", block["title"])
	}
	if _, ok := block[content.EmbeddedBlockMetaKey]; !ok {
		t.Fatalf("expected embedded block metadata")
	}
}

func TestEmbeddedBlockBridgeListConflicts(t *testing.T) {
	ctx := context.Background()

	contentRepo := content.NewMemoryContentRepository()
	localeRepo := content.NewMemoryLocaleRepository()

	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	contentID := uuid.New()
	now := time.Unix(0, 0).UTC()
	record := &content.Content{
		ID:            contentID,
		ContentTypeID: uuid.New(),
		Status:        "draft",
		Slug:          "conflict",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		CreatedAt:     now,
		UpdatedAt:     now,
		Translations: []*content.ContentTranslation{{
			ID:        uuid.New(),
			ContentID: contentID,
			LocaleID:  localeID,
			Title:     "Conflict",
			Content: map[string]any{
				content.EmbeddedBlocksKey: []map[string]any{{
					content.EmbeddedBlockTypeKey: "hero",
					"title":                      "Embedded Title",
				}},
			},
			CreatedAt: now,
			UpdatedAt: now,
		}},
	}
	if _, err := contentRepo.Create(ctx, record); err != nil {
		t.Fatalf("seed content: %v", err)
	}

	blockSvc := newBlockService()
	def, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	pageID := uuid.New()
	instance, err := blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: def.ID,
		PageID:       &pageID,
		Region:       "blocks",
		Position:     0,
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if _, err := blockSvc.AddTranslation(ctx, blocks.AddTranslationInput{
		BlockInstanceID: instance.ID,
		LocaleID:        localeID,
		Content: map[string]any{
			"title": "Legacy Title",
		},
	}); err != nil {
		t.Fatalf("add translation: %v", err)
	}

	bridge := blocks.NewEmbeddedBlockBridge(
		blockSvc,
		localeRepo,
		stubContentPageResolver{pages: map[uuid.UUID][]uuid.UUID{contentID: {pageID}}},
		blocks.WithEmbeddedBlocksContentRepository(contentRepo),
	)

	conflicts, err := bridge.ListConflicts(ctx, blocks.ConflictReportOptions{ContentIDs: []uuid.UUID{contentID}})
	if err != nil {
		t.Fatalf("list conflicts: %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Issue != content.ConflictContentMismatch {
		t.Fatalf("expected conflict issue %s, got %s", content.ConflictContentMismatch, conflicts[0].Issue)
	}
}

func TestEmbeddedBlockBridgeValidateBlockAvailability(t *testing.T) {
	ctx := context.Background()

	localeRepo := content.NewMemoryLocaleRepository()
	localeID := uuid.New()
	localeRepo.Put(&content.Locale{ID: localeID, Code: "en", Display: "English"})

	blockSvc := newBlockService()
	bridge := blocks.NewEmbeddedBlockBridge(blockSvc, localeRepo, stubContentPageResolver{})

	availability := cmsschema.BlockAvailability{
		Allow: []string{"hero"},
	}
	blocksPayload := []map[string]any{
		{content.EmbeddedBlockTypeKey: "promo"},
		{content.EmbeddedBlockTypeKey: "hero"},
	}

	err := bridge.ValidateBlockAvailability(ctx, "page", availability, blocksPayload)
	if err == nil {
		t.Fatal("expected availability validation error")
	}
	var validationErr *content.EmbeddedBlockValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected embedded block validation error, got %v", err)
	}
	if len(validationErr.Issues) != 1 {
		t.Fatalf("expected 1 availability issue, got %d", len(validationErr.Issues))
	}
	if validationErr.Issues[0].Type != "promo" {
		t.Fatalf("expected promo block issue, got %v", validationErr.Issues[0].Type)
	}
}
