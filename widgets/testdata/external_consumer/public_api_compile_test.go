package externalconsumer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/blocks"
	"github.com/goliatone/go-cms/content"
	"github.com/goliatone/go-cms/pages"
	"github.com/google/uuid"
)

var _ func(*cms.Module) content.Service = (*cms.Module).Content
var _ func(*cms.Module) content.ContentTypeService = (*cms.Module).ContentTypes
var _ func(*cms.Module) pages.Service = (*cms.Module).Pages
var _ func(*cms.Module) blocks.Service = (*cms.Module).Blocks

func compileTypedPublicCalls(
	ctx context.Context,
	contentSvc content.Service,
	contentTypeSvc content.ContentTypeService,
	pageSvc pages.Service,
	blockSvc blocks.Service,
) {
	actorID := uuid.New()
	contentID := uuid.New()
	contentTypeID := uuid.New()
	pageID := uuid.New()
	blockDefinitionID := uuid.New()
	instanceID := uuid.New()
	localeID := uuid.New()
	templateID := uuid.New()
	version := 1
	region := "main"
	position := 0
	updatedTypeName := "Article v2"
	updatedDefinitionName := "hero-alt"

	_, _ = contentSvc.Create(ctx, content.CreateContentRequest{
		ContentTypeID: contentTypeID,
		Slug:          "typed-content",
		Status:        "draft",
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
		Translations: []content.ContentTranslationInput{{
			Locale:  "en",
			Title:   "Typed Content",
			Content: map[string]any{"headline": "Hello"},
		}},
		AllowMissingTranslations: true,
	})
	_, _ = contentSvc.Get(ctx, contentID, content.WithTranslations())
	_, _ = contentSvc.List(ctx, content.WithTranslations())
	_, _ = contentSvc.CheckTranslations(ctx, contentID, []string{"en"}, content.TranslationCheckOptions{})
	_, _ = contentSvc.AvailableLocales(ctx, contentID, content.TranslationCheckOptions{})
	_, _ = contentSvc.Update(ctx, content.UpdateContentRequest{ID: contentID, UpdatedBy: actorID})
	_ = contentSvc.Delete(ctx, content.DeleteContentRequest{ID: contentID, DeletedBy: actorID, HardDelete: true})
	_, _ = contentSvc.UpdateTranslation(ctx, content.UpdateContentTranslationRequest{
		ContentID: contentID,
		Locale:    "en",
		Title:     "Updated",
		Content:   map[string]any{"headline": "Updated"},
		UpdatedBy: actorID,
	})
	_ = contentSvc.DeleteTranslation(ctx, content.DeleteContentTranslationRequest{ContentID: contentID, Locale: "en", DeletedBy: actorID})
	_, _ = contentSvc.Schedule(ctx, content.ScheduleContentRequest{ContentID: contentID, PublishAt: ptrTime(time.Now().UTC()), ScheduledBy: actorID})
	_, _ = contentSvc.CreateDraft(ctx, content.CreateContentDraftRequest{ContentID: contentID, CreatedBy: actorID, UpdatedBy: actorID})
	_, _ = contentSvc.PublishDraft(ctx, content.PublishContentDraftRequest{ContentID: contentID, Version: version, PublishedBy: actorID})
	_, _ = contentSvc.PreviewDraft(ctx, content.PreviewContentDraftRequest{ContentID: contentID, Version: version})
	_, _ = contentSvc.ListVersions(ctx, contentID)
	_, _ = contentSvc.RestoreVersion(ctx, content.RestoreContentVersionRequest{ContentID: contentID, Version: version, RestoredBy: actorID})

	_, _ = contentTypeSvc.Create(ctx, content.CreateContentTypeRequest{
		Name:      "Article",
		Schema:    map[string]any{"fields": []any{"headline"}},
		CreatedBy: actorID,
		UpdatedBy: actorID,
	})
	_, _ = contentTypeSvc.Update(ctx, content.UpdateContentTypeRequest{ID: contentTypeID, Name: &updatedTypeName, UpdatedBy: actorID})
	_ = contentTypeSvc.Delete(ctx, content.DeleteContentTypeRequest{ID: contentTypeID, DeletedBy: actorID, HardDelete: true})
	_, _ = contentTypeSvc.Get(ctx, contentTypeID)
	_, _ = contentTypeSvc.GetBySlug(ctx, "article")
	_, _ = contentTypeSvc.List(ctx)
	_, _ = contentTypeSvc.Search(ctx, "article")

	_, _ = pageSvc.Create(ctx, pages.CreatePageRequest{
		ContentID:  contentID,
		TemplateID: templateID,
		Slug:       "home",
		Status:     "draft",
		CreatedBy:  actorID,
		UpdatedBy:  actorID,
		Translations: []pages.PageTranslationInput{{
			Locale: "en",
			Title:  "Home",
			Path:   "/",
		}},
		AllowMissingTranslations: true,
	})
	_, _ = pageSvc.Get(ctx, pageID)
	_, _ = pageSvc.List(ctx)
	_, _ = pageSvc.CheckTranslations(ctx, pageID, []string{"en"}, pages.TranslationCheckOptions{})
	_, _ = pageSvc.AvailableLocales(ctx, pageID, pages.TranslationCheckOptions{})
	_, _ = pageSvc.Update(ctx, pages.UpdatePageRequest{ID: pageID, UpdatedBy: actorID})
	_ = pageSvc.Delete(ctx, pages.DeletePageRequest{ID: pageID, DeletedBy: actorID, HardDelete: true})
	_, _ = pageSvc.UpdateTranslation(ctx, pages.UpdatePageTranslationRequest{PageID: pageID, Locale: "en", Title: "Home", Path: "/", UpdatedBy: actorID})
	_ = pageSvc.DeleteTranslation(ctx, pages.DeletePageTranslationRequest{PageID: pageID, Locale: "en", DeletedBy: actorID})
	_, _ = pageSvc.Move(ctx, pages.MovePageRequest{PageID: pageID, NewParentID: &pageID, ActorID: actorID})
	_, _ = pageSvc.Duplicate(ctx, pages.DuplicatePageRequest{PageID: pageID, Slug: "home-copy", CreatedBy: actorID, UpdatedBy: actorID})
	_, _ = pageSvc.Schedule(ctx, pages.SchedulePageRequest{PageID: pageID, PublishAt: ptrTime(time.Now().UTC()), ScheduledBy: actorID})
	_, _ = pageSvc.CreateDraft(ctx, pages.CreatePageDraftRequest{PageID: pageID, CreatedBy: actorID, UpdatedBy: actorID})
	_, _ = pageSvc.PublishDraft(ctx, pages.PublishPageDraftRequest{PageID: pageID, Version: version, PublishedBy: actorID})
	_, _ = pageSvc.PreviewDraft(ctx, pages.PreviewPageDraftRequest{PageID: pageID, Version: version})
	_, _ = pageSvc.ListVersions(ctx, pageID)
	_, _ = pageSvc.RestoreVersion(ctx, pages.RestorePageVersionRequest{PageID: pageID, Version: version, RestoredBy: actorID})

	_, _ = blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{Name: "hero", Schema: map[string]any{"fields": []any{"headline"}}})
	_, _ = blockSvc.GetDefinition(ctx, blockDefinitionID)
	_, _ = blockSvc.ListDefinitions(ctx)
	_, _ = blockSvc.UpdateDefinition(ctx, blocks.UpdateDefinitionInput{ID: blockDefinitionID, Name: &updatedDefinitionName})
	_ = blockSvc.DeleteDefinition(ctx, blocks.DeleteDefinitionRequest{ID: blockDefinitionID, HardDelete: true})
	_ = blockSvc.SyncRegistry(ctx)
	_, _ = blockSvc.CreateDefinitionVersion(ctx, blocks.CreateDefinitionVersionInput{DefinitionID: blockDefinitionID, Schema: map[string]any{"fields": []any{"headline"}}})
	_, _ = blockSvc.GetDefinitionVersion(ctx, blockDefinitionID, "hero@v1.0.0")
	_, _ = blockSvc.ListDefinitionVersions(ctx, blockDefinitionID)

	_, _ = blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID:  blockDefinitionID,
		PageID:        &pageID,
		Region:        region,
		Position:      position,
		Configuration: map[string]any{"headline": "Hello"},
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
	})
	_, _ = blockSvc.ListPageInstances(ctx, pageID)
	_, _ = blockSvc.ListGlobalInstances(ctx)
	_, _ = blockSvc.UpdateInstance(ctx, blocks.UpdateInstanceInput{InstanceID: instanceID, Region: &region, Position: &position, UpdatedBy: actorID})
	_ = blockSvc.DeleteInstance(ctx, blocks.DeleteInstanceRequest{ID: instanceID, DeletedBy: actorID, HardDelete: true})

	_, _ = blockSvc.AddTranslation(ctx, blocks.AddTranslationInput{BlockInstanceID: instanceID, LocaleID: localeID, Content: map[string]any{"headline": "Hello"}})
	_, _ = blockSvc.UpdateTranslation(ctx, blocks.UpdateTranslationInput{BlockInstanceID: instanceID, LocaleID: localeID, Content: map[string]any{"headline": "Updated"}, UpdatedBy: actorID})
	_ = blockSvc.DeleteTranslation(ctx, blocks.DeleteTranslationRequest{BlockInstanceID: instanceID, LocaleID: localeID, DeletedBy: actorID, AllowMissingTranslations: true})
	_, _ = blockSvc.GetTranslation(ctx, instanceID, localeID)
	_, _ = blockSvc.CreateDraft(ctx, blocks.CreateInstanceDraftRequest{InstanceID: instanceID, CreatedBy: actorID, UpdatedBy: actorID})
	_, _ = blockSvc.PublishDraft(ctx, blocks.PublishInstanceDraftRequest{InstanceID: instanceID, Version: version, PublishedBy: actorID})
	_, _ = blockSvc.ListVersions(ctx, instanceID)
	_, _ = blockSvc.RestoreVersion(ctx, blocks.RestoreInstanceVersionRequest{InstanceID: instanceID, Version: version, RestoredBy: actorID})

	_ = errors.Is(blocks.ErrTranslationsDisabled, blocks.ErrTranslationsDisabled)
	_ = errors.Is(content.ErrSchedulingDisabled, content.ErrSchedulingDisabled)
	_ = errors.Is(pages.ErrVersioningDisabled, pages.ErrVersioningDisabled)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func TestExternalPublicAPICompiles(t *testing.T) {}
