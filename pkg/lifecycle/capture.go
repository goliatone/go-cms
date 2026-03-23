package lifecycle

import (
	"context"
	"sync"
)

// CaptureHook records events for assertions in tests.
type CaptureHook struct {
	mu     sync.Mutex
	Events []Event
	Err    error
}

// Notify records the event and returns any configured error.
func (h *CaptureHook) Notify(_ context.Context, event Event) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	h.Events = append(h.Events, NormalizeEvent(event))
	h.mu.Unlock()
	return h.Err
}
