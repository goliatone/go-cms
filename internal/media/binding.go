package media

import (
	"maps"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Binding associates a CMS entity slot with an upstream media reference.
type Binding struct {
	Slot           string                    `json:"slot"`
	Reference      interfaces.MediaReference `json:"reference"`
	Renditions     []string                  `json:"renditions,omitempty"`
	Required       []string                  `json:"required,omitempty"`
	Locale         string                    `json:"locale,omitempty"`
	FallbackLocale string                    `json:"fallback_locale,omitempty"`
	Gallery        bool                      `json:"gallery,omitempty"`
	Position       int                       `json:"position,omitempty"`
	Metadata       map[string]any            `json:"metadata,omitempty"`
}

// BindingSet groups bindings under semantic keys (e.g. hero_image, gallery).
type BindingSet map[string][]Binding

// CloneBindingSet performs a deep copy of the binding set to avoid shared references.
func CloneBindingSet(src BindingSet) BindingSet {
	if len(src) == 0 {
		return nil
	}
	cloned := make(BindingSet, len(src))
	for key, bindings := range src {
		if len(bindings) == 0 {
			cloned[key] = nil
			continue
		}
		target := make([]Binding, len(bindings))
		for i, binding := range bindings {
			target[i] = Binding{
				Slot:           binding.Slot,
				Reference:      binding.Reference,
				Renditions:     append([]string(nil), binding.Renditions...),
				Required:       append([]string(nil), binding.Required...),
				Locale:         binding.Locale,
				FallbackLocale: binding.FallbackLocale,
				Gallery:        binding.Gallery,
				Position:       binding.Position,
			}
			if len(binding.Metadata) > 0 {
				target[i].Metadata = maps.Clone(binding.Metadata)
			}
		}
		cloned[key] = target
	}
	return cloned
}
