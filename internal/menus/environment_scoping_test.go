package menus_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/google/uuid"
)

func TestMenuServiceEnvironmentScopedCodes(t *testing.T) {
	ctx := context.Background()
	envRepo := environments.NewMemoryRepository()
	envSvc := environments.NewService(envRepo)

	if _, err := envSvc.CreateEnvironment(ctx, environments.CreateEnvironmentInput{Key: "default", IsDefault: true}); err != nil {
		t.Fatalf("create default environment: %v", err)
	}
	if _, err := envSvc.CreateEnvironment(ctx, environments.CreateEnvironmentInput{Key: "staging"}); err != nil {
		t.Fatalf("create staging environment: %v", err)
	}

	fixture := loadServiceFixture(t)
	svc := newServiceWithLocales(t, fixture.locales(), func(menus.AddMenuItemInput) uuid.UUID { return uuid.New() }, nil,
		menus.WithEnvironmentService(envSvc),
	)

	defaultMenu, err := svc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:           "main",
		EnvironmentKey: "default",
		CreatedBy:      uuid.Nil,
		UpdatedBy:      uuid.Nil,
	})
	if err != nil {
		t.Fatalf("create default menu: %v", err)
	}

	if _, err := svc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:           "main",
		EnvironmentKey: "staging",
		CreatedBy:      uuid.Nil,
		UpdatedBy:      uuid.Nil,
	}); err != nil {
		t.Fatalf("create staging menu: %v", err)
	}

	if _, err := svc.CreateMenu(ctx, menus.CreateMenuInput{
		Code:           "main",
		EnvironmentKey: "default",
		CreatedBy:      uuid.Nil,
		UpdatedBy:      uuid.Nil,
	}); !errors.Is(err, menus.ErrMenuCodeExists) {
		t.Fatalf("expected ErrMenuCodeExists, got %v", err)
	}

	got, err := svc.GetMenuByCode(ctx, "main", "default")
	if err != nil {
		t.Fatalf("get menu by code: %v", err)
	}
	if got.ID != defaultMenu.ID {
		t.Fatalf("expected default menu, got %s", got.ID)
	}

}
