package media_test

import (
	"testing"
	"time"

	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

func TestNormalizeNilAsset(t *testing.T) {
	if media.Normalize(nil) != nil {
		t.Fatalf("expected nil attachment for nil asset")
	}
}

func TestNormalizeClonesState(t *testing.T) {
	now := time.Now().UTC()
	asset := &interfaces.MediaAsset{
		Reference: interfaces.MediaReference{
			ID:         "asset-1",
			Collection: "images",
		},
		Source: &interfaces.MediaResource{
			URL:          "https://cdn.example.com/assets/original.jpg",
			MimeType:     "image/jpeg",
			Size:         1024,
			Width:        800,
			Height:       600,
			Duration:     time.Second,
			Hash:         "checksum-original",
			SignedURL:    &interfaces.SignedURL{URL: "https://signed.example.com/original", Method: "GET", ExpiresAt: now.Add(time.Hour)},
			LastModified: now,
			Metadata: map[string]any{
				"focus": "center",
			},
		},
		Renditions: map[string]*interfaces.MediaResource{
			"thumb": {
				URL:      "https://cdn.example.com/assets/thumb.jpg",
				MimeType: "image/jpeg",
				Size:     256,
				Width:    160,
				Height:   120,
				Metadata: map[string]any{"quality": "low"},
			},
		},
		Metadata: interfaces.MediaMetadata{
			ID:       "asset-1",
			Name:     "Homepage Hero",
			MimeType: "image/jpeg",
			Size:     1024,
			Width:    800,
			Height:   600,
			Tags:     []string{"hero", "homepage"},
			Attributes: map[string]any{
				"alt": "Hero image",
			},
			Checksums: map[string]string{
				"md5": "abc123",
			},
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now,
		},
	}

	normalized := media.Normalize(asset)
	if normalized == nil {
		t.Fatalf("expected attachment, got nil")
	}
	if normalized.Reference.ID != "asset-1" {
		t.Fatalf("expected reference ID asset-1 got %s", normalized.Reference.ID)
	}
	if normalized.Source == nil {
		t.Fatalf("expected source resource to be populated")
	}
	if normalized.Source.URL != "https://cdn.example.com/assets/original.jpg" {
		t.Fatalf("incorrect source URL: %s", normalized.Source.URL)
	}
	if normalized.Renditions["thumb"] == nil {
		t.Fatalf("expected thumb rendition to exist")
	}
	if normalized.Metadata.Tags[0] != "hero" {
		t.Fatalf("expected first tag to remain hero, got %s", normalized.Metadata.Tags[0])
	}

	// Mutate original to ensure clone integrity.
	asset.Metadata.Tags[0] = "mutated"
	asset.Metadata.Attributes["alt"] = "Updated alt"
	asset.Metadata.Checksums["md5"] = "xyz789"
	asset.Source.Metadata["focus"] = "left"
	asset.Renditions["thumb"].Metadata["quality"] = "high"

	if normalized.Metadata.Tags[0] != "hero" {
		t.Fatalf("expected cloned tags to remain unchanged, got %s", normalized.Metadata.Tags[0])
	}
	if normalized.Metadata.Attributes["alt"] != "Hero image" {
		t.Fatalf("expected cloned attributes to remain unchanged")
	}
	if normalized.Metadata.Checksums["md5"] != "abc123" {
		t.Fatalf("expected cloned checksums to remain unchanged")
	}
	if normalized.Source.Metadata["focus"] != "center" {
		t.Fatalf("expected cloned source metadata to remain unchanged")
	}
	if normalized.Renditions["thumb"].Metadata["quality"] != "low" {
		t.Fatalf("expected cloned rendition metadata to remain unchanged")
	}
}
