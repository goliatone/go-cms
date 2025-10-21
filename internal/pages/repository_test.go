package pages_test

import (
	"path/filepath"
	"testing"

	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/pkg/testsupport"
)

func TestPageRepository_Fixtures(t *testing.T) {
	t.Skip("pending repository implementation")

	var samples []pages.Page
	if err := testsupport.LoadGolden(filepath.Join("testdata", "hierarchical_pages.json"), &samples); err != nil {
		t.Fatalf("load page fixture: %v", err)
	}

	var expect []map[string]any
	if err := testsupport.LoadGolden(filepath.Join("testdata", "hierarchical_pages_output.json"), &expect); err != nil {
		t.Fatalf("load page output: %v", err)
	}

	_ = samples
	_ = expect
}
