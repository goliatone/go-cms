package externalconsumer_test

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/widgets"
	"github.com/google/uuid"
)

var _ func(*cms.Module) widgets.Service = (*cms.Module).Widgets

func compileTypedWidgetCalls(ctx context.Context, svc widgets.Service) {
	definitionID := uuid.New()
	instanceID := uuid.New()
	placementID := uuid.New()
	localeID := uuid.New()
	authorID := uuid.New()
	areaCode := "sidebar.primary"
	position := 1

	_, _ = svc.RegisterDefinition(ctx, widgets.RegisterDefinitionInput{
		Name: "promo",
		Schema: map[string]any{
			"type": "object",
		},
	})
	_, _ = svc.GetDefinition(ctx, definitionID)
	_, _ = svc.ListDefinitions(ctx)
	_ = svc.DeleteDefinition(ctx, widgets.DeleteDefinitionRequest{ID: definitionID, HardDelete: true})
	_ = svc.SyncRegistry(ctx)

	_, _ = svc.CreateInstance(ctx, widgets.CreateInstanceInput{
		DefinitionID:  definitionID,
		AreaCode:      &areaCode,
		Placement:     map[string]any{"page_id": "home", "locale": "en"},
		Configuration: map[string]any{"headline": "Hello"},
		Position:      0,
		CreatedBy:     authorID,
		UpdatedBy:     authorID,
	})
	_, _ = svc.UpdateInstance(ctx, widgets.UpdateInstanceInput{
		InstanceID:    instanceID,
		Configuration: map[string]any{"headline": "Updated"},
		Placement:     map[string]any{"page_id": "home", "locale": "en"},
		Position:      &position,
		UpdatedBy:     authorID,
		AreaCode:      &areaCode,
	})
	_, _ = svc.GetInstance(ctx, instanceID)
	_, _ = svc.ListInstancesByDefinition(ctx, definitionID)
	_, _ = svc.ListInstancesByArea(ctx, areaCode)
	_, _ = svc.ListAllInstances(ctx)
	_ = svc.DeleteInstance(ctx, widgets.DeleteInstanceRequest{
		InstanceID: instanceID,
		DeletedBy:  authorID,
		HardDelete: true,
	})

	_, _ = svc.AddTranslation(ctx, widgets.AddTranslationInput{
		InstanceID: instanceID,
		LocaleID:   localeID,
		Content:    map[string]any{"headline": "Hola"},
	})
	_, _ = svc.UpdateTranslation(ctx, widgets.UpdateTranslationInput{
		InstanceID: instanceID,
		LocaleID:   localeID,
		Content:    map[string]any{"headline": "Bonjour"},
	})
	_, _ = svc.GetTranslation(ctx, instanceID, localeID)
	_ = svc.DeleteTranslation(ctx, widgets.DeleteTranslationRequest{
		InstanceID: instanceID,
		LocaleID:   localeID,
	})

	_, _ = svc.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
		Code:  areaCode,
		Name:  "Sidebar Primary",
		Scope: widgets.AreaScopeGlobal,
	})
	_, _ = svc.ListAreaDefinitions(ctx)
	_, _ = svc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   areaCode,
		LocaleID:   &localeID,
		InstanceID: instanceID,
		Position:   &position,
		Metadata:   map[string]any{"page_id": "home", "locale": "en"},
	})
	_ = svc.RemoveWidgetFromArea(ctx, widgets.RemoveWidgetFromAreaInput{
		AreaCode:   areaCode,
		LocaleID:   &localeID,
		InstanceID: instanceID,
	})
	_, _ = svc.ReorderAreaWidgets(ctx, widgets.ReorderAreaWidgetsInput{
		AreaCode: areaCode,
		LocaleID: &localeID,
		Items:    []widgets.AreaWidgetOrder{{PlacementID: placementID, Position: 2}},
	})
	_, _ = svc.ResolveArea(ctx, widgets.ResolveAreaInput{
		AreaCode:          areaCode,
		LocaleID:          &localeID,
		FallbackLocaleIDs: []uuid.UUID{localeID},
		Now:               time.Now().UTC(),
	})
	_, _ = svc.EvaluateVisibility(ctx, &widgets.Instance{ID: instanceID}, widgets.VisibilityContext{
		Now:      time.Now().UTC(),
		LocaleID: &localeID,
	})
}

func TestExternalWidgetAPICompiles(t *testing.T) {}
