package auditcmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/jobs"
	"github.com/goliatone/go-cms/internal/logging"
)

type stubWorker struct {
	processErr error
	calls      int
}

func (s *stubWorker) Process(context.Context) error {
	s.calls++
	return s.processErr
}

type stubAuditLog struct {
	events     []jobs.AuditEvent
	listErr    error
	clearErr   error
	listCalls  int
	clearCalls int
}

func (s *stubAuditLog) List(context.Context) ([]jobs.AuditEvent, error) {
	s.listCalls++
	if s.listErr != nil {
		return nil, s.listErr
	}
	copyEvents := make([]jobs.AuditEvent, len(s.events))
	copy(copyEvents, s.events)
	return copyEvents, nil
}

func (s *stubAuditLog) Clear(context.Context) error {
	s.clearCalls++
	return s.clearErr
}

func TestReplayAuditHandlerInvokesWorker(t *testing.T) {
	worker := &stubWorker{}
	handler := NewReplayAuditHandler(worker, logging.NoOp())

	if err := handler.Execute(context.Background(), ReplayAuditCommand{}); err != nil {
		t.Fatalf("replay execute: %v", err)
	}
	if worker.calls != 1 {
		t.Fatalf("expected worker to be called once, got %d", worker.calls)
	}
}

func TestReplayAuditHandlerPropagatesError(t *testing.T) {
	worker := &stubWorker{processErr: errors.New("boom")}
	handler := NewReplayAuditHandler(worker, logging.NoOp())

	err := handler.Execute(context.Background(), ReplayAuditCommand{})
	if err == nil {
		t.Fatal("expected error from worker")
	}
	if !errors.Is(err, worker.processErr) {
		t.Fatalf("expected worker error, got %v", err)
	}
}

func TestExportAuditHandlerRespectsLimit(t *testing.T) {
	log := &stubAuditLog{
		events: []jobs.AuditEvent{
			{EntityType: "content", EntityID: "1", Action: "publish", OccurredAt: time.Now()},
			{EntityType: "content", EntityID: "2", Action: "publish", OccurredAt: time.Now()},
			{EntityType: "page", EntityID: "3", Action: "unpublish", OccurredAt: time.Now()},
		},
	}
	handler := NewExportAuditHandler(log, logging.NoOp())
	limit := 2

	if err := handler.Execute(context.Background(), ExportAuditCommand{MaxRecords: &limit}); err != nil {
		t.Fatalf("export execute: %v", err)
	}
	if log.listCalls != 1 {
		t.Fatalf("expected list to be called once, got %d", log.listCalls)
	}
}

func TestExportAuditHandlerPropagatesError(t *testing.T) {
	log := &stubAuditLog{listErr: errors.New("list failed")}
	handler := NewExportAuditHandler(log, logging.NoOp())

	err := handler.Execute(context.Background(), ExportAuditCommand{})
	if err == nil {
		t.Fatal("expected list error")
	}
	if !errors.Is(err, log.listErr) {
		t.Fatalf("expected list error, got %v", err)
	}
}

func TestCleanupAuditHandlerDryRun(t *testing.T) {
	log := &stubAuditLog{
		events: []jobs.AuditEvent{{EntityType: "content", EntityID: "1"}},
	}
	handler := NewCleanupAuditHandler(log, logging.NoOp())

	if err := handler.Execute(context.Background(), CleanupAuditCommand{DryRun: true}); err != nil {
		t.Fatalf("cleanup dry run: %v", err)
	}
	if log.clearCalls != 0 {
		t.Fatalf("expected clear not to be called, got %d", log.clearCalls)
	}
}

func TestCleanupAuditHandlerClearsEvents(t *testing.T) {
	log := &stubAuditLog{
		events: []jobs.AuditEvent{{EntityType: "content", EntityID: "1"}},
	}
	handler := NewCleanupAuditHandler(log, logging.NoOp())

	if err := handler.Execute(context.Background(), CleanupAuditCommand{}); err != nil {
		t.Fatalf("cleanup execute: %v", err)
	}
	if log.listCalls != 1 {
		t.Fatalf("expected list to be called once, got %d", log.listCalls)
	}
	if log.clearCalls != 1 {
		t.Fatalf("expected clear calls 1, got %d", log.clearCalls)
	}
}

func TestCleanupAuditHandlerPropagatesErrors(t *testing.T) {
	listErr := errors.New("list boom")
	log := &stubAuditLog{listErr: listErr}
	handler := NewCleanupAuditHandler(log, logging.NoOp())

	err := handler.Execute(context.Background(), CleanupAuditCommand{})
	if err == nil {
		t.Fatal("expected list error")
	}
	if !errors.Is(err, listErr) {
		t.Fatalf("expected list error, got %v", err)
	}

	log.listErr = nil
	log.clearErr = errors.New("clear boom")

	err = handler.Execute(context.Background(), CleanupAuditCommand{})
	if err == nil {
		t.Fatal("expected clear error")
	}
	if !errors.Is(err, log.clearErr) {
		t.Fatalf("expected clear error, got %v", err)
	}
}
