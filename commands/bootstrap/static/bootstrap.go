package bootstrap

import (
	"fmt"
	"strings"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/commands"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Options captures the tunable configuration for the static CLI module.
type Options struct {
	OutputDir      string
	BaseURL        string
	Logger         interfaces.LoggerProvider
	Storage        interfaces.StorageProvider
	EnableCommands bool // collect command handlers for CLI execution when true
}

// Resources groups the module runtime and optional command registry used by CLI commands.
type Resources struct {
	Module    *cms.Module
	Collector *CommandCollector
}

// CommandCollector records handlers registered by the DI container so CLI commands can
// invoke them directly when dispatcher integrations are requested.
type CommandCollector struct {
	handlers []any
}

// RegisterCommand satisfies commands.CommandRegistry.
func (c *CommandCollector) RegisterCommand(handler any) error {
	c.handlers = append(c.handlers, handler)
	return nil
}

// Handlers returns the collected handlers.
func (c *CommandCollector) Handlers() []any {
	if len(c.handlers) == 0 {
		return nil
	}
	out := make([]any, len(c.handlers))
	copy(out, c.handlers)
	return out
}

// BuildModule initialises a cms.Module configured for generator operations and, when requested,
// collects command handlers for CLI invocation.
func BuildModule(opts Options) (*Resources, error) {
	cfg := cms.DefaultConfig()
	cfg.Generator.Enabled = true
	if trimmed := strings.TrimSpace(opts.OutputDir); trimmed != "" {
		cfg.Generator.OutputDir = trimmed
	}
	if trimmed := strings.TrimSpace(opts.BaseURL); trimmed != "" {
		cfg.Generator.BaseURL = trimmed
	}

	var collector *CommandCollector
	diOpts := []di.Option{}

	if opts.Logger != nil {
		diOpts = append(diOpts, di.WithLoggerProvider(opts.Logger))
	}
	if opts.Storage != nil {
		diOpts = append(diOpts, di.WithGeneratorStorage(opts.Storage))
	}

	module, err := cms.New(cfg, diOpts...)
	if err != nil {
		return nil, fmt.Errorf("initialise cms module: %w", err)
	}

	if opts.EnableCommands {
		collector = &CommandCollector{
			handlers: make([]any, 0),
		}
		if _, err := commands.RegisterContainerCommands(module.Container(), commands.RegistrationOptions{
			Registry:       collector,
			LoggerProvider: opts.Logger,
		}); err != nil {
			return nil, fmt.Errorf("register static commands: %w", err)
		}
	}

	return &Resources{
		Module:    module,
		Collector: collector,
	}, nil
}
