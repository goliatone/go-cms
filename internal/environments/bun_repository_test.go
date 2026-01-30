package environments_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/environments"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestBunEnvironmentRepositoryCRUD(t *testing.T) {
	ctx := context.Background()

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)

	if _, err := bunDB.NewCreateTable().Model((*environments.Environment)(nil)).IfNotExists().Exec(ctx); err != nil {
		t.Fatalf("create environments table: %v", err)
	}

	repo := environments.NewBunEnvironmentRepository(bunDB)
	now := time.Date(2024, 6, 2, 10, 0, 0, 0, time.UTC)

	env := &environments.Environment{
		ID:        uuid.MustParse("00000000-0000-0000-0000-00000000d001"),
		Key:       "dev",
		Name:      "Development",
		IsActive:  true,
		IsDefault: true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	created, err := repo.Create(ctx, env)
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	if created.ID != env.ID {
		t.Fatalf("expected id %s, got %s", env.ID, created.ID)
	}

	byID, err := repo.GetByID(ctx, env.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID.Key != "dev" {
		t.Fatalf("expected key dev, got %s", byID.Key)
	}

	byKey, err := repo.GetByKey(ctx, "dev")
	if err != nil {
		t.Fatalf("get by key: %v", err)
	}
	if byKey.ID != env.ID {
		t.Fatalf("expected id %s, got %s", env.ID, byKey.ID)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list environments: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 environment, got %d", len(list))
	}

	active, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 || active[0].ID != env.ID {
		t.Fatalf("expected active environment")
	}

	def, err := repo.GetDefault(ctx)
	if err != nil {
		t.Fatalf("get default: %v", err)
	}
	if def.ID != env.ID {
		t.Fatalf("expected default %s, got %s", env.ID, def.ID)
	}

	updatedName := "Dev"
	env.Name = updatedName
	env.IsActive = false
	env.IsDefault = false
	env.UpdatedAt = now.Add(time.Hour)
	updated, err := repo.Update(ctx, env)
	if err != nil {
		t.Fatalf("update environment: %v", err)
	}
	if updated.Name != updatedName || updated.IsActive || updated.IsDefault {
		t.Fatalf("unexpected update result")
	}

	if err := repo.Delete(ctx, env.ID); err != nil {
		t.Fatalf("delete environment: %v", err)
	}
	if _, err := repo.GetByID(ctx, env.ID); err == nil {
		t.Fatalf("expected not found after delete")
	} else {
		var nf *environments.NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("expected not found error, got %v", err)
		}
	}
}
