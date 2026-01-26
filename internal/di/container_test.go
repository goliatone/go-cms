package di_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/activity"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

type warnRecordingLogger struct {
	warns []string
}

func (l *warnRecordingLogger) Trace(string, ...any) {}
func (l *warnRecordingLogger) Debug(string, ...any) {}
func (l *warnRecordingLogger) Info(string, ...any)  {}
func (l *warnRecordingLogger) Error(string, ...any) {}
func (l *warnRecordingLogger) Fatal(string, ...any) {}

func (l *warnRecordingLogger) Warn(msg string, _ ...any) {
	l.warns = append(l.warns, msg)
}

func (l *warnRecordingLogger) WithFields(map[string]any) interfaces.Logger {
	return l
}

func (l *warnRecordingLogger) WithContext(context.Context) interfaces.Logger {
	return l
}

type singleLoggerProvider struct {
	logger interfaces.Logger
}

func (p *singleLoggerProvider) GetLogger(string) interfaces.Logger {
	return p.logger
}

type stubWorkflowEngine struct {
	inputs []interfaces.TransitionInput
}

func (s *stubWorkflowEngine) Transition(ctx context.Context, input interfaces.TransitionInput) (*interfaces.TransitionResult, error) {
	s.inputs = append(s.inputs, input)
	to := input.TargetState
	if strings.TrimSpace(string(to)) == "" {
		to = input.CurrentState
	}
	return &interfaces.TransitionResult{
		EntityID:    input.EntityID,
		EntityType:  input.EntityType,
		Transition:  input.Transition,
		FromState:   input.CurrentState,
		ToState:     to,
		CompletedAt: time.Unix(0, 0),
		ActorID:     input.ActorID,
		Metadata:    input.Metadata,
	}, nil
}

func (s *stubWorkflowEngine) AvailableTransitions(context.Context, interfaces.TransitionQuery) ([]interfaces.WorkflowTransition, error) {
	return nil, nil
}

func (s *stubWorkflowEngine) RegisterWorkflow(context.Context, interfaces.WorkflowDefinition) error {
	return nil
}

type staticWorkflowDefinitionStore struct {
	definitions []interfaces.WorkflowDefinition
	err         error
}

func (s *staticWorkflowDefinitionStore) ListWorkflowDefinitions(context.Context) ([]interfaces.WorkflowDefinition, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.definitions, nil
}

func TestContainerDefaultWorkflowEngine(t *testing.T) {
	cfg := cms.DefaultConfig()

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	if container.WorkflowEngine() == nil {
		t.Fatal("expected default workflow engine to be configured")
	}
}

func TestContainerWorkflowEngineOverride(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Workflow.Provider = "custom"

	stub := &stubWorkflowEngine{}
	container, err := di.NewContainer(cfg, di.WithWorkflowEngine(stub))
	if err != nil {
		t.Fatalf("new container with custom workflow engine: %v", err)
	}

	if container.WorkflowEngine() != stub {
		t.Fatal("expected workflow engine to match injected instance")
	}
}

func TestContainerWorkflowCustomProviderRequiresEngine(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Workflow.Provider = "custom"

	if _, err := di.NewContainer(cfg); !errors.Is(err, di.ErrWorkflowEngineNotProvided) {
		t.Fatalf("expected ErrWorkflowEngineNotProvided, got %v", err)
	}
}

func TestContainerRegistersWorkflowDefinitionsFromConfig(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Workflow.Definitions = []cms.WorkflowDefinitionConfig{
		{
			Entity: "page",
			States: []cms.WorkflowStateConfig{
				{Name: "draft", Initial: true},
				{Name: "review"},
				{Name: "translated"},
			},
			Transitions: []cms.WorkflowTransitionConfig{
				{Name: "submit_review", From: "draft", To: "review"},
				{Name: "translate", From: "review", To: "translated"},
			},
		},
	}

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	engine := container.WorkflowEngine()
	if engine == nil {
		t.Fatalf("expected workflow engine")
	}

	transitions, err := engine.AvailableTransitions(context.Background(), interfaces.TransitionQuery{
		EntityType: "page",
		State:      interfaces.WorkflowState("review"),
	})
	if err != nil {
		t.Fatalf("available transitions: %v", err)
	}
	if len(transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(transitions))
	}
	if transitions[0].Name != "translate" {
		t.Fatalf("expected translate transition, got %s", transitions[0].Name)
	}
}

