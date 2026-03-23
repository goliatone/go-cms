package lifecycle

import (
	"context"
	"errors"
	"maps"
	"strings"
	"time"
)

// Event describes a root-record lifecycle transition that downstream systems can consume.
type Event struct {
	ResourceType    string
	RecordID        string
	Transition      string
	TranslationID   string
	Locale          string
	Locales         []string
	Status          string
	EnvironmentKey  string
	ContentTypeID   string
	ContentTypeSlug string
	SearchEnabled   bool
	SearchIndex     string
	OccurredAt      time.Time
	Metadata        map[string]any
}

// Hook receives normalized lifecycle events.
type Hook interface {
	Notify(ctx context.Context, event Event) error
}

// HookFunc allows plain functions to satisfy Hook.
type HookFunc func(ctx context.Context, event Event) error

// Notify dispatches to the underlying function.
func (fn HookFunc) Notify(ctx context.Context, event Event) error {
	if fn == nil {
		return nil
	}
	return fn(ctx, event)
}

// Hooks fans out events to zero or more hooks.
type Hooks []Hook

// Enabled reports whether there are any hooks to notify.
func (h Hooks) Enabled() bool {
	return len(h) > 0
}

// Notify forwards the event to all hooks, returning a joined error if any fail.
func (h Hooks) Notify(ctx context.Context, event Event) error {
	if len(h) == 0 {
		return nil
	}
	normalized := NormalizeEvent(event)
	if normalized.ResourceType == "" || normalized.RecordID == "" || normalized.Transition == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var errs []error
	for _, hook := range h {
		if hook == nil {
			continue
		}
		if err := hook.Notify(ctx, normalized); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// NormalizeEvent trims string fields, clones slices/maps, and ensures a timestamp is present.
func NormalizeEvent(event Event) Event {
	normalized := event
	normalized.ResourceType = strings.TrimSpace(event.ResourceType)
	normalized.RecordID = strings.TrimSpace(event.RecordID)
	normalized.Transition = strings.TrimSpace(event.Transition)
	normalized.TranslationID = strings.TrimSpace(event.TranslationID)
	normalized.Locale = strings.TrimSpace(event.Locale)
	normalized.Status = strings.TrimSpace(event.Status)
	normalized.EnvironmentKey = strings.TrimSpace(event.EnvironmentKey)
	normalized.ContentTypeID = strings.TrimSpace(event.ContentTypeID)
	normalized.ContentTypeSlug = strings.TrimSpace(event.ContentTypeSlug)
	normalized.SearchIndex = strings.TrimSpace(event.SearchIndex)
	normalized.Locales = cloneStrings(event.Locales)
	normalized.Metadata = cloneMap(event.Metadata)
	if normalized.OccurredAt.IsZero() {
		normalized.OccurredAt = time.Now()
	}
	return normalized
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, 0, len(src))
	for _, value := range src {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			dst = append(dst, trimmed)
		}
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	maps.Copy(dst, src)
	return dst
}
