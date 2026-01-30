package di_test

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

const defaultEnvironmentID = "00000000-0000-0000-0000-000000000001"

type envRow struct {
	ID        string `bun:"id"`
	Key       string `bun:"key"`
	Name      string `bun:"name"`
	IsActive  bool   `bun:"is_active"`
	IsDefault bool   `bun:"is_default"`
}

type envSeed struct {
	bun.BaseModel `bun:"table:environments"`
	ID            uuid.UUID `bun:"id,pk"`
	Key           string    `bun:"key"`
	Name          string    `bun:"name"`
	Description   string    `bun:"description"`
	IsActive      bool      `bun:"is_active"`
	IsDefault     bool      `bun:"is_default"`
	CreatedAt     time.Time `bun:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

func createEnvironmentTable(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS environments (
			id TEXT PRIMARY KEY,
			key TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT,
			is_active INTEGER NOT NULL DEFAULT 1,
			is_default INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP
		);
	`); err != nil {
		t.Fatalf("create environments table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_environments_default
		ON environments(is_default) WHERE is_default = 1;
	`); err != nil {
		t.Fatalf("create environments default index: %v", err)
	}
}

func TestContainerBootstrapsDefaultEnvironment(t *testing.T) {
	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)
	createEnvironmentTable(t, bunDB)

	cfg := cms.DefaultConfig()
	cfg.Features.Environments = true

	if _, err := di.NewContainer(cfg, di.WithBunDB(bunDB)); err != nil {
		t.Fatalf("new container: %v", err)
	}

	ctx := context.Background()
	count, err := bunDB.NewSelect().Table("environments").Count(ctx)
	if err != nil {
		t.Fatalf("count environments: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 environment, got %d", count)
	}

	var env envRow
	if err := bunDB.NewSelect().
		Table("environments").
		Column("id", "key", "name", "is_active", "is_default").
		Scan(ctx, &env); err != nil {
		t.Fatalf("select environment: %v", err)
	}

	if env.ID != defaultEnvironmentID {
		t.Fatalf("expected default environment id %s, got %s", defaultEnvironmentID, env.ID)
	}
	if env.Key != "default" {
		t.Fatalf("expected default environment key, got %q", env.Key)
	}
	if env.Name != "Default" {
		t.Fatalf("expected default environment name, got %q", env.Name)
	}
	if !env.IsDefault {
		t.Fatalf("expected environment to be default")
	}
	if !env.IsActive {
		t.Fatalf("expected environment to be active")
	}
}

func TestContainerReconcilesConfiguredEnvironments(t *testing.T) {
	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)
	createEnvironmentTable(t, bunDB)

	now := time.Now().UTC()
	if _, err := bunDB.NewInsert().Model(&envSeed{
		ID:        uuid.New(),
		Key:       "dev",
		Name:      "Development",
		IsActive:  true,
		IsDefault: false,
		CreatedAt: now,
		UpdatedAt: now,
	}).Exec(context.Background()); err != nil {
		t.Fatalf("insert dev environment: %v", err)
	}
	if _, err := bunDB.NewInsert().Model(&envSeed{
		ID:        uuid.New(),
		Key:       "prod",
		Name:      "Production",
		IsActive:  true,
		IsDefault: true,
		CreatedAt: now,
		UpdatedAt: now,
	}).Exec(context.Background()); err != nil {
		t.Fatalf("insert prod environment: %v", err)
	}

	cfg := cms.DefaultConfig()
	cfg.Features.Environments = true
	cfg.Environments = cms.EnvironmentsConfig{
		DefaultKey: "dev",
		Definitions: []cms.EnvironmentConfig{
			{Key: "dev", Name: "Dev"},
			{Key: "prod", Name: "Production", Disabled: true},
			{Key: "staging", Name: "Staging"},
		},
	}

	if _, err := di.NewContainer(cfg, di.WithBunDB(bunDB)); err != nil {
		t.Fatalf("new container: %v", err)
	}

	ctx := context.Background()
	count, err := bunDB.NewSelect().Table("environments").Count(ctx)
	if err != nil {
		t.Fatalf("count environments: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 environments, got %d", count)
	}

	var envs []envRow
	if err := bunDB.NewSelect().
		Table("environments").
		Column("key", "name", "is_active", "is_default").
		OrderExpr("key ASC").
		Scan(ctx, &envs); err != nil {
		t.Fatalf("list environments: %v", err)
	}

	found := map[string]envRow{}
	for _, env := range envs {
		found[env.Key] = env
	}

	dev := found["dev"]
	if dev.Name != "Dev" {
		t.Fatalf("expected dev name to be updated, got %q", dev.Name)
	}
	if !dev.IsDefault {
		t.Fatalf("expected dev to be default")
	}
	if !dev.IsActive {
		t.Fatalf("expected dev to remain active")
	}

	prod := found["prod"]
	if prod.IsDefault {
		t.Fatalf("expected prod to no longer be default")
	}
	if prod.IsActive {
		t.Fatalf("expected prod to be inactive")
	}

	staging := found["staging"]
	if staging.Name != "Staging" {
		t.Fatalf("expected staging to be created, got %q", staging.Name)
	}
	if staging.IsDefault {
		t.Fatalf("expected staging to not be default")
	}
	if !staging.IsActive {
		t.Fatalf("expected staging to be active")
	}
}
