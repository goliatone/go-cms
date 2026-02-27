package themes

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Manifest mirrors the expected theme.json structure.
type Manifest struct {
	Name          string              `json:"name"`
	Description   *string             `json:"description,omitempty"`
	Version       string              `json:"version"`
	Author        *string             `json:"author,omitempty"`
	WidgetAreas   []ThemeWidgetArea   `json:"widget_areas,omitempty"`
	MenuLocations []ThemeMenuLocation `json:"menu_locations,omitempty"`
	Assets        *ThemeAssets        `json:"assets,omitempty"`
	Metadata      map[string]any      `json:"metadata,omitempty"`
}

// LoadManifest reads and parses a manifest from disk.
func LoadManifest(path string) (*Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("themes: open manifest: %w", err)
	}
	defer file.Close()
	return ParseManifest(file)
}

// ParseManifest decodes manifest JSON from a reader.
func ParseManifest(r io.Reader) (*Manifest, error) {
	var manifest Manifest
	if err := json.NewDecoder(r).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("themes: parse manifest: %w", err)
	}
	return &manifest, nil
}

// ManifestToThemeInput converts a manifest into a registration payload.
func ManifestToThemeInput(themePath string, manifest *Manifest) (RegisterThemeInput, error) {
	if manifest == nil {
		return RegisterThemeInput{}, fmt.Errorf("themes: manifest required")
	}
	if manifest.Name == "" {
		return RegisterThemeInput{}, fmt.Errorf("themes: manifest missing name")
	}
	if manifest.Version == "" {
		return RegisterThemeInput{}, fmt.Errorf("themes: manifest missing version")
	}

	config := ThemeConfig{
		WidgetAreas:   manifest.WidgetAreas,
		MenuLocations: manifest.MenuLocations,
		Assets:        manifest.Assets,
		Metadata:      manifest.Metadata,
	}

	return RegisterThemeInput{
		Name:        manifest.Name,
		Description: manifest.Description,
		Version:     manifest.Version,
		Author:      manifest.Author,
		ThemePath:   filepath.Clean(themePath),
		Config:      config,
	}, nil
}