func TestContainerRegistersWorkflowDefinitionsFromStore(t *testing.T) {
	cfg := cms.DefaultConfig()

	store := &staticWorkflowDefinitionStore{
		definitions: []interfaces.WorkflowDefinition{
			{
				EntityType:   "page",
				InitialState: interfaces.WorkflowState("draft"),
				States: []interfaces.WorkflowStateDefinition{
					{Name: interfaces.WorkflowState("draft")},
					{Name: interfaces.WorkflowState("localized")},
				},
				Transitions: []interfaces.WorkflowTransition{
					{Name: "localize", From: interfaces.WorkflowState("draft"), To: interfaces.WorkflowState("localized")},
				},
			},
		},
	}

	container, err := di.NewContainer(cfg, di.WithWorkflowDefinitionStore(store))
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	engine := container.WorkflowEngine()
	if engine == nil {
		t.Fatalf("expected workflow engine")
	}

	transitions, err := engine.AvailableTransitions(context.Background(), interfaces.TransitionQuery{
		EntityType: "page",
		State:      interfaces.WorkflowState("draft"),
	})
	if err != nil {
		t.Fatalf("available transitions: %v", err)
	}
	if len(transitions) != 1 || transitions[0].Name != "localize" {
		t.Fatalf("expected store-provided transition, got %+v", transitions)
	}
}

func TestContainerWidgetServiceDisabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	svc := container.WidgetService()
	if svc == nil {
		t.Fatalf("expected widget service, got nil")
	}

	if _, err := svc.RegisterDefinition(context.Background(), widgets.RegisterDefinitionInput{Name: "any"}); err == nil {
		t.Fatalf("expected error when widget feature disabled")
	} else if err != widgets.ErrFeatureDisabled {
		t.Fatalf("expected ErrFeatureDisabled, got %v", err)
	}
}

func TestContainerWidgetServiceEnabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Widgets = true

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}
	svc := container.WidgetService()

	if svc == nil {
		t.Fatalf("expected widget service")
	}

	definitions, err := svc.ListDefinitions(context.Background())
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if len(definitions) == 0 {
		t.Fatalf("expected built-in widget definitions to be registered")
	}

	found := false
	for _, def := range definitions {
		if def != nil && def.Name == "newsletter_signup" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected newsletter_signup definition to be registered")
	}
}

func TestContainerWidgetRegistryDeduplicatesConfiguredDefinitions(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Widgets = true
	cfg.Widgets.Definitions = []cms.WidgetDefinitionConfig{
		{
			Name: "hero_banner",
			Schema: map[string]any{
				"fields": []any{
					map[string]any{"name": "cta"},
				},
			},
		},
		{
			Name: "HERO_BANNER",
			Schema: map[string]any{
				"fields": []any{
					map[string]any{"name": "cta"},
				},
			},
			Defaults: map[string]any{
				"cta": "Join now",
			},
		},
	}

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	svc := container.WidgetService()
	definitions, err := svc.ListDefinitions(context.Background())
	if err != nil {
		t.Fatalf("list definitions: %v", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(definitions))
	}
	def := definitions[0]
	if def.Name != "HERO_BANNER" {
		t.Fatalf("expected HERO_BANNER, got %s", def.Name)
	}
	if def.Defaults["cta"] != "Join now" {
		t.Fatalf("expected defaults to come from latest config entry")
	}
}

func TestContainerWidgetConfigRespectsFeatureFlag(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Widgets.Definitions = []cms.WidgetDefinitionConfig{
		{
			Name: "marketing_banner",
			Schema: map[string]any{
				"fields": []any{
					map[string]any{"name": "headline"},
				},
			},
		},
	}

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	svc := container.WidgetService()
	if svc == nil {
		t.Fatalf("expected widget service instance")
	}

	if _, err := svc.ListDefinitions(context.Background()); !errors.Is(err, widgets.ErrFeatureDisabled) {
		t.Fatalf("expected ErrFeatureDisabled when widgets feature disabled, got %v", err)
	}
}

