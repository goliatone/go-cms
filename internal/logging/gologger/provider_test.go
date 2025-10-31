package gologger

import (
	"context"
	"testing"

	glog "github.com/goliatone/go-logger/glog"
)

func TestNewProviderCreatesLogger(t *testing.T) {
	p, err := NewProvider(Config{
		Level:  "debug",
		Format: "console",
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	logger := p.GetLogger("cms.test")
	if logger == nil {
		t.Fatal("expected logger, got nil")
	}

	child := logger.WithFields(map[string]any{"module": "cms.test"})
	if child == nil {
		t.Fatal("expected WithFields to return logger")
	}

	// Ensure chained operations do not panic.
	child.Debug("adapter.initialised")
}

func TestAdapterDelegatesToUnderlyingLogger(t *testing.T) {
	stub := &stubLogger{}
	adapted := wrap(stub)

	adapted.Trace("trace", "key", "value")
	adapted.Debug("debug")
	adapted.Info("info")
	adapted.Warn("warn")
	adapted.Error("error")
	adapted.Fatal("fatal")

	fields := map[string]any{"entity": "content"}
	child := adapted.WithFields(fields)
	if child == nil {
		t.Fatal("expected WithFields to return logger")
	}

	fields["entity"] = "page"
	if len(stub.fields) != 1 {
		t.Fatalf("expected fields to be recorded once, got %d", len(stub.fields))
	}
	if stub.fields[0]["entity"] != "content" {
		t.Fatalf("expected fields to be cloned, got %v", stub.fields[0]["entity"])
	}

	ctx := context.WithValue(context.Background(), struct{}{}, "value")
	adapted.WithContext(ctx)
	if len(stub.contexts) != 1 || stub.contexts[0] != ctx {
		t.Fatalf("expected context propagation, got %#v", stub.contexts)
	}

	wantCalls := []string{"trace", "debug", "info", "warn", "error", "fatal"}
	if len(stub.calls) != len(wantCalls) {
		t.Fatalf("expected %d calls, got %d", len(wantCalls), len(stub.calls))
	}
	for i, want := range wantCalls {
		if stub.calls[i] != want {
			t.Fatalf("call %d: expected %q, got %q", i, want, stub.calls[i])
		}
	}
}

type stubLogger struct {
	calls    []string
	fields   []map[string]any
	contexts []context.Context
}

var _ glog.Logger = (*stubLogger)(nil)
var _ glog.FieldsLogger = (*stubLogger)(nil)

func (s *stubLogger) Trace(string, ...any) { s.calls = append(s.calls, "trace") }
func (s *stubLogger) Debug(string, ...any) { s.calls = append(s.calls, "debug") }
func (s *stubLogger) Info(string, ...any)  { s.calls = append(s.calls, "info") }
func (s *stubLogger) Warn(string, ...any)  { s.calls = append(s.calls, "warn") }
func (s *stubLogger) Error(string, ...any) { s.calls = append(s.calls, "error") }
func (s *stubLogger) Fatal(string, ...any) { s.calls = append(s.calls, "fatal") }

func (s *stubLogger) WithContext(ctx context.Context) glog.Logger {
	s.contexts = append(s.contexts, ctx)
	return s
}

func (s *stubLogger) WithFields(fields map[string]any) glog.Logger {
	copied := make(map[string]any, len(fields))
	for k, v := range fields {
		copied[k] = v
	}
	s.fields = append(s.fields, copied)
	return s
}
