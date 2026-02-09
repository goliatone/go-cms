package cms_test

import (
	"context"
	"slices"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/blocks"
	"github.com/goliatone/go-cms/content"
	"github.com/goliatone/go-cms/pages"
	"github.com/google/uuid"
)

func TestPublicContractsSupportTypedCRUDAndTranslations(t *testing.T) {
	t.Parallel()

	cfg := cms.DefaultConfig()
	cfg.I18N.Locales = []string{"en", "es"}
	cfg.I18N.RequireTranslations = true

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("cms.New: %v", err)
	}

	ctx := context.Background()
	actorID := uuid.New()

	contentTypeSvc := module.ContentTypes()
	contentTypeRecord, err := contentTypeSvc.Create(ctx, content.CreateContentTypeRequest{
		Name:      "Article",
		Schema:    map[string]any{"fields": []any{"title", "body"}},
		CreatedBy: actorID,
		UpdatedBy: actorID,
	})
	if err != nil {
		t.Fatalf("content types create: %v", err)
	}

	if _, err := contentTypeSvc.Get(ctx, contentTypeRecord.ID); err != nil {
		t.Fatalf("content types get: %v", err)
	}
	if _, err := contentTypeSvc.GetBySlug(ctx, contentTypeRecord.Slug); err != nil {
		t.Fatalf("content types get by slug: %v", err)
	}
	if records, err := contentTypeSvc.List(ctx); err != nil || len(records) == 0 {
		t.Fatalf("content types list: len=%d err=%v", len(records), err)
	}
	if records, err := contentTypeSvc.Search(ctx, "article"); err != nil || len(records) == 0 {
		t.Fatalf("content types search: len=%d err=%v", len(records), err)
	}
	updatedTypeName := "Article Post"
	if _, err := contentTypeSvc.Update(ctx, content.UpdateContentTypeRequest{
		ID:        contentTypeRecord.ID,
		Name:      &updatedTypeName,
		UpdatedBy: actorID,
	}); err != nil {
		t.Fatalf("content types update: %v", err)
	}

	contentSvc := module.Content()
	contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID:            contentTypeRecord.ID,
		Slug:                     "hello-world",
		Status:                   "draft",
		CreatedBy:                actorID,
		UpdatedBy:                actorID,
		AllowMissingTranslations: true,
		Translations: []content.ContentTranslationInput{{
			Locale:  "en",
			Title:   "Hello world",
			Content: map[string]any{"title": "Hello", "body": "Initial"},
		}},
	})
	if err != nil {
		t.Fatalf("content create: %v", err)
	}

	if _, err := contentSvc.Get(ctx, contentRecord.ID, content.WithTranslations()); err != nil {
		t.Fatalf("content get: %v", err)
	}
	if records, err := contentSvc.List(ctx, content.WithTranslations()); err != nil || len(records) == 0 {
		t.Fatalf("content list: len=%d err=%v", len(records), err)
	}
	missingLocales, err := contentSvc.CheckTranslations(ctx, contentRecord.ID, []string{"en", "es"}, content.TranslationCheckOptions{})
	if err != nil {
		t.Fatalf("content check translations: %v", err)
	}
	if !slices.Contains(missingLocales, "es") {
		t.Fatalf("expected missing locale es, got %v", missingLocales)
	}
	availableLocales, err := contentSvc.AvailableLocales(ctx, contentRecord.ID, content.TranslationCheckOptions{})
	if err != nil {
		t.Fatalf("content available locales: %v", err)
	}
	if !slices.Contains(availableLocales, "en") {
		t.Fatalf("expected available locale en, got %v", availableLocales)
	}
	if _, err := contentSvc.Update(ctx, content.UpdateContentRequest{
		ID:                       contentRecord.ID,
		Status:                   "draft",
		UpdatedBy:                actorID,
		AllowMissingTranslations: true,
		Translations: []content.ContentTranslationInput{{
			Locale:  "en",
			Title:   "Hello updated",
			Content: map[string]any{"title": "Hello", "body": "Updated"},
		}},
	}); err != nil {
		t.Fatalf("content update: %v", err)
	}
	pageSvc := module.Pages()
	pageRecord, err := pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:                contentRecord.ID,
		TemplateID:               uuid.New(),
		Slug:                     "home",
		Status:                   "draft",
		CreatedBy:                actorID,
		UpdatedBy:                actorID,
		AllowMissingTranslations: true,
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Home",
			Path:   "/",
		}},
	})
	if err != nil {
		t.Fatalf("pages create: %v", err)
	}

	if _, err := pageSvc.Get(ctx, pageRecord.ID); err != nil {
		t.Fatalf("pages get: %v", err)
	}
	if records, err := pageSvc.List(ctx); err != nil || len(records) == 0 {
		t.Fatalf("pages list: len=%d err=%v", len(records), err)
	}
	missingPageLocales, err := pageSvc.CheckTranslations(ctx, pageRecord.ID, []string{"en", "es"}, pages.TranslationCheckOptions{})
	if err != nil {
		t.Fatalf("pages check translations: %v", err)
	}
	if !slices.Contains(missingPageLocales, "es") {
		t.Fatalf("expected missing page locale es, got %v", missingPageLocales)
	}
	availablePageLocales, err := pageSvc.AvailableLocales(ctx, pageRecord.ID, pages.TranslationCheckOptions{})
	if err != nil {
		t.Fatalf("pages available locales: %v", err)
	}
	if !slices.Contains(availablePageLocales, "en") {
		t.Fatalf("expected available page locale en, got %v", availablePageLocales)
	}
	if _, err := pageSvc.Update(ctx, pages.UpdatePageRequest{
		ID:                       pageRecord.ID,
		Status:                   "draft",
		UpdatedBy:                actorID,
		AllowMissingTranslations: true,
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Home updated",
			Path:   "/",
		}},
	}); err != nil {
		t.Fatalf("pages update: %v", err)
	}
	if _, err := pageSvc.UpdateTranslation(ctx, pages.UpdatePageTranslationRequest{
		PageID:    pageRecord.ID,
		Locale:    "en",
		Title:     "Home translated",
		Path:      "/",
		UpdatedBy: actorID,
	}); err != nil {
		t.Fatalf("pages update translation: %v", err)
	}

	blockSvc := module.Blocks()
	definitionRecord, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"headline"}},
	})
	if err != nil {
		t.Fatalf("blocks register definition: %v", err)
	}
	if _, err := blockSvc.GetDefinition(ctx, definitionRecord.ID); err != nil {
		t.Fatalf("blocks get definition: %v", err)
	}
	if records, err := blockSvc.ListDefinitions(ctx); err != nil || len(records) == 0 {
		t.Fatalf("blocks list definitions: len=%d err=%v", len(records), err)
	}
	if _, err := blockSvc.ListDefinitionVersions(ctx, definitionRecord.ID); err != nil {
		t.Fatalf("blocks list definition versions: %v", err)
	}
	updatedDefinitionName := "hero banner"
	if _, err := blockSvc.UpdateDefinition(ctx, blocks.UpdateDefinitionInput{
		ID:   definitionRecord.ID,
		Name: &updatedDefinitionName,
	}); err != nil {
		t.Fatalf("blocks update definition: %v", err)
	}

	instanceRecord, err := blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: definitionRecord.ID,
		PageID:       &pageRecord.ID,
		Region:       "main",
		Position:     0,
		Configuration: map[string]any{
			"headline": "Hello",
		},
		CreatedBy: actorID,
		UpdatedBy: actorID,
	})
	if err != nil {
		t.Fatalf("blocks create instance: %v", err)
	}
	if records, err := blockSvc.ListPageInstances(ctx, pageRecord.ID); err != nil || len(records) == 0 {
		t.Fatalf("blocks list page instances: len=%d err=%v", len(records), err)
	}
	position := 1
	if _, err := blockSvc.UpdateInstance(ctx, blocks.UpdateInstanceInput{
		InstanceID: instanceRecord.ID,
		Position:   &position,
		UpdatedBy:  actorID,
	}); err != nil {
		t.Fatalf("blocks update instance: %v", err)
	}

	localeID := uuid.New()
	if _, err := blockSvc.AddTranslation(ctx, blocks.AddTranslationInput{
		BlockInstanceID: instanceRecord.ID,
		LocaleID:        localeID,
		Content:         map[string]any{"headline": "Hello"},
	}); err != nil {
		t.Fatalf("blocks add translation: %v", err)
	}
	if _, err := blockSvc.GetTranslation(ctx, instanceRecord.ID, localeID); err != nil {
		t.Fatalf("blocks get translation: %v", err)
	}
	if _, err := blockSvc.UpdateTranslation(ctx, blocks.UpdateTranslationInput{
		BlockInstanceID: instanceRecord.ID,
		LocaleID:        localeID,
		Content:         map[string]any{"headline": "Hello updated"},
		UpdatedBy:       actorID,
	}); err != nil {
		t.Fatalf("blocks update translation: %v", err)
	}
	if err := blockSvc.DeleteTranslation(ctx, blocks.DeleteTranslationRequest{
		BlockInstanceID:          instanceRecord.ID,
		LocaleID:                 localeID,
		DeletedBy:                actorID,
		AllowMissingTranslations: true,
	}); err != nil {
		t.Fatalf("blocks delete translation: %v", err)
	}

	if err := blockSvc.DeleteInstance(ctx, blocks.DeleteInstanceRequest{
		ID:         instanceRecord.ID,
		DeletedBy:  actorID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("blocks delete instance: %v", err)
	}
	if err := blockSvc.DeleteDefinition(ctx, blocks.DeleteDefinitionRequest{
		ID:         definitionRecord.ID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("blocks delete definition: %v", err)
	}

	if err := pageSvc.Delete(ctx, pages.DeletePageRequest{
		ID:         pageRecord.ID,
		DeletedBy:  actorID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("pages delete: %v", err)
	}
	if err := contentSvc.Delete(ctx, content.DeleteContentRequest{
		ID:         contentRecord.ID,
		DeletedBy:  actorID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("content delete: %v", err)
	}
	if err := contentTypeSvc.Delete(ctx, content.DeleteContentTypeRequest{
		ID:         contentTypeRecord.ID,
		DeletedBy:  actorID,
		HardDelete: true,
	}); err != nil {
		t.Fatalf("content types delete: %v", err)
	}
}
