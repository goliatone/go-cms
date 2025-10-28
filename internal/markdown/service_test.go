package markdown

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/goliatone/go-cms/pkg/interfaces"
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
