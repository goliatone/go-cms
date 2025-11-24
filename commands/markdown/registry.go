package markdownadapter

import (
	"context"
	"errors"

	markdowncmd "github.com/goliatone/go-cms/internal/commands/markdown"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	command "github.com/goliatone/go-command"
)

// CommandRegistry is the minimal registration contract expected when wiring command handlers.
type CommandRegistry interface {
	RegisterCommand(handler any) error
}

// CronRegistrar matches the function signature used by go-command registries.
type CronRegistrar func(command.HandlerConfig, any) error

// HandlerSet groups the Markdown command handlers produced by RegisterMarkdownCommands.
type HandlerSet struct {
	Import *markdowncmd.ImportDirectoryHandler
	Sync   *markdowncmd.SyncDirectoryHandler
}

// Option customises handler wiring during registration.
type Option func(*options)

type options struct {
	importHandlerOpts []markdowncmd.ImportDirectoryOption
	syncHandlerOpts   []markdowncmd.SyncDirectoryOption
}

// WithImportHandlerOptions forwards options to the ImportDirectoryHandler constructor.
func WithImportHandlerOptions(opts ...markdowncmd.ImportDirectoryOption) Option {
	return func(cfg *options) {
		cfg.importHandlerOpts = append(cfg.importHandlerOpts, opts...)
	}
}

// WithSyncHandlerOptions forwards options to the SyncDirectoryHandler constructor.
func WithSyncHandlerOptions(opts ...markdowncmd.SyncDirectoryOption) Option {
	return func(cfg *options) {
		cfg.syncHandlerOpts = append(cfg.syncHandlerOpts, opts...)
	}
}

// RegisterMarkdownCommands builds Markdown command handlers and registers them with the provided
// registry. A HandlerSet containing the constructed handlers is returned so callers can wire
// additional integrations (dispatcher, cron) as needed.
func RegisterMarkdownCommands(reg CommandRegistry, service interfaces.MarkdownService, provider interfaces.LoggerProvider, gates markdowncmd.FeatureGates, opts ...Option) (*HandlerSet, error) {
	if service == nil {
		return nil, errors.New("markdown command registration: service is nil")
	}

	cfg := options{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	logger := logging.ModuleLogger(provider, "cms.commands.markdown")
	logger = logging.WithFields(logger, map[string]any{
		"component":      "command",
		"command_module": "markdown",
	})

	importHandler := markdowncmd.NewImportDirectoryHandler(service, logger, gates, cfg.importHandlerOpts...)
	syncHandler := markdowncmd.NewSyncDirectoryHandler(service, logger, gates, cfg.syncHandlerOpts...)

	if reg != nil {
		if err := reg.RegisterCommand(importHandler); err != nil {
			return nil, err
		}
		if err := reg.RegisterCommand(syncHandler); err != nil {
			return nil, err
		}
	}

	return &HandlerSet{
		Import: importHandler,
		Sync:   syncHandler,
	}, nil
}

// RegisterMarkdownCron wires the provided sync handler into a cron registrar using the supplied
// command configuration and message payload. The handler is executed with a background context.
func RegisterMarkdownCron(reg CronRegistrar, handler *markdowncmd.SyncDirectoryHandler, cfg command.HandlerConfig, msg markdowncmd.SyncDirectoryCommand) error {
	if reg == nil || handler == nil {
		return nil
	}
	return reg(cfg, func() error {
		return handler.Execute(context.Background(), msg)
	})
}
