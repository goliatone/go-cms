package externalconsumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/widgets"
	"github.com/google/uuid"
)

var _ func(*cms.Module) cms.MenuService = (*cms.Module).Menus
var _ func(*cms.Module) cms.AdminPageReadService = (*cms.Module).AdminPageRead
var _ func(*cms.Module) cms.AdminContentReadService = (*cms.Module).AdminContentRead
var _ func(*cms.Module) cms.AdminContentWriteService = (*cms.Module).AdminContentWrite
var _ func(*cms.Module) cms.AdminBlockReadService = (*cms.Module).AdminBlockRead
var _ func(*cms.Module) cms.AdminBlockWriteService = (*cms.Module).AdminBlockWrite
var _ func(*cms.Module) cms.LocaleService = (*cms.Module).Locales

func compilePublicRefinementCalls(
	ctx context.Context,
	menuSvc cms.MenuService,
	adminSvc cms.AdminPageReadService,
	adminContentRead cms.AdminContentReadService,
	adminContentWrite cms.AdminContentWriteService,
	adminBlockRead cms.AdminBlockReadService,
	adminBlockWrite cms.AdminBlockWriteService,
	localeSvc cms.LocaleService,
	widgetSvc widgets.Service,
) {
	actorID := uuid.New()

	_, _ = menuSvc.GetMenuByLocation(ctx, "site.primary")
	_, _ = menuSvc.ResolveNavigationByLocation(ctx, "site.primary", "en")
	_ = menuSvc.ResetMenuByCode(ctx, "primary", actorID, true)

	_ = cms.SeedMenu(ctx, cms.SeedMenuOptions{
		Menus:    menuSvc,
		MenuCode: "primary",
		Locale:   "en",
		Actor:    actorID,
		Items: []cms.SeedMenuItem{
			{
				Path:   "primary.home",
				Type:   "item",
				Target: map[string]any{"type": "url", "url": "/"},
				Translations: []cms.MenuItemTranslationInput{
					{Locale: "en", Label: "Home"},
				},
			},
		},
	})

	_, _, _ = adminSvc.List(ctx, cms.AdminPageListOptions{Locale: "en"})
	_, _ = adminSvc.Get(ctx, uuid.New().String(), cms.AdminPageGetOptions{Locale: "en"})
	_, _, _ = adminContentRead.List(ctx, cms.AdminContentListOptions{Locale: "en"})
	_, _ = adminContentRead.Get(ctx, uuid.New().String(), cms.AdminContentGetOptions{Locale: "en"})
	_, _ = adminContentWrite.Create(ctx, cms.AdminContentCreateRequest{
		ContentTypeID: uuid.New(),
		Title:         "Home",
		Slug:          "home",
		Locale:        "en",
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
	})
	_, _ = adminContentWrite.Update(ctx, cms.AdminContentUpdateRequest{
		ID:        uuid.New(),
		Title:     "Home",
		Locale:    "en",
		UpdatedBy: actorID,
	})
	_ = adminContentWrite.Delete(ctx, cms.AdminContentDeleteRequest{ID: uuid.New(), DeletedBy: actorID, HardDelete: true})
	_, _ = adminContentWrite.CreateTranslation(ctx, cms.AdminContentCreateTranslationRequest{
		SourceID:     uuid.New(),
		SourceLocale: "en",
		TargetLocale: "es",
		ActorID:      actorID,
	})
	_, _, _ = adminBlockRead.ListDefinitions(ctx, cms.AdminBlockDefinitionListOptions{})
	_, _ = adminBlockRead.GetDefinition(ctx, uuid.New().String(), cms.AdminBlockDefinitionGetOptions{})
	_, _ = adminBlockRead.ListDefinitionVersions(ctx, uuid.New().String())
	_, _ = adminBlockRead.ListContentBlocks(ctx, uuid.New().String(), cms.AdminBlockListOptions{Locale: "en"})
	_, _ = adminBlockWrite.CreateDefinition(ctx, cms.AdminBlockDefinitionCreateRequest{
		Name:   "Hero",
		Schema: map[string]any{"type": "object"},
	})
	_, _ = adminBlockWrite.UpdateDefinition(ctx, cms.AdminBlockDefinitionUpdateRequest{ID: uuid.New()})
	_ = adminBlockWrite.DeleteDefinition(ctx, cms.AdminBlockDefinitionDeleteRequest{ID: uuid.New(), HardDelete: true})
	_, _ = adminBlockWrite.SaveBlock(ctx, cms.AdminBlockSaveRequest{
		ID:           uuid.New(),
		DefinitionID: uuid.New(),
		ContentID:    uuid.New(),
		Region:       "main",
		Locale:       "en",
		Data:         map[string]any{"headline": "Hello"},
		UpdatedBy:    actorID,
	})
	_ = adminBlockWrite.DeleteBlock(ctx, cms.AdminBlockDeleteRequest{ID: uuid.New(), DeletedBy: actorID, HardDelete: true})

	_, _ = localeSvc.ResolveByCode(ctx, "en")
	_, localeErr := localeSvc.ResolveByCode(ctx, "unknown-locale")
	_ = errors.Is(localeErr, cms.ErrUnknownLocale)

	_, assignErr := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   "sidebar.primary",
		LocaleID:   ptrUUID(uuid.New()),
		InstanceID: uuid.New(),
	})
	_ = errors.Is(assignErr, widgets.ErrAreaPlacementExists)
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}

func TestExternalPublicRefinementAPICompiles(t *testing.T) {}
