package cms

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	persistence "github.com/goliatone/go-persistence-bun"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

const (
	// MigrationDialectPostgres is the canonical PostgreSQL validation target.
	MigrationDialectPostgres = "postgres"
	// MigrationDialectSQLite is the canonical SQLite validation target.
	MigrationDialectSQLite = "sqlite"

	// MigrationSourceName is the canonical ordered migration source name.
	MigrationSourceName = "go-cms"
	// MigrationSourceKey is the durable source-stable migration identity key.
	MigrationSourceKey = "go-cms"
	// MigrationSourceOrder is the canonical order for go-cms in the shared
	// auth/users/cms/services migration graph.
	MigrationSourceOrder = 40
)

// MigrationSourceDescriptor describes the embedded go-cms migration source.
type MigrationSourceDescriptor struct {
	Name              string
	SourceKey         string
	Order             int
	DependsOn         []string
	Label             string
	Root              fs.FS
	ValidationTargets []string
}

// StableMigrationBackfillResult describes raw go-cms marker compatibility work.
type StableMigrationBackfillResult struct {
	MatchedRawMarkers int
	InsertedMarkers   int
	ExistingMarkers   int
}

type migrationSourceConfig struct {
	name              string
	sourceKey         string
	order             int
	label             string
	dependencies      []string
	validationTargets []string
}

// MigrationSourceOption configures a go-cms migration source descriptor.
type MigrationSourceOption func(*migrationSourceConfig)

// WithMigrationSourceDependencies declares source-stable dependencies by key.
func WithMigrationSourceDependencies(sourceKeys ...string) MigrationSourceOption {
	return func(cfg *migrationSourceConfig) {
		if cfg == nil {
			return
		}
		cfg.dependencies = append(cfg.dependencies, normalizeMigrationValues(sourceKeys)...)
	}
}

// WithMigrationSourceOrder overrides the source-stable order.
//
// Only change this for a new application graph before any database has applied
// go-cms source-stable migrations. Once released, source key and order are part
// of the durable migration identity.
func WithMigrationSourceOrder(order int) MigrationSourceOption {
	return func(cfg *migrationSourceConfig) {
		if cfg == nil {
			return
		}
		cfg.order = order
	}
}

// WithMigrationValidationTargets overrides dialect validation targets.
func WithMigrationValidationTargets(targets ...string) MigrationSourceOption {
	return func(cfg *migrationSourceConfig) {
		if cfg == nil {
			return
		}
		cfg.validationTargets = normalizeMigrationValues(targets)
	}
}

// DefaultMigrationSourceDescriptor returns the canonical go-cms migration
// descriptor for hosts that assemble their own ordered migration graph.
func DefaultMigrationSourceDescriptor(opts ...MigrationSourceOption) (MigrationSourceDescriptor, error) {
	root, err := fs.Sub(GetMigrationsFS(), "data/sql/migrations")
	if err != nil {
		return MigrationSourceDescriptor{}, fmt.Errorf("go-cms migrations: resolve root: %w", err)
	}

	cfg := defaultMigrationSourceConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if len(cfg.validationTargets) == 0 {
		cfg.validationTargets = []string{MigrationDialectPostgres, MigrationDialectSQLite}
	}

	return MigrationSourceDescriptor{
		Name:              cfg.name,
		SourceKey:         cfg.sourceKey,
		Order:             cfg.order,
		DependsOn:         append([]string(nil), cfg.dependencies...),
		Label:             cfg.label,
		Root:              root,
		ValidationTargets: append([]string(nil), cfg.validationTargets...),
	}, nil
}

// StableOrderedMigrationSource returns the source-stable ordered migration
// source for go-cms. Use this when registering go-cms alongside other package
// migration sources in a shared migration manager.
func StableOrderedMigrationSource(opts ...MigrationSourceOption) (persistence.OrderedMigrationSource, error) {
	descriptor, err := DefaultMigrationSourceDescriptor(opts...)
	if err != nil {
		return persistence.OrderedMigrationSource{}, err
	}

	sourceOpts := []persistence.OrderedMigrationSourceOption{
		persistence.WithOrderedMigrationDialectOptions(
			persistence.WithDialectSourceLabel(descriptor.Label),
			persistence.WithValidationTargets(descriptor.ValidationTargets...),
		),
	}
	if len(descriptor.DependsOn) > 0 {
		sourceOpts = append(sourceOpts, persistence.WithOrderedMigrationDependencies(descriptor.DependsOn...))
	}

	return persistence.NewStableOrderedMigrationSource(
		descriptor.Name,
		descriptor.Root,
		descriptor.SourceKey,
		descriptor.Order,
		sourceOpts...,
	), nil
}

// LegacyOrderedMigrationSource returns the pre-source-stable go-cms ordered
// source. It is intended for compatibility backfills and repair planning only.
func LegacyOrderedMigrationSource(opts ...MigrationSourceOption) (persistence.OrderedMigrationSource, error) {
	descriptor, err := DefaultMigrationSourceDescriptor(opts...)
	if err != nil {
		return persistence.OrderedMigrationSource{}, err
	}
	return persistence.OrderedMigrationSource{
		Name: descriptor.Name,
		Root: descriptor.Root,
		Options: []persistence.DialectMigrationOption{
			persistence.WithDialectSourceLabel(descriptor.Label),
			persistence.WithValidationTargets(descriptor.ValidationTargets...),
		},
	}, nil
}

