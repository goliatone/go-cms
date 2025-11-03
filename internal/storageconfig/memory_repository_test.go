package storageconfig

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/pkg/storage"
)

func TestMemoryRepository_CRUDEvents(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

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

	created, err := repo.Upsert(ctx, profile)
	if err != nil {
		t.Fatalf("Upsert() create error = %v", err)
	}
	if created.Name != profile.Name || created.Description != profile.Description {
		t.Fatalf("Upsert() stored profile mismatch: got %+v", created)
	}

	assertEvent(t, events, ChangeCreated)

	fetched, err := repo.Get(ctx, "primary")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if fetched.Provider != "bun" || fetched.Config.DSN != "postgres://primary" {
		t.Fatalf("Get() returned unexpected profile %+v", fetched)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].Name != "primary" {
		t.Fatalf("List() returned %+v", list)
	}

	// Update profile
	profile.Description = "Primary database (rw)"
	profile.Config.Options["pool"] = 10
	if _, err := repo.Upsert(ctx, profile); err != nil {
		t.Fatalf("Upsert() update error = %v", err)
	}
	assertEvent(t, events, ChangeUpdated)

	if err := repo.Delete(ctx, "primary"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	assertEvent(t, events, ChangeDeleted)

	if _, err := repo.Get(ctx, "primary"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("Get() expected ErrProfileNotFound, got %v", err)
	}
}

func TestMemoryRepository_DeleteMissing(t *testing.T) {
	repo := NewMemoryRepository()
	err := repo.Delete(context.Background(), "missing")
	if !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("expected ErrProfileNotFound, got %v", err)
	}
}

func TestMemoryRepository_RequiresName(t *testing.T) {
	repo := NewMemoryRepository()
	if _, err := repo.Upsert(context.Background(), storage.Profile{}); !errors.Is(err, ErrProfileNameRequired) {
		t.Fatalf("expected ErrProfileNameRequired, got %v", err)
	}
	if _, err := repo.Get(context.Background(), " "); !errors.Is(err, ErrProfileNameRequired) {
		t.Fatalf("expected ErrProfileNameRequired, got %v", err)
	}
	if err := repo.Delete(context.Background(), ""); !errors.Is(err, ErrProfileNameRequired) {
		t.Fatalf("expected ErrProfileNameRequired, got %v", err)
	}
}

func assertEvent(t *testing.T, ch <-chan ChangeEvent, expect ChangeType) {
	t.Helper()
	select {
	case evt, ok := <-ch:
		if !ok {
			t.Fatalf("event channel closed unexpectedly")
		}
		if evt.Type != expect {
			t.Fatalf("expected event %s, got %s", expect, evt.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s event", expect)
	}
}
