package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHugoParser_Extract(t *testing.T) {
	parser := NewHugoParser()

	input := mustReadFile(t, "hugo_basic_input.txt")
	wantOutput := mustReadFile(t, "hugo_basic_output.golden")

	gotContent, shortcodes, err := parser.Extract(input)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}

	if strings.TrimSpace(gotContent) != strings.TrimSpace(wantOutput) {
		t.Fatalf("Extract() output mismatch\n got: %q\nwant: %q", gotContent, wantOutput)
	}

	if len(shortcodes) != 2 {
		t.Fatalf("expected 2 shortcodes, got %d", len(shortcodes))
	}
	if shortcodes[0].Name != "youtube" {
		t.Fatalf("expected first shortcode youtube, got %s", shortcodes[0].Name)
	}
	if shortcodes[1].Inner != "Stay safe!" {
		t.Fatalf("expected inner content 'Stay safe!', got %q", shortcodes[1].Inner)
	}
}

func TestHugoParser_Mismatched(t *testing.T) {
	parser := NewHugoParser()
	input := "{{< alert type=\"warning\" >}}Oops{{< /youtube >}}"

	if _, _, err := parser.Extract(input); err == nil {
		t.Fatal("expected error for mismatched shortcode closure")
	}
}

func mustReadFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}