// BackfillStableMigrationMarkers copies existing raw go-cms migration markers
// into source-stable ordered marker names. Call this before migrating an existing
// database that previously registered go-cms via RegisterDialectMigrations and
// is now moving into a shared source-stable ordered migration graph.
func BackfillStableMigrationMarkers(ctx context.Context, db *bun.DB, opts ...MigrationSourceOption) (StableMigrationBackfillResult, error) {
	if db == nil {
		return StableMigrationBackfillResult{}, fmt.Errorf("go-cms migrations: nil database")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	source, err := StableOrderedMigrationSource(opts...)
	if err != nil {
		return StableMigrationBackfillResult{}, err
	}
	manager := persistence.NewMigrations()
	if err := manager.RegisterOrderedMigrationSources(source); err != nil {
		return StableMigrationBackfillResult{}, err
	}
	plan, err := manager.Plan(ctx, db)
	if err != nil {
		return StableMigrationBackfillResult{}, err
	}

	stableByRaw := make(map[string]string, len(plan.Entries))
	stableNames := make([]string, 0, len(plan.Entries))
	rawNames := make([]string, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		if entry.OriginalVersion == "" || entry.SyntheticName == "" {
			continue
		}
		stableByRaw[entry.OriginalVersion] = entry.SyntheticName
		rawNames = append(rawNames, entry.OriginalVersion)
		stableNames = append(stableNames, entry.SyntheticName)
	}
	if len(rawNames) == 0 {
		return StableMigrationBackfillResult{}, nil
	}

	migrator := migrate.NewMigrator(db, migrate.NewMigrations(), migrate.WithUpsert(true))
	if err := migrator.Init(ctx); err != nil {
		return StableMigrationBackfillResult{}, fmt.Errorf("go-cms migrations: initialize migration tables: %w", err)
	}

	var rawRows []migrate.Migration
	if err := db.NewSelect().
		Model(&rawRows).
		ModelTableExpr("bun_migrations AS migration").
		Where("name IN (?)", bun.In(rawNames)).
		Scan(ctx); err != nil {
		return StableMigrationBackfillResult{}, fmt.Errorf("go-cms migrations: read raw markers: %w", err)
	}
	if len(rawRows) == 0 {
		return StableMigrationBackfillResult{}, nil
	}
	if err := verifyStableMigrationBackfillSchema(ctx, db); err != nil {
		return StableMigrationBackfillResult{}, err
	}

	var stableRows []migrate.Migration
	if err := db.NewSelect().
		Model(&stableRows).
		ModelTableExpr("bun_migrations AS migration").
		Where("name IN (?)", bun.In(stableNames)).
		Scan(ctx); err != nil {
		return StableMigrationBackfillResult{}, fmt.Errorf("go-cms migrations: read stable markers: %w", err)
	}
	existing := make(map[string]struct{}, len(stableRows))
	for _, row := range stableRows {
		existing[row.Name] = struct{}{}
	}

	result := StableMigrationBackfillResult{
		MatchedRawMarkers: len(rawRows),
		ExistingMarkers:   len(existing),
	}
	toInsert := make([]migrate.Migration, 0, len(rawRows))
	for _, raw := range rawRows {
		stableName := stableByRaw[raw.Name]
		if stableName == "" {
			continue
		}
		if _, ok := existing[stableName]; ok {
			continue
		}
		migratedAt := raw.MigratedAt
		if migratedAt.IsZero() {
			migratedAt = time.Now().UTC()
		}
		toInsert = append(toInsert, migrate.Migration{
			Name:       stableName,
			GroupID:    raw.GroupID,
			MigratedAt: migratedAt,
		})
	}
	if len(toInsert) == 0 {
		return result, nil
	}

	for idx := range toInsert {
		if err := migrator.MarkApplied(ctx, &toInsert[idx]); err != nil {
			return StableMigrationBackfillResult{}, fmt.Errorf("go-cms migrations: insert stable marker %q: %w", toInsert[idx].Name, err)
		}
	}
	result.InsertedMarkers = len(toInsert)
	return result, nil
}

func verifyStableMigrationBackfillSchema(ctx context.Context, db *bun.DB) error {
	const expected = 5
	requiredTables := []string{"locales", "content_types", "contents", "menus", "menu_items"}

	var count int
	dialect := ""
	if db != nil && db.Dialect() != nil {
		dialect = strings.ToLower(strings.TrimSpace(db.Dialect().Name().String()))
	}
	switch dialect {
	case "sqlite":
		if err := db.NewSelect().
			TableExpr("sqlite_master").
			ColumnExpr("COUNT(*)").
			Where("type = ?", "table").
			Where("name IN (?)", bun.In(requiredTables)).
			Scan(ctx, &count); err != nil {
			return fmt.Errorf("go-cms migrations: verify sqlite schema: %w", err)
		}
	default:
		if err := db.NewSelect().
			TableExpr("information_schema.tables").
			ColumnExpr("COUNT(*)").
			Where("table_schema = current_schema()").
			Where("table_name IN (?)", bun.In(requiredTables)).
			Scan(ctx, &count); err != nil {
			return fmt.Errorf("go-cms migrations: verify postgres schema: %w", err)
		}
	}
	if count != expected {
		return fmt.Errorf(
			"go-cms migrations: refusing stable marker backfill because required schema tables are missing: found %d of %d",
			count,
			expected,
		)
	}
	return nil
}

func defaultMigrationSourceConfig() migrationSourceConfig {
	return migrationSourceConfig{
		name:              MigrationSourceName,
		sourceKey:         MigrationSourceKey,
		order:             MigrationSourceOrder,
		label:             MigrationSourceName,
		validationTargets: []string{MigrationDialectPostgres, MigrationDialectSQLite},
	}
}

func normalizeMigrationValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
