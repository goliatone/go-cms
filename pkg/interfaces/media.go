package interfaces

import (
	"context"
	"time"
)

// MediaProvider supplies metadata and access details for externally managed assets.
type MediaProvider interface {
	// Resolve fetches a single media asset using the supplied request parameters.
	Resolve(ctx context.Context, req MediaResolveRequest) (*MediaAsset, error)
	// ResolveBatch fetches multiple media assets. Implementations should de-duplicate references internally.
	ResolveBatch(ctx context.Context, reqs []MediaResolveRequest) (map[string]*MediaAsset, error)
	// Invalidate clears cached lookups for the provided references, allowing subsequent resolve calls to refresh state.
	Invalidate(ctx context.Context, refs ...MediaReference) error
}

// MediaReference uniquely identifies a media asset within the provider.
type MediaReference struct {
	ID         string
	Path       string
	Collection string
	Locale     string
	Variant    string
	Attributes map[string]string
}

// MediaResolveRequest controls which parts of an asset should be resolved.
type MediaResolveRequest struct {
	Reference         MediaReference
	Renditions        []string
	IncludeSource     bool
	IncludeSignedURLs bool
	SignedURLTTL      time.Duration
	Purpose           string
	Context           map[string]string
}

// MediaAsset encapsulates metadata and resolved resources for a media item.
type MediaAsset struct {
	Reference  MediaReference
	Source     *MediaResource
	Renditions map[string]*MediaResource
	Metadata   MediaMetadata
}

// MediaResource describes a concrete file representation (original or derivative).
type MediaResource struct {
	URL          string
	SignedURL    *SignedURL
	MimeType     string
	Size         int64
	Width        int
	Height       int
	Duration     time.Duration
	Hash         string
	LastModified time.Time
	Metadata     map[string]any
}

// MediaMetadata captures descriptive properties of the media asset.
type MediaMetadata struct {
	ID          string
	Name        string
	Description string
	MimeType    string
	Size        int64
	Width       int
	Height      int
	Duration    time.Duration
	AltText     string
	Caption     string
	Tags        []string
	Attributes  map[string]any
	Checksums   map[string]string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   string
	UpdatedBy   string
}

// SignedURL represents an expiring URL that grants temporary access to a media resource.
type SignedURL struct {
	URL       string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}
