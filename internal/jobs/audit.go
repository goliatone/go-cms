package jobs

import (
	"context"
	"sync"
	"time"
)

// AuditEvent captures a change applied by the scheduler worker.
type AuditEvent struct {
	EntityType string
	EntityID   string
	Action     string
	OccurredAt time.Time
	Metadata   map[string]any
}

// AuditRecorder persists audit events.
type AuditRecorder interface {
	Record(ctx context.Context, event AuditEvent) error
}

// InMemoryAuditRecorder accumulates audit events in-memory for tests.
type InMemoryAuditRecorder struct {
	mu     sync.Mutex
	events []AuditEvent
	err    error
}

// NewInMemoryAuditRecorder constructs an empty recorder.
func NewInMemoryAuditRecorder() *InMemoryAuditRecorder {
	return &InMemoryAuditRecorder{}
}

// Record stores the supplied event.
func (r *InMemoryAuditRecorder) Record(_ context.Context, event AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	copied := event
	if copied.Metadata != nil {
		metadata := make(map[string]any, len(copied.Metadata))
		for k, v := range copied.Metadata {
			metadata[k] = v
		}
		copied.Metadata = metadata
	}
	r.events = append(r.events, copied)
	return nil
}

// Events returns a snapshot of recorded audit entries.
func (r *InMemoryAuditRecorder) Events() []AuditEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]AuditEvent, len(r.events))
	copy(out, r.events)
	return out
}

// Fail configures the recorder to return the supplied error on subsequent Record calls.
func (r *InMemoryAuditRecorder) Fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}
