package cms_test

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cms "github.com/goliatone/go-cms"
	persistence "github.com/goliatone/go-persistence-bun"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/migrate"
)

const (
	cmsMigrationSourceLabel = "go-cms"
	cmsDialectPostgres      = "postgres"
	cmsDialectSQLite        = "sqlite"
)

type migrationTestConfig struct {
	driver string
	server string
}

func (c migrationTestConfig) GetDebug() bool {
	return false
}

func (c migrationTestConfig) GetDriver() string {
	return c.driver
}

func (c migrationTestConfig) GetServer() string {
	return c.server
}

func (c migrationTestConfig) GetPingTimeout() time.Duration {
	return time.Second
}

func (c migrationTestConfig) GetOtelIdentifier() string {
	return ""
}

func TestMigrationRegistrationSQLiteApplyRollbackReapply(t *testing.T) {
	t.Parallel()

	client, db := newSQLiteMigrationClient(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	registerCMSDialectMigrations(t, client)

	if err := client.ValidateDialects(ctx); err != nil {
		t.Fatalf("validate dialects: %v", err)
	}
	if err := client.Migrate(ctx); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	assertTableExistsSQLite(t, db, "locales")
	assertTableExistsSQLite(t, db, "contents")
	migratedCount := countAppliedMigrations(t, db)
	if migratedCount == 0 {
		t.Fatalf("expected applied migrations after migrate")
	}
	if stableCount := countAppliedMigrationsLikeSQLite(t, db, "ordsrc_%"); stableCount != 0 {
		t.Fatalf("expected standalone sqlite migration markers to remain raw, got stable=%d", stableCount)
	}

	if err := client.RollbackAll(ctx); err != nil {
		t.Fatalf("rollback sqlite: %v", err)
	}
	if rolledBackCount := countAppliedMigrations(t, db); rolledBackCount != 0 {
		t.Fatalf("expected no applied migrations after rollback, got %d", rolledBackCount)
	}

	if err := client.Migrate(ctx); err != nil {
		t.Fatalf("reapply sqlite migrations: %v", err)
	}
	assertTableExistsSQLite(t, db, "locales")
	assertTableExistsSQLite(t, db, "contents")
	if reappliedCount := countAppliedMigrations(t, db); reappliedCount != migratedCount {
		t.Fatalf("unexpected applied migration count after reapply: got=%d want=%d", reappliedCount, migratedCount)
	}
	if stableCount := countAppliedMigrationsLikeSQLite(t, db, "ordsrc_%"); stableCount != 0 {
		t.Fatalf("expected standalone reapplied sqlite migration markers to remain raw, got stable=%d", stableCount)
	}
}

func TestMigrationRegistrationPostgresApplyRollbackReapply(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("GO_CMS_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set GO_CMS_TEST_POSTGRES_DSN to run postgres migration integration test")
	}

	client, db, schemaName := newPostgresMigrationClient(t, dsn)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	registerCMSDialectMigrations(t, client)

	if err := client.ValidateDialects(ctx); err != nil {
		t.Fatalf("validate dialects: %v", err)
	}
	if err := client.Migrate(ctx); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}
	assertTableExistsPostgres(t, db, schemaName, "locales")
	assertTableExistsPostgres(t, db, schemaName, "contents")
	assertColumnExistsPostgres(t, db, schemaName, "menu_items", "canonical_key")
	migratedCount := countAppliedMigrations(t, db)
	if migratedCount == 0 {
		t.Fatalf("expected applied migrations after migrate")
	}

	if err := client.RollbackAll(ctx); err != nil {
		t.Fatalf("rollback postgres: %v", err)
	}
	if rolledBackCount := countAppliedMigrations(t, db); rolledBackCount != 0 {
		t.Fatalf("expected no applied migrations after rollback, got %d", rolledBackCount)
	}

	if err := client.Migrate(ctx); err != nil {
		t.Fatalf("reapply postgres migrations: %v", err)
	}
	assertTableExistsPostgres(t, db, schemaName, "locales")
	assertTableExistsPostgres(t, db, schemaName, "contents")
	assertColumnExistsPostgres(t, db, schemaName, "menu_items", "canonical_key")
	if reappliedCount := countAppliedMigrations(t, db); reappliedCount != migratedCount {
		t.Fatalf("unexpected applied migration count after reapply: got=%d want=%d", reappliedCount, migratedCount)
	}
}

