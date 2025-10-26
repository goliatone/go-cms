package themes

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/goliatone/go-cms/pkg/testsupport"
	"github.com/google/uuid"
)

func TestServiceRegisterAndActivateTheme(t *testing.T) {
	ctx := context.Background()
	idGen := sequentialIDs(
		"00000000-0000-0000-0000-00000000a001",
		"00000000-0000-0000-0000-00000000b001",
	)

	themeRepo := NewMemoryThemeRepository()
	templateRepo := NewMemoryTemplateRepository()
	svc := NewService(themeRepo, templateRepo,
		WithThemeIDGenerator(idGen),
		WithNow(func() time.Time { return time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC) }),
	)

	themeInput := RegisterThemeInput{
		Name:      "Aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
		Config: ThemeConfig{
			WidgetAreas: []ThemeWidgetArea{{Code: "header", Name: "Header"}},
		},
	}

	theme, err := svc.RegisterTheme(ctx, themeInput)
	if err != nil {
		t.Fatalf("register theme: %v", err)
	}
	if theme.ID != uuid.MustParse("00000000-0000-0000-0000-00000000a001") {
		t.Fatalf("unexpected theme id %s", theme.ID)
	}
	if theme.IsActive {
		t.Fatalf("theme should be inactive on creation")
	}

	templateInput := RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "Landing",
		Slug:         "landing",
		TemplatePath: "templates/landing.html.tmpl",
		Regions: map[string]TemplateRegion{
			"hero": {Name: "Hero", AcceptsBlocks: true},
		},
	}
	if _, err := svc.RegisterTemplate(ctx, templateInput); err != nil {
		t.Fatalf("register template: %v", err)
	}

	activated, err := svc.ActivateTheme(ctx, theme.ID)
	if err != nil {
		t.Fatalf("activate theme: %v", err)
	}
	if !activated.IsActive {
		t.Fatalf("expected theme to be active")
	}
}

func TestServiceRegisterTemplateSlugConflict(t *testing.T) {
	ctx := context.Background()
	themeRepo := NewMemoryThemeRepository()
	templateRepo := NewMemoryTemplateRepository()
	svc := NewService(themeRepo, templateRepo)

	theme, err := svc.RegisterTheme(ctx, RegisterThemeInput{
		Name:      "Aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
	})
	if err != nil {
		t.Fatalf("register theme: %v", err)
	}

	input := RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "Landing",
		Slug:         "landing",
		TemplatePath: "templates/landing.html",
		Regions: map[string]TemplateRegion{
			"hero": {Name: "Hero", AcceptsBlocks: true},
		},
	}
	if _, err := svc.RegisterTemplate(ctx, input); err != nil {
		t.Fatalf("register template: %v", err)
	}
	if _, err := svc.RegisterTemplate(ctx, input); err != ErrTemplateSlugConflict {
		t.Fatalf("expected slug conflict, got %v", err)
	}
}

func TestServiceTemplateRegions(t *testing.T) {
	ctx := context.Background()
	themeRepo := NewMemoryThemeRepository()
	templateRepo := NewMemoryTemplateRepository()
	svc := NewService(themeRepo, templateRepo)

	theme, _ := svc.RegisterTheme(ctx, RegisterThemeInput{
		Name:      "Aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
	})
	template, _ := svc.RegisterTemplate(ctx, RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "Landing",
		Slug:         "landing",
		TemplatePath: "templates/landing.html",
		Regions: map[string]TemplateRegion{
			"hero":    {Name: "Hero", AcceptsBlocks: true},
			"sidebar": {Name: "Sidebar", AcceptsWidgets: true},
		},
	})

	regions, err := svc.TemplateRegions(ctx, template.ID)
	if err != nil {
		t.Fatalf("template regions: %v", err)
	}
	if len(regions) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(regions))
	}
}

