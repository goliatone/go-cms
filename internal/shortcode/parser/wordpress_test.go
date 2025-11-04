package parser

import (
	"strings"
	"testing"
)

func TestWordPressPreprocessor_Process(t *testing.T) {
	pre := NewWordPressPreprocessor()

	input := mustReadFile(t, "wordpress_basic_input.txt")
	want := mustReadFile(t, "wordpress_to_hugo.golden")

	got := pre.Process(input)
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("Process() mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestWordPressPreprocessor_Idempotent(t *testing.T) {
	pre := NewWordPressPreprocessor()
	input := "Just text without shortcodes"
	if out := pre.Process(input); out != input {
		t.Fatalf("expected output to equal input when no shortcodes present")
	}
}
