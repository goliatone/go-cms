package storageconfig

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/pkg/storage"
	_ "github.com/mattn/go-sqlite3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestBunRepository_CRUDEvents(t *testing.T) {
	db := newTestDB(t)
	repo := NewBunRepository(db)
	ctx := context.Background()

	if _, err := repo.List(ctx); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	events, err := repo.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	profile := storage.Profile{
		Name:        "primary",
		Description: "Primary database",
		Provider:    "bun",
		Config: storage.Config{
			Name:   "primary",
			Driver: "bun",
			DSN:    "postgres://primary",
			Options: map[string]any{
				"pool": 5,
			},
		},
		Fallbacks: []string{"replica"},
		Labels: map[string]string{
			"tier": "primary",
		},
		Default: true,
	}

	stored, err := repo.Upsert(ctx, profile)
	if err != nil {
		t.Fatalf("Upsert() create error = %v", err)
	}
	if stored.Name != "primary" || stored.Config.DSN != "postgres://primary" {
		t.Fatalf("Upsert() returned %+v", stored)
	}
	assertEvent(t, events, ChangeCreated)

	profile.Description = "Primary database (rw)"
	profile.Config.Options["pool"] = 10
	if _, err := repo.Upsert(ctx, profile); err != nil {
		t.Fatalf("Upsert() update error = %v", err)
	}
	assertEvent(t, events, ChangeUpdated)

	fetched, err := repo.Get(ctx, "primary")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if fetched.Description != "Primary database (rw)" {
		t.Fatalf("Get() returned %+v", fetched)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].Name != "primary" {
		t.Fatalf("List() returned %+v", list)
	}

	if err := repo.Delete(ctx, "primary"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	assertEvent(t, events, ChangeDeleted)

	if _, err := repo.Get(ctx, "primary"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound, got %v", err)
	}
}

func TestBunRepository_DeleteMissing(t *testing.T) {
	db := newTestDB(t)
	repo := NewBunRepository(db)

	err := repo.Delete(context.Background(), "missing")
	if !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound, got %v", err)
	}
}

func TestBunRepository_RequiresName(t *testing.T) {
	db := newTestDB(t)
	repo := NewBunRepository(db)
	ctx := context.Background()

	if _, err := repo.Upsert(ctx, storage.Profile{}); !errors.Is(err, ErrProfileNameRequired) {
		t.Fatalf("expected ErrProfileNameRequired, got %v", err)
	}
	if _, err := repo.Get(ctx, " "); !errors.Is(err, ErrProfileNameRequired) {
		t.Fatalf("expected ErrProfileNameRequired, got %v", err)
	}
	if err := repo.Delete(ctx, ""); !errors.Is(err, ErrProfileNameRequired) {
		t.Fatalf("expected ErrProfileNameRequired, got %v", err)
	}
}

func newTestDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open("sqlite3", "file:storageconfig_test?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = sqldb.Close()
	})

	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() {
		_ = db.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := db.NewCreateTable().Model((*profileModel)(nil)).IfNotExists().Exec(ctx); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}
