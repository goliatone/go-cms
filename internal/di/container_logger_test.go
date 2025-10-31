package di_test

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

func TestContainerSchedulerLoggingWithConsoleFallback(t *testing.T) {
	cfg := cms.DefaultConfig()
	cfg.Features.Logger = true
	cfg.Features.Versioning = true
	cfg.Features.Scheduling = true

	rec := newRecordingProvider()

	if _, err := di.NewContainer(cfg, di.WithLoggerProvider(rec)); err != nil {
		t.Fatalf("NewContainer returned error: %v", err)
	}

	entry := rec.find("scheduler.configured")
	if entry == nil {
		t.Fatalf("expected scheduler.configured log entry, got %#v", rec.entries)
	}
	if got := entry.fields["provider"]; got != "in-memory" {
		t.Fatalf("expected provider field to be in-memory, got %v", got)
	}
	if got := entry.fields["module"]; got != "cms.scheduler" {
		t.Fatalf("expected module field to be cms.scheduler, got %v", got)
	}
}

type recordingProvider struct {
	entries []recordedEntry
}

type recordedEntry struct {
	level  string
	msg    string
	fields map[string]any
}

func newRecordingProvider() *recordingProvider {
	return &recordingProvider{entries: []recordedEntry{}}
}

func (p *recordingProvider) GetLogger(name string) interfaces.Logger {
	return &recordingLogger{
		provider: p,
		fields: map[string]any{
			"logger": name,
		},
	}
}

func (p *recordingProvider) record(entry recordedEntry) {
	p.entries = append(p.entries, entry)
}

func (p *recordingProvider) find(msg string) *recordedEntry {
	for i := range p.entries {
		if p.entries[i].msg == msg {
			return &p.entries[i]
		}
	}
	return nil
}

type recordingLogger struct {
	provider *recordingProvider
	fields   map[string]any
}

var _ interfaces.Logger = (*recordingLogger)(nil)

func (l *recordingLogger) Trace(msg string, args ...any) { l.log("TRACE", msg, args...) }
func (l *recordingLogger) Debug(msg string, args ...any) { l.log("DEBUG", msg, args...) }
func (l *recordingLogger) Info(msg string, args ...any)  { l.log("INFO", msg, args...) }
func (l *recordingLogger) Warn(msg string, args ...any)  { l.log("WARN", msg, args...) }
func (l *recordingLogger) Error(msg string, args ...any) { l.log("ERROR", msg, args...) }
func (l *recordingLogger) Fatal(msg string, args ...any) { l.log("FATAL", msg, args...) }

func (l *recordingLogger) WithFields(fields map[string]any) interfaces.Logger {
	if len(fields) == 0 {
		return l
	}
	merged := make(map[string]any, len(l.fields)+len(fields))
	for key, value := range l.fields {
		merged[key] = value
	}
	for key, value := range fields {
		merged[key] = value
	}
	return &recordingLogger{
		provider: l.provider,
		fields:   merged,
	}
}

func (l *recordingLogger) WithContext(context.Context) interfaces.Logger {
	return &recordingLogger{
		provider: l.provider,
		fields:   cloneFields(l.fields),
	}
}

func (l *recordingLogger) log(level, msg string, args ...any) {
	fields := cloneFields(l.fields)
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			break
		}
		key, _ := args[i].(string)
		if key == "" {
			continue
		}
		fields[key] = args[i+1]
	}
	l.provider.record(recordedEntry{
		level:  level,
		msg:    msg,
		fields: fields,
	})
}

func cloneFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return map[string]any{}
	}
	copied := make(map[string]any, len(fields))
	for key, value := range fields {
		copied[key] = value
	}
	return copied
}