func TestServiceListActiveSummaries(t *testing.T) {
	ctx := context.Background()
	themeRepo := NewMemoryThemeRepository()
	templateRepo := NewMemoryTemplateRepository()
	svc := NewService(themeRepo, templateRepo)

	basePath := "public"
	theme, _ := svc.RegisterTheme(ctx, RegisterThemeInput{
		Name:      "Aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
		Config: ThemeConfig{
			WidgetAreas: []ThemeWidgetArea{{Code: "header", Name: "Header"}},
			Assets: &ThemeAssets{
				BasePath: &basePath,
				Styles:   []string{"css/site.css"},
			},
		},
	})
	svc.RegisterTemplate(ctx, RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "Landing",
		Slug:         "landing",
		TemplatePath: "templates/landing.html",
		Regions: map[string]TemplateRegion{
			"hero": {Name: "Hero", AcceptsBlocks: true},
		},
	})
	if _, err := svc.ActivateTheme(ctx, theme.ID); err != nil {
		t.Fatalf("activate: %v", err)
	}

	summaries, err := svc.ListActiveSummaries(ctx)
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if len(summaries[0].Assets.Styles) != 1 || summaries[0].Assets.Styles[0] != "public/css/site.css" {
		t.Fatalf("assets not resolved: %+v", summaries[0].Assets)
	}
}

