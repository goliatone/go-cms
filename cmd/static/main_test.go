package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	staticcmd "github.com/goliatone/go-cms/internal/commands/static"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/google/uuid"
)

type stubHandlers struct {
	build   *stubBuildHandler
	diff    *stubDiffHandler
	clean   *stubCleanHandler
	sitemap *stubSitemapHandler
}

type stubBuildHandler struct {
	last staticcmd.BuildSiteCommand
}

func (s *stubBuildHandler) Execute(ctx context.Context, msg staticcmd.BuildSiteCommand) error {
	s.last = msg
	if msg.ResultCallback != nil {
		metadata := map[string]any{
			"operation": "build",
		}
		var result *generator.BuildResult = &generator.BuildResult{
			PagesBuilt: 1,
			Duration:   123,
			Metrics: generator.BuildMetrics{
				ContextDuration: 10,
			},
		}
		if msg.AssetsOnly {
			metadata["operation"] = "build_assets"
			result = nil
		} else if len(msg.PageIDs) == 1 && len(msg.Locales) == 1 {
			metadata["operation"] = "build_page"
			metadata["page_id"] = msg.PageIDs[0]
			metadata["locale"] = strings.TrimSpace(msg.Locales[0])
			result = nil
		}
		msg.ResultCallback(staticcmd.ResultEnvelope{
			Result:   result,
			Metadata: metadata,
		})
	}
	return nil
}

type stubDiffHandler struct {
	last staticcmd.DiffSiteCommand
}

func (s *stubDiffHandler) Execute(ctx context.Context, msg staticcmd.DiffSiteCommand) error {
	s.last = msg
	if msg.ResultCallback != nil {
		msg.ResultCallback(staticcmd.ResultEnvelope{
			Result: &generator.BuildResult{
				DryRun:     true,
				PagesBuilt: 0,
			},
			Metadata: map[string]any{
				"operation": "diff",
			},
		})
	}
	return nil
}

type stubCleanHandler struct {
	calls int
	err   error
}

func (s *stubCleanHandler) Execute(ctx context.Context, msg staticcmd.CleanSiteCommand) error {
	s.calls++
	return s.err
}

type stubSitemapHandler struct {
	calls int
	err   error
	last  staticcmd.BuildSitemapCommand
}

func (s *stubSitemapHandler) Execute(ctx context.Context, msg staticcmd.BuildSitemapCommand) error {
	s.calls++
	s.last = msg
	if msg.ResultCallback != nil {
		msg.ResultCallback(staticcmd.ResultEnvelope{
			Result:   nil,
			Metadata: map[string]any{"operation": "build_sitemap"},
		})
	}
	return s.err
}

var activeStubHandlers *stubHandlers

func withStubModule(t *testing.T) func() {
	original := moduleBuilder
	stubs := &stubHandlers{
		build:   &stubBuildHandler{},
		diff:    &stubDiffHandler{},
		clean:   &stubCleanHandler{},
		sitemap: &stubSitemapHandler{},
	}
	activeStubHandlers = stubs

	moduleBuilder = func(opts moduleOptions) (*moduleResources, error) {
		return &moduleResources{
			handlers: handlerSet{
				build:   stubs.build,
				diff:    stubs.diff,
				clean:   stubs.clean,
				sitemap: stubs.sitemap,
			},
		}, nil
	}

	t.Cleanup(func() {
		moduleBuilder = original
		activeStubHandlers = nil
	})
	return func() {
		moduleBuilder = original
		activeStubHandlers = nil
	}
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOutput := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOutput)
		log.SetFlags(prevFlags)
	})
	return &buf
}

func TestRunBuild_UsesCommandHandler(t *testing.T) {
	restore := withStubModule(t)
	defer restore()

	buf := captureLogs(t)

	id := uuid.New()
	if err := run([]string{"build", "--page", id.String(), "--locale", "en"}); err != nil {
		t.Fatalf("run build: %v", err)
	}

	got := activeStubHandlers.build.last
	if len(got.PageIDs) != 1 || got.PageIDs[0] != id {
		t.Fatalf("expected page id %s, got %#v", id, got.PageIDs)
	}
	if len(got.Locales) != 1 || got.Locales[0] != "en" {
		t.Fatalf("expected locale en, got %#v", got.Locales)
	}
	if got.AssetsOnly {
		t.Fatal("expected assetsOnly to be false")
	}
	logOutput := buf.String()
	if !strings.Contains(logOutput, "module=static operation=build summary") &&
		!strings.Contains(logOutput, "module=static operation=build_page") {
		t.Fatalf("expected build summary or build_page log, got %q", logOutput)
	}
}

