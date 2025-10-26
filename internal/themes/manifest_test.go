package themes

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/goliatone/go-cms/pkg/testsupport"
)

func TestManifestParsingFixture(t *testing.T) {
	manifestPath := filepath.Join("testdata", "aurora_manifest.json")

	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	if manifest.Name == "" {
		t.Fatalf("expected manifest name to be populated")
	}

	input, err := ManifestToThemeInput("themes/aurora", manifest)
	if err != nil {
		t.Fatalf("manifest to input: %v", err)
	}

	var want RegisterThemeInput
	goldenPath := filepath.Join("testdata", "aurora_manifest.golden.json")
	if err := testsupport.LoadGolden(goldenPath, &want); err != nil {
		t.Fatalf("load manifest golden: %v", err)
	}

	if !reflect.DeepEqual(want, input) {
		t.Fatalf("manifest conversion mismatch:\nwant: %#v\n got: %#v", want, input)
	}
}

func TestManifestToThemeInputValidation(t *testing.T) {
	if _, err := ManifestToThemeInput("themes/aurora", nil); err == nil {
		t.Fatalf("expected error when manifest is nil")
	}

	if _, err := ManifestToThemeInput("themes/aurora", &Manifest{Version: "1.0.0"}); err == nil {
		t.Fatalf("expected error when name missing")
	}

	if _, err := ManifestToThemeInput("themes/aurora", &Manifest{Name: "aurora"}); err == nil {
		t.Fatalf("expected error when version missing")
	}
}
