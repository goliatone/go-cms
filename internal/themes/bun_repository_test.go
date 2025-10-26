package themes_test

import (
	"context"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/pkg/testsupport"
	repocache "github.com/goliatone/go-repository-cache/cache"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
)

func TestThemeRepositories_WithBunAndCache(t *testing.T) {
	ctx := context.Background()

	sqlDB, err := testsupport.NewSQLiteMemoryDB()
	if err != nil {
		t.Fatalf("new sqlite db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	bunDB.SetMaxOpenConns(1)

	registerThemeModels(t, bunDB)

	cacheCfg := repocache.DefaultConfig()
	cacheCfg.TTL = time.Minute
	cacheSvc, err := repocache.NewCacheService(cacheCfg)
	if err != nil {
		t.Fatalf("cache service: %v", err)
	}
	keySerializer := repocache.NewDefaultKeySerializer()

	now := time.Date(2024, 5, 2, 9, 0, 0, 0, time.UTC)
	themeRepo := themes.NewBunThemeRepositoryWithCache(bunDB, cacheSvc, keySerializer)
	templateRepo := themes.NewBunTemplateRepositoryWithCache(bunDB, cacheSvc, keySerializer)
	svc := themes.NewService(
		themeRepo,
		templateRepo,
		themes.WithThemeIDGenerator(sequentialUUIDs(
			"00000000-0000-0000-0000-00000000a301",
			"00000000-0000-0000-0000-00000000b401",
		)),
		themes.WithNow(func() time.Time { return now }),
	)

	basePath := "public"
	themeInput := themes.RegisterThemeInput{
		Name:      "aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
		Config: themes.ThemeConfig{
			WidgetAreas: []themes.ThemeWidgetArea{
				{Code: "header.global", Name: "Global Header"},
			},
			Assets: &themes.ThemeAssets{
				BasePath: &basePath,
				Styles:   []string{"css/base.css"},
				Scripts:  []string{"js/site.js"},
			},
		},
	}
	theme, err := svc.RegisterTheme(ctx, themeInput)
	if err != nil {
		t.Fatalf("register theme: %v", err)
	}

	template, err := svc.RegisterTemplate(ctx, themes.RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "Landing",
		Slug:         "landing",
		TemplatePath: "templates/landing.html.tmpl",
		Regions: map[string]themes.TemplateRegion{
			"hero": {Name: "Hero Banner", AcceptsBlocks: true},
		},
	})
	if err != nil {
		t.Fatalf("register template: %v", err)
	}

	if _, err := svc.ActivateTheme(ctx, theme.ID); err != nil {
		t.Fatalf("activate theme: %v", err)
	}

	if _, err := svc.GetTheme(ctx, theme.ID); err != nil {
		t.Fatalf("first get theme: %v", err)
	}
	if _, err := svc.GetTheme(ctx, theme.ID); err != nil {
		t.Fatalf("cached get theme: %v", err)
	}

	if _, err := svc.GetTemplate(ctx, template.ID); err != nil {
		t.Fatalf("first get template: %v", err)
	}
	if _, err := svc.GetTemplate(ctx, template.ID); err != nil {
		t.Fatalf("cached get template: %v", err)
	}

	regions, err := svc.TemplateRegions(ctx, template.ID)
	if err != nil {
		t.Fatalf("template regions: %v", err)
	}
	if len(regions) != 1 || regions[0].Key != "hero" {
		t.Fatalf("unexpected regions %#v", regions)
	}

	newPath := "templates/landing_v2.html.tmpl"
	if _, err := svc.UpdateTemplate(ctx, themes.UpdateTemplateInput{
		TemplateID:   template.ID,
		TemplatePath: &newPath,
	}); err != nil {
		t.Fatalf("update template: %v", err)
	}

	updated, err := svc.GetTemplate(ctx, template.ID)
	if err != nil {
		t.Fatalf("get updated template: %v", err)
	}
	if updated.TemplatePath != newPath {
		t.Fatalf("expected template path %q, got %q", newPath, updated.TemplatePath)
	}

	summaries, err := svc.ListActiveSummaries(ctx)
	if err != nil {
		t.Fatalf("list active summaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected one active theme summary, got %d", len(summaries))
	}
	if len(summaries[0].Assets.Styles) != 1 || summaries[0].Assets.Styles[0] != "public/css/base.css" {
		t.Fatalf("expected resolved styles, got %#v", summaries[0].Assets.Styles)
	}
	if len(summaries[0].Assets.Scripts) != 1 || summaries[0].Assets.Scripts[0] != "public/js/site.js" {
		t.Fatalf("expected resolved scripts, got %#v", summaries[0].Assets.Scripts)
	}
}

func registerThemeModels(t *testing.T, db *bun.DB) {
	t.Helper()

	ctx := context.Background()
	models := []any{
		(*themes.Theme)(nil),
		(*themes.Template)(nil),
	}
	for _, model := range models {
		if _, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx); err != nil {
			t.Fatalf("create table %T: %v", model, err)
		}
	}
}

func sequentialUUIDs(values ...string) themes.IDGenerator {
	ids := make([]uuid.UUID, len(values))
	for i, value := range values {
		ids[i] = uuid.MustParse(value)
	}
	var idx int
	return func() uuid.UUID {
		if idx >= len(ids) {
			return uuid.New()
		}
		id := ids[idx]
		idx++
		return id
	}
}
