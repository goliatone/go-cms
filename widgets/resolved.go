package widgets

import "github.com/google/uuid"

// ResolvedWidget pairs a widget instance with its placement metadata.
type ResolvedWidget struct {
	Instance *Instance `json:"instance"`
	// Config is the fully resolved payload hosts should render. It starts from
	// the base widget instance configuration and overlays the first matching
	// translation in the requested locale/fallback chain.
	Config map[string]any `json:"config,omitempty"`
	// ResolvedTranslation identifies the translation that produced Config when a
	// locale-specific or fallback translation matched.
	ResolvedTranslation *Translation `json:"resolved_translation,omitempty"`
	// ResolvedLocaleID records which locale from the requested resolution chain
	// produced the final localized Config when a translation match was found.
	ResolvedLocaleID *uuid.UUID     `json:"resolved_locale_id,omitempty"`
	Placement        *AreaPlacement `json:"placement"`
}
