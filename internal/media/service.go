package media

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

var (
	// ErrProviderUnavailable reports that no upstream media provider has been configured.
	ErrProviderUnavailable = errors.New("media: provider unavailable")
	// ErrAssetNotFound indicates that the requested media asset could not be located.
	ErrAssetNotFound = errors.New("media: asset not found")
	// ErrRenditionMissing reports that a required rendition was not supplied by the provider.
	ErrRenditionMissing = errors.New("media: required rendition missing")
)

// ResolveOptions configures how media bindings should be resolved.
type ResolveOptions struct {
	IncludeSignedURLs bool
	SignedURLTTL      time.Duration
	CacheTTL          time.Duration
}

// Service resolves media bindings into normalized attachments.
type Service interface {
	ResolveBindings(ctx context.Context, bindings BindingSet, opts ResolveOptions) (map[string][]*Attachment, error)
	Invalidate(ctx context.Context, bindings BindingSet) error
}

// ServiceOption customises the media service behaviour.
type ServiceOption func(*service)

// WithCache configures a cache provider and default TTL for resolved attachments.
func WithCache(cache interfaces.CacheProvider, ttl time.Duration) ServiceOption {
	return func(s *service) {
		s.cache = cache
		s.defaultCacheTTL = ttl
	}
}

// WithDefaultSignedURLTTL sets the fallback TTL used when signed URLs are requested without an explicit expiry.
func WithDefaultSignedURLTTL(ttl time.Duration) ServiceOption {
	return func(s *service) {
		s.defaultSignedURLTTL = ttl
	}
}

// WithDefaultCacheTTL sets the fallback TTL applied when ResolveOptions do not specify one.
func WithDefaultCacheTTL(ttl time.Duration) ServiceOption {
	return func(s *service) {
		s.defaultCacheTTL = ttl
	}
}

type service struct {
	provider            interfaces.MediaProvider
	cache               interfaces.CacheProvider
	defaultCacheTTL     time.Duration
	defaultSignedURLTTL time.Duration
}

