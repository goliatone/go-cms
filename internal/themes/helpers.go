package themes

import (
	"fmt"
	"strings"
)

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := strings.Clone(*value)
	return &cloned
}

func mergeMetadata(a, b map[string]any) map[string]any {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := deepCloneMap(a)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range b {
		out[key] = value
	}
	return out
}

func cloneThemeConfig(cfg ThemeConfig) ThemeConfig {
	return ThemeConfig{
		WidgetAreas:   cloneWidgetAreas(cfg.WidgetAreas),
		MenuLocations: cloneMenuLocations(cfg.MenuLocations),
		Assets:        cloneAssets(cfg.Assets),
		Metadata:      deepCloneMap(cfg.Metadata),
	}
}

func cloneWidgetAreas(src []ThemeWidgetArea) []ThemeWidgetArea {
	if len(src) == 0 {
		return nil
	}
	out := make([]ThemeWidgetArea, len(src))
	for i, area := range src {
		cloned := area
		cloned.Description = cloneString(area.Description)
		out[i] = cloned
	}
	return out
}

func cloneMenuLocations(src []ThemeMenuLocation) []ThemeMenuLocation {
	if len(src) == 0 {
		return nil
	}
	out := make([]ThemeMenuLocation, len(src))
	for i, location := range src {
		cloned := location
		cloned.Description = cloneString(location.Description)
		out[i] = cloned
	}
	return out
}

func validateThemeConfig(cfg ThemeConfig) error {
	for _, area := range cfg.WidgetAreas {
		if strings.TrimSpace(area.Code) == "" || strings.TrimSpace(area.Name) == "" {
			return ErrThemeWidgetAreaInvalid
		}
	}
	for _, location := range cfg.MenuLocations {
		if strings.TrimSpace(location.Code) == "" || strings.TrimSpace(location.Name) == "" {
			return fmt.Errorf("themes: menu location code and name required")
		}
	}
	return nil
}
