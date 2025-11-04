package shortcode

import (
	"context"
	"errors"
	"html/template"
	"sync"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/logging"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

func TestServiceProcessRecordsMetrics(t *testing.T) {
	metrics := newMetricsStub()
	renderer := &stubRenderer{result: template.HTML("<div>ok</div>")}
	parser := stubParser{
		transformed: "prefix <!-- shortcode:0 --> suffix",
		shortcodes: []interfaces.ParsedShortcode{
			{Name: "example"},
		},
	}

	service := NewService(nil, renderer,
		WithParser(parser),
		WithMetrics(metrics),
		WithLogger(logging.NoOp()),
	)

	output, err := service.Process(context.Background(), "ignored", interfaces.ShortcodeProcessOptions{})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if output != "prefix <div>ok</div> suffix" {
		t.Fatalf("unexpected output: %s", output)
	}

	if got := metrics.durationCount("example"); got != 1 {
		t.Fatalf("expected 1 duration record, got %d", got)
	}
	if got := metrics.errorCount("example"); got != 0 {
		t.Fatalf("expected 0 render errors, got %d", got)
	}
}

func TestServiceProcessRecordsMetricsOnError(t *testing.T) {
	wantErr := errors.New("render failed")
	metrics := newMetricsStub()
	renderer := &stubRenderer{err: wantErr}
	parser := stubParser{
		transformed: "prefix <!-- shortcode:0 --> suffix",
		shortcodes: []interfaces.ParsedShortcode{
			{Name: "example"},
		},
	}

	service := NewService(nil, renderer,
		WithParser(parser),
		WithMetrics(metrics),
		WithLogger(logging.NoOp()),
	)

	_, err := service.Process(context.Background(), "ignored", interfaces.ShortcodeProcessOptions{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}

	if got := metrics.durationCount("example"); got != 1 {
		t.Fatalf("expected duration recorded even on error, got %d", got)
	}
	if got := metrics.errorCount("example"); got != 1 {
		t.Fatalf("expected 1 render error, got %d", got)
	}
}

func TestServiceRenderRecordsMetrics(t *testing.T) {
	metrics := newMetricsStub()
	renderer := &stubRenderer{result: template.HTML("<span/>")}

	service := NewService(nil, renderer,
		WithMetrics(metrics),
		WithLogger(logging.NoOp()),
	)

	_, err := service.Render(interfaces.ShortcodeContext{}, "example", nil, "")
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	if got := metrics.durationCount("example"); got != 1 {
		t.Fatalf("expected duration recorded for render, got %d", got)
	}
	if got := metrics.errorCount("example"); got != 0 {
		t.Fatalf("expected no render errors, got %d", got)
	}
}

type stubRenderer struct {
	result template.HTML
	err    error
}

func (r *stubRenderer) Render(_ interfaces.ShortcodeContext, _ string, _ map[string]any, _ string) (template.HTML, error) {
	if r.err != nil {
		return "", r.err
	}
	return r.result, nil
}

func (r *stubRenderer) RenderAsync(_ interfaces.ShortcodeContext, _ string, _ map[string]any, _ string) (<-chan template.HTML, <-chan error) {
	out := make(chan template.HTML, 1)
	errCh := make(chan error, 1)
	if r.err != nil {
		errCh <- r.err
	} else {
		out <- r.result
	}
	close(out)
	close(errCh)
	return out, errCh
}

type stubParser struct {
	transformed string
	shortcodes  []interfaces.ParsedShortcode
	err         error
}

func (p stubParser) Parse(string) ([]interfaces.ParsedShortcode, error) {
	return p.shortcodes, p.err
}

func (p stubParser) Extract(string) (string, []interfaces.ParsedShortcode, error) {
	return p.transformed, p.shortcodes, p.err
}

type metricsStub struct {
	mu        sync.Mutex
	durations map[string][]time.Duration
	errors    map[string]int
	cacheHits map[string]int
}

func newMetricsStub() *metricsStub {
	return &metricsStub{
		durations: map[string][]time.Duration{},
		errors:    map[string]int{},
		cacheHits: map[string]int{},
	}
}

func (m *metricsStub) ObserveRenderDuration(shortcode string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durations[shortcode] = append(m.durations[shortcode], duration)
}

func (m *metricsStub) IncrementRenderError(shortcode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[shortcode]++
}

func (m *metricsStub) IncrementCacheHit(shortcode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheHits[shortcode]++
}

func (m *metricsStub) durationCount(shortcode string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.durations[shortcode])
}

func (m *metricsStub) errorCount(shortcode string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.errors[shortcode]
}

func (m *metricsStub) cacheHitCount(shortcode string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cacheHits[shortcode]
}
