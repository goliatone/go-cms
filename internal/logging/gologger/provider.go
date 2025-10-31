package gologger

import (
	"context"
	"fmt"
	"sort"
	"strings"

	glog "github.com/goliatone/go-logger/glog"

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Config captures the options exposed by the go-logger adapter.
type Config struct {
	Level     string
	Format    string
	AddSource bool
	Focus     []string
}

// Provider wraps go-logger so it satisfies the CMS logging interfaces.
type Provider struct {
	root *glog.BaseLogger
}

// NewProvider constructs a logger provider backed by go-logger. The returned
// provider can be injected into the DI container to supply module loggers.
func NewProvider(cfg Config) (*Provider, error) {
	options := []glog.Option{}

	if level := normalizeLevel(cfg.Level); level != "" {
		options = append(options, glog.WithLevel(level))
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "", "json":
		options = append(options, glog.WithLoggerTypeJSON())
	case "console":
		options = append(options, glog.WithLoggerTypeConsole())
	case "pretty":
		options = append(options, glog.WithLoggerTypePretty())
	default:
		return nil, fmt.Errorf("logging: unsupported go-logger format %q", cfg.Format)
	}

	if cfg.AddSource {
		options = append(options, glog.WithAddSource(true))
	}

	root := glog.NewLogger(options...)
	if len(cfg.Focus) > 0 {
		root.Focus(normalizeFocus(cfg.Focus)...)
	}

	return &Provider{root: root}, nil
}

// GetLogger satisfies interfaces.LoggerProvider by adapting go-logger child loggers.
func (p *Provider) GetLogger(name string) interfaces.Logger {
	if p == nil {
		return logging.NoOp()
	}
	name = strings.TrimSpace(name)
	var inner glog.Logger
	if name == "" {
		inner = p.root
	} else {
		inner = p.root.GetLogger(name)
	}
	return wrap(inner)
}

// wrap adapts a go-logger Logger into the CMS logging contract.
func wrap(inner glog.Logger) interfaces.Logger {
	if inner == nil {
		return logging.NoOp()
	}
	return &adapter{inner: inner}
}

type adapter struct {
	inner glog.Logger
}

func (l *adapter) Trace(msg string, args ...any) { l.inner.Trace(msg, args...) }
func (l *adapter) Debug(msg string, args ...any) { l.inner.Debug(msg, args...) }
func (l *adapter) Info(msg string, args ...any)  { l.inner.Info(msg, args...) }
func (l *adapter) Warn(msg string, args ...any)  { l.inner.Warn(msg, args...) }
func (l *adapter) Error(msg string, args ...any) { l.inner.Error(msg, args...) }
func (l *adapter) Fatal(msg string, args ...any) { l.inner.Fatal(msg, args...) }

func (l *adapter) WithFields(fields map[string]any) interfaces.Logger {
	if len(fields) == 0 {
		return l
	}

	if with, ok := l.inner.(glog.FieldsLogger); ok {
		return wrap(with.WithFields(cloneFields(fields)))
	}

	// Best effort: fall back to sorted key/value pairs via With.
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := make([]any, 0, len(keys)*2)
	for _, k := range keys {
		args = append(args, k, fields[k])
	}
	if with, ok := l.inner.(interface{ With(...any) *glog.BaseLogger }); ok {
		return wrap(with.With(args...))
	}
	return l
}

func (l *adapter) WithContext(ctx context.Context) interfaces.Logger {
	if ctx == nil {
		return l
	}
	return wrap(l.inner.WithContext(ctx))
}

func cloneFields(fields map[string]any) map[string]any {
	copied := make(map[string]any, len(fields))
	for k, v := range fields {
		copied[k] = v
	}
	return copied
}

func normalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "":
		return ""
	case "trace":
		return glog.Trace
	case "debug":
		return glog.Debug
	case "info":
		return glog.Info
	case "warn", "warning":
		return glog.Warn
	case "error":
		return glog.Error
	case "fatal":
		return glog.Fatal
	default:
		return ""
	}
}

func normalizeFocus(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
