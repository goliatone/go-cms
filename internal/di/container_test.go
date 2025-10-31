package di_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/blocks"
	contentcmd "github.com/goliatone/go-cms/internal/commands/content"
	fixtures "github.com/goliatone/go-cms/internal/commands/fixtures"
	markdowncmd "github.com/goliatone/go-cms/internal/commands/markdown"
	pagescmd "github.com/goliatone/go-cms/internal/commands/pages"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/domain"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
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
	if seeder, ok := typeRepo.(interface{ Put(*content.ContentType) }); ok {
		ctID := uuid.New()
		seeder.Put(&content.ContentType{ID: ctID, Name: "article"})

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
	if seeder, ok := typeRepo.(interface{ Put(*content.ContentType) }); ok {
		ctID := uuid.New()
		seeder.Put(&content.ContentType{ID: ctID, Name: "article"})

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

func TestContainerRegistersCommandsWhenEnabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Features.Scheduling = true
	cfg.Features.MediaLibrary = true
	cfg.Features.Widgets = true
	cfg.Commands.Enabled = true
	cfg.Commands.AutoRegisterDispatcher = true

	registry := fixtures.NewRecordingRegistry()
	dispatcher := fixtures.NewRecordingDispatcher()
	cronRecorder := fixtures.NewCronRecorder()

	container, err := di.NewContainer(
		cfg,
		di.WithCommandRegistry(registry),
		di.WithCommandDispatcher(dispatcher),
		di.WithCronRegistrar(cronRecorder.Registrar()),
	)
	if err != nil {
		t.Fatalf("new container with commands: %v", err)
	}

	handlers := container.CommandHandlers()
	const expectedHandlers = 14
	if got := len(handlers); got != expectedHandlers {
		t.Fatalf("expected %d command handlers, got %d", expectedHandlers, got)
	}
	if len(registry.Handlers) != expectedHandlers {
		t.Fatalf("expected registry to record %d handlers, got %d", expectedHandlers, len(registry.Handlers))
	}
	if len(dispatcher.Handlers) != expectedHandlers {
		t.Fatalf("expected dispatcher to receive %d handlers, got %d", expectedHandlers, len(dispatcher.Handlers))
	}
	if len(cronRecorder.Registrations) != 0 {
		t.Fatalf("expected no cron registrations, got %d", len(cronRecorder.Registrations))
	}

	var foundSchedule bool
	for _, handler := range handlers {
		if _, ok := handler.(*contentcmd.ScheduleContentHandler); ok {
			foundSchedule = true
			break
		}
	}
	if !foundSchedule {
		t.Fatal("expected content schedule handler to be registered")
	}
}

func TestContainerRegistersMarkdownCommands(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Markdown = true
	cfg.Commands.Enabled = true

	registry := fixtures.NewRecordingRegistry()

	container, err := di.NewContainer(
		cfg,
		di.WithCommandRegistry(registry),
		di.WithMarkdownService(fakeMarkdownService{}),
	)
	if err != nil {
		t.Fatalf("new container with markdown: %v", err)
	}

	var importFound, syncFound bool
	for _, handler := range container.CommandHandlers() {
		switch handler.(type) {
		case *markdowncmd.ImportDirectoryHandler:
			importFound = true
		case *markdowncmd.SyncDirectoryHandler:
			syncFound = true
		}
	}
	if !importFound {
		t.Fatal("expected markdown import handler to be registered")
	}
	if !syncFound {
		t.Fatal("expected markdown sync handler to be registered")
	}

	if len(registry.Handlers) == 0 {
		t.Fatal("expected registry to record markdown handlers")
	}
}

func TestContainerCronRegistrationUsesConfig(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Features.Scheduling = true
	cfg.Features.MediaLibrary = true
	cfg.Commands.Enabled = true
	cfg.Commands.AutoRegisterDispatcher = true
	cfg.Commands.AutoRegisterCron = true
	cfg.Commands.CleanupAuditCron = "0 3 * * *"

	registry := fixtures.NewRecordingRegistry()
	dispatcher := fixtures.NewRecordingDispatcher()
	cronRecorder := fixtures.NewCronRecorder()

	container, err := di.NewContainer(
		cfg,
		di.WithCommandRegistry(registry),
		di.WithCommandDispatcher(dispatcher),
		di.WithCronRegistrar(cronRecorder.Registrar()),
	)
	if err != nil {
		t.Fatalf("new container with cron commands: %v", err)
	}

	if len(cronRecorder.Registrations) != 1 {
		t.Fatalf("expected 1 cron registration, got %d", len(cronRecorder.Registrations))
	}
	if cronRecorder.Registrations[0].Config.Expression != "0 3 * * *" {
		t.Fatalf("expected cron expression 0 3 * * *, got %q", cronRecorder.Registrations[0].Config.Expression)
	}
	fn := cronRecorder.Registrations[0].Handler
	if fn == nil {
		t.Fatal("expected cron handler function to be registered")
	}

	// Ensure cron handler executes successfully.
	if err := fn(); err != nil {
		t.Fatalf("cron handler execution failed: %v", err)
	}

	if len(container.CommandHandlers()) == 0 {
		t.Fatal("expected command handlers when commands enabled")
	}
}

func TestContainerCronRegistrationDisabledWhenCommandsDisabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Features.Scheduling = true
	cfg.Commands.Enabled = false
	cfg.Commands.AutoRegisterCron = true

	registry := fixtures.NewRecordingRegistry()
	dispatcher := fixtures.NewRecordingDispatcher()
	cronRecorder := fixtures.NewCronRecorder()

	container, err := di.NewContainer(cfg,
		di.WithCommandRegistry(registry),
		di.WithCommandDispatcher(dispatcher),
		di.WithCronRegistrar(cronRecorder.Registrar()),
	)
	if err != nil {
		t.Fatalf("new container with commands disabled: %v", err)
	}

	if len(cronRecorder.Registrations) != 0 {
		t.Fatalf("expected no cron registrations when commands disabled, got %d", len(cronRecorder.Registrations))
	}
	if len(container.CommandHandlers()) != 0 {
		t.Fatalf("expected no command handlers when commands disabled")
	}
}

