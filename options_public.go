package cms

import (
	"github.com/goliatone/go-cms/generator"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/pkg/lifecycle"
	"github.com/uptrace/bun"
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

// WithLifecycleHooks appends lifecycle hooks that receive root-record mutation events.
func WithLifecycleHooks(hooks lifecycle.Hooks) Option {
	return di.WithLifecycleHooks(hooks)
}

// WithBunDB binds the CMS repositories and bun-backed storage adapter to an existing runtime bun handle.
func WithBunDB(db *bun.DB) Option {
	return di.WithBunDB(db)
}