func TestStableMigrationRegistrationSQLiteBackfillsLegacyDialectMarkers(t *testing.T) {
	legacyClient, db, dsn := newSQLiteMigrationClientWithDSN(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	registerCMSDialectMigrations(t, legacyClient)
	if err := legacyClient.Migrate(ctx); err != nil {
		t.Fatalf("migrate legacy sqlite: %v", err)
	}
	rawCount := countAppliedMigrations(t, db)
	if rawCount == 0 {
		t.Fatalf("expected raw applied migrations")
	}
	if stableCount := countAppliedMigrationsLikeSQLite(t, db, "ordsrc_%"); stableCount != 0 {
		t.Fatalf("expected no stable markers before backfill, got %d", stableCount)
	}

	stableClient, err := persistence.New(migrationTestConfig{
		driver: "sqlite3",
		server: dsn,
	}, db, sqlitedialect.New())
	if err != nil {
		t.Fatalf("persistence.New stable sqlite: %v", err)
	}
	result, err := cms.BackfillStableMigrationMarkers(ctx, stableClient.DB())
	if err != nil {
		t.Fatalf("backfill stable migration markers: %v", err)
	}
	if result.MatchedRawMarkers != rawCount || result.InsertedMarkers != rawCount {
		t.Fatalf("unexpected backfill result: got=%+v raw=%d", result, rawCount)
	}
	result, err = cms.BackfillStableMigrationMarkers(ctx, stableClient.DB())
	if err != nil {
		t.Fatalf("backfill stable migration markers again: %v", err)
	}
	if result.MatchedRawMarkers != rawCount || result.InsertedMarkers != 0 || result.ExistingMarkers != rawCount {
		t.Fatalf("unexpected idempotent backfill result: got=%+v raw=%d", result, rawCount)
	}
	registerCMSStableMigrations(t, stableClient)
	if err := stableClient.Migrate(ctx); err != nil {
		t.Fatalf("migrate stable sqlite after backfill: %v", err)
	}
	if totalCount := countAppliedMigrations(t, db); totalCount != rawCount*2 {
		t.Fatalf("expected raw plus stable markers only, got total=%d raw=%d", totalCount, rawCount)
	}
	if stableCount := countAppliedMigrationsLikeSQLite(t, db, "ordsrc_%"); stableCount != rawCount {
		t.Fatalf("expected all raw markers to have stable aliases, got stable=%d raw=%d", stableCount, rawCount)
	}
}

func TestStableMigrationRegistrationSQLiteBackfillsCustomOrder(t *testing.T) {
	legacyClient, db, dsn := newSQLiteMigrationClientWithDSN(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	registerCMSDialectMigrations(t, legacyClient)
	if err := legacyClient.Migrate(ctx); err != nil {
		t.Fatalf("migrate legacy sqlite: %v", err)
	}
	rawCount := countAppliedMigrations(t, db)
	if rawCount == 0 {
		t.Fatalf("expected raw applied migrations")
	}

	stableClient, err := persistence.New(migrationTestConfig{
		driver: "sqlite3",
		server: dsn,
	}, db, sqlitedialect.New())
	if err != nil {
		t.Fatalf("persistence.New stable sqlite: %v", err)
	}
	opts := []cms.MigrationSourceOption{cms.WithMigrationSourceOrder(44)}
	result, err := cms.BackfillStableMigrationMarkers(ctx, stableClient.DB(), opts...)
	if err != nil {
		t.Fatalf("backfill stable migration markers: %v", err)
	}
	if result.MatchedRawMarkers != rawCount || result.InsertedMarkers != rawCount {
		t.Fatalf("unexpected backfill result: got=%+v raw=%d", result, rawCount)
	}

	source, err := cms.StableOrderedMigrationSource(opts...)
	if err != nil {
		t.Fatalf("stable ordered migration source: %v", err)
	}
	if err := stableClient.RegisterOrderedMigrationSources(source); err != nil {
		t.Fatalf("register ordered migration source: %v", err)
	}
	if err := stableClient.Migrate(ctx); err != nil {
		t.Fatalf("migrate stable sqlite after custom-order backfill: %v", err)
	}
	if stableCount := countAppliedMigrationsLikeSQLite(t, db, "ordsrc_000044_%"); stableCount != rawCount {
		t.Fatalf("expected custom-order stable markers, got stable=%d raw=%d", stableCount, rawCount)
	}
	if wrongOrderCount := countAppliedMigrationsLikeSQLite(t, db, "ordsrc_000040_%"); wrongOrderCount != 0 {
		t.Fatalf("expected no default-order stable markers, got %d", wrongOrderCount)
	}
}

func TestStableMigrationBackfillRejectsRawMarkersWithoutCMSSchema(t *testing.T) {
	client, db := newSQLiteMigrationClient(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	migrator := migrate.NewMigrator(client.DB(), migrate.NewMigrations(), migrate.WithUpsert(true))
	if err := migrator.Init(ctx); err != nil {
		t.Fatalf("init migrator: %v", err)
	}
	if err := migrator.MarkApplied(ctx, &migrate.Migration{Name: "20250102000000", GroupID: 1}); err != nil {
		t.Fatalf("insert raw marker: %v", err)
	}

	_, err := cms.BackfillStableMigrationMarkers(ctx, client.DB())
	if err == nil {
		t.Fatalf("expected schema mismatch error")
	}
	if !strings.Contains(err.Error(), "required schema tables are missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStableMigrationRegistrationSQLiteFreshDatabaseUsesSourceStableMarkers(t *testing.T) {
	client, db := newSQLiteMigrationClient(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	registerCMSStableMigrations(t, client)
	if err := client.Migrate(ctx); err != nil {
		t.Fatalf("migrate stable sqlite: %v", err)
	}
	migratedCount := countAppliedMigrations(t, db)
	if migratedCount == 0 {
		t.Fatalf("expected applied migrations")
	}
	if stableCount := countAppliedMigrationsLikeSQLite(t, db, "ordsrc_%"); stableCount != migratedCount {
		t.Fatalf("expected all fresh sqlite migration markers to be source-stable, got stable=%d total=%d", stableCount, migratedCount)
	}
}

func registerCMSDialectMigrations(t *testing.T, client *persistence.Client) {
	t.Helper()

	migrationsRoot, err := fs.Sub(cms.GetMigrationsFS(), "data/sql/migrations")
	if err != nil {
		t.Fatalf("migrations fs root: %v", err)
	}

	client.RegisterDialectMigrations(
		migrationsRoot,
		persistence.WithDialectSourceLabel(cmsMigrationSourceLabel),
		persistence.WithValidationTargets(cmsDialectPostgres, cmsDialectSQLite),
	)
}

func registerCMSStableMigrations(t *testing.T, client *persistence.Client) {
	t.Helper()

	source, err := cms.StableOrderedMigrationSource()
	if err != nil {
		t.Fatalf("stable ordered migration source: %v", err)
	}

	if err := client.RegisterOrderedMigrationSources(source); err != nil {
		t.Fatalf("register ordered migration source: %v", err)
	}
}

func newSQLiteMigrationClient(t *testing.T) (*persistence.Client, *sql.DB) {
	t.Helper()

	dsn := "file:" + filepath.Join(t.TempDir(), "cms_migrations.db") + "?cache=shared&_fk=1"
	client, db, _ := newSQLiteMigrationClientForDSN(t, dsn)
	return client, db
}

func newSQLiteMigrationClientWithDSN(t *testing.T) (*persistence.Client, *sql.DB, string) {
	t.Helper()

	dsn := "file:" + filepath.Join(t.TempDir(), "cms_migrations.db") + "?cache=shared&_fk=1"
	return newSQLiteMigrationClientForDSN(t, dsn)
}

func newSQLiteMigrationClientForDSN(t *testing.T, dsn string) (*persistence.Client, *sql.DB, string) {
	t.Helper()

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	client, err := persistence.New(migrationTestConfig{
		driver: "sqlite3",
		server: dsn,
	}, db, sqlitedialect.New())
	if err != nil {
		_ = db.Close()
		t.Fatalf("persistence.New sqlite: %v", err)
	}
	return client, db, dsn
}

func newPostgresMigrationClient(t *testing.T, dsn string) (*persistence.Client, *sql.DB, string) {
	t.Helper()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	ctx := context.Background()
	schemaName := fmt.Sprintf("gocms_mig_%d_%d", time.Now().UnixNano(), rand.Intn(10000))
	if _, err := db.ExecContext(ctx, `CREATE SCHEMA "`+schemaName+`"`); err != nil {
		_ = db.Close()
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `SET search_path TO public`)
		_, _ = db.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS "`+schemaName+`" CASCADE`)
	})
	if _, err := db.ExecContext(ctx, `SET search_path TO "`+schemaName+`"`); err != nil {
		_ = db.Close()
		t.Fatalf("set search path: %v", err)
	}

	client, err := persistence.New(migrationTestConfig{
		driver: "postgres",
		server: dsn,
	}, db, pgdialect.New())
	if err != nil {
		_ = db.Close()
		t.Fatalf("persistence.New postgres: %v", err)
	}

	return client, db, schemaName
}

func assertTableExistsSQLite(t *testing.T, db *sql.DB, table string) {
	t.Helper()

	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
	if err != nil || name != table {
		t.Fatalf("expected sqlite table %q to exist, err=%v", table, err)
	}
}

func assertTableExistsPostgres(t *testing.T, db *sql.DB, schema, table string) {
	t.Helper()

	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = $2
		)`,
		schema,
		table,
	).Scan(&exists)
	if err != nil || !exists {
		t.Fatalf("expected postgres table %s.%s to exist, err=%v", schema, table, err)
	}
}

func assertColumnExistsPostgres(t *testing.T, db *sql.DB, schema, table, column string) {
	t.Helper()

	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2 AND column_name = $3
		)`,
		schema,
		table,
		column,
	).Scan(&exists)
	if err != nil || !exists {
		t.Fatalf("expected postgres column %s.%s.%s to exist, err=%v", schema, table, column, err)
	}
}

func countAppliedMigrations(t *testing.T, db *sql.DB) int {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM bun_migrations`).Scan(&count); err != nil {
		t.Fatalf("count bun_migrations: %v", err)
	}
	return count
}

func countAppliedMigrationsLikeSQLite(t *testing.T, db *sql.DB, pattern string) int {
	t.Helper()

	var count int
	if err := db.QueryRow(`SELECT count(*) FROM bun_migrations WHERE name LIKE ?`, pattern).Scan(&count); err != nil {
		t.Fatalf("count bun_migrations like %q: %v", pattern, err)
	}
	return count
}
