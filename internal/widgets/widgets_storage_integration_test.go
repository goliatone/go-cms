package widgets_test

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/testsupport"
	repocache "github.com/goliatone/go-repository-cache/cache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestWidgetsService_WithBunStorageAndCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2024, 5, 1, 12, 30, 0, 0, time.UTC)

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)

	registerWidgetModels(t, bunDB)

	cacheCfg := repocache.DefaultConfig()
	cacheCfg.TTL = time.Minute
	cacheSvc, err := repocache.NewCacheService(cacheCfg)
	if err != nil {
		t.Fatalf("cache service: %v", err)
	}
	keySerializer := repocache.NewDefaultKeySerializer()

	defRepo := widgets.NewBunDefinitionRepositoryWithCache(bunDB, cacheSvc, keySerializer)
	instRepo := widgets.NewBunInstanceRepositoryWithCache(bunDB, cacheSvc, keySerializer)
	trRepo := widgets.NewBunTranslationRepositoryWithCache(bunDB, cacheSvc, keySerializer)
	areaDefRepo := widgets.NewBunAreaDefinitionRepository(bunDB)
	areaPlacementRepo := widgets.NewBunAreaPlacementRepository(bunDB)

	service := widgets.NewService(
		defRepo,
		instRepo,
		trRepo,
		widgets.WithClock(func() time.Time { return now }),
		widgets.WithAreaDefinitionRepository(areaDefRepo),
		widgets.WithAreaPlacementRepository(areaPlacementRepo),
	)

	areaDefinition, err := service.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
		Code:  "sidebar.primary",
		Name:  "Primary Sidebar",
		Scope: widgets.AreaScopeGlobal,
	})
	if err != nil {
		t.Fatalf("register area definition: %v", err)
	}

	definition, err := service.RegisterDefinition(ctx, widgets.RegisterDefinitionInput{
		Name: "newsletter_signup",
		Schema: map[string]any{
			"fields": []any{
				map[string]any{"name": "headline"},
				map[string]any{"name": "cta_text"},
			},
		},
		Defaults: map[string]any{
			"cta_text": "Subscribe",
		},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	authorID := uuid.MustParse("00000000-0000-0000-0000-000000000321")
	instance, err := service.CreateInstance(ctx, widgets.CreateInstanceInput{
		DefinitionID:  definition.ID,
		Configuration: map[string]any{"headline": "Stay informed"},
		Placement: map[string]any{
			"layout": "stacked",
		},
		VisibilityRules: map[string]any{
			"schedule": map[string]any{
				"starts_at": now.Add(-time.Hour).Format(time.RFC3339),
				"ends_at":   now.Add(time.Hour).Format(time.RFC3339),
			},
		},
		CreatedBy: authorID,
		UpdatedBy: authorID,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	localeID := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	if _, err := service.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   areaDefinition.Code,
		LocaleID:   &localeID,
		InstanceID: instance.ID,
		Metadata: map[string]any{
			"display": "desktop",
		},
	}); err != nil {
		t.Fatalf("assign widget to area: %v", err)
	}

	translation, err := service.AddTranslation(ctx, widgets.AddTranslationInput{
		InstanceID: instance.ID,
		LocaleID:   localeID,
		Content: map[string]any{
			"headline": "Mantente informado",
		},
	})
	if err != nil {
		t.Fatalf("add translation: %v", err)
	}

	definitions, err := service.ListDefinitions(ctx)
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("expected single definition, got %d", len(definitions))
	}

	byDefinition, err := service.ListInstancesByDefinition(ctx, definition.ID)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	if len(byDefinition) != 1 {
		t.Fatalf("expected single instance for definition")
	}

	visible, err := service.EvaluateVisibility(ctx, instance, widgets.VisibilityContext{
		Now:      now,
		LocaleID: &localeID,
	})
	if err != nil {
		t.Fatalf("evaluate visibility: %v", err)
	}
	if !visible {
		t.Fatalf("expected widget to be visible within schedule window")
	}

	resolved, err := service.ResolveArea(ctx, widgets.ResolveAreaInput{
		AreaCode: areaDefinition.Code,
		LocaleID: &localeID,
		Now:      now,
	})
	if err != nil {
		t.Fatalf("resolve area: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected resolved widgets")
	}
	if resolved[0].Instance.ID != instance.ID {
		t.Fatalf("unexpected instance resolved")
	}
	if len(resolved[0].Instance.Translations) != 1 {
		t.Fatalf("expected translations to be attached after resolution")
	}
	if resolved[0].Instance.Translations[0].ID != translation.ID {
		t.Fatalf("unexpected translation resolved")
	}

	// Second resolution should use cached repository data without errors.
	if _, err := service.ResolveArea(ctx, widgets.ResolveAreaInput{
		AreaCode: areaDefinition.Code,
		LocaleID: &localeID,
		Now:      now,
	}); err != nil {
		t.Fatalf("resolve area cached run: %v", err)
	}
}

func registerWidgetModels(t *testing.T, db *bun.DB) {
	t.Helper()
	ctx := context.Background()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS widget_definitions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			schema TEXT NOT NULL,
			defaults TEXT,
			category TEXT,
			icon TEXT,
			deleted_at TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS widget_instances (
			id TEXT PRIMARY KEY,
			definition_id TEXT NOT NULL,
			block_instance_id TEXT,
			area_code TEXT,
			placement_metadata TEXT,
			configuration TEXT NOT NULL,
			visibility_rules TEXT,
			publish_on TEXT,
			unpublish_on TEXT,
			position INTEGER NOT NULL DEFAULT 0,
			created_by TEXT NOT NULL,
			updated_by TEXT NOT NULL,
			deleted_at TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS widget_translations (
			id TEXT PRIMARY KEY,
			widget_instance_id TEXT NOT NULL,
			locale_id TEXT NOT NULL,
			content TEXT NOT NULL,
			deleted_at TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS widget_area_definitions (
			id TEXT PRIMARY KEY,
			code TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			scope TEXT NOT NULL DEFAULT 'global',
			theme_id TEXT,
			template_id TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS widget_area_placements (
			id TEXT PRIMARY KEY,
			area_code TEXT NOT NULL,
			locale_id TEXT,
			instance_id TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			metadata TEXT,
			created_at TEXT,
			updated_at TEXT
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
}
