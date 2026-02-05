package interfaces

import "errors"

// ErrTranslationMissing is returned when a requested locale translation is missing.
var ErrTranslationMissing = errors.New("translation missing")

// TranslationMeta describes locale resolution for a translated record.
type TranslationMeta struct {
	RequestedLocale string `json:"requested_locale"`
	ResolvedLocale  string `json:"resolved_locale"`
	// AvailableLocales is scoped to the bundle's underlying entity (page vs content),
	// not a union across multiple bundles.
	AvailableLocales       []string `json:"available_locales"`
	MissingRequestedLocale bool     `json:"missing_requested_locale"`
	FallbackUsed           bool     `json:"fallback_used"`
	PrimaryLocale          string   `json:"primary_locale"`
}

// TranslationBundle wraps requested/resolved translations with locale metadata.
type TranslationBundle[T any] struct {
	Meta      TranslationMeta `json:"meta"`
	Requested *T              `json:"requested"`
	Resolved  *T              `json:"resolved"`
}
