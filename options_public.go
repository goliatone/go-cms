package cms

import (
	"github.com/goliatone/go-cms/generator"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Option customises module wiring at construction time.
type Option = di.Option

// WithTemplate overrides the default template renderer.
func WithTemplate(tr interfaces.TemplateRenderer) Option {
	return di.WithTemplate(tr)
}

// WithGeneratorStorage overrides the storage provider used by the static generator.
func WithGeneratorStorage(sp interfaces.StorageProvider) Option {
	return di.WithGeneratorStorage(sp)
}

// WithGeneratorAssetResolver overrides the generator asset resolver.
func WithGeneratorAssetResolver(resolver generator.AssetResolver) Option {
	return di.WithGeneratorAssetResolver(resolver)
}
