package menuscmd

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/internal/menus"
)

type trackingMenuService struct {
	menus.Service
	invalidateCalls int
}

func (t *trackingMenuService) InvalidateCache(ctx context.Context) error {
	t.invalidateCalls++
	if t.Service != nil {
		return t.Service.InvalidateCache(ctx)
	}
	return nil
}

func TestInvalidateMenuCacheHandler(t *testing.T) {
	ctx := context.Background()
	baseService := menus.NewService(
		menus.NewMemoryMenuRepository(),
		menus.NewMemoryMenuItemRepository(),
		menus.NewMemoryMenuItemTranslationRepository(),
		content.NewMemoryLocaleRepository(),
	)

	tracking := &trackingMenuService{Service: baseService}
	handler := NewInvalidateMenuCacheHandler(tracking, logging.NoOp(), FeatureGates{
		MenusEnabled: func() bool { return true },
	})

	if err := handler.Execute(ctx, InvalidateMenuCacheCommand{}); err != nil {
		t.Fatalf("execute invalidate: %v", err)
	}
	if tracking.invalidateCalls != 1 {
		t.Fatalf("expected invalidate calls 1, got %d", tracking.invalidateCalls)
	}
}

func TestInvalidateMenuCacheHandlerFeatureDisabled(t *testing.T) {
	tracking := &trackingMenuService{Service: menus.NewService(
		menus.NewMemoryMenuRepository(),
		menus.NewMemoryMenuItemRepository(),
		menus.NewMemoryMenuItemTranslationRepository(),
		content.NewMemoryLocaleRepository(),
	)}

	handler := NewInvalidateMenuCacheHandler(tracking, logging.NoOp(), FeatureGates{
		MenusEnabled: func() bool { return false },
	})

	err := handler.Execute(context.Background(), InvalidateMenuCacheCommand{})
	if err == nil {
		t.Fatal("expected module disabled error")
	}
	if !errors.Is(err, ErrMenusModuleDisabled) {
		t.Fatalf("expected ErrMenusModuleDisabled, got %v", err)
	}
	if tracking.invalidateCalls != 0 {
		t.Fatalf("expected no invalidation calls, got %d", tracking.invalidateCalls)
	}
}
