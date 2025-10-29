package logging

import (
	"context"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

const (
	rootModule      = "cms"
	contentModule   = "cms.content"
	pagesModule     = "cms.pages"
	schedulerModule = "cms.scheduler"
	markdownModule  = "cms.markdown"
)

const (
	fieldMarkdownPath   = "markdown_path"
	fieldMarkdownLocale = "locale"
	fieldMarkdownAction = "sync_action"
)

// ModuleLogger returns a module-scoped logger, defaulting to a no-op
// implementation when no provider is supplied. The returned logger attaches
// the module identifier as structured context so downstream entries can be
// filtered predictably.
func ModuleLogger(provider interfaces.LoggerProvider, module string) interfaces.Logger {
	if module == "" {
		module = rootModule
	}

	logger := NoOp()
	if provider != nil {
		if provided := provider.GetLogger(module); provided != nil {
			logger = provided
		}
	}

	if fieldsLogger, ok := logger.(interfaces.FieldsLogger); ok {
		return fieldsLogger.WithFields(map[string]any{
			"module": module,
		})
	}

	return WithFields(logger, map[string]any{
		"module": module,
	})
}

// ContentLogger returns the logger namespace reserved for content services.
func ContentLogger(provider interfaces.LoggerProvider) interfaces.Logger {
	return ModuleLogger(provider, contentModule)
}

// PagesLogger returns the logger namespace reserved for page services.
func PagesLogger(provider interfaces.LoggerProvider) interfaces.Logger {
	return ModuleLogger(provider, pagesModule)
}

// SchedulerLogger returns the logger namespace reserved for scheduler workers.
func SchedulerLogger(provider interfaces.LoggerProvider) interfaces.Logger {
	return ModuleLogger(provider, schedulerModule)
}

// MarkdownLogger returns the logger namespace reserved for markdown workflows.
func MarkdownLogger(provider interfaces.LoggerProvider) interfaces.Logger {
	return ModuleLogger(provider, markdownModule)
}

// WithMarkdownContext enriches the provided logger with common markdown fields such as
// file path, locale, and sync action. Empty values are ignored.
func WithMarkdownContext(logger interfaces.Logger, path, locale, action string) interfaces.Logger {
	fields := map[string]any{}
	if trimmed := strings.TrimSpace(path); trimmed != "" {
		fields[fieldMarkdownPath] = trimmed
	}
	if trimmed := strings.TrimSpace(locale); trimmed != "" {
		fields[fieldMarkdownLocale] = trimmed
	}
	if trimmed := strings.TrimSpace(action); trimmed != "" {
		fields[fieldMarkdownAction] = trimmed
	}
	return WithFields(logger, fields)
}

// NoOp returns a logger that drops every log entry. It satisfies the Logger
// contract so services can safely operate when logging is disabled.
func NoOp() interfaces.Logger {
	return noopLogger{}
}

type noopLogger struct{}

var _ interfaces.Logger = noopLogger{}

func (noopLogger) Trace(string, ...any) {}
func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}
func (noopLogger) Fatal(string, ...any) {}

func (n noopLogger) WithFields(map[string]any) interfaces.Logger {
	return n
}

func (n noopLogger) WithContext(context.Context) interfaces.Logger {
	return n
}
