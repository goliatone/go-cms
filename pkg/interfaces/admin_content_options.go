package interfaces

// AdminContentIncludeOptions declares optional heavy-field inclusion for admin content reads.
type AdminContentIncludeOptions struct {
	IncludeData     bool
	IncludeMetadata bool
	IncludeBlocks   bool
}

// AdminContentIncludeDefaults defines default include behavior per endpoint.
type AdminContentIncludeDefaults struct {
	List AdminContentIncludeOptions
	Get  AdminContentIncludeOptions
}

// AdminContentListOptions defines the admin content list read contract.
type AdminContentListOptions struct {
	Locale                   string
	FallbackLocale           string
	AllowMissingTranslations bool
	IncludeData              bool
	IncludeMetadata          bool
	IncludeBlocks            bool
	EnvironmentKey           string
	DefaultIncludes          *AdminContentIncludeDefaults
	Page                     int
	PerPage                  int
	SortBy                   string
	SortDesc                 bool
	Search                   string
	Filters                  map[string]any
	Fields                   []string
}

// AdminContentGetOptions defines the admin content detail read contract.
type AdminContentGetOptions struct {
	Locale                   string
	FallbackLocale           string
	AllowMissingTranslations bool
	IncludeData              bool
	IncludeMetadata          bool
	IncludeBlocks            bool
	EnvironmentKey           string
	DefaultIncludes          *AdminContentIncludeDefaults
}
