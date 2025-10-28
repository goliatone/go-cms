package commands

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
	goerrors "github.com/goliatone/go-errors"
)

type testMessage struct{}

func (testMessage) Type() string { return "cms.test.message" }

func (testMessage) Validate() error { return nil }

type invalidMessage struct{}

func (invalidMessage) Type() string { return "cms.test.invalid" }

func (invalidMessage) Validate() error {
	return validationError()
}

func validationError() error {
	return errors.New("invalid")
}

func TestHandlerExecuteSuccess(t *testing.T) {
	called := false
	h := NewHandler[testMessage](func(ctx context.Context, msg testMessage) error {
		called = true
		return nil
	})

	if err := h.Execute(context.Background(), testMessage{}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !called {
		t.Fatal("expected handler to be invoked")
	}
}

func TestHandlerValidationShortCircuitsExecution(t *testing.T) {
	called := false
	h := NewHandler[invalidMessage](func(ctx context.Context, msg invalidMessage) error {
		called = true
		return nil
	})

	err := h.Execute(context.Background(), invalidMessage{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryValidation) {
		t.Fatalf("expected validation category, got %v", err)
	}
	if called {
		t.Fatal("expected handler not to run when validation fails")
	}
}

func TestHandlerContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	h := NewHandler[testMessage](func(ctx context.Context, msg testMessage) error {
		called = true
		return nil
	})

	err := h.Execute(ctx, testMessage{})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category, got %v", err)
	}
	if called {
		t.Fatal("expected handler not to run when context is cancelled")
	}
}

func TestHandlerWrapsExecutionError(t *testing.T) {
	execErr := errors.New("boom")
	h := NewHandler[testMessage](func(ctx context.Context, msg testMessage) error {
		return execErr
	})

	err := h.Execute(context.Background(), testMessage{})
	if err == nil {
		t.Fatal("expected wrapped execution error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category, got %v", err)
	}
	if !goerrors.HasCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category to propagate, got %v", err)
	}
}

