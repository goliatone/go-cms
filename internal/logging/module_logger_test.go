package logging

import (
	"context"
	"testing"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

type recordingLogger struct {
	fields   []map[string]any
	contexts []context.Context
}

func (r *recordingLogger) Trace(string, ...any) {}
func (r *recordingLogger) Debug(string, ...any) {}
func (r *recordingLogger) Info(string, ...any)  {}
func (r *recordingLogger) Warn(string, ...any)  {}
func (r *recordingLogger) Error(string, ...any) {}
func (r *recordingLogger) Fatal(string, ...any) {}

func (r *recordingLogger) WithFields(fields map[string]any) interfaces.Logger {
	if fields == nil {
		fields = map[string]any{}
	}
	copied := make(map[string]any, len(fields))
	for k, v := range fields {
		copied[k] = v
	}
	r.fields = append(r.fields, copied)
	return r
}

func (r *recordingLogger) WithContext(ctx context.Context) interfaces.Logger {
	r.contexts = append(r.contexts, ctx)
	return r
}

type stubProvider struct {
	requested []string
	logger    interfaces.Logger
}

func (s *stubProvider) GetLogger(name string) interfaces.Logger {
	s.requested = append(s.requested, name)
	return s.logger
}

func TestModuleLoggerFallsBackToNoOp(t *testing.T) {
	logger := ModuleLogger(nil, "cms.test")
	if _, ok := logger.(noopLogger); !ok {
		t.Fatalf("expected noopLogger fallback, got %T", logger)
	}
	// Ensure WithContext/WithFields do not panic.
	ctx := context.Background()
	logger = logger.WithContext(ctx)
	logger = logger.WithFields(map[string]any{"foo": "bar"})
	logger.Debug("noop")
}

func TestModuleLoggerUsesProviderAndAnnotatesFields(t *testing.T) {
	rec := &recordingLogger{}
	provider := &stubProvider{logger: rec}

	logger := ModuleLogger(provider, pagesModule)

	if len(provider.requested) != 1 || provider.requested[0] != pagesModule {
		t.Fatalf("expected module %s, got %v", pagesModule, provider.requested)
	}

	if len(rec.fields) != 1 {
		t.Fatalf("expected module fields to be applied once, got %d", len(rec.fields))
	}

	if got, ok := rec.fields[0]["module"]; !ok || got != pagesModule {
		t.Fatalf("expected module field %s, got %v", pagesModule, rec.fields[0]["module"])
	}

	logger.Info("with provider")
}

func TestModuleLoggerDefaultsToRootModule(t *testing.T) {
	rec := &recordingLogger{}
	provider := &stubProvider{logger: rec}

	_ = ModuleLogger(provider, "")

	if len(provider.requested) != 1 || provider.requested[0] != rootModule {
		t.Fatalf("expected default module %s, got %v", rootModule, provider.requested)
	}
	if rec.fields[0]["module"] != rootModule {
		t.Fatalf("expected module field %s, got %v", rootModule, rec.fields[0]["module"])
	}
}

func TestContentLoggerRequestsContentModule(t *testing.T) {
	provider := &stubProvider{logger: &recordingLogger{}}
	_ = ContentLogger(provider)
	if len(provider.requested) == 0 || provider.requested[0] != contentModule {
		t.Fatalf("expected content module request, got %v", provider.requested)
	}
}

func TestSchedulerLoggerRequestsSchedulerModule(t *testing.T) {
	provider := &stubProvider{logger: &recordingLogger{}}
	_ = SchedulerLogger(provider)
	if len(provider.requested) == 0 || provider.requested[0] != schedulerModule {
		t.Fatalf("expected scheduler module request, got %v", provider.requested)
	}
}
