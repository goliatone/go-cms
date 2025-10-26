package themes

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// ThemeSeed describes a theme and its templates for bootstrapping.
type ThemeSeed struct {
	Theme     RegisterThemeInput
	Templates []RegisterTemplateInput
}

// Registry stores built-in or host-defined theme seeds.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]ThemeSeed
}

// NewRegistry constructs an empty theme registry.
func NewRegistry() *Registry {
	return &Registry{
		entries: make(map[string]ThemeSeed),
	}
}

// Register adds a theme seed to the registry, overriding existing keys.
func (r *Registry) Register(seed ThemeSeed) {
	if seed.Theme.Name == "" {
		return
	}
	key := canonicalKey(seed.Theme.Name)
	if key == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[key] = seed
}

// List returns all registered seeds.
func (r *Registry) List() []ThemeSeed {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]ThemeSeed, 0, len(r.entries))
	for _, seed := range r.entries {
		out = append(out, seed)
	}
	return out
}

func canonicalKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// Bootstrap applies seeds to the provided service, tolerating duplicates.
func Bootstrap(ctx context.Context, svc Service, seeds []ThemeSeed) error {
	for _, seed := range seeds {
		theme, err := svc.RegisterTheme(ctx, seed.Theme)
		if err != nil {
			if errors.Is(err, ErrThemeExists) {
				theme, err = svc.GetThemeByName(ctx, seed.Theme.Name)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		for _, templateInput := range seed.Templates {
			input := templateInput
			input.ThemeID = theme.ID
			if _, err := svc.RegisterTemplate(ctx, input); err != nil {
				if errors.Is(err, ErrTemplateSlugConflict) {
					continue
				}
				return err
			}
		}

		if seed.Theme.Activate {
			if _, err := svc.ActivateTheme(ctx, theme.ID); err != nil {
				if !errors.Is(err, ErrThemeActivationMissingTemplates) {
					return err
				}
			}
		}
	}
	return nil
}
