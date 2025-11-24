package commands

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	auditcmd "github.com/goliatone/go-cms/internal/commands/audit"
	staticcmd "github.com/goliatone/go-cms/internal/commands/static"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

func TestRegisterContainerCommandsBuildsHandlers(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Features.Scheduling = true
	cfg.Features.MediaLibrary = true
	cfg.Features.Widgets = true
	cfg.Features.Markdown = true
	cfg.Generator.Enabled = true

	registry := &recordingRegistry{}
	dispatcher := &recordingDispatcher{}
	cron := &recordingCron{}

	container, err := di.NewContainer(cfg, di.WithMarkdownService(fakeMarkdownService{}))
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	result, err := RegisterContainerCommands(container, RegistrationOptions{
		Registry:         registry,
		Dispatcher:       dispatcher,
		CronRegistrar:    cron.Registrar(),
		CleanupAuditCron: "@weekly",
	})
	if err != nil {
		t.Fatalf("register commands: %v", err)
	}

	if len(result.Handlers) == 0 {
		t.Fatal("expected command handlers to be constructed")
	}
	if len(result.Handlers) != len(registry.handlers) {
		t.Fatalf("expected registry to record all handlers, got %d of %d", len(registry.handlers), len(result.Handlers))
	}
	if len(dispatcher.subscriptions) == 0 {
		t.Fatal("expected dispatcher subscriptions when dispatcher provided")
	}
	if len(cron.registrations) == 0 {
		t.Fatal("expected cron registrations when cron registrar provided")
	}
	if got := cron.registrations[0].config.Expression; got != "@weekly" {
		t.Fatalf("expected cleanup cron expression override, got %q", got)
	}
}

func TestRegisterContainerCommandsWithoutRegistrars(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	result, err := RegisterContainerCommands(container, RegistrationOptions{})
	if err != nil {
		t.Fatalf("register commands: %v", err)
	}
	if len(result.Handlers) == 0 {
		t.Fatal("expected handlers to be built even without registrars")
	}
	if len(result.Subscriptions) != 0 {
		t.Fatalf("expected no dispatcher subscriptions without dispatcher, got %d", len(result.Subscriptions))
	}
}

func TestRegisterContainerCommandsSkipsSitemapWhenDisabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Versioning = true
	cfg.Generator.Enabled = true
	cfg.Generator.GenerateSitemap = false

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	result, err := RegisterContainerCommands(container, RegistrationOptions{})
	if err != nil {
		t.Fatalf("register commands: %v", err)
	}

	for _, handler := range result.Handlers {
		if _, ok := handler.(*staticcmd.BuildSitemapHandler); ok {
			t.Fatal("expected sitemap handler not to be registered when sitemap generation is disabled")
		}
	}
}

func TestRegisterContainerCommandsRegistersAuditWhenSchedulingDisabled(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Scheduling = false

	container, err := di.NewContainer(cfg)
	if err != nil {
		t.Fatalf("new container: %v", err)
	}

	result, err := RegisterContainerCommands(container, RegistrationOptions{})
	if err != nil {
		t.Fatalf("register commands: %v", err)
	}

	var hasExport, hasReplay bool
	for _, handler := range result.Handlers {
		switch handler.(type) {
		case *auditcmd.ExportAuditHandler:
			hasExport = true
		case *auditcmd.ReplayAuditHandler:
			hasReplay = true
		}
	}
	if !hasExport {
		t.Fatal("expected export audit handler registered when recorder is available")
	}
	if !hasReplay {
		t.Fatal("expected replay audit handler registered when worker is available")
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
	return nil, nil
}

func (fakeMarkdownService) Sync(context.Context, string, interfaces.SyncOptions) (*interfaces.SyncResult, error) {
	return nil, nil
}

type recordingRegistry struct {
	handlers []any
}

func (r *recordingRegistry) RegisterCommand(handler any) error {
	r.handlers = append(r.handlers, handler)
	return nil
}

type cronRegistration struct {
	config  command.HandlerConfig
	handler func() error
}

type recordingCron struct {
	registrations []cronRegistration
	err           error
}

func (c *recordingCron) Registrar() CronRegistrar {
	return func(cfg command.HandlerConfig, handler any) error {
		if c.err != nil {
			return c.err
		}
		var fn func() error
		if h, ok := handler.(func() error); ok {
			fn = h
		}
		c.registrations = append(c.registrations, cronRegistration{
			config:  cfg,
			handler: fn,
		})
		return nil
	}
}

type recordingDispatcher struct {
	handlers      []any
	subscriptions []*recordingSubscription
	err           error
}

func (d *recordingDispatcher) RegisterCommand(handler any) (CommandSubscription, error) {
	if d.err != nil {
		return nil, d.err
	}
	d.handlers = append(d.handlers, handler)
	sub := &recordingSubscription{handler: handler}
	d.subscriptions = append(d.subscriptions, sub)
	return sub, nil
}

type recordingSubscription struct {
	handler      any
	unsubscribed bool
}

func (s *recordingSubscription) Unsubscribe() {
	s.unsubscribed = true
}
