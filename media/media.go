package media

import (
	"time"

	internalmedia "github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Re-exported errors from the internal media package.
var (
	ErrProviderUnavailable = internalmedia.ErrProviderUnavailable
	ErrAssetNotFound       = internalmedia.ErrAssetNotFound
	ErrRenditionMissing    = internalmedia.ErrRenditionMissing
)

// Re-exported types from the internal media package.
type (
	Binding        = internalmedia.Binding
	BindingSet     = internalmedia.BindingSet
	Attachment     = internalmedia.Attachment
	Resource       = internalmedia.Resource
	ResolveOptions = internalmedia.ResolveOptions
	Service        = internalmedia.Service
	ServiceOption  = internalmedia.ServiceOption
)

// CloneBindingSet performs a deep copy of the binding set to avoid shared references.
func CloneBindingSet(src BindingSet) BindingSet {
	return internalmedia.CloneBindingSet(src)
}

// Normalize converts a resolved media asset into an Attachment DTO.
func Normalize(asset *interfaces.MediaAsset) *Attachment {
	return internalmedia.Normalize(asset)
}

// NormalizeMany converts multiple resolved assets into attachments keyed by identifier.
func NormalizeMany(assets map[string]*interfaces.MediaAsset) map[string]*Attachment {
	return internalmedia.NormalizeMany(assets)
}

// CloneAttachments performs a deep copy of a resolved media attachment map.
func CloneAttachments(src map[string][]*Attachment) map[string][]*Attachment {
	return internalmedia.CloneAttachments(src)
}

// CloneAttachment copies an attachment and its nested resources.
func CloneAttachment(att *Attachment) *Attachment {
	return internalmedia.CloneAttachment(att)
}

// WithCache configures a cache provider and default TTL for resolved attachments.
func WithCache(cache interfaces.CacheProvider, ttl time.Duration) ServiceOption {
	return internalmedia.WithCache(cache, ttl)
}

// WithDefaultSignedURLTTL sets the fallback TTL used when signed URLs are requested without an explicit expiry.
func WithDefaultSignedURLTTL(ttl time.Duration) ServiceOption {
	return internalmedia.WithDefaultSignedURLTTL(ttl)
}

// WithDefaultCacheTTL sets the fallback TTL applied when ResolveOptions do not specify one.
func WithDefaultCacheTTL(ttl time.Duration) ServiceOption {
	return internalmedia.WithDefaultCacheTTL(ttl)
}

// NewService constructs a media helper service that delegates to the configured provider.
func NewService(provider interfaces.MediaProvider, opts ...ServiceOption) Service {
	return internalmedia.NewService(provider, opts...)
}

// NewNoOpService returns a media service that always returns empty results.
func NewNoOpService() Service {
	return internalmedia.NewNoOpService()
}