func TestRunBuild_AssetsOnlyLogsOperation(t *testing.T) {
	restore := withStubModule(t)
	defer restore()

	buf := captureLogs(t)

	if err := run([]string{"build", "--assets"}); err != nil {
		t.Fatalf("run build assets: %v", err)
	}
	if !activeStubHandlers.build.last.AssetsOnly {
		t.Fatal("expected AssetsOnly flag to be set")
	}
	if !strings.Contains(buf.String(), "module=static operation=build_assets") {
		t.Fatalf("expected build_assets log, got %q", buf.String())
	}
}

func TestRunDiff_UsesCommandHandler(t *testing.T) {
	restore := withStubModule(t)
	defer restore()

	buf := captureLogs(t)

	if err := run([]string{"diff", "--force", "--locale", "fr"}); err != nil {
		t.Fatalf("run diff: %v", err)
	}

	got := activeStubHandlers.diff.last
	if !got.Force {
		t.Fatal("expected force flag to propagate")
	}
	if len(got.Locales) != 1 || got.Locales[0] != "fr" {
		t.Fatalf("expected locale fr, got %#v", got.Locales)
	}
	if !strings.Contains(buf.String(), "module=static operation=build summary") {
		t.Fatalf("expected diff summary log, got %q", buf.String())
	}
}

func TestRunClean_UsesCommandHandler(t *testing.T) {
	restore := withStubModule(t)
	defer restore()

	buf := captureLogs(t)

	if err := run([]string{"clean"}); err != nil {
		t.Fatalf("run clean: %v", err)
	}
	if activeStubHandlers.clean.calls != 1 {
		t.Fatalf("expected clean handler called once, got %d", activeStubHandlers.clean.calls)
	}
	if !strings.Contains(buf.String(), "module=static operation=clean") {
		t.Fatalf("expected clean log, got %q", buf.String())
	}
}

func TestRunSitemap_UsesCommandHandler(t *testing.T) {
	restore := withStubModule(t)
	defer restore()

	buf := captureLogs(t)

	if err := run([]string{"sitemap"}); err != nil {
		t.Fatalf("run sitemap: %v", err)
	}
	if activeStubHandlers.sitemap.calls != 1 {
		t.Fatalf("expected sitemap handler called once, got %d", activeStubHandlers.sitemap.calls)
	}
	if !strings.Contains(buf.String(), "module=static operation=build_sitemap") {
		t.Fatalf("expected build_sitemap log, got %q", buf.String())
	}
}

func TestRunSitemap_HandlerMissing(t *testing.T) {
	original := moduleBuilder
	moduleBuilder = func(opts moduleOptions) (*moduleResources, error) {
		return &moduleResources{
			handlers: handlerSet{
				build: &stubBuildHandler{},
				diff:  &stubDiffHandler{},
				clean: &stubCleanHandler{},
			},
		}, nil
	}
	t.Cleanup(func() { moduleBuilder = original })

	captureLogs(t)

	err := run([]string{"sitemap"})
	if err == nil || !strings.Contains(err.Error(), "sitemap handler not configured") {
		t.Fatalf("expected sitemap handler error, got %v", err)
	}
}

func TestRun_ErrorsWhenHandlersMissing(t *testing.T) {
	original := moduleBuilder
	moduleBuilder = func(opts moduleOptions) (*moduleResources, error) {
		return &moduleResources{}, nil
	}
	t.Cleanup(func() { moduleBuilder = original })

	err := run([]string{"build"})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected handler error, got %v", err)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := run([]string{"unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown subcommand") {
		t.Fatalf("expected unknown subcommand error, got %v", err)
	}
}

func TestRun_NoArgs(t *testing.T) {
	err := run([]string{})
	if err == nil || !strings.Contains(err.Error(), "missing subcommand") {
		t.Fatalf("expected missing subcommand error, got %v", err)
	}
}

func TestRunHandlersPropagateErrors(t *testing.T) {
	original := moduleBuilder
	moduleBuilder = func(opts moduleOptions) (*moduleResources, error) {
		build := &stubBuildHandlerWithError{err: errors.New("boom")}
		diff := &stubDiffHandler{}
		clean := &stubCleanHandler{}
		activeStubHandlers = &stubHandlers{
			build:   &stubBuildHandler{},
			diff:    diff,
			clean:   clean,
			sitemap: &stubSitemapHandler{},
		}
		return &moduleResources{
			handlers: handlerSet{
				build: build,
				diff:  diff,
				clean: clean,
			},
		}, nil
	}
	t.Cleanup(func() {
		moduleBuilder = original
		activeStubHandlers = nil
	})

	captureLogs(t)

	err := run([]string{"build"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected propagated error, got %v", err)
	}
}

type stubBuildHandlerWithError struct {
	err error
}

func (s *stubBuildHandlerWithError) Execute(ctx context.Context, msg staticcmd.BuildSiteCommand) error {
	return s.err
}
