package blocks_test

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/pkg/testsupport"
	repocache "github.com/goliatone/go-repository-cache/cache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestBlocksService_WithBunStorageAndCache(t *testing.T) {
	ctx := context.Background()

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)

	registerBlockModels(t, bunDB)

	cacheCfg := repocache.DefaultConfig()
	cacheCfg.TTL = time.Minute
	cacheService, err := repocache.NewCacheService(cacheCfg)
	if err != nil {
		t.Fatalf("cache service: %v", err)
	}
	keySerializer := repocache.NewDefaultKeySerializer()

	defRepo := blocks.NewBunDefinitionRepositoryWithCache(bunDB, cacheService, keySerializer)
	defVersionRepo := blocks.NewBunDefinitionVersionRepositoryWithCache(bunDB, cacheService, keySerializer)
	instRepo := blocks.NewBunInstanceRepositoryWithCache(bunDB, cacheService, keySerializer)
	trRepo := blocks.NewBunTranslationRepositoryWithCache(bunDB, cacheService, keySerializer)

	svc := blocks.NewService(defRepo, instRepo, trRepo, blocks.WithDefinitionVersionRepository(defVersionRepo))

	def, err := svc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name:   "hero",
		Schema: map[string]any{"fields": []any{"title"}},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	pageID := uuid.MustParse("00000000-0000-0000-0000-00000000baba")
	inst, err := svc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: def.ID,
		PageID:       &pageID,
		Region:       "hero",
		Position:     0,
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	localeID := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	if _, err := svc.AddTranslation(ctx, blocks.AddTranslationInput{
		BlockInstanceID: inst.ID,
		LocaleID:        localeID,
		Content:         map[string]any{"title": "Hello"},
	}); err != nil {
		t.Fatalf("add translation: %v", err)
	}

	if _, err := svc.GetTranslation(ctx, inst.ID, localeID); err != nil {
		t.Fatalf("cached get translation: %v", err)
	}
	if _, err := svc.GetTranslation(ctx, inst.ID, localeID); err != nil {
		t.Fatalf("second cached get translation: %v", err)
	}
}

func registerBlockModels(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS block_definitions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			icon TEXT,
			schema TEXT,
			schema_version TEXT,
			defaults TEXT,
			editor_style_url TEXT,
			frontend_style_url TEXT,
			deleted_at TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS block_definition_versions (
			id TEXT PRIMARY KEY,
			definition_id TEXT NOT NULL,
			schema_version TEXT NOT NULL,
			schema TEXT NOT NULL,
			defaults TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS block_instances (
			id TEXT PRIMARY KEY,
			page_id TEXT,
			region TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			definition_id TEXT NOT NULL,
			configuration TEXT,
			is_global BOOLEAN DEFAULT FALSE,
			current_version INTEGER NOT NULL DEFAULT 1,
			published_version INTEGER,
			published_at TEXT,
			published_by TEXT,
			created_by TEXT NOT NULL,
			updated_by TEXT NOT NULL,
			deleted_at TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS block_versions (
			id TEXT PRIMARY KEY,
			block_instance_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			status TEXT NOT NULL,
			snapshot TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TEXT,
			published_at TEXT,
			published_by TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS block_translations (
			id TEXT PRIMARY KEY,
			block_instance_id TEXT NOT NULL,
			locale_id TEXT NOT NULL,
			content TEXT,
			media_bindings TEXT,
			attribute_overrides TEXT,
			deleted_at TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
}
