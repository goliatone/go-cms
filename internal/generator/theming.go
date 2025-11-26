package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goliatone/go-cms/internal/themes"
	gotheme "github.com/goliatone/go-theme"
	"github.com/google/uuid"
)

type themeManifestLoader interface {
	Load(themePath string) (*gotheme.Manifest, error)
}

type fsThemeManifestLoader struct{}

func (fsThemeManifestLoader) Load(themePath string) (*gotheme.Manifest, error) {
	cleaned := filepath.Clean(strings.TrimSpace(themePath))
	if cleaned == "" {
		return nil, fmt.Errorf("theme path required")
	}

	return gotheme.LoadDir(os.DirFS(cleaned), ".")
}

type themeSelector struct {
	registry       *gotheme.MemoryRegistry
	loader         themeManifestLoader
	defaultTheme   string
	defaultVariant string

	mu        sync.Mutex
	manifests map[uuid.UUID]*gotheme.Manifest
}

func newThemeSelector(cfg ThemingConfig, loader themeManifestLoader) *themeSelector {
	if loader == nil {
		loader = fsThemeManifestLoader{}
	}
	return &themeSelector{
		registry:       gotheme.NewRegistry(),
		loader:         loader,
		defaultTheme:   strings.TrimSpace(cfg.DefaultTheme),
		defaultVariant: strings.TrimSpace(cfg.DefaultVariant),
		manifests:      map[uuid.UUID]*gotheme.Manifest{},
	}
}

func (s *themeSelector) Selection(themeRecord *themes.Theme, variant string) (*gotheme.Selection, error) {
	if themeRecord == nil {
		return nil, nil
	}

	if _, err := s.ensureManifest(themeRecord); err != nil {
		return nil, err
	}

	selector := gotheme.Selector{
		Registry:       s.registry,
		DefaultTheme:   s.defaultTheme,
		DefaultVariant: s.defaultVariant,
	}

	resolvedVariant := strings.TrimSpace(variant)
	if resolvedVariant == "" {
		resolvedVariant = s.defaultVariant
	}

	selection, err := selector.Select(themeRecord.Name, resolvedVariant)
	if err != nil {
		return nil, fmt.Errorf("select theme %s: %w", themeRecord.Name, err)
	}
	return selection, nil
}

func (s *themeSelector) ensureManifest(themeRecord *themes.Theme) (*gotheme.Manifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if manifest, ok := s.manifests[themeRecord.ID]; ok {
		return manifest, nil
	}

	manifest, err := s.loader.Load(themeRecord.ThemePath)
	if err != nil {
		return nil, fmt.Errorf("load theme manifest from %s: %w", themeRecord.ThemePath, err)
	}

	normalized := *manifest
	if strings.TrimSpace(normalized.Name) == "" || !strings.EqualFold(normalized.Name, themeRecord.Name) {
		normalized.Name = strings.TrimSpace(themeRecord.Name)
	}
	if strings.TrimSpace(normalized.Version) == "" {
		normalized.Version = strings.TrimSpace(themeRecord.Version)
	}
	if normalized.Name == "" {
		return nil, fmt.Errorf("theme name required for manifest registration")
	}

	if err := s.registry.Register(&normalized); err != nil {
		return nil, fmt.Errorf("register theme manifest: %w", err)
	}
	s.manifests[themeRecord.ID] = &normalized
	return &normalized, nil
}
