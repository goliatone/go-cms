package console

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Level represents the severity attached to a log entry.
type Level uint8

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String renders the severity label used in console output.
func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "INFO"
	}
}

// Options configures the console logger provider.
type Options struct {
	Writer   io.Writer
	TimeFunc func() time.Time
	MinLevel *Level
}

type resolvedOptions struct {
	writer   io.Writer
	clock    func() time.Time
	minLevel Level
}

func resolveOptions(opts Options) resolvedOptions {
	cfg := resolvedOptions{
		writer:   opts.Writer,
		clock:    opts.TimeFunc,
		minLevel: LevelDebug,
	}
	if cfg.writer == nil {
		cfg.writer = os.Stdout
	}
	if cfg.clock == nil {
		cfg.clock = time.Now
	}
	if opts.MinLevel != nil {
		cfg.minLevel = *opts.MinLevel
	}
	return cfg
}

type provider struct {
	writer   io.Writer
	clock    func() time.Time
	minLevel Level
	mu       *sync.Mutex
}

// NewProvider constructs a console-backed logger provider that satisfies the CMS
// logging interfaces. Callers can override defaults via Options, otherwise logs
// are written to stdout with a minimum severity of DEBUG.
func NewProvider(opts Options) interfaces.LoggerProvider {
	options := resolveOptions(opts)
	return &provider{
		writer:   options.writer,
		clock:    options.clock,
		minLevel: options.minLevel,
		mu:       &sync.Mutex{},
	}
}

func (p *provider) GetLogger(name string) interfaces.Logger {
	return &consoleLogger{
		provider: p,
		fields: map[string]any{
			"logger": name,
		},
	}
}

type consoleLogger struct {
	provider *provider
	fields   map[string]any
	ctx      context.Context
}

var (
	_ interfaces.Logger       = (*consoleLogger)(nil)
	_ interfaces.FieldsLogger = (*consoleLogger)(nil)
)

func (l *consoleLogger) Trace(msg string, args ...any) { l.log(LevelTrace, msg, args...) }
func (l *consoleLogger) Debug(msg string, args ...any) { l.log(LevelDebug, msg, args...) }
func (l *consoleLogger) Info(msg string, args ...any)  { l.log(LevelInfo, msg, args...) }
func (l *consoleLogger) Warn(msg string, args ...any)  { l.log(LevelWarn, msg, args...) }
func (l *consoleLogger) Error(msg string, args ...any) { l.log(LevelError, msg, args...) }
func (l *consoleLogger) Fatal(msg string, args ...any) { l.log(LevelFatal, msg, args...) }

func (l *consoleLogger) WithFields(fields map[string]any) interfaces.Logger {
	if len(fields) == 0 {
		return l
	}
	cloned := make(map[string]any, len(l.fields)+len(fields))
	for key, value := range l.fields {
		cloned[key] = value
	}
	for key, value := range fields {
		cloned[key] = value
	}
	return &consoleLogger{
		provider: l.provider,
		fields:   cloned,
		ctx:      l.ctx,
	}
}

func (l *consoleLogger) WithContext(ctx context.Context) interfaces.Logger {
	return &consoleLogger{
		provider: l.provider,
		fields:   cloneMap(l.fields),
		ctx:      ctx,
	}
}

func (l *consoleLogger) enabled(level Level) bool {
	return level >= l.provider.minLevel
}

func (l *consoleLogger) log(level Level, msg string, args ...any) {
	if !l.enabled(level) || l.provider == nil {
		return
	}

	fields := cloneMap(l.fields)

	if ctxFields := logging.ContextFields(l.ctx); len(ctxFields) > 0 {
		if fields == nil {
			fields = ctxFields
		} else {
			for key, value := range ctxFields {
				fields[key] = value
			}
		}
	}

	argFields := argsToFields(args)
	if len(argFields) > 0 {
		if fields == nil {
			fields = argFields
		} else {
			for key, value := range argFields {
				fields[key] = value
			}
		}
	}

	entry := formatEntry(l.provider.clock().UTC(), level.String(), msg, fields)

	l.provider.mu.Lock()
	defer l.provider.mu.Unlock()

	if _, err := io.WriteString(l.provider.writer, entry+"\n"); err != nil {
		// The console logger is best-effort; we intentionally swallow write errors
		// to avoid cascading failures during diagnostics.
		_ = err
	}
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func argsToFields(args []any) map[string]any {
	if len(args) == 0 {
		return nil
	}
	fields := map[string]any{}
	for i := 0; i < len(args); i++ {
		// If we reach the last argument and there's no value pair, promote it
		// to a positional field to avoid dropping data.
		if i == len(args)-1 {
			fields[fieldKey(i)] = args[i]
			break
		}
		key := args[i]
		value := args[i+1]
		i++

		switch k := key.(type) {
		case string:
			if k == "" {
				fields[fieldKey(i/2)] = value
			} else {
				fields[k] = value
			}
		default:
			fields[fieldKey(i/2)] = value
		}
	}
	return fields
}

func fieldKey(position int) string {
	return fmt.Sprintf("field_%d", position)
}

func formatEntry(ts time.Time, level, msg string, fields map[string]any) string {
	builder := strings.Builder{}
	builder.Grow(64 + len(msg) + len(fields)*16)
	builder.WriteString(ts.Format(time.RFC3339Nano))
	builder.WriteByte(' ')
	builder.WriteString(level)
	builder.WriteByte(' ')
	builder.WriteString(msg)

	if len(fields) == 0 {
		return builder.String()
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		builder.WriteByte(' ')
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(formatValue(fields[key]))
	}
	return builder.String()
}

func formatValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return quoteIfNeeded(v)
	case time.Time:
		return quoteIfNeeded(v.UTC().Format(time.RFC3339Nano))
	case *time.Time:
		if v == nil {
			return "null"
		}
		return quoteIfNeeded(v.UTC().Format(time.RFC3339Nano))
	case error:
		return quoteIfNeeded(v.Error())
	case fmt.Stringer:
		return quoteIfNeeded(v.String())
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return quoteIfNeeded(fmt.Sprint(v))
	}
}

func quoteIfNeeded(value string) string {
	if value == "" {
		return `""`
	}
	for _, r := range value {
		if r <= 0x20 || r == '=' {
			return strconv.Quote(value)
		}
	}
	return value
}