func TestContainerContentRetentionLimitTriggersWarning(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Retention.Content = 1

	recLogger := &warnRecordingLogger{}
	container, err := di.NewContainer(cfg, di.WithLoggerProvider(&singleLoggerProvider{logger: recLogger}))
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	typeRepo := container.ContentTypeRepository()
	if seeder, ok := typeRepo.(interface {
		Put(*content.ContentType) error
	}); ok {
		ctID := uuid.New()
		if err := seeder.Put(&content.ContentType{ID: ctID, Name: "article", Slug: "article"}); err != nil {
			t.Fatalf("seed content type: %v", err)
		}

		ctx := context.Background()
		author := uuid.New()
		contentSvc := container.ContentService()

		created, err := contentSvc.Create(ctx, content.CreateContentRequest{
			ContentTypeID: ctID,
			Slug:          "retention-check",
			Status:        string(domain.StatusDraft),
			CreatedBy:     author,
			UpdatedBy:     author,
			Translations: []content.ContentTranslationInput{
				{
					Locale:  "en",
					Title:   "Retention Check",
					Content: map[string]any{"body": "draft"},
				},
			},
		})
		if err != nil {
			t.Fatalf("create content: %v", err)
		}

		snapshot := content.ContentVersionSnapshot{
			Fields: map[string]any{"headline": "first"},
			Translations: []content.ContentVersionTranslationSnapshot{
				{Locale: "en", Title: "Draft", Content: map[string]any{"body": "draft"}},
			},
		}

		if _, err := contentSvc.CreateDraft(ctx, content.CreateContentDraftRequest{
			ContentID: created.ID,
			Snapshot:  snapshot,
			CreatedBy: author,
		}); err != nil {
			t.Fatalf("first draft: %v", err)
		}

		_, err = contentSvc.CreateDraft(ctx, content.CreateContentDraftRequest{
			ContentID: created.ID,
			Snapshot:  snapshot,
			CreatedBy: author,
		})
		if !errors.Is(err, content.ErrContentVersionRetentionExceeded) {
			t.Fatalf("expected ErrContentVersionRetentionExceeded, got %v", err)
		}
		if len(recLogger.warns) == 0 {
			t.Fatalf("expected retention warning to be recorded")
		}
	} else {
		t.Fatalf("content type repository is not seedable")
	}
}

func TestContainerPageRetentionLimitTriggersWarning(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Retention.Content = 1
	cfg.Retention.Pages = 1

	recLogger := &warnRecordingLogger{}
	container, err := di.NewContainer(cfg, di.WithLoggerProvider(&singleLoggerProvider{logger: recLogger}))
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	typeRepo := container.ContentTypeRepository()
	if seeder, ok := typeRepo.(interface {
		Put(*content.ContentType) error
	}); ok {
		ctID := uuid.New()
		if err := seeder.Put(&content.ContentType{ID: ctID, Name: "article", Slug: "article"}); err != nil {
			t.Fatalf("seed content type: %v", err)
		}

		ctx := context.Background()
		author := uuid.New()
		contentSvc := container.ContentService()
		contentRecord, err := contentSvc.Create(ctx, content.CreateContentRequest{
			ContentTypeID: ctID,
			Slug:          "page-retention",
			Status:        string(domain.StatusDraft),
			CreatedBy:     author,
			UpdatedBy:     author,
			Translations: []content.ContentTranslationInput{
				{
					Locale:  "en",
					Title:   "Retention Page Content",
					Content: map[string]any{"body": "body"},
				},
			},
		})
		if err != nil {
			t.Fatalf("create content: %v", err)
		}

		pageSvc := container.PageService()
		pageRecord, err := pageSvc.Create(ctx, pages.CreatePageRequest{
			ContentID:  contentRecord.ID,
			TemplateID: uuid.New(),
			Slug:       "page-retention",
			Status:     string(domain.StatusDraft),
			CreatedBy:  author,
			UpdatedBy:  author,
			Translations: []pages.PageTranslationInput{
				{Locale: "en", Title: "Retention Page", Path: "/page-retention"},
			},
		})
		if err != nil {
			t.Fatalf("create page: %v", err)
		}

		snapshot := pages.PageVersionSnapshot{
			Metadata: map[string]any{"note": "first draft"},
		}
		if _, err := pageSvc.CreateDraft(ctx, pages.CreatePageDraftRequest{
			PageID:    pageRecord.ID,
			Snapshot:  snapshot,
			CreatedBy: author,
		}); err != nil {
			t.Fatalf("first page draft: %v", err)
		}

		_, err = pageSvc.CreateDraft(ctx, pages.CreatePageDraftRequest{
			PageID:    pageRecord.ID,
			Snapshot:  snapshot,
			CreatedBy: author,
		})
		if !errors.Is(err, pages.ErrVersionRetentionExceeded) {
			t.Fatalf("expected ErrVersionRetentionExceeded, got %v", err)
		}
		if len(recLogger.warns) == 0 {
			t.Fatalf("expected retention warning to be recorded for pages")
		}
	} else {
		t.Fatalf("content type repository is not seedable")
	}
}

