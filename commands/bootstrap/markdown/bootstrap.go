package bootstrap

import (
	"fmt"
	"strings"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/commands"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// Options captures the tunable configuration shared across markdown CLI commands.
type Options struct {
	ContentDir     string
	Pattern        string
	Recursive      bool
	LocalePatterns map[string]string
	DefaultLocale  string
	Locales        []string
	LoggerProvider interfaces.LoggerProvider
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

// BuildModule constructs a cms.Module configured for markdown ingestion using the supplied options.
func BuildModule(opts Options) (*Resources, error) {
	cfg := cms.DefaultConfig()

	cfg.Features.Markdown = true
	cfg.Markdown.Enabled = true
	cfg.Markdown.ContentDir = strings.TrimSpace(opts.ContentDir)
	if cfg.Markdown.ContentDir == "" {
		cfg.Markdown.ContentDir = "content"
	}
	if trimmed := strings.TrimSpace(opts.Pattern); trimmed != "" {
		cfg.Markdown.Pattern = trimmed
	}
	if opts.LocalePatterns != nil {
		cfg.Markdown.LocalePatterns = opts.LocalePatterns
	}
	cfg.Markdown.Recursive = opts.Recursive

	defaultLocale := strings.TrimSpace(opts.DefaultLocale)
	if defaultLocale != "" {
		cfg.Markdown.DefaultLocale = defaultLocale
		cfg.DefaultLocale = defaultLocale
	}

	if len(opts.Locales) > 0 {
		cfg.Markdown.Locales = cloneStrings(opts.Locales)
		cfg.I18N.Locales = cloneStrings(opts.Locales)
	} else if len(cfg.I18N.Locales) == 0 {
		cfg.I18N.Locales = []string{cfg.DefaultLocale}
	}

	var collector *CommandCollector
	diOpts := []di.Option{}

	if opts.LoggerProvider != nil {
		diOpts = append(diOpts, di.WithLoggerProvider(opts.LoggerProvider))
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
			LoggerProvider: opts.LoggerProvider,
		}); err != nil {
			return nil, fmt.Errorf("register markdown commands: %w", err)
		}
	}

	return &Resources{
		Module:    module,
		Collector: collector,
	}, nil
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

// SplitLocales parses a comma separated locale list into a trimmed slice.
func SplitLocales(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	locales := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			locales = append(locales, trimmed)
		}
	}
	return locales
}

// ParseUUID converts the supplied string into a UUID, returning uuid.Nil when the input is empty.
func ParseUUID(value string) (uuid.UUID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return uuid.Nil, nil
	}
	return uuid.Parse(trimmed)
}

// ParseUUIDPointer returns a pointer to the parsed UUID, or nil when the value is empty.
func ParseUUIDPointer(value string) (*uuid.UUID, error) {
	id, err := ParseUUID(value)
	if err != nil {
		return nil, err
	}
	if id == uuid.Nil {
		return nil, nil
	}
	return &id, nil
}
