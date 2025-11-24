package widgetscmd

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/google/uuid"
)

type trackingWidgetService struct {
	widgets.Service
	syncCalls    int
	resolveCalls int
	lastResolve  widgets.ResolveAreaInput
}

func (t *trackingWidgetService) SyncRegistry(ctx context.Context) error {
	t.syncCalls++
	if t.Service != nil {
		return t.Service.SyncRegistry(ctx)
	}
	return nil
}

func (t *trackingWidgetService) ResolveArea(ctx context.Context, input widgets.ResolveAreaInput) ([]*widgets.ResolvedWidget, error) {
	t.resolveCalls++
	t.lastResolve = input
	if t.Service != nil {
		return t.Service.ResolveArea(ctx, input)
	}
	return nil, nil
}

func TestSyncWidgetRegistryHandlerRegistersDefinitions(t *testing.T) {
	ctx := context.Background()
	defRepo := widgets.NewMemoryDefinitionRepository()
	instRepo := widgets.NewMemoryInstanceRepository()
	translationRepo := widgets.NewMemoryTranslationRepository()
	areaRepo := widgets.NewMemoryAreaDefinitionRepository()
	placementRepo := widgets.NewMemoryAreaPlacementRepository()
	registry := widgets.NewRegistry()

	baseService := widgets.NewService(
		defRepo,
		instRepo,
		translationRepo,
		widgets.WithAreaDefinitionRepository(areaRepo),
		widgets.WithAreaPlacementRepository(placementRepo),
		widgets.WithRegistry(registry),
	)

	tracking := &trackingWidgetService{Service: baseService}
	handler := NewSyncWidgetRegistryHandler(tracking, logging.NoOp(), FeatureGates{
		WidgetsEnabled: func() bool { return true },
	})

	registry.Register(widgets.RegisterDefinitionInput{
		Name:   "announcement_panel",
		Schema: map[string]any{"fields": []any{"title"}},
	})

	if err := handler.Execute(ctx, SyncWidgetRegistryCommand{}); err != nil {
		t.Fatalf("execute sync: %v", err)
	}
	if tracking.syncCalls != 1 {
		t.Fatalf("expected sync calls 1, got %d", tracking.syncCalls)
	}

	defs, err := tracking.ListDefinitions(ctx)
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	found := false
	for _, def := range defs {
		if def != nil && def.Name == "announcement_panel" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected announcement_panel definition to be registered")
	}
}

func TestSyncWidgetRegistryHandlerFeatureDisabled(t *testing.T) {
	baseService := widgets.NewService(
		widgets.NewMemoryDefinitionRepository(),
		widgets.NewMemoryInstanceRepository(),
		widgets.NewMemoryTranslationRepository(),
		widgets.WithAreaDefinitionRepository(widgets.NewMemoryAreaDefinitionRepository()),
		widgets.WithAreaPlacementRepository(widgets.NewMemoryAreaPlacementRepository()),
	)
	tracking := &trackingWidgetService{Service: baseService}

	handler := NewSyncWidgetRegistryHandler(tracking, logging.NoOp(), FeatureGates{
		WidgetsEnabled: func() bool { return false },
	})

	err := handler.Execute(context.Background(), SyncWidgetRegistryCommand{})
	if err == nil {
		t.Fatal("expected module disabled error")
	}
	if !errors.Is(err, ErrWidgetsModuleDisabled) {
		t.Fatalf("expected ErrWidgetsModuleDisabled, got %v", err)
	}
	if tracking.syncCalls != 0 {
		t.Fatalf("expected no sync calls, got %d", tracking.syncCalls)
	}
}

func TestRefreshWidgetAreaHandlerResolvesArea(t *testing.T) {
	ctx := context.Background()
	defRepo := widgets.NewMemoryDefinitionRepository()
	instRepo := widgets.NewMemoryInstanceRepository()
	translationRepo := widgets.NewMemoryTranslationRepository()
	areaRepo := widgets.NewMemoryAreaDefinitionRepository()
	placementRepo := widgets.NewMemoryAreaPlacementRepository()
	baseService := widgets.NewService(
		defRepo,
		instRepo,
		translationRepo,
		widgets.WithAreaDefinitionRepository(areaRepo),
		widgets.WithAreaPlacementRepository(placementRepo),
	)

	tracking := &trackingWidgetService{Service: baseService}

	handler := NewRefreshWidgetAreaHandler(tracking, logging.NoOp(), FeatureGates{
		WidgetsEnabled: func() bool { return true },
	})

	def, err := tracking.RegisterDefinition(ctx, widgets.RegisterDefinitionInput{
		Name:   "feature_card",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	instance, err := tracking.CreateInstance(ctx, widgets.CreateInstanceInput{
		DefinitionID: def.ID,
		Position:     0,
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if _, err := tracking.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
		Code: "homepage.hero",
		Name: "Homepage Hero",
	}); err != nil {
		t.Fatalf("register area: %v", err)
	}

	if _, err := tracking.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   "homepage.hero",
		InstanceID: instance.ID,
	}); err != nil {
		t.Fatalf("assign widget: %v", err)
	}

	if err := handler.Execute(ctx, RefreshWidgetAreaCommand{AreaCode: "homepage.hero"}); err != nil {
		t.Fatalf("execute refresh: %v", err)
	}
	if tracking.resolveCalls != 1 {
		t.Fatalf("expected resolve calls 1, got %d", tracking.resolveCalls)
	}
	if tracking.lastResolve.AreaCode != "homepage.hero" {
		t.Fatalf("expected area code homepage.hero, got %s", tracking.lastResolve.AreaCode)
	}
}

func TestRefreshWidgetAreaHandlerFeatureDisabled(t *testing.T) {
	baseService := widgets.NewService(
		widgets.NewMemoryDefinitionRepository(),
		widgets.NewMemoryInstanceRepository(),
		widgets.NewMemoryTranslationRepository(),
		widgets.WithAreaDefinitionRepository(widgets.NewMemoryAreaDefinitionRepository()),
		widgets.WithAreaPlacementRepository(widgets.NewMemoryAreaPlacementRepository()),
	)
	tracking := &trackingWidgetService{Service: baseService}

	handler := NewRefreshWidgetAreaHandler(tracking, logging.NoOp(), FeatureGates{
		WidgetsEnabled: func() bool { return false },
	})

	err := handler.Execute(context.Background(), RefreshWidgetAreaCommand{AreaCode: "homepage.hero"})
	if err == nil {
		t.Fatal("expected module disabled error")
	}
	if !errors.Is(err, ErrWidgetsModuleDisabled) {
		t.Fatalf("expected ErrWidgetsModuleDisabled, got %v", err)
	}
	if tracking.resolveCalls != 0 {
		t.Fatalf("expected no resolve calls, got %d", tracking.resolveCalls)
	}
}