func TestContainerBlockRetentionLimitEnforced(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Retention.Blocks = 1

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	ctx := context.Background()
	blockSvc := container.BlockService()

	definition, err := blockSvc.RegisterDefinition(ctx, blocks.RegisterDefinitionInput{
		Name: "hero_banner",
		Schema: map[string]any{
			"fields": []any{"headline"},
		},
	})
	if err != nil {
		t.Fatalf("register definition: %v", err)
	}

	author := uuid.New()
	instance, err := blockSvc.CreateInstance(ctx, blocks.CreateInstanceInput{
		DefinitionID: definition.ID,
		Region:       "hero",
		Position:     0,
		IsGlobal:     true,
		Configuration: map[string]any{
			"headline": "Welcome",
		},
		CreatedBy: author,
		UpdatedBy: author,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	snapshot := blocks.BlockVersionSnapshot{
		Configuration: map[string]any{"headline": "Draft"},
	}
	if _, err := blockSvc.CreateDraft(ctx, blocks.CreateInstanceDraftRequest{
		InstanceID: instance.ID,
		Snapshot:   snapshot,
		CreatedBy:  author,
	}); err != nil {
		t.Fatalf("first block draft: %v", err)
	}

	_, err = blockSvc.CreateDraft(ctx, blocks.CreateInstanceDraftRequest{
		InstanceID: instance.ID,
		Snapshot:   snapshot,
		CreatedBy:  author,
	})
	if !errors.Is(err, blocks.ErrInstanceVersionRetentionExceeded) {
		t.Fatalf("expected ErrInstanceVersionRetentionExceeded, got %v", err)
	}
}

func TestContainerThemeServiceDisabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	svc := container.ThemeService()
	if svc == nil {
		t.Fatalf("expected theme service, got nil")
	}

	if _, err := svc.ListThemes(context.Background()); !errors.Is(err, themes.ErrFeatureDisabled) {
		t.Fatalf("expected ErrFeatureDisabled, got %v", err)
	}
}

func TestContainerThemeServiceEnabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Themes = true

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}
	svc := container.ThemeService()

	theme, err := svc.RegisterTheme(context.Background(), themes.RegisterThemeInput{
		Name:      "aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
		Config: themes.ThemeConfig{
			WidgetAreas: []themes.ThemeWidgetArea{{Code: "hero", Name: "Hero"}},
		},
	})
	if err != nil {
		t.Fatalf("register theme: %v", err)
	}

	_, err = svc.RegisterTemplate(context.Background(), themes.RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "landing",
		Slug:         "landing",
		TemplatePath: "templates/landing.html",
		Regions: map[string]themes.TemplateRegion{
			"hero": {Name: "Hero", AcceptsBlocks: true},
		},
	})
	if err != nil {
		t.Fatalf("register template: %v", err)
	}

	if _, err := svc.ActivateTheme(context.Background(), theme.ID); err != nil {
		t.Fatalf("activate theme: %v", err)
	}

	summaries, err := svc.ListActiveSummaries(context.Background())
	if err != nil {
		t.Fatalf("list active summaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 active theme, got %d", len(summaries))
	}
}