func TestContainerCommandRegistrationRespectsFeatureGates(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Commands.Enabled = true

	registry := fixtures.NewRecordingRegistry()
	container, err := di.NewContainer(cfg, di.WithCommandRegistry(registry))
	if err != nil {
		t.Fatalf("new container with gated commands: %v", err)
	}

	handlers := container.CommandHandlers()
	const expected = 6
	if len(handlers) != expected {
		t.Fatalf("expected %d handlers when scheduling/media disabled, got %d", expected, len(handlers))
	}

	for _, handler := range handlers {
		switch handler.(type) {
		case *contentcmd.ScheduleContentHandler:
			t.Fatal("did not expect content schedule handler when scheduling disabled")
		case *pagescmd.SchedulePageHandler:
			t.Fatal("did not expect page schedule handler when scheduling disabled")
		}
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

func TestContainerCommandsDisabledSkipsRegistration(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Commands.Enabled = false

	registry := fixtures.NewRecordingRegistry()
	dispatcher := fixtures.NewRecordingDispatcher()

	container, err := di.NewContainer(cfg,
		di.WithCommandRegistry(registry),
		di.WithCommandDispatcher(dispatcher),
	)
	if err != nil {
		t.Fatalf("new container with commands disabled: %v", err)
	}

	if len(container.CommandHandlers()) != 0 {
		t.Fatalf("expected no command handlers when commands disabled")
	}
	if len(registry.Handlers) != 0 {
		t.Fatalf("expected registry to remain empty when commands disabled")
	}
	if len(dispatcher.Handlers) != 0 {
		t.Fatalf("expected dispatcher to remain empty when commands disabled")
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
