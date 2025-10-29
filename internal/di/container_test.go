package di_test

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-cms"
	contentcmd "github.com/goliatone/go-cms/internal/commands/content"
	fixtures "github.com/goliatone/go-cms/internal/commands/fixtures"
	markdowncmd "github.com/goliatone/go-cms/internal/commands/markdown"
	pagescmd "github.com/goliatone/go-cms/internal/commands/pages"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

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

	if _, err := svc.Build(context.Background(), generator.BuildOptions{}); !errors.Is(err, generator.ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got %v", err)
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
