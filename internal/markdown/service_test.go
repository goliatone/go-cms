package markdown

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/testsupport"
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

func TestServiceImportLogsGolden(t *testing.T) {
	contentStub := newStubContentService()
	pageStub := newStubPageService()
	logger := newGoldenLogger()

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

	assertGoldenLogs(t, logger.entries(), filepath.Join("testdata", "logs", "import_success.golden.json"))
}

func TestServiceImportLogsErrorGolden(t *testing.T) {
	contentStub := newStubContentService()
	pageStub := newStubPageService()
	logger := newGoldenLogger()

	svc := newImportService(t, contentStub, pageStub, WithLogger(logger))

	if _, err := svc.Import(context.Background(), nil, interfaces.ImportOptions{}); err == nil {
		t.Fatal("expected error when document is nil")
	}

	assertGoldenLogs(t, logger.entries(), filepath.Join("testdata", "logs", "import_failure.golden.json"))
}

type logEntry struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

type goldenLogger struct {
	ctx     map[string]any
	records *[]logEntry
}

func newGoldenLogger() *goldenLogger {
	entrySlice := make([]logEntry, 0)
	return &goldenLogger{
		ctx:     map[string]any{},
		records: &entrySlice,
	}
}

func (l *goldenLogger) Trace(msg string, args ...any) { l.record("trace", msg, args...) }
func (l *goldenLogger) Debug(msg string, args ...any) { l.record("debug", msg, args...) }
func (l *goldenLogger) Info(msg string, args ...any)  { l.record("info", msg, args...) }
func (l *goldenLogger) Warn(msg string, args ...any)  { l.record("warn", msg, args...) }
func (l *goldenLogger) Error(msg string, args ...any) { l.record("error", msg, args...) }
func (l *goldenLogger) Fatal(msg string, args ...any) { l.record("fatal", msg, args...) }

func (l *goldenLogger) WithFields(fields map[string]any) interfaces.Logger {
	merged := cloneFields(l.ctx)
	for key, value := range fields {
		merged[key] = normaliseLogValue(value)
	}
	return &goldenLogger{
		ctx:     merged,
		records: l.records,
	}
}

func (l *goldenLogger) WithContext(context.Context) interfaces.Logger {
	return &goldenLogger{
		ctx:     cloneFields(l.ctx),
		records: l.records,
	}
}

func (l *goldenLogger) entries() []logEntry {
	if l.records == nil {
		return nil
	}
	return append([]logEntry(nil), (*l.records)...)
}

func (l *goldenLogger) record(level, msg string, args ...any) {
	if l.records == nil {
		return
	}
	message := msg
	if len(args) > 0 {
		message = fmt.Sprintf(msg, args...)
	}
	fields := cloneFields(l.ctx)
	entry := logEntry{
		Level:   level,
		Message: message,
	}
	if len(fields) > 0 {
		entry.Fields = fields
	}
	*l.records = append(*l.records, entry)
}

func assertGoldenLogs(t *testing.T, entries []logEntry, goldenPath string) {
	t.Helper()

	var want []logEntry
	if err := testsupport.LoadGolden(goldenPath, &want); err != nil {
		t.Fatalf("load golden %s: %v", goldenPath, err)
	}

	if reflect.DeepEqual(entries, want) {
		return
	}

	gotJSON, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatalf("marshal got logs: %v", err)
	}
	wantJSON, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("marshal want logs: %v", err)
	}

	t.Fatalf("markdown logs mismatch\nwant:\n%s\n\ngot:\n%s", string(wantJSON), string(gotJSON))
}

func cloneFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(fields))
	for key, value := range fields {
		out[key] = normaliseLogValue(value)
	}
	return out
}

func normaliseLogValue(value any) any {
	switch v := value.(type) {
	case error:
		return v.Error()
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	default:
		return v
	}
}
