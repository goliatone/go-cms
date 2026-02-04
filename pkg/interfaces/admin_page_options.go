package interfaces

// AdminPageIncludeOptions declares optional heavy-field inclusion for admin reads.
type AdminPageIncludeOptions struct {
	IncludeContent bool
	IncludeBlocks  bool
	IncludeData    bool
}

// AdminPageIncludeDefaults defines default include behavior per endpoint.
type AdminPageIncludeDefaults struct {
	List AdminPageIncludeOptions
	Get  AdminPageIncludeOptions
}

// AdminPageListOptions defines the admin page list read contract.
//
// Behavior contract:
//   - Locale resolution: if a translation exists for Locale, use it and set ResolvedLocale = Locale.
//   - If missing and FallbackLocale is set, use it and set ResolvedLocale = FallbackLocale.
//   - If still missing, return a record with empty localized fields while preserving identifiers/status.
//   - RequestedLocale is always set to Locale, even when missing, to avoid UI ambiguity.
//   - When IncludeData is true, translation content/meta fields are merged into Data
//     (path/meta/summary/tags/etc) and used for Content/Meta fallbacks.
//   - Blocks payload: when IncludeBlocks is true, prefer embedded blocks arrays; fall back to legacy
//     []string block IDs when embedded blocks are absent. Blocks is omitted unless IncludeBlocks is true.
//   - Content preserves structure when it is not a simple string.
type AdminPageListOptions struct {
	Locale                   string
	FallbackLocale           string
	AllowMissingTranslations bool
	IncludeContent           bool
	IncludeBlocks            bool
	IncludeData              bool
	EnvironmentKey           string
	DefaultIncludes          *AdminPageIncludeDefaults
	Page                     int
	PerPage                  int
	SortBy                   string
	SortDesc                 bool
	Search                   string
	Filters                  map[string]any
}

// AdminPageGetOptions defines the admin page detail read contract.
type AdminPageGetOptions struct {
	Locale                   string
	FallbackLocale           string
	AllowMissingTranslations bool
	IncludeContent           bool
	IncludeBlocks            bool
	IncludeData              bool
	EnvironmentKey           string
	DefaultIncludes          *AdminPageIncludeDefaults
}
