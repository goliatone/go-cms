package media_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

func TestServiceResolveBindings(t *testing.T) {
	provider := &stubProvider{
		assets: map[string]*interfaces.MediaAsset{
			"asset-1": {
				Reference: interfaces.MediaReference{ID: "asset-1"},
				Metadata:  interfaces.MediaMetadata{ID: "asset-1", MimeType: "image/png"},
				Source:    &interfaces.MediaResource{URL: "https://cdn.local/full.png"},
				Renditions: map[string]*interfaces.MediaResource{
					"thumb": {URL: "https://cdn.local/thumb.png"},
				},
			},
		},
	}

	svc := media.NewService(provider)
	bindings := media.BindingSet{
		"hero": {
			{Slot: "hero", Reference: interfaces.MediaReference{ID: "asset-1"}, Required: []string{"thumb"}},
		},
	}

	resolved, err := svc.ResolveBindings(context.Background(), bindings, media.ResolveOptions{})
	if err != nil {
		t.Fatalf("resolve bindings: %v", err)
	}
	attachments := resolved["hero"]
	if len(attachments) != 1 {
		t.Fatalf("expected one attachment got %d", len(attachments))
	}
	if attachments[0].Metadata.ID != "asset-1" {
		t.Fatalf("unexpected asset id %s", attachments[0].Metadata.ID)
	}
	if attachments[0].Renditions["thumb"].URL != "https://cdn.local/thumb.png" {
		t.Fatalf("expected thumb rendition to be populated")
	}
}

func TestServiceResolveBindingsCachesResults(t *testing.T) {
	provider := &stubProvider{
		assets: map[string]*interfaces.MediaAsset{
			"asset-1": {
				Reference: interfaces.MediaReference{ID: "asset-1"},
				Metadata:  interfaces.MediaMetadata{ID: "asset-1"},
			},
		},
	}
	cache := newMemoryCache()
	svc := media.NewService(provider, media.WithCache(cache, time.Minute))
	bindings := media.BindingSet{
		"hero": {{Slot: "hero", Reference: interfaces.MediaReference{ID: "asset-1"}}},
	}

	if _, err := svc.ResolveBindings(context.Background(), bindings, media.ResolveOptions{}); err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if _, err := svc.ResolveBindings(context.Background(), bindings, media.ResolveOptions{}); err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if provider.resolveCount("asset-1") != 1 {
		t.Fatalf("expected provider resolve once, got %d", provider.resolveCount("asset-1"))
	}
}

func TestServiceResolveBindingsMissingRendition(t *testing.T) {
	provider := &stubProvider{
		assets: map[string]*interfaces.MediaAsset{
			"asset-1": {
				Reference: interfaces.MediaReference{ID: "asset-1"},
				Metadata:  interfaces.MediaMetadata{ID: "asset-1"},
			},
		},
	}
	svc := media.NewService(provider)
	bindings := media.BindingSet{
		"hero": {{Slot: "hero", Reference: interfaces.MediaReference{ID: "asset-1"}, Required: []string{"thumb"}}},
	}

	_, err := svc.ResolveBindings(context.Background(), bindings, media.ResolveOptions{})
	if !errors.Is(err, media.ErrRenditionMissing) {
		t.Fatalf("expected ErrRenditionMissing got %v", err)
	}
}

func TestServiceResolveBindingsFallbackLocale(t *testing.T) {
	provider := &stubProvider{
		assets: map[string]*interfaces.MediaAsset{
			"asset-1:en": {
				Reference: interfaces.MediaReference{ID: "asset-1", Locale: "en"},
				Metadata:  interfaces.MediaMetadata{ID: "asset-1"},
			},
		},
	}
	svc := media.NewService(provider)
	bindings := media.BindingSet{
		"hero": {{
			Slot:           "hero",
			Reference:      interfaces.MediaReference{ID: "asset-1"},
			Locale:         "es",
			FallbackLocale: "en",
		}},
	}

	resolved, err := svc.ResolveBindings(context.Background(), bindings, media.ResolveOptions{})
	if err != nil {
		t.Fatalf("resolve with fallback: %v", err)
	}
	if resolved["hero"][0].Reference.Locale != "en" {
		t.Fatalf("expected fallback locale to resolve asset")
	}
}

func TestServiceInvalidateClearsCache(t *testing.T) {
	provider := &stubProvider{
		assets: map[string]*interfaces.MediaAsset{
			"asset-1": {Reference: interfaces.MediaReference{ID: "asset-1"}},
		},
	}
	cache := newMemoryCache()
	svc := media.NewService(provider, media.WithCache(cache, time.Minute))
	bindings := media.BindingSet{
		"hero": {{Slot: "hero", Reference: interfaces.MediaReference{ID: "asset-1"}}},
	}

	if _, err := svc.ResolveBindings(context.Background(), bindings, media.ResolveOptions{}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if err := svc.Invalidate(context.Background(), bindings); err != nil {
		t.Fatalf("invalidate: %v", err)
	}
	if cache.len() != 0 {
		t.Fatalf("expected cache to be cleared, items=%d", cache.len())
	}
	if len(provider.invalidated) != 1 {
		t.Fatalf("expected provider invalidate to run")
	}
}

type stubProvider struct {
	assets      map[string]*interfaces.MediaAsset
	resolves    map[string]int
	invalidated []interfaces.MediaReference
}

func (s *stubProvider) Resolve(_ context.Context, req interfaces.MediaResolveRequest) (*interfaces.MediaAsset, error) {
	if s.resolves == nil {
		s.resolves = make(map[string]int)
	}
	key := req.Reference.ID
	if req.Reference.Locale != "" {
		if asset, ok := s.assets[key+":"+req.Reference.Locale]; ok {
			s.resolves[key+":"+req.Reference.Locale]++
			return asset, nil
		}
	}
	asset, ok := s.assets[key]
	if ok {
		s.resolves[key]++
		return asset, nil
	}
	s.resolves["miss"]++
	return nil, nil
}

func (s *stubProvider) ResolveBatch(ctx context.Context, reqs []interfaces.MediaResolveRequest) (map[string]*interfaces.MediaAsset, error) {
	result := make(map[string]*interfaces.MediaAsset, len(reqs))
	for _, req := range reqs {
		asset, _ := s.Resolve(ctx, req)
		key := req.Reference.ID
		if req.Reference.Locale != "" {
			key += ":" + req.Reference.Locale
		}
		result[key] = asset
	}
	return result, nil
}

func (s *stubProvider) Invalidate(_ context.Context, refs ...interfaces.MediaReference) error {
	s.invalidated = append(s.invalidated, refs...)
	return nil
}

func (s *stubProvider) resolveCount(key string) int {
	if s.resolves == nil {
		return 0
	}
	return s.resolves[key]
}

type memoryCache struct {
	store map[string]any
}

func newMemoryCache() *memoryCache {
	return &memoryCache{store: make(map[string]any)}
}

func (m *memoryCache) Get(_ context.Context, key string) (any, error) {
	value, ok := m.store[key]
	if !ok {
		return nil, errors.New("miss")
	}
	return value, nil
}

func (m *memoryCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	m.store[key] = value
	return nil
}

func (m *memoryCache) Delete(_ context.Context, key string) error {
	delete(m.store, key)
	return nil
}

func (m *memoryCache) Clear(_ context.Context) error {
	m.store = make(map[string]any)
	return nil
}

func (m *memoryCache) len() int {
	return len(m.store)
}
