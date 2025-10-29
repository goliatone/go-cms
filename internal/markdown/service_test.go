package markdown

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

func TestServiceLoad(t *testing.T) {
	svc := newTestService(t, true)

	doc, err := svc.Load(context.Background(), "en/about.md", interfaces.LoadOptions{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if doc.Locale != "en" {
		t.Fatalf("expected locale en, got %s", doc.Locale)
	}
	if len(doc.BodyHTML) == 0 {
		t.Fatalf("expected BodyHTML to be populated")
	}
	if len(doc.Checksum) == 0 {
		t.Fatalf("expected checksum to be populated")
	}
}

func TestServiceLoadDirectory_MixedLocales(t *testing.T) {
	svc := newTestService(t, true)

	docs, err := svc.LoadDirectory(context.Background(), ".", interfaces.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadDirectory: %v", err)
	}

	if len(docs) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(docs))
	}

	locales := map[string]int{}
	var foundBlog bool
	for _, doc := range docs {
		locales[doc.Locale]++
		if filepath.Ext(doc.FilePath) != ".md" {
			t.Fatalf("expected markdown file, got %s", doc.FilePath)
		}
		if len(doc.Checksum) == 0 {
			t.Fatalf("expected checksum set for %s", doc.FilePath)
		}
		if doc.FilePath == "en/blog/post.md" {
			foundBlog = true
		}
	}

	if locales["en"] != 2 || locales["es"] != 1 {
		t.Fatalf("unexpected locale distribution: %#v", locales)
	}
	if !foundBlog {
		t.Fatalf("expected to include en/blog/post.md")
	}
}

func TestServiceLoadDirectory_NonRecursiveOverride(t *testing.T) {
	svc := newTestService(t, true)

	no := false
	docs, err := svc.LoadDirectory(context.Background(), "en", interfaces.LoadOptions{
		Recursive: &no,
	})
	if err != nil {
		t.Fatalf("LoadDirectory override: %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].FilePath != "en/about.md" {
		t.Fatalf("expected en/about.md, got %s", docs[0].FilePath)
	}
}

func newTestService(tb testing.TB, recursive bool) *Service {
	tb.Helper()

	baseCfg := Config{
		BasePath:      filepath.Join("testdata", "site"),
		DefaultLocale: "en",
		Locales:       []string{"en", "es"},
		LocalePatterns: map[string]string{
			"es": "es/*.md",
		},
		Pattern:   "*.md",
		Recursive: recursive,
	}

	svc, err := NewService(baseCfg, nil)
	if err != nil {
		tb.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestServiceImportLogsResult(t *testing.T) {
	contentStub := newStubContentService()
	pageStub := newStubPageService()
	logger := &recordingLogger{}

	svc := newImportService(t, contentStub, pageStub, WithLogger(logger))

	doc, err := svc.Load(context.Background(), "en/about.md", interfaces.LoadOptions{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	opts := interfaces.ImportOptions{
		ContentTypeID: uuid.New(),
		AuthorID:      uuid.New(),
	}

	if _, err := svc.Import(context.Background(), doc, opts); err != nil {
		t.Fatalf("Import: %v", err)
	}

	var foundContext, foundSummary bool
	for _, fields := range logger.fields {
		if path, ok := fields["markdown_path"]; ok && path == "en/about.md" {
			if action, ok := fields["sync_action"]; ok && action == "import" {
				foundContext = true
			}
		}
		if _, ok := fields["created"]; ok {
			foundSummary = true
		}
	}

	if !foundContext {
		t.Fatalf("expected markdown context fields recorded, got %#v", logger.fields)
	}
	if !foundSummary {
		t.Fatalf("expected import summary fields recorded, got %#v", logger.fields)
	}
}

func TestServiceImportLogsError(t *testing.T) {
	contentStub := newStubContentService()
	pageStub := newStubPageService()
	logger := &recordingLogger{}

	svc := newImportService(t, contentStub, pageStub, WithLogger(logger))

	if _, err := svc.Import(context.Background(), nil, interfaces.ImportOptions{}); err == nil {
		t.Fatal("expected error when document is nil")
	}

	var foundAction, foundError bool
	for _, fields := range logger.fields {
		if action, ok := fields["sync_action"]; ok && action == "import" {
			foundAction = true
		}
		if reason, ok := fields["error"]; ok && reason == "document_nil" {
			foundError = true
		}
	}
	if !foundAction {
		t.Fatalf("expected sync_action field recorded, got %#v", logger.fields)
	}
	if !foundError {
		t.Fatalf("expected error field recorded, got %#v", logger.fields)
	}
}

type recordingLogger struct {
	fields []map[string]any
}

func (r *recordingLogger) Trace(string, ...any) {}
func (r *recordingLogger) Debug(string, ...any) {}
func (r *recordingLogger) Info(string, ...any)  {}
func (r *recordingLogger) Warn(string, ...any)  {}
func (r *recordingLogger) Error(string, ...any) {}
func (r *recordingLogger) Fatal(string, ...any) {}

func (r *recordingLogger) WithFields(fields map[string]any) interfaces.Logger {
	if fields == nil {
		return r
	}
	copied := make(map[string]any, len(fields))
	for k, v := range fields {
		copied[k] = v
	}
	r.fields = append(r.fields, copied)
	return r
}

func (r *recordingLogger) WithContext(context.Context) interfaces.Logger {
	return r
}
