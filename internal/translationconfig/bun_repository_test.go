package translationconfig

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestBunRepository_CRUDEvents(t *testing.T) {
	db := newTestDB(t)
	repo := NewBunRepository(db)
	ctx := context.Background()

	if _, err := repo.Get(ctx); !errors.Is(err, ErrSettingsNotFound) {
		t.Fatalf("expected ErrSettingsNotFound, got %v", err)
	}

	events, err := repo.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if _, err := repo.Upsert(ctx, Settings{TranslationsEnabled: true, RequireTranslations: true}); err != nil {
		t.Fatalf("Upsert() create error = %v", err)
	}
	assertEvent(t, events, ChangeCreated)

	if _, err := repo.Upsert(ctx, Settings{TranslationsEnabled: true, RequireTranslations: false}); err != nil {
		t.Fatalf("Upsert() update error = %v", err)
	}
	assertEvent(t, events, ChangeUpdated)

	fetched, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if fetched.TranslationsEnabled != true || fetched.RequireTranslations != false {
		t.Fatalf("Get() returned %+v", fetched)
	}

	if err := repo.Delete(ctx); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	assertEvent(t, events, ChangeDeleted)

	if _, err := repo.Get(ctx); !errors.Is(err, ErrSettingsNotFound) {
		t.Fatalf("expected ErrSettingsNotFound, got %v", err)
	}
}

func TestBunRepository_DeleteMissing(t *testing.T) {
	db := newTestDB(t)
	repo := NewBunRepository(db)

	if err := repo.Delete(context.Background()); !errors.Is(err, ErrSettingsNotFound) {
		t.Fatalf("expected ErrSettingsNotFound, got %v", err)
	}
}

func newTestDB(t *testing.T) *bun.DB {
	t.Helper()

	sqldb, err := sql.Open("sqlite3", "file:translationconfig_test?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })

	db := bun.NewDB(sqldb, sqlitedialect.New())
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := db.NewCreateTable().Model((*settingsModel)(nil)).IfNotExists().Exec(ctx); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}
