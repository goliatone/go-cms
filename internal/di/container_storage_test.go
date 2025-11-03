package di

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/internal/storageconfig"
	"github.com/goliatone/go-cms/pkg/storage"
)

func TestContainer_StorageProfileHotSwap(t *testing.T) {
	repo := storageconfig.NewMemoryRepository()
	cfg := runtimeconfig.DefaultConfig()
	cfg.Storage.Profiles = []storage.Profile{
		{
			Name:     "primary",
			Provider: "bun",
			Default:  true,
			Config: storage.Config{
				Name:   "primary",
				Driver: "sqlite3",
				DSN:    fmt.Sprintf("file:storage_profile_hot_swap_base_%d?mode=memory&cache=shared&_fk=1", time.Now().UnixNano()),
			},
		},
	}

	container, err := NewContainer(cfg, WithStorageRepository(repo))
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	t.Cleanup(func() {
		if container.storageCancel != nil {
			container.storageCancel()
		}
		if container.storageHandle != nil {
			container.storageHandle.Close(context.Background())
		}
	})

	container.storageMu.Lock()
	originalDB := container.bunDB
	container.storageMu.Unlock()
	if originalDB == nil {
		t.Fatal("expected bunDB to be initialised")
	}

	update := storage.Profile{
		Name:     "primary",
		Provider: "bun",
		Default:  true,
		Config: storage.Config{
			Name:   "primary-update",
			Driver: "sqlite3",
			DSN:    fmt.Sprintf("file:storage_profile_hot_swap_update_%d?mode=memory&cache=shared&_fk=1", time.Now().UnixNano()),
		},
	}
	if _, err := repo.Upsert(context.Background(), update); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		container.storageMu.Lock()
		defer container.storageMu.Unlock()
		return container.bunDB != nil && container.bunDB != originalDB && container.activeProfile == "primary"
	})
}

func TestContainer_StorageProfileUpdateFailureKeepsPrevious(t *testing.T) {
	repo := storageconfig.NewMemoryRepository()
	cfg := runtimeconfig.DefaultConfig()
	cfg.Storage.Profiles = []storage.Profile{
		{
			Name:     "primary",
			Provider: "bun",
			Default:  true,
			Config: storage.Config{
				Name:   "primary",
				Driver: "sqlite3",
				DSN:    fmt.Sprintf("file:storage_profile_failure_base_%d?mode=memory&cache=shared&_fk=1", time.Now().UnixNano()),
			},
		},
	}

	failingFactory := func(context.Context, storage.Profile) (StorageFactoryResult, error) {
		return StorageFactoryResult{}, errors.New("expected failure")
	}

	container, err := NewContainer(cfg, WithStorageRepository(repo), WithStorageFactory("broken", failingFactory))
	if err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}
	t.Cleanup(func() {
		if container.storageCancel != nil {
			container.storageCancel()
		}
		if container.storageHandle != nil {
			container.storageHandle.Close(context.Background())
		}
	})

	container.storageMu.Lock()
	originalDB := container.bunDB
	container.storageMu.Unlock()
	if originalDB == nil {
		t.Fatal("expected bunDB to be initialised")
	}

	brokenProfile := storage.Profile{
		Name:     "broken",
		Provider: "broken",
		Default:  true,
		Config: storage.Config{
			Name:   "broken",
			Driver: "sqlite3",
			DSN:    "ignored",
		},
	}
	if _, err := repo.Upsert(context.Background(), brokenProfile); err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	// Expect the container to retain the original handle.
	waitFor(t, 2*time.Second, func() bool {
		container.storageMu.Lock()
		defer container.storageMu.Unlock()
		return container.bunDB == originalDB && container.activeProfile == "primary"
	})
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}