func TestContainerPageServiceIntegratesFeatureServices(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Widgets = true
	cfg.Features.Themes = true
	cfg.Features.MediaLibrary = true

	mediaProvider := &recordingMediaProvider{}
	container, err := di.NewContainer(cfg, di.WithMedia(mediaProvider))
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	themeSvc := container.ThemeService()
	theme, err := themeSvc.RegisterTheme(ctx, themes.RegisterThemeInput{
		Name:      "aurora",
		Version:   "1.0.0",
		ThemePath: "themes/aurora",
		Config: themes.ThemeConfig{
			WidgetAreas: []themes.ThemeWidgetArea{
				{Code: "hero", Name: "Hero"},
			},
		},
	})
	if err != nil {
		t.Fatalf("register theme: %v", err)
	}
	template, err := themeSvc.RegisterTemplate(ctx, themes.RegisterTemplateInput{
		ThemeID:      theme.ID,
		Name:         "landing",
		Slug:         "landing",
		TemplatePath: "templates/landing.html",
		Regions: map[string]themes.TemplateRegion{
			"hero": {
				Name:           "Hero",
				AcceptsWidgets: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("register template: %v", err)
	}

	widgetSvc := container.WidgetService()
	if _, err := widgetSvc.RegisterAreaDefinition(ctx, widgets.RegisterAreaDefinitionInput{
		Code:       "hero",
		Name:       "Hero",
		Scope:      widgets.AreaScopeTemplate,
		TemplateID: &template.ID,
	}); err != nil {
		t.Fatalf("register area definition: %v", err)
	}
	definition, err := widgetSvc.RegisterDefinition(ctx, widgets.RegisterDefinitionInput{
		Name: "hero_banner",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("register widget definition: %v", err)
	}
	instance, err := widgetSvc.CreateInstance(ctx, widgets.CreateInstanceInput{
		DefinitionID: definition.ID,
		Position:     0,
		CreatedBy:    uuid.New(),
		UpdatedBy:    uuid.New(),
	})
	if err != nil {
		t.Fatalf("create widget instance: %v", err)
	}
	if _, err := widgetSvc.AssignWidgetToArea(ctx, widgets.AssignWidgetToAreaInput{
		AreaCode:   "hero",
		InstanceID: instance.ID,
	}); err != nil {
		t.Fatalf("assign widget to area: %v", err)
	}

	pageID := uuid.New()
	translation := &pages.PageTranslation{
		ID:        uuid.New(),
		PageID:    pageID,
		LocaleID:  uuid.New(),
		Title:     "Home",
		Path:      "/",
		CreatedAt: now,
		UpdatedAt: now,
		MediaBindings: media.BindingSet{
			"hero_image": {
				{
					Slot: "hero_image",
					Reference: interfaces.MediaReference{
						ID: "asset-1",
					},
				},
			},
		},
	}
	page := &pages.Page{
		ID:             pageID,
		ContentID:      uuid.New(),
		CurrentVersion: 1,
		TemplateID:     template.ID,
		Slug:           "home",
		Status:         string(domain.StatusPublished),
		CreatedBy:      uuid.New(),
		UpdatedBy:      uuid.New(),
		CreatedAt:      now,
		UpdatedAt:      now,
		Translations:   []*pages.PageTranslation{translation},
	}
	if _, err := container.PageRepository().Create(ctx, page); err != nil {
		t.Fatalf("seed page: %v", err)
	}

	results, err := container.PageService().List(ctx)
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 page, got %d", len(results))
	}

	enriched := results[0]
	widgetPlacements := enriched.Widgets["hero"]
	if len(widgetPlacements) == 0 {
		t.Fatalf("expected hero widgets to be attached")
	}
	if len(enriched.Translations) == 0 {
		t.Fatalf("expected translations to be present")
	}
	resolved := enriched.Translations[0].ResolvedMedia["hero_image"]
	if len(resolved) == 0 {
		t.Fatalf("expected media bindings to be resolved")
	}
	if mediaProvider.resolveCalls == 0 {
		t.Fatalf("expected media provider to be invoked")
	}
}

func TestContainerGeneratorServiceDisabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Generator.Enabled = false

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	svc := container.GeneratorService()
	if svc == nil {
		t.Fatal("expected generator service reference")
	}

	if _, err := svc.Build(context.Background(), generator.BuildOptions{}); !errors.Is(err, generator.ErrServiceDisabled) {
		t.Fatalf("expected ErrServiceDisabled, got %v", err)
	}
}

func TestContainerGeneratorServiceEnabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Themes = true
	cfg.Features.Widgets = true
	cfg.Generator.Enabled = true

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	svc := container.GeneratorService()
	if svc == nil {
		t.Fatal("expected generator service")
	}

	result, err := svc.Build(context.Background(), generator.BuildOptions{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if result == nil {
		t.Fatal("expected build result")
	}
	if result.PagesBuilt != 0 {
		t.Fatalf("expected zero pages built with empty repositories, got %d", result.PagesBuilt)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no build errors, got %d", len(result.Errors))
	}
}

type recordingMediaProvider struct {
	resolveCalls int
}

func (p *recordingMediaProvider) Resolve(_ context.Context, req interfaces.MediaResolveRequest) (*interfaces.MediaAsset, error) {
	p.resolveCalls++
	url := "https://cdn.example.test/assets/" + strings.TrimPrefix(req.Reference.ID, "/")
	return &interfaces.MediaAsset{
		Reference: req.Reference,
		Source: &interfaces.MediaResource{
			URL: url,
		},
		Renditions: map[string]*interfaces.MediaResource{
			"original": {URL: url},
		},
	}, nil
}

func (p *recordingMediaProvider) ResolveBatch(ctx context.Context, reqs []interfaces.MediaResolveRequest) (map[string]*interfaces.MediaAsset, error) {
	out := make(map[string]*interfaces.MediaAsset, len(reqs))
	for _, req := range reqs {
		asset, err := p.Resolve(ctx, req)
		if err != nil {
			return nil, err
		}
		key := req.Reference.ID
		if key == "" {
			key = req.Reference.Path
		}
		out[key] = asset
	}
	return out, nil
}

func (p *recordingMediaProvider) Invalidate(context.Context, ...interfaces.MediaReference) error {
	return nil
}

func TestContainerTranslationFlagsPropagateToServices(t *testing.T) {
	t.Parallel()

	type expectation struct {
		contentErr bool
		pageErr    bool
		menuErr    bool
	}

	cases := []struct {
		name     string
		enabled  bool
		required bool
		override bool
		expect   expectation
	}{
		{
			name:     "enabledAndRequired",
			enabled:  true,
			required: true,
			expect: expectation{
				contentErr: true,
				pageErr:    true,
				menuErr:    true,
			},
		},
		{
			name:     "enabledAndOptional",
			enabled:  true,
			required: false,
			expect: expectation{
				contentErr: false,
				pageErr:    false,
				menuErr:    false,
			},
		},
		{
			name:     "disabledIgnoresRequirement",
			enabled:  false,
			required: true,
			expect: expectation{
				contentErr: false,
				pageErr:    false,
				menuErr:    false,
			},
		},
		{
			name:     "overrideBypassesRequirement",
			enabled:  true,
			required: true,
			override: true,
			expect: expectation{
				contentErr: false,
				pageErr:    false,
				menuErr:    false,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := cms.DefaultConfig()
			cfg.DefaultLocale = "en"
			cfg.I18N.Enabled = tc.enabled
			cfg.I18N.RequireTranslations = tc.required
			cfg.I18N.Locales = []string{"en"}

			container, err := di.NewContainer(cfg)
			if err != nil {
				t.Fatalf("new container: %v", err)
			}

			typeRepo := container.ContentTypeRepository()
			seeder, ok := typeRepo.(interface {
				Put(*content.ContentType) error
			})
			if !ok {
				t.Fatalf("content type repository cannot be seeded")
			}
			typeID := uuid.New()
			if err := seeder.Put(&content.ContentType{
				ID:   typeID,
				Name: "article",
				Slug: "article",
			}); err != nil {
				t.Fatalf("seed content type: %v", err)
			}

			ctx := context.Background()
			author := uuid.New()

			contentSvc := container.ContentService()
			pageSvc := container.PageService()
			menuSvc := container.MenuService()

			contentReq := content.CreateContentRequest{
				ContentTypeID:            typeID,
				Slug:                     "content-" + strings.ReplaceAll(uuid.NewString(), "-", ""),
				Status:                   string(domain.StatusDraft),
				CreatedBy:                author,
				UpdatedBy:                author,
				AllowMissingTranslations: tc.override,
			}

			contentRecord, err := contentSvc.Create(ctx, contentReq)
			if tc.expect.contentErr {
				if !errors.Is(err, content.ErrNoTranslations) {
					t.Fatalf("expected ErrNoTranslations, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("create content without translations: %v", err)
				}
				if contentRecord == nil {
					t.Fatalf("expected content record, got nil")
				}
				if len(contentRecord.Translations) != 0 {
					t.Fatalf("expected no translations, got %d", len(contentRecord.Translations))
				}
			}

			seedContent, err := contentSvc.Create(ctx, content.CreateContentRequest{
				ContentTypeID: typeID,
				Slug:          "seed-" + strings.ReplaceAll(uuid.NewString(), "-", ""),
				Status:        string(domain.StatusDraft),
				CreatedBy:     author,
				UpdatedBy:     author,
				Translations: []content.ContentTranslationInput{
					{
						Locale: "en",
						Title:  "Seed",
					},
				},
			})
			if err != nil {
				t.Fatalf("seed content: %v", err)
			}

			pageReq := pages.CreatePageRequest{
				ContentID:                seedContent.ID,
				TemplateID:               uuid.New(),
				Slug:                     "page-" + strings.ReplaceAll(uuid.NewString(), "-", ""),
				Status:                   string(domain.StatusDraft),
				CreatedBy:                author,
				UpdatedBy:                author,
				AllowMissingTranslations: tc.override,
			}

			pageRecord, err := pageSvc.Create(ctx, pageReq)
			if tc.expect.pageErr {
				if !errors.Is(err, pages.ErrNoPageTranslations) {
					t.Fatalf("expected ErrNoPageTranslations, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("create page without translations: %v", err)
				}
				if pageRecord == nil {
					t.Fatalf("expected page record, got nil")
				}
				if len(pageRecord.Translations) != 0 {
					t.Fatalf("expected no page translations, got %d", len(pageRecord.Translations))
				}
			}

			menuDesc := "test"
			menu, err := menuSvc.CreateMenu(ctx, menus.CreateMenuInput{
				Code:        "menu-" + strings.ReplaceAll(uuid.NewString(), "-", ""),
				Description: &menuDesc,
				CreatedBy:   author,
				UpdatedBy:   author,
			})
			if err != nil {
				t.Fatalf("create menu: %v", err)
			}

			_, err = menuSvc.AddMenuItem(ctx, menus.AddMenuItemInput{
				MenuID:                   menu.ID,
				Position:                 0,
				Target:                   map[string]any{"type": "external", "url": "https://example.com"},
				CreatedBy:                author,
				UpdatedBy:                author,
				AllowMissingTranslations: tc.override,
			})
			if tc.expect.menuErr {
				if !errors.Is(err, menus.ErrMenuItemTranslations) {
					t.Fatalf("expected ErrMenuItemTranslations, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("add menu item without translations: %v", err)
			}
		})
	}
}

type fakeMarkdownService struct{}

func (fakeMarkdownService) Load(context.Context, string, interfaces.LoadOptions) (*interfaces.Document, error) {
	return nil, nil
}

func (fakeMarkdownService) LoadDirectory(context.Context, string, interfaces.LoadOptions) ([]*interfaces.Document, error) {
	return nil, nil
}

func (fakeMarkdownService) Render(context.Context, []byte, interfaces.ParseOptions) ([]byte, error) {
	return nil, nil
}

func (fakeMarkdownService) RenderDocument(context.Context, *interfaces.Document, interfaces.ParseOptions) ([]byte, error) {
	return nil, nil
}

func (fakeMarkdownService) Import(context.Context, *interfaces.Document, interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	return nil, nil
}

func (fakeMarkdownService) ImportDirectory(context.Context, string, interfaces.ImportOptions) (*interfaces.ImportResult, error) {
	return &interfaces.ImportResult{}, nil
}

func (fakeMarkdownService) Sync(context.Context, string, interfaces.SyncOptions) (*interfaces.SyncResult, error) {
	return &interfaces.SyncResult{}, nil
}

func TestContainerShortcodeCacheProviderSelection(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Shortcodes = true
	cfg.Shortcodes.Enabled = true
	cfg.Shortcodes.Cache.Enabled = true
	cfg.Shortcodes.Cache.Provider = "shortcodes"

	cache := &trackingCache{store: map[string]any{}}
	metrics := &shortcodeTestMetrics{cacheHits: map[string]int{}}

	container, err := di.NewContainer(cfg,
		di.WithShortcodeCacheProvider("shortcodes", cache),
		di.WithShortcodeMetrics(metrics),
	)
	if err != nil {
		t.Fatalf("NewContainer error: %v", err)
	}

	svc := container.ShortcodeService()
	if svc == nil {
		t.Fatal("expected shortcode service to be configured")
	}

	ctx := interfaces.ShortcodeContext{}
	if _, err := svc.Render(ctx, "youtube", map[string]any{"id": "dQw4w9WgXcQ"}, ""); err != nil {
		t.Fatalf("first render error: %v", err)
	}
	if cache.setCount != 1 {
		t.Fatalf("expected cache Set to be called once, got %d", cache.setCount)
	}

	if _, err := svc.Render(ctx, "youtube", map[string]any{"id": "dQw4w9WgXcQ"}, ""); err != nil {
		t.Fatalf("second render error: %v", err)
	}
	if cache.hitCount != 1 {
		t.Fatalf("expected cache hit on second render, got %d", cache.hitCount)
	}
	if metrics.cacheHits["youtube"] != 1 {
		t.Fatalf("expected metrics to record cache hit, got %d", metrics.cacheHits["youtube"])
	}
}

func TestContainerActivityHooksFanOut(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Activity = true
	cfg.Activity.Enabled = true

	hook := &activity.CaptureHook{}
	container, err := di.NewContainer(cfg, di.WithActivityHooks(activity.Hooks{hook}))
	if err != nil {
		t.Fatalf("NewContainer error: %v", err)
	}

	menuSvc := container.MenuService()
	actorID := uuid.New()
	if _, err := menuSvc.CreateMenu(context.Background(), menus.CreateMenuInput{
		Code:      "primary",
		CreatedBy: actorID,
		UpdatedBy: actorID,
	}); err != nil {
		t.Fatalf("create menu: %v", err)
	}

	if len(hook.Events) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(hook.Events))
	}
	event := hook.Events[0]
	if event.Verb != "create" || event.ObjectType != "menu" {
		t.Fatalf("unexpected event payload: %+v", event)
	}
	if event.ActorID != actorID.String() {
		t.Fatalf("expected actor %s got %s", actorID, event.ActorID)
	}
	if event.Channel != cfg.Activity.Channel {
		t.Fatalf("expected channel %s got %s", cfg.Activity.Channel, event.Channel)
	}
	if event.Metadata["code"] != "primary" {
		t.Fatalf("expected code metadata primary got %v", event.Metadata["code"])
	}
}

type trackingCache struct {
	store    map[string]any
	setCount int
	hitCount int
}

func (c *trackingCache) Get(_ context.Context, key string) (any, error) {
	if value, ok := c.store[key]; ok {
		c.hitCount++
		return value, nil
	}
	return nil, errors.New("miss")
}

func (c *trackingCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	if c.store == nil {
		c.store = map[string]any{}
	}
	c.store[key] = value
	c.setCount++
	return nil
}

func (c *trackingCache) Delete(_ context.Context, key string) error {
	delete(c.store, key)
	return nil
}

func (c *trackingCache) Clear(_ context.Context) error {
	for key := range c.store {
		delete(c.store, key)
	}
	return nil
}

type shortcodeTestMetrics struct {
	cacheHits map[string]int
}

func (m *shortcodeTestMetrics) ObserveRenderDuration(string, time.Duration) {}

func (m *shortcodeTestMetrics) IncrementRenderError(string) {}

func (m *shortcodeTestMetrics) IncrementCacheHit(shortcode string) {
	m.cacheHits[shortcode]++
}
