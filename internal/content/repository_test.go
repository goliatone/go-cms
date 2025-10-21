package content_test

import (
	"path/filepath"
	"testing"

	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/pkg/testsupport"
)

func TestContentRepository_Fixtures(t *testing.T) {
	t.Skip("pending repository implementation")

	var sample content.Content
	if err := testsupport.LoadGolden(filepath.Join("testdata", "basic_content.json"), &sample); err != nil {
		t.Fatalf("load content fixture: %v", err)
	}

	var expect map[string]any
	if err := testsupport.LoadGolden(filepath.Join("testdata", "basic_content_output.json"), &expect); err != nil {
		t.Fatalf("load content output: %v", err)
	}

	_ = sample
	_ = expect
}