// NewService constructs a media helper service that delegates to the configured provider.
func NewService(provider interfaces.MediaProvider, opts ...ServiceOption) Service {
	s := &service{
		provider:            provider,
		defaultCacheTTL:     5 * time.Minute,
		defaultSignedURLTTL: 15 * time.Minute,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ResolveBindings resolves the supplied bindings into normalized attachments.
func (s *service) ResolveBindings(ctx context.Context, bindings BindingSet, opts ResolveOptions) (map[string][]*Attachment, error) {
	if s.provider == nil {
		return nil, ErrProviderUnavailable
	}
	if len(bindings) == 0 {
		return map[string][]*Attachment{}, nil
	}
	resolved := make(map[string][]*Attachment, len(bindings))
	for key, list := range bindings {
		if len(list) == 0 {
			resolved[key] = nil
			continue
		}
		attachments := make([]*Attachment, 0, len(list))
		for _, binding := range list {
			attachment, err := s.resolveBinding(ctx, binding, opts)
			if err != nil {
				return nil, fmt.Errorf("media slot %s: %w", key, err)
			}
			attachments = append(attachments, attachment)
		}
		resolved[key] = attachments
	}
	return resolved, nil
}

// Invalidate evicts cached lookups and instructs the provider to refresh downstream entries.
func (s *service) Invalidate(ctx context.Context, bindings BindingSet) error {
	if s.provider == nil {
		return ErrProviderUnavailable
	}
	var refs []interfaces.MediaReference
	for _, list := range bindings {
		for _, binding := range list {
			ref := binding.Reference
			if binding.Locale != "" {
				ref.Locale = binding.Locale
			}
			refs = append(refs, ref)
			if s.cache != nil {
				_ = s.cache.Delete(ctx, cacheKey(binding))
			}
		}
	}
	if len(refs) == 0 {
		return nil
	}
	return s.provider.Invalidate(ctx, refs...)
}

func (s *service) resolveBinding(ctx context.Context, binding Binding, opts ResolveOptions) (*Attachment, error) {
	primary := binding
	if binding.Locale != "" {
		primary.Reference.Locale = binding.Locale
	}
	attachment, err := s.resolveWithContext(ctx, primary, opts)
	if err == nil {
		return attachment, nil
	}
	if errors.Is(err, ErrAssetNotFound) && binding.FallbackLocale != "" && binding.FallbackLocale != binding.Locale {
		fallback := binding
		fallback.Reference.Locale = binding.FallbackLocale
		fallback.Locale = binding.FallbackLocale
		fallback.FallbackLocale = ""
		return s.resolveWithContext(ctx, fallback, opts)
	}
	return nil, err
}

func (s *service) resolveWithContext(ctx context.Context, binding Binding, opts ResolveOptions) (*Attachment, error) {
	useCache := s.cache != nil && !opts.IncludeSignedURLs
	var cachedKey string
	if useCache {
		cachedKey = cacheKey(binding)
		if cached, err := s.cache.Get(ctx, cachedKey); err == nil {
			if attachment, ok := cached.(*Attachment); ok && attachment != nil {
				return attachment, nil
			}
		}
	}

	req := interfaces.MediaResolveRequest{
		Reference:         binding.Reference,
		Renditions:        unionRenditions(binding),
		IncludeSource:     true,
		IncludeSignedURLs: opts.IncludeSignedURLs,
		SignedURLTTL:      s.signedURLTTL(opts),
		Purpose:           binding.Slot,
		Context: map[string]string{
			"slot": binding.Slot,
		},
	}
	if binding.Locale != "" {
		req.Reference.Locale = binding.Locale
	}

	asset, err := s.provider.Resolve(ctx, req)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, ErrAssetNotFound
	}
	attachment := Normalize(asset)
	if attachment == nil {
		return nil, ErrAssetNotFound
	}
	for _, name := range binding.Required {
		if attachment.Renditions == nil || attachment.Renditions[name] == nil {
			return nil, fmt.Errorf("%w: %s", ErrRenditionMissing, name)
		}
	}

	if useCache {
		if ttl := s.cacheTTL(opts); ttl > 0 {
			_ = s.cache.Set(ctx, cachedKey, attachment, ttl)
		}
	}

	return attachment, nil
}

func (s *service) cacheTTL(opts ResolveOptions) time.Duration {
	if opts.CacheTTL > 0 {
		return opts.CacheTTL
	}
	return s.defaultCacheTTL
}

func (s *service) signedURLTTL(opts ResolveOptions) time.Duration {
	if opts.SignedURLTTL > 0 {
		return opts.SignedURLTTL
	}
	return s.defaultSignedURLTTL
}

func cacheKey(binding Binding) string {
	ref := binding.Reference
	base := strings.TrimSpace(ref.ID)
	if base == "" {
		base = strings.TrimSpace(ref.Path)
	}
	if base == "" {
		base = strings.TrimSpace(ref.Collection)
	}
	parts := []string{base}
	if binding.Slot != "" {
		parts = append(parts, "slot="+binding.Slot)
	}
	locale := binding.Locale
	if locale == "" {
		locale = ref.Locale
	}
	if locale != "" {
		parts = append(parts, "locale="+locale)
	}
	if len(binding.Renditions) > 0 {
		r := append([]string(nil), binding.Renditions...)
		sort.Strings(r)
		parts = append(parts, "rend="+strings.Join(r, ","))
	}
	if len(binding.Required) > 0 {
		req := append([]string(nil), binding.Required...)
		sort.Strings(req)
		parts = append(parts, "req="+strings.Join(req, ","))
	}
	return strings.Join(parts, "|")
}

func unionRenditions(binding Binding) []string {
	set := make(map[string]struct{})
	for _, item := range binding.Renditions {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	for _, item := range binding.Required {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

type noopService struct{}

// NewNoOpService returns a media service that acts as a passthrough for disabled configurations.
func NewNoOpService() Service { return noopService{} }

func (noopService) ResolveBindings(context.Context, BindingSet, ResolveOptions) (map[string][]*Attachment, error) {
	return map[string][]*Attachment{}, nil
}

func (noopService) Invalidate(context.Context, BindingSet) error {
	return nil
}
