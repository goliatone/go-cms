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
var _ func(*cms.Module) cms.LocaleService = (*cms.Module).Locales

func compilePublicRefinementCalls(
	ctx context.Context,
	menuSvc cms.MenuService,
	adminSvc cms.AdminPageReadService,
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
