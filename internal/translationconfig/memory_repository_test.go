package translationconfig

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryRepository_CRUDEvents(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	if _, err := repo.Get(ctx); !errors.Is(err, ErrSettingsNotFound) {
		t.Fatalf("expected ErrSettingsNotFound, got %v", err)
	}

	events, err := repo.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	settings := Settings{
		TranslationsEnabled: true,
		RequireTranslations: true,
	}
	if _, err := repo.Upsert(ctx, settings); err != nil {
		t.Fatalf("Upsert() create error = %v", err)
	}
	assertEvent(t, events, ChangeCreated)

	settings.RequireTranslations = false
	if _, err := repo.Upsert(ctx, settings); err != nil {
		t.Fatalf("Upsert() update error = %v", err)
	}
	assertEvent(t, events, ChangeUpdated)

	fetched, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if fetched != settings {
		t.Fatalf("Get() returned %+v, want %+v", fetched, settings)
	}

	if err := repo.Delete(ctx); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	assertEvent(t, events, ChangeDeleted)
}

func TestMemoryRepository_DeleteMissing(t *testing.T) {
	repo := NewMemoryRepository()
	if err := repo.Delete(context.Background()); !errors.Is(err, ErrSettingsNotFound) {
		t.Fatalf("expected ErrSettingsNotFound, got %v", err)
	}
}

func assertEvent(t *testing.T, events <-chan ChangeEvent, want ChangeType) {
	t.Helper()
	select {
	case evt := <-events:
		if evt.Type != want {
			t.Fatalf("expected event %s, got %s", want, evt.Type)
		}
	default:
		t.Fatalf("expected event %s, got none", want)
	}
}
