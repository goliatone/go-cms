package integration

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/runtimeconfig"
	"github.com/goliatone/go-cms/internal/storageconfig"
	"github.com/goliatone/go-cms/pkg/storage"
)

func TestStorageAdminApplyConfigMaintainsContinuity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, "primary.sqlite")
	nextPath := filepath.Join(tempDir, "primary_rotated.sqlite")
	baseDSN := fmt.Sprintf("file:%s?_fk=1", basePath)
	nextDSN := fmt.Sprintf("file:%s?_fk=1", nextPath)

	primeDatabase := func(dsn string) {
		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			t.Fatalf("open sqlite %s: %v", dsn, err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS kv (k TEXT PRIMARY KEY, v TEXT)`); err != nil {
			t.Fatalf("prime sqlite %s: %v", dsn, err)
		}
	}

	primeDatabase(baseDSN)
	primeDatabase(nextDSN)

	cfg := runtimeconfig.DefaultConfig()
	cfg.Storage.Profiles = []storage.Profile{
		{
			Name:        "primary",
			Provider:    "bun",
			Description: "Base profile",
			Default:     true,
			Config: storage.Config{
				Name:   "primary",
				Driver: "sqlite3",
				DSN:    baseDSN,
			},
		},
	}

	repo := storageconfig.NewMemoryRepository()
	module, err := cms.New(cfg, di.WithStorageRepository(repo))
	if err != nil {
		t.Fatalf("cms.New() error = %v", err)
	}

	provider := module.Container().StorageProvider()
	if _, err := provider.Exec(ctx, `CREATE TABLE IF NOT EXISTS kv (k TEXT PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create table on primary: %v", err)
	}

	admin := module.StorageAdmin()
	if admin == nil {
		t.Fatalf("expected storage admin service to be initialised")
	}

	errCh := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 40; i++ {
			select {
			case <-time.After(5 * time.Second):
				errCh <- fmt.Errorf("write loop stalled")
				return
			default:
			}
			key := fmt.Sprintf("k%d", i)
			value := fmt.Sprintf("v%d", i)
			current := module.Container().StorageProvider()
			if _, err := current.Exec(ctx, `INSERT OR REPLACE INTO kv(k, v) VALUES (?, ?)`, key, value); err != nil {
				errCh <- fmt.Errorf("write iteration %d failed: %w", i, err)
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
	}()

	update := runtimeconfig.StorageConfig{
		Profiles: []storage.Profile{
			{
				Name:        "primary",
				Provider:    "bun",
				Description: "Rotated primary storage",
				Default:     true,
				Config: storage.Config{
					Name:   "primary",
					Driver: "sqlite3",
					DSN:    nextDSN,
				},
			},
		},
	}
	if err := admin.ApplyConfig(ctx, update); err != nil {
		t.Fatalf("ApplyConfig() error = %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for write loop")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("write loop error: %v", err)
		}
	default:
	}

	verifyCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	waitFor(t, verifyCtx, func() error {
		db, err := sql.Open("sqlite3", nextDSN)
		if err != nil {
			return err
		}
		defer db.Close()

		var count int
		if err := db.QueryRowContext(verifyCtx, `SELECT COUNT(*) FROM kv`).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("no rows recorded in rotated database yet")
		}
		return nil
	})
}

func waitFor(t *testing.T, ctx context.Context, fn func() error) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		err := fn()
		if err == nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("condition not met: %v (last error: %v)", ctx.Err(), err)
		case <-ticker.C:
		}
	}
}