func TestServiceContractUsingFixtures(t *testing.T) {
	ctx := context.Background()
	seeds, expect := loadThemeSeedsFromFixture(t, filepath.Join("testdata", "basic_themes.json"))

	idValues := make([]string, len(expect.IDSequence))
	for i, id := range expect.IDSequence {
		idValues[i] = id.String()
	}

	now := time.Date(2024, 5, 1, 15, 0, 0, 0, time.UTC)
	svc := NewService(
		NewMemoryThemeRepository(),
		NewMemoryTemplateRepository(),
		WithThemeIDGenerator(sequentialIDs(idValues...)),
		WithNow(func() time.Time { return now }),
	)

	if err := Bootstrap(ctx, svc, seeds); err != nil {
		t.Fatalf("bootstrap themes: %v", err)
	}

	themes, err := svc.ListThemes(ctx)
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	if len(themes) != len(expect.ThemeIDs) {
		t.Fatalf("expected %d themes, got %d", len(expect.ThemeIDs), len(themes))
	}
	for _, theme := range themes {
		wantID, ok := expect.ThemeIDs[theme.Name]
		if !ok {
			t.Fatalf("unexpected theme %q", theme.Name)
		}
		if theme.ID != wantID {
			t.Fatalf("theme %q id mismatch: want %s got %s", theme.Name, wantID, theme.ID)
		}
	}

	active, err := svc.ListActiveThemes(ctx)
	if err != nil {
		t.Fatalf("list active themes: %v", err)
	}
	gotActive := make([]string, len(active))
	for i, theme := range active {
		gotActive[i] = theme.Name
	}
	sort.Strings(gotActive)
	wantActive := append([]string{}, expect.ActiveThemes...)
	sort.Strings(wantActive)
	if !reflect.DeepEqual(wantActive, gotActive) {
		t.Fatalf("active themes mismatch: want %v, got %v", wantActive, gotActive)
	}

	for name, themeID := range expect.ThemeIDs {
		templates, err := svc.ListTemplates(ctx, themeID)
		if err != nil {
			t.Fatalf("list templates for %q: %v", name, err)
		}
		if len(templates) == 0 {
			t.Fatalf("expected templates for theme %q", name)
		}
		for _, tpl := range templates {
			key := templateExpectationKey(name, tpl.Slug)
			wantID, ok := expect.TemplateIDs[key]
			if !ok {
				t.Fatalf("unexpected template %q for theme %q", tpl.Slug, name)
			}
			if tpl.ID != wantID {
				t.Fatalf("template %s id mismatch: want %s got %s", key, wantID, tpl.ID)
			}
		}
	}

	auroraID := expect.ThemeIDs["aurora"]
	regionIndex, err := svc.ThemeRegionIndex(ctx, auroraID)
	if err != nil {
		t.Fatalf("theme region index: %v", err)
	}

	var wantRegion map[string][]RegionInfo
	if err := testsupport.LoadGolden(filepath.Join("testdata", "aurora_regions.golden.json"), &wantRegion); err != nil {
		t.Fatalf("load region golden: %v", err)
	}
	if !reflect.DeepEqual(wantRegion, regionIndex) {
		t.Fatalf("region index mismatch: want %#v got %#v", wantRegion, regionIndex)
	}

	landingRegions, err := svc.TemplateRegions(ctx, expect.TemplateIDs["aurora/landing"])
	if err != nil {
		t.Fatalf("template regions: %v", err)
	}
	if !reflect.DeepEqual(wantRegion["landing"], landingRegions) {
		t.Fatalf("template regions mismatch: want %#v got %#v", wantRegion["landing"], landingRegions)
	}

	landingName := "Landing Hero"
	landingPath := "templates/landing_v2.html.tmpl"
	layoutMetadata := map[string]any{"layout": "hero"}
	updated, err := svc.UpdateTemplate(ctx, UpdateTemplateInput{
		TemplateID:   expect.TemplateIDs["aurora/landing"],
		Name:         &landingName,
		TemplatePath: &landingPath,
		Metadata:     layoutMetadata,
	})
	if err != nil {
		t.Fatalf("update template: %v", err)
	}
	if updated.Name != landingName {
		t.Fatalf("expected updated name %q, got %q", landingName, updated.Name)
	}
	if updated.TemplatePath != landingPath {
		t.Fatalf("expected updated path %q, got %q", landingPath, updated.TemplatePath)
	}
	if v := updated.Metadata["layout"]; v != "hero" {
		t.Fatalf("expected metadata layout hero, got %v", v)
	}

	promo, err := svc.RegisterTemplate(ctx, RegisterTemplateInput{
		ThemeID:      auroraID,
		Name:         "Promotional",
		Slug:         "promo",
		TemplatePath: "templates/promo.html.tmpl",
		Regions: map[string]TemplateRegion{
			"body": {Name: "Body", AcceptsBlocks: true},
		},
	})
	if err != nil {
		t.Fatalf("register promo template: %v", err)
	}

	if err := svc.DeleteTemplate(ctx, promo.ID); err != nil {
		t.Fatalf("delete promo template: %v", err)
	}
	if _, err := svc.GetTemplate(ctx, promo.ID); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("expected ErrTemplateNotFound after delete, got %v", err)
	}

	if _, err := svc.DeactivateTheme(ctx, auroraID); err != nil {
		t.Fatalf("deactivate theme: %v", err)
	}
	if _, err := svc.ActivateTheme(ctx, auroraID); err != nil {
		t.Fatalf("reactivate theme: %v", err)
	}

	summaries, err := svc.ListActiveSummaries(ctx)
	if err != nil {
		t.Fatalf("list active summaries: %v", err)
	}
	if len(summaries) == 0 {
		t.Fatalf("expected summaries after activation")
	}
	var auroraSummary *ThemeSummary
	for i := range summaries {
		if summaries[i].Theme.Name == "aurora" {
			auroraSummary = &summaries[i]
			break
		}
	}
	if auroraSummary == nil {
		t.Fatalf("expected aurora summary")
	}
	expectedStyles := []string{"public/css/base.css", "public/css/aurora.css"}
	if !reflect.DeepEqual(expectedStyles, auroraSummary.Assets.Styles) {
		t.Fatalf("expected styles %v, got %v", expectedStyles, auroraSummary.Assets.Styles)
	}
	expectedScripts := []string{"public/js/site.js"}
	if !reflect.DeepEqual(expectedScripts, auroraSummary.Assets.Scripts) {
		t.Fatalf("expected scripts %v, got %v", expectedScripts, auroraSummary.Assets.Scripts)
	}
	if auroraSummary.Assets.Images != nil {
		t.Fatalf("expected no images, got %v", auroraSummary.Assets.Images)
	}
}

