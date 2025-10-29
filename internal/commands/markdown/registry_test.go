package markdowncmd

import (
	"testing"

	"github.com/goliatone/go-cms/internal/commands"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

func TestRegisterMarkdownCommandsHandlerOptionsApplied(t *testing.T) {
	service := &stubMarkdownService{}
	importApplied := false
	syncApplied := false

	_, err := RegisterMarkdownCommands(nil, service, nil, FeatureGates{
		MarkdownEnabled: func() bool { return true },
	},
		WithImportHandlerOptions(func(h *commands.Handler[ImportDirectoryCommand]) {
			importApplied = true
		}),
		WithSyncHandlerOptions(func(h *commands.Handler[SyncDirectoryCommand]) {
			syncApplied = true
		}),
	)
	if err != nil {
		t.Fatalf("register markdown commands: %v", err)
	}
	if !importApplied {
		t.Fatal("expected import handler options applied")
	}
	if !syncApplied {
		t.Fatal("expected sync handler options applied")
	}
}

func TestRegisterMarkdownCommandsRegistersHandlers(t *testing.T) {
	reg := &recordingRegistry{}
	service := &stubMarkdownService{}

	set, err := RegisterMarkdownCommands(reg, service, nil, FeatureGates{
		MarkdownEnabled: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("register markdown commands: %v", err)
	}
	if set == nil {
		t.Fatal("expected handler set returned")
	}
	if set.Import == nil || set.Sync == nil {
		t.Fatalf("expected import and sync handlers, got %#v", set)
	}
	if len(reg.handlers) != 2 {
		t.Fatalf("expected two handlers registered, got %d", len(reg.handlers))
	}
	if reg.handlers[0] != set.Import {
		t.Fatalf("expected import handler registered first, got %#v", reg.handlers[0])
	}
	if reg.handlers[1] != set.Sync {
		t.Fatalf("expected sync handler registered second, got %#v", reg.handlers[1])
	}
}

func TestRegisterMarkdownCommandsNilRegistrySkipsRegistration(t *testing.T) {
	service := &stubMarkdownService{}
	set, err := RegisterMarkdownCommands(nil, service, nil, FeatureGates{
		MarkdownEnabled: func() bool { return true },
	})
	if err != nil {
		t.Fatalf("register markdown commands: %v", err)
	}
	if set == nil || set.Import == nil || set.Sync == nil {
		t.Fatalf("expected handlers built when registry nil, got %#v", set)
	}
}

func TestRegisterMarkdownCommandsNilServiceError(t *testing.T) {
	if _, err := RegisterMarkdownCommands(nil, nil, nil, FeatureGates{}); err == nil {
		t.Fatal("expected error when service nil")
	}
}

func TestRegisterMarkdownCronRegistersHandler(t *testing.T) {
	service := &stubMarkdownService{
		syncResult: &interfaces.SyncResult{},
	}
	handler := NewSyncDirectoryHandler(service, logging.NoOp(), FeatureGates{
		MarkdownEnabled: func() bool { return true },
	})
	recorder := &recordingCron{}

	cfg := command.HandlerConfig{Expression: "@daily"}
	msg := SyncDirectoryCommand{Directory: "content"}

	if err := RegisterMarkdownCron(recorder.register, handler, cfg, msg); err != nil {
		t.Fatalf("register markdown cron: %v", err)
	}

	if len(recorder.registrations) != 1 {
		t.Fatalf("expected one cron registration, got %d", len(recorder.registrations))
	}
	reg := recorder.registrations[0]
	if reg.config.Expression != cfg.Expression {
		t.Fatalf("expected cron expression %q, got %q", cfg.Expression, reg.config.Expression)
	}
	if reg.handler == nil {
		t.Fatal("expected cron handler function recorded")
	}
	if err := reg.handler(); err != nil {
		t.Fatalf("executing cron handler: %v", err)
	}
	if len(service.syncCalls) != 1 {
		t.Fatalf("expected sync call executed, got %d", len(service.syncCalls))
	}
}

func TestRegisterMarkdownCronNoOpWhenRegistrarNil(t *testing.T) {
	service := &stubMarkdownService{}
	handler := NewSyncDirectoryHandler(service, logging.NoOp(), FeatureGates{
		MarkdownEnabled: func() bool { return true },
	})
	if err := RegisterMarkdownCron(nil, handler, command.HandlerConfig{}, SyncDirectoryCommand{Directory: "content"}); err != nil {
		t.Fatalf("expected nil error when registrar nil, got %v", err)
	}
	if len(service.syncCalls) != 0 {
		t.Fatalf("expected no sync calls when registrar nil, got %d", len(service.syncCalls))
	}
}

func TestRegisterMarkdownCronNoOpWhenHandlerNil(t *testing.T) {
	recorder := &recordingCron{}
	if err := RegisterMarkdownCron(recorder.register, nil, command.HandlerConfig{}, SyncDirectoryCommand{Directory: "content"}); err != nil {
		t.Fatalf("expected nil error when handler nil, got %v", err)
	}
	if len(recorder.registrations) != 0 {
		t.Fatalf("expected no registrations when handler nil, got %d", len(recorder.registrations))
	}
}

type recordingRegistry struct {
	handlers []any
	err      error
}

func (r *recordingRegistry) RegisterCommand(handler any) error {
	if r.err != nil {
		return r.err
	}
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

func (r *recordingCron) register(cfg command.HandlerConfig, handler any) error {
	if r.err != nil {
		return r.err
	}
	var fn func() error
	if h, ok := handler.(func() error); ok {
		fn = h
	}
	r.registrations = append(r.registrations, cronRegistration{
		config:  cfg,
		handler: fn,
	})
	return nil
}
