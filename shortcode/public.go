package shortcode

import (
	internal "github.com/goliatone/go-cms/internal/shortcode"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

type (
	Service             = internal.Service
	ServiceOption       = internal.ServiceOption
	Registry            = internal.Registry
	DefinitionValidator = internal.DefinitionValidator
	Renderer            = internal.Renderer
	RendererOption      = internal.RendererOption
	Validator           = internal.Validator
)

var (
	ErrDuplicateDefinition = internal.ErrDuplicateDefinition
	ErrInvalidDefinition   = internal.ErrInvalidDefinition
	ErrUnknownParameter    = internal.ErrUnknownParameter
	ErrMissingParameter    = internal.ErrMissingParameter
	ErrParameterType       = internal.ErrParameterType
)

func NewValidator() *Validator {
	return internal.NewValidator()
}

func NewRegistry(validator DefinitionValidator) *Registry {
	return internal.NewRegistry(validator)
}

func NewRenderer(registry interfaces.ShortcodeRegistry, validator *Validator, opts ...RendererOption) *Renderer {
	return internal.NewRenderer(registry, validator, opts...)
}

func NewService(registry interfaces.ShortcodeRegistry, renderer interfaces.ShortcodeRenderer, opts ...ServiceOption) *Service {
	return internal.NewService(registry, renderer, opts...)
}

func NewNoOpService() interfaces.ShortcodeService {
	return internal.NewNoOpService()
}

func RegisterBuiltIns(registry interfaces.ShortcodeRegistry, names []string) error {
	return internal.RegisterBuiltIns(registry, names)
}

func BuiltInDefinitions() []interfaces.ShortcodeDefinition {
	return internal.BuiltInDefinitions()
}

func NewSanitizer() interfaces.ShortcodeSanitizer {
	return internal.NewSanitizer()
}

func WithRendererSanitizer(s interfaces.ShortcodeSanitizer) RendererOption {
	return internal.WithRendererSanitizer(s)
}

func WithRendererCache(cache interfaces.CacheProvider) RendererOption {
	return internal.WithRendererCache(cache)
}

func WithRendererMetrics(metrics interfaces.ShortcodeMetrics) RendererOption {
	return internal.WithRendererMetrics(metrics)
}

func WithWordPressSyntax(enabled bool) ServiceOption {
	return internal.WithWordPressSyntax(enabled)
}

func WithDefaultSanitizer(s interfaces.ShortcodeSanitizer) ServiceOption {
	return internal.WithDefaultSanitizer(s)
}

func WithDefaultCache(cache interfaces.CacheProvider) ServiceOption {
	return internal.WithDefaultCache(cache)
}

func WithLogger(logger interfaces.Logger) ServiceOption {
	return internal.WithLogger(logger)
}

func WithMetrics(metrics interfaces.ShortcodeMetrics) ServiceOption {
	return internal.WithMetrics(metrics)
}
