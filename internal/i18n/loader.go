package i18n

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Fixture represents a serialised bundle of configuration + translations.
type Fixture struct {
	Config       Config                       `json:"config"`
	Translations map[string]map[string]string `json:"translations"`
}

//go:embed testdata/translations_fixture.json
var defaultFixtureData embed.FS

// DefaultFixture loads the built-in translation fixture.
func DefaultFixture() (*Fixture, error) {
	data, err := defaultFixtureData.ReadFile("testdata/translations_fixture.json")
	if err != nil {
		return nil, fmt.Errorf("i18n: read embedded fixture: %w", err)
	}

	return decodeFixture(bytes.NewReader(data))
}

// Loader reads translation fixtures from disk.
type Loader struct {
	path string
}

// NewLoader constructs a loader that reads the provided file path.
func NewLoader(path string) *Loader {
	return &Loader{path: path}
}

// Load parses the configured fixture file.
func (l *Loader) Load(ctx context.Context) (*Fixture, error) {
	if l == nil || l.path == "" {
		return nil, errors.New("i18n: loader path cannot be empty")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	file, err := os.Open(l.path)
	if err != nil {
		return nil, fmt.Errorf("i18n: open fixture %q: %w", l.path, err)
	}
	defer file.Close()

	return decodeFixture(file)
}

func decodeFixture(r io.Reader) (*Fixture, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()

	var fx Fixture
	if err := decoder.Decode(&fx); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	if fx.Translations == nil {
		fx.Translations = map[string]map[string]string{}
	}

	return &fx, nil
}
