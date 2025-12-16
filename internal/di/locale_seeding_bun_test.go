package di_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/identity"
	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func createLocaleTable(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.NewCreateTable().Model((*content.Locale)(nil)).IfNotExists().Exec(ctx); err != nil {
		t.Fatalf("create locales table: %v", err)
	}
}

func TestContainerSeedsLocalesInEmptyDatabase(t *testing.T) {
	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)
	createLocaleTable(t, bunDB)

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.RequireTranslations = false
	cfg.I18N.Locales = []string{"en", "es"}

	if _, err := di.NewContainer(cfg, di.WithBunDB(bunDB)); err != nil {
		t.Fatalf("new container: %v", err)
	}

	ctx := context.Background()
	count, err := bunDB.NewSelect().Model((*content.Locale)(nil)).Count(ctx)
	if err != nil {
		t.Fatalf("count locales: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 locales, got %d", count)
	}

	var en content.Locale
	if err := bunDB.NewSelect().Model(&en).Where("code = ?", "en").Scan(ctx); err != nil {
		t.Fatalf("select en locale: %v", err)
	}
	expected := identity.LocaleUUID("en")
	if en.ID != expected {
		t.Fatalf("expected deterministic en locale id %s, got %s", expected, en.ID)
	}
}

func TestContainerSeedsLocalesInEmptyDatabaseWhenTranslationsRequired(t *testing.T) {
	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)
	createLocaleTable(t, bunDB)

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.RequireTranslations = true
	cfg.I18N.Locales = []string{"en"}

	if _, err := di.NewContainer(cfg, di.WithBunDB(bunDB)); err != nil {
		t.Fatalf("new container: %v", err)
	}

	ctx := context.Background()
	count, err := bunDB.NewSelect().Model((*content.Locale)(nil)).Count(ctx)
	if err != nil {
		t.Fatalf("count locales: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 locale, got %d", count)
	}
}

func TestContainerDoesNotOverrideExistingLocales(t *testing.T) {
	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)
	createLocaleTable(t, bunDB)

	existingID := uuid.New()
	if _, err := bunDB.NewInsert().Model(&content.Locale{
		ID:        existingID,
		Code:      "en",
		Display:   "Existing",
		IsActive:  true,
		IsDefault: true,
	}).Exec(context.Background()); err != nil {
		t.Fatalf("insert existing locale: %v", err)
	}

	cfg := cms.DefaultConfig()
	cfg.DefaultLocale = "en"
	cfg.I18N.Enabled = true
	cfg.I18N.RequireTranslations = true
	cfg.I18N.Locales = []string{"en", "es"}

	if _, err := di.NewContainer(cfg, di.WithBunDB(bunDB)); err != nil {
		t.Fatalf("new container: %v", err)
	}

	ctx := context.Background()
	count, err := bunDB.NewSelect().Model((*content.Locale)(nil)).Count(ctx)
	if err != nil {
		t.Fatalf("count locales: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected locales to remain unchanged, got %d records", count)
	}

	var en content.Locale
	if err := bunDB.NewSelect().Model(&en).Where("code = ?", "en").Scan(ctx); err != nil {
		t.Fatalf("select en locale: %v", err)
	}
	if en.ID != existingID {
		t.Fatalf("expected existing locale id %s, got %s", existingID, en.ID)
	}
	if en.Display != "Existing" {
		t.Fatalf("expected existing locale display to remain, got %q", en.Display)
	}
}
