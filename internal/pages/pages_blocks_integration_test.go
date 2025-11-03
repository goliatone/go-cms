package pages_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/google/uuid"
)

func TestIntegrationListIncludesBlocks(t *testing.T) {
	ctx := context.Background()

	cfg := cms.DefaultConfig()
	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	contentSvc := container.ContentService()
	localeRepo := container.LocaleRepository()
	locale, err := localeRepo.GetByCode(ctx, "en")
	if err != nil {
		t.Fatalf("resolve locale: %v", err)
	}

	typeID := uuid.New()
	if repo, ok := container.ContentTypeRepository().(interface{ Put(*content.ContentType) }); ok {
		repo.Put(&content.ContentType{ID: typeID, Name: "page"})
	}

	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: typeID,
		Slug:          "integration",
		CreatedBy:     uuid.New(),
		UpdatedBy:     uuid.New(),
		Translations: []content.ContentTranslationInput{{
			Locale: "en",
			Title:  "Integration",
		}},
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	blockSvc := container.BlockService()
	definition, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		defs, listErr := blockSvc.ListDefinitions(ctx)
		if listErr != nil {
			t.Fatalf("list definitions: %v", listErr)
		}
		for _, def := range defs {
			if def.Name == "hero" {
				definition = def
				break
			}
		}
		if definition == nil {
			t.Fatalf("expected hero definition to exist")
		}
	}

	pageSvc := container.PageService()
	page, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  contentRecord.ID,
		TemplateID: uuid.New(),
		Slug:       "integration",
		CreatedBy:  uuid.New(),
		UpdatedBy:  uuid.New(),
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Integration",
			Path:   "/integration",
		}},
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	blockInstance, err := blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: definition.ID,
		PageID:       &page.ID,
		Region:       "hero",
		Position:     0,
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
	})
	if err != nil {
		t.Fatalf("create block instance: %v", err)
	}

	if _, err := blockSvc.AddTranslation(ctx, blocks.AddTranslationInput{
		BlockInstanceID: blockInstance.ID,
		LocaleID:        locale.ID,
		Content: map[string]any{
			"title": "Hello",
		},
	}); err != nil {
		t.Fatalf("add block translation: %v", err)
	}

	pagesList, err := pageSvc.List(ctx)
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(pagesList) == 0 {
		t.Fatalf("expected at least one page")
	}
	if len(pagesList[0].Blocks) == 0 {
		t.Fatalf("expected blocks to be enriched")
	}
}