func TestBootstrapSeedsThemes(t *testing.T) {
	ctx := context.Background()
	idGen := sequentialIDs(
		"00000000-0000-0000-0000-00000000a001",
		"00000000-0000-0000-0000-00000000b001",
		"00000000-0000-0000-0000-00000000b002",
	)

	themeRepo := NewMemoryThemeRepository()
	templateRepo := NewMemoryTemplateRepository()
	svc := NewService(themeRepo, templateRepo, WithThemeIDGenerator(idGen))

	registry := NewRegistry()
	registry.Register(ThemeSeed{
		Theme: RegisterThemeInput{
			Name:      "Aurora",
			Version:   "1.0.0",
			ThemePath: "themes/aurora",
			Activate:  true,
		},
		Templates: []RegisterTemplateInput{
			{
				Name:         "Landing",
				Slug:         "landing",
				TemplatePath: "templates/landing.html",
				Regions: map[string]TemplateRegion{
					"hero": {Name: "Hero", AcceptsBlocks: true},
				},
			},
		},
	})

	if err := Bootstrap(ctx, svc, registry.List()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	// Idempotent
	if err := Bootstrap(ctx, svc, registry.List()); err != nil {
		t.Fatalf("bootstrap second run: %v", err)
	}

	themes, _ := svc.ListThemes(ctx)
	if len(themes) != 1 {
		t.Fatalf("expected one theme, got %d", len(themes))
	}
	active, _ := svc.ListActiveThemes(ctx)
	if len(active) != 1 {
		t.Fatalf("expected active theme after bootstrap")
	}
}

type themeFixtureFile struct {
	Themes []themeFixtureRecord `json:"themes"`
}

type themeFixtureRecord struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description *string                `json:"description"`
	Version     string                 `json:"version"`
	Author      *string                `json:"author"`
	IsActive    bool                   `json:"is_active"`
	ThemePath   string                 `json:"theme_path"`
	Config      *ThemeConfig           `json:"config"`
	Templates   []templateFixtureEntry `json:"templates"`
}

type templateFixtureEntry struct {
	ID           string                    `json:"id"`
	ThemeID      string                    `json:"theme_id"`
	Name         string                    `json:"name"`
	Slug         string                    `json:"slug"`
	Description  *string                   `json:"description"`
	TemplatePath string                    `json:"template_path"`
	Regions      map[string]TemplateRegion `json:"regions"`
	Metadata     map[string]any            `json:"metadata"`
}

type fixtureExpectations struct {
	ThemeIDs     map[string]uuid.UUID
	TemplateIDs  map[string]uuid.UUID
	ActiveThemes []string
	IDSequence   []uuid.UUID
}

func loadThemeSeedsFromFixture(t *testing.T, fixturePath string) ([]ThemeSeed, fixtureExpectations) {
	t.Helper()

	raw, err := testsupport.LoadFixture(fixturePath)
	if err != nil {
		t.Fatalf("load fixture %q: %v", fixturePath, err)
	}

	var payload themeFixtureFile
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode theme fixture: %v", err)
	}

	expect := fixtureExpectations{
		ThemeIDs:    make(map[string]uuid.UUID, len(payload.Themes)),
		TemplateIDs: make(map[string]uuid.UUID),
	}
	seeds := make([]ThemeSeed, 0, len(payload.Themes))

	for _, theme := range payload.Themes {
		themeID := uuid.MustParse(theme.ID)
		expect.ThemeIDs[theme.Name] = themeID
		if theme.IsActive {
			expect.ActiveThemes = append(expect.ActiveThemes, theme.Name)
		}
		expect.IDSequence = append(expect.IDSequence, themeID)

		seed := ThemeSeed{
			Theme: RegisterThemeInput{
				Name:        theme.Name,
				Description: cloneString(theme.Description),
				Version:     theme.Version,
				Author:      cloneString(theme.Author),
				ThemePath:   theme.ThemePath,
				Activate:    theme.IsActive,
			},
		}
		if theme.Config != nil {
			seed.Theme.Config = cloneThemeConfig(*theme.Config)
		}

		templateInputs := make([]RegisterTemplateInput, 0, len(theme.Templates))
		for _, tpl := range theme.Templates {
			tplID := uuid.MustParse(tpl.ID)
			key := templateExpectationKey(theme.Name, tpl.Slug)
			expect.TemplateIDs[key] = tplID
			expect.IDSequence = append(expect.IDSequence, tplID)

			templateInputs = append(templateInputs, RegisterTemplateInput{
				Name:         tpl.Name,
				Slug:         tpl.Slug,
				Description:  cloneString(tpl.Description),
				TemplatePath: tpl.TemplatePath,
				Regions:      cloneTemplateRegions(tpl.Regions),
				Metadata:     deepCloneMap(tpl.Metadata),
			})
		}
		seed.Templates = templateInputs

		seeds = append(seeds, seed)
	}

	return seeds, expect
}

func templateExpectationKey(themeName, slug string) string {
	return themeName + "/" + canonicalSlug(slug)
}

func sequentialIDs(values ...string) IDGenerator {
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
