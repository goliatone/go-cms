package markdown

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

func TestParseFrontMatter(t *testing.T) {
	data := readFixture(t, "testdata/basic.md")

	fm, body, err := ParseFrontMatter(data)
	if err != nil {
		t.Fatalf("ParseFrontMatter: %v", err)
	}

	if fm.Title != "Sample Document" {
		t.Fatalf("FrontMatter Title mismatch, got %q", fm.Title)
	}
	if fm.Slug != "sample-document" {
		t.Fatalf("FrontMatter Slug mismatch, got %q", fm.Slug)
	}
	if len(fm.Tags) != 2 || fm.Tags[0] != "cms" {
		t.Fatalf("FrontMatter Tags mismatch: %#v", fm.Tags)
	}
	if fm.Custom["custom_flag"] != true {
		t.Fatalf("FrontMatter Custom flag missing: %#v", fm.Custom)
	}
	if fm.Raw["summary"] != "Sample summary goes here" {
		t.Fatalf("FrontMatter Raw summary missing: %#v", fm.Raw)
	}
	if len(body) == 0 || !strings.Contains(string(body), "# Sample Document") {
		t.Fatalf("Markdown body not returned correctly: %q", string(body))
	}
}

func TestBuildDocument(t *testing.T) {
	data := readFixture(t, "testdata/basic.md")
	modified := time.Now().UTC()

	doc, err := BuildDocument("testdata/basic.md", "en", data, modified)
	if err != nil {
		t.Fatalf("BuildDocument: %v", err)
	}

	if doc.FilePath != "testdata/basic.md" {
		t.Fatalf("expected FilePath to be set, got %q", doc.FilePath)
	}
	if doc.Locale != "en" {
		t.Fatalf("expected Locale to be en, got %q", doc.Locale)
	}
	if doc.LastModified != modified {
		t.Fatalf("expected LastModified to equal the provided timestamp")
	}
	if len(doc.Body) == 0 {
		t.Fatalf("expected Body to contain markdown content")
	}
}

func TestGoldmarkParser_Parse(t *testing.T) {
	parser := NewGoldmarkParser(interfaces.ParseOptions{})

	html, err := parser.Parse([]byte("# Heading\n\nHello **world**"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	got := string(html)
	if !strings.Contains(got, "<h1") || !strings.Contains(got, "Heading</h1>") {
		t.Fatalf("expected rendered HTML to include <h1>Heading</h1>, got %q", got)
	}
	if !strings.Contains(got, "<strong>world</strong>") {
		t.Fatalf("expected rendered HTML to include <strong>, got %q", got)
	}
}

func TestGoldmarkParser_ParseWithOptions(t *testing.T) {
	parser := NewGoldmarkParser(interfaces.ParseOptions{})

	html, err := parser.ParseWithOptions([]byte("line one\nline two"), interfaces.ParseOptions{
		HardWraps: true,
	})
	if err != nil {
		t.Fatalf("ParseWithOptions: %v", err)
	}

	if !strings.Contains(string(html), "line one<br>") {
		t.Fatalf("expected hard wraps in HTML output, got %q", string(html))
	}
}

func readFixture(tb testing.TB, path string) []byte {
	tb.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}