func TestHandlerHonoursTimeoutOption(t *testing.T) {
	h := NewHandler[testMessage](func(ctx context.Context, msg testMessage) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
			return nil
		}
	}, WithTimeout[testMessage](10*time.Millisecond))

	err := h.Execute(context.Background(), testMessage{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !goerrors.IsCategory(err, goerrors.CategoryCommand) {
		t.Fatalf("expected command category for timeout, got %v", err)
	}
}

func TestHandlerLogsIncludeMessageFields(t *testing.T) {
	logger := newRecordingLogger()
	h := NewHandler[testMessage](
		func(ctx context.Context, msg testMessage) error { return nil },
		WithLogger[testMessage](logger),
		WithOperation[testMessage]("test.operation"),
		WithMessageFields[testMessage](func(testMessage) map[string]any {
			return map[string]any{"entity_id": "abc123"}
		}),
	)

	if err := h.Execute(context.Background(), testMessage{}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	successEntries := logger.findByMessage("command.execute.success")
	if len(successEntries) != 1 {
		t.Fatalf("expected 1 success log entry, got %d", len(successEntries))
	}
	entry := successEntries[0]
	if entry.fields["operation"] != "test.operation" {
		t.Fatalf("expected operation field, got %v", entry.fields["operation"])
	}
	if entry.fields["entity_id"] != "abc123" {
		t.Fatalf("expected entity_id field, got %v", entry.fields["entity_id"])
	}
	if entry.fields["command"] != "cms.test.message" {
		t.Fatalf("expected command field, got %v", entry.fields["command"])
	}
}

func TestHandlerTelemetryLogsFailure(t *testing.T) {
	logger := newRecordingLogger()
	h := NewHandler[testMessage](
		func(ctx context.Context, msg testMessage) error { return errors.New("boom") },
		WithLogger[testMessage](logger),
		WithOperation[testMessage]("test.operation"),
		WithMessageFields[testMessage](func(testMessage) map[string]any {
			return map[string]any{"entity_id": "xyz"}
		}),
		WithTelemetry(DefaultTelemetry[testMessage](logger)),
	)

	err := h.Execute(context.Background(), testMessage{})
	if err == nil {
		t.Fatal("expected execution error")
	}

	failureEntries := logger.findByMessage("command.execute.failed")
	if len(failureEntries) != 1 {
		t.Fatalf("expected telemetry failure log, got %d entries", len(failureEntries))
	}
	entry := failureEntries[0]
	if entry.fields["entity_id"] != "xyz" {
		t.Fatalf("expected entity_id field, got %v", entry.fields["entity_id"])
	}
	if entry.fields["command"] != "cms.test.message" {
		t.Fatalf("expected command field, got %v", entry.fields["command"])
	}
	foundErrorArg := false
	for i := 0; i < len(entry.args)-1; i += 2 {
		if key, _ := entry.args[i].(string); key == "error" {
			foundErrorArg = true
			break
		}
	}
	if !foundErrorArg {
		t.Fatalf("expected error argument in telemetry log, got %#v", entry.args)
	}
}

func TestHandlerTelemetryMetadata(t *testing.T) {
	var captured TelemetryInfo
	h := NewHandler[testMessage](
		func(ctx context.Context, msg testMessage) error { return nil },
		WithTelemetry(func(ctx context.Context, msg testMessage, info TelemetryInfo) {
			captured = info
		}),
	)

	if err := h.Execute(context.Background(), testMessage{}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if captured.Status != TelemetryStatusSuccess {
		t.Fatalf("expected success status, got %s", captured.Status)
	}
	if captured.Command != "cms.test.message" {
		t.Fatalf("expected command metadata, got %s", captured.Command)
	}
	if captured.Duration <= 0 {
		t.Fatalf("expected positive duration, got %v", captured.Duration)
	}
}

type logEntry struct {
	level  string
	msg    string
	args   []any
	fields map[string]any
}

type recordingSink struct {
	entries []logEntry
}

type recordingLogger struct {
	fields map[string]any
	sink   *recordingSink
}

func newRecordingLogger() *recordingLogger {
	return &recordingLogger{
		fields: map[string]any{},
		sink:   &recordingSink{entries: []logEntry{}},
	}
}

func (l *recordingLogger) Trace(msg string, args ...any) {
	l.record("trace", msg, args...)
}

func (l *recordingLogger) Debug(msg string, args ...any) {
	l.record("debug", msg, args...)
}

func (l *recordingLogger) Info(msg string, args ...any) {
	l.record("info", msg, args...)
}

func (l *recordingLogger) Warn(msg string, args ...any) {
	l.record("warn", msg, args...)
}

func (l *recordingLogger) Error(msg string, args ...any) {
	l.record("error", msg, args...)
}

func (l *recordingLogger) Fatal(msg string, args ...any) {
	l.record("fatal", msg, args...)
}

func (l *recordingLogger) WithFields(fields map[string]any) interfaces.Logger {
	if len(fields) == 0 {
		return l
	}
	merged := make(map[string]any, len(l.fields)+len(fields))
	for k, v := range l.fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return &recordingLogger{
		fields: merged,
		sink:   l.sink,
	}
}

func (l *recordingLogger) WithContext(context.Context) interfaces.Logger {
	return l
}

func (l *recordingLogger) record(level, msg string, args ...any) {
	entryFields := make(map[string]any, len(l.fields))
	for k, v := range l.fields {
		entryFields[k] = v
	}
	l.sink.entries = append(l.sink.entries, logEntry{
		level:  level,
		msg:    msg,
		args:   append([]any(nil), args...),
		fields: entryFields,
	})
}

func (l *recordingLogger) findByMessage(msg string) []logEntry {
	var list []logEntry
	for _, entry := range l.sink.entries {
		if entry.msg == msg {
			list = append(list, entry)
		}
	}
	return list
}
