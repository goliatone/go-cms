package bootstrap

import (
	"fmt"
	"strings"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// Options captures configuration for markdown CLI bootstraps.
type Options struct {
	ContentDir          string
	Pattern             string
	Recursive           bool
	LocalePatterns      map[string]string
	DefaultLocale       string
	Locales             []string
	TranslationsEnabled *bool
	RequireTranslations *bool
	LoggerProvider      interfaces.LoggerProvider
}

// Module wraps the cms module and the configured markdown service/logger.
type Module struct {
	Module  *cms.Module
	Service interfaces.MarkdownService
	Logger  interfaces.Logger
}

// BuildModule constructs a CMS module configured for markdown operations.
func BuildModule(opts Options) (*Module, error) {
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

	if opts.TranslationsEnabled != nil {
		cfg.I18N.Enabled = *opts.TranslationsEnabled
	}
	if opts.RequireTranslations != nil {
		cfg.I18N.RequireTranslations = *opts.RequireTranslations
	}

	diOpts := []di.Option{}
	if opts.LoggerProvider != nil {
		diOpts = append(diOpts, di.WithLoggerProvider(opts.LoggerProvider))
	}

	module, err := cms.New(cfg, diOpts...)
	if err != nil {
		return nil, fmt.Errorf("initialise cms module: %w", err)
	}

	service := module.Markdown()
	if service == nil {
		return nil, fmt.Errorf("markdown service not configured; ensure markdown feature is enabled")
	}

	logger := logging.MarkdownLogger(module.Container().LoggerProvider())

	return &Module{
		Module:  module,
		Service: service,
		Logger:  logger,
	}, nil
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

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}
