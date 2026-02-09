package cms_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/widgets"
	"github.com/google/uuid"
)

func TestModule_Widgets_AssignWidgetToAreaDuplicateReturnsStableSentinel(t *testing.T) {
	t.Parallel()

	cfg := cms.DefaultConfig()
	cfg.Features.Widgets = true

	module, err := cms.New(cfg)
	if err != nil {
		t.Fatalf("new module: %v", err)
	}

	ctx := context.Background()
	authorID := uuid.New()
	areaCode := "sidebar.primary"

	localeRecord, err := module.Container().LocaleRepository().GetByCode(ctx, "en")
	if err != nil {
		t.Fatalf("resolve locale: %v", err)
	}

	if _, err := module.Widgets().RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
		Code:  areaCode,
		Name:  "Primary Sidebar",
		Scope: widgets.AreaScopeGlobal,
	}); err != nil {
		t.Fatalf("register area: %v", err)
	}

	definition, err := module.Widgets().RegisterDefinition(ctx, widgets.RegisterDefinitionInput{
		Name:   "promo",
		Schema: map[string]any{"type": "object"},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	instance, err := module.Widgets().CreateInstance(ctx, widgets.CreateInstanceInput{
		DefinitionID: definition.ID,
		CreatedBy:    authorID,
		UpdatedBy:    authorID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if _, err := module.Widgets().AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   areaCode,
		LocaleID:   &localeRecord.ID,
		InstanceID: instance.ID,
	}); err != nil {
		t.Fatalf("first assignment: %v", err)
	}

	_, err = module.Widgets().AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   areaCode,
		LocaleID:   &localeRecord.ID,
		InstanceID: instance.ID,
	})
	if !errors.Is(err, widgets.ErrAreaPlacementExists) {
		t.Fatalf("expected ErrAreaPlacementExists, got %v", err)
	}
}
