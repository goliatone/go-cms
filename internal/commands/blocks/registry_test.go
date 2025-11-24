package blockscmd

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/logging"
)

type trackingBlockService struct {
	blocks.Service
	syncCalls int
}

func (t *trackingBlockService) SyncRegistry(ctx context.Context) error {
	t.syncCalls++
	if t.Service != nil {
		return t.Service.SyncRegistry(ctx)
	}
	return nil
}

func TestSyncBlockRegistryHandlerRegistersDefinitions(t *testing.T) {
	ctx := context.Background()
	defRepo := blocks.NewMemoryDefinitionRepository()
	instRepo := blocks.NewMemoryInstanceRepository()
	translationRepo := blocks.NewMemoryTranslationRepository()
	versionRepo := blocks.NewMemoryInstanceVersionRepository()
	registry := blocks.NewRegistry()

	baseService := blocks.NewService(
		defRepo,
		instRepo,
		translationRepo,
		blocks.WithInstanceVersionRepository(versionRepo),
		blocks.WithRegistry(registry),
	)

	tracking := &trackingBlockService{Service: baseService}
	handler := NewSyncBlockRegistryHandler(tracking, logging.NoOp(), FeatureGates{
		BlocksEnabled: func() bool { return true },
	})

	registry.Register(blocks.RegisterDefinitionInput{
		Name:   "promo_banner",
		Schema: map[string]any{"fields": []any{"title"}},
	})

	if err := handler.Execute(ctx, SyncBlockRegistryCommand{}); err != nil {
		t.Fatalf("execute sync: %v", err)
	}
	if tracking.syncCalls != 1 {
		t.Fatalf("expected sync to be called once, got %d", tracking.syncCalls)
	}

	defs, err := tracking.ListDefinitions(ctx)
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	found := false
	for _, def := range defs {
		if def != nil && def.Name == "promo_banner" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected promo_banner definition to be registered")
	}
}

func TestSyncBlockRegistryHandlerFeatureDisabled(t *testing.T) {
	baseService := blocks.NewService(
		blocks.NewMemoryDefinitionRepository(),
		blocks.NewMemoryInstanceRepository(),
		blocks.NewMemoryTranslationRepository(),
		blocks.WithInstanceVersionRepository(blocks.NewMemoryInstanceVersionRepository()),
	)
	tracking := &trackingBlockService{Service: baseService}

	handler := NewSyncBlockRegistryHandler(tracking, logging.NoOp(), FeatureGates{
		BlocksEnabled: func() bool { return false },
	})

	err := handler.Execute(context.Background(), SyncBlockRegistryCommand{})
	if err == nil {
		t.Fatal("expected module disabled error")
	}
	if !errors.Is(err, ErrBlocksModuleDisabled) {
		t.Fatalf("expected ErrBlocksModuleDisabled, got %v", err)
	}
	if tracking.syncCalls != 0 {
		t.Fatalf("expected no sync calls, got %d", tracking.syncCalls)
	}
}
