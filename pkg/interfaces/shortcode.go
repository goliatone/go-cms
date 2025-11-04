package interfaces

import (
	"context"
	"html/template"
	"time"
)

// ShortcodeRegistry describes the lifecycle contract for registering and resolving
// shortcode definitions. Implementations must be safe for concurrent use and
// honour the validation semantics documented in SHORTCODE_TDD.md.
type ShortcodeRegistry interface {
	// Register stores a definition and returns an error when a shortcode
	// with the same name already exists or the definition fails validation.
	Register(definition ShortcodeDefinition) error

	// Get returns the definition for the supplied shortcode name.
	Get(name string) (ShortcodeDefinition, bool)

	// List exposes the current catalogue, sorted at the implementor's discretion.
	List() []ShortcodeDefinition

	// Remove deletes the shortcode from the registry. Removing an unknown shortcode
	// must be a no-op.
	Remove(name string)
}

// ShortcodeRenderer executes a shortcode definition and returns HTML output.
type ShortcodeRenderer interface {
	Render(ctx ShortcodeContext, shortcode string, params map[string]any, inner string) (template.HTML, error)
	RenderAsync(ctx ShortcodeContext, shortcode string, params map[string]any, inner string) (<-chan template.HTML, <-chan error)
}

// ShortcodeParser extracts shortcode invocations from arbitrary content.
type ShortcodeParser interface {
	Parse(content string) ([]ParsedShortcode, error)
	Extract(content string) (placeholders string, shortcodes []ParsedShortcode, err error)
}

// ShortcodeSanitizer encapsulates sanitisation helpers applied after rendering.
type ShortcodeSanitizer interface {
	Sanitize(html string) (string, error)
	ValidateURL(raw string) error
	ValidateAttributes(attrs map[string]any) error
}

// ShortcodeDefinition captures the metadata, validation schema, and template
// references that the registry stores.
type ShortcodeDefinition struct {
	Name        string
	Version     string
	Description string
	Category    string
	Icon        string
	AllowInner  bool
	Async       bool
	CacheTTL    time.Duration
	Schema      ShortcodeSchema
	Template    string
	Handler     ShortcodeHandler
}

// ShortcodeSchema defines the contract for parameters accepted by a shortcode.
type ShortcodeSchema struct {
	Params   []ShortcodeParam
	Defaults map[string]any
}

// ShortcodeParam describes a single parameter, including optional custom validation.
type ShortcodeParam struct {
	Name     string
	Type     ShortcodeParamType
	Required bool
	Default  any
	Validate ShortcodeValidator
}

// ShortcodeParamType enumerates the supported parameter coercions.
type ShortcodeParamType string

const (
	ShortcodeParamString ShortcodeParamType = "string"
	ShortcodeParamInt    ShortcodeParamType = "int"
	ShortcodeParamBool   ShortcodeParamType = "bool"
	ShortcodeParamArray  ShortcodeParamType = "array"
	ShortcodeParamURL    ShortcodeParamType = "url"
)

// ShortcodeValidator allows definitions to perform custom validation.
type ShortcodeValidator func(value any) error

// ShortcodeHandler executes the shortcode with resolved parameters.
type ShortcodeHandler func(ctx ShortcodeContext, params map[string]any, inner string) (template.HTML, error)

// ShortcodeContext provides runtime metadata surfaced during rendering.
type ShortcodeContext struct {
	Context   context.Context
	Locale    string
	Cache     CacheProvider
	Sanitizer ShortcodeSanitizer
}

// ParsedShortcode represents a parsed invocation discovered by the parser layer.
type ParsedShortcode struct {
	Name   string
	Params map[string]any
	Inner  string
}
