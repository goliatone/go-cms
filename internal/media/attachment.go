package media

import (
	"maps"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// Attachment normalizes a resolved media asset for downstream consumption.
type Attachment struct {
	Reference  interfaces.MediaReference `json:"reference"`
	Metadata   interfaces.MediaMetadata  `json:"metadata"`
	Source     *Resource                 `json:"source,omitempty"`
	Renditions map[string]*Resource      `json:"renditions,omitempty"`
}

// Resource captures a concrete representation of a media asset (original or derivative).
type Resource struct {
	URL          string                `json:"url"`
	MimeType     string                `json:"mime_type,omitempty"`
	Size         int64                 `json:"size,omitempty"`
	Width        int                   `json:"width,omitempty"`
	Height       int                   `json:"height,omitempty"`
	Duration     time.Duration         `json:"duration,omitempty"`
	Hash         string                `json:"hash,omitempty"`
	SignedURL    *interfaces.SignedURL `json:"signed_url,omitempty"`
	LastModified time.Time             `json:"last_modified,omitempty"`
	Metadata     map[string]any        `json:"metadata,omitempty"`
}

// Normalize converts a resolved media asset into an Attachment DTO.
func Normalize(asset *interfaces.MediaAsset) *Attachment {
	if asset == nil {
		return nil
	}

	attachment := &Attachment{
		Reference:  asset.Reference,
		Metadata:   cloneMetadata(asset.Metadata),
		Renditions: make(map[string]*Resource, len(asset.Renditions)),
	}

	if asset.Source != nil {
		attachment.Source = normalizeResource(asset.Source)
	}

	for name, rendition := range asset.Renditions {
		attachment.Renditions[name] = normalizeResource(rendition)
	}

	return attachment
}

// NormalizeMany converts multiple resolved assets into attachments keyed by identifier.
func NormalizeMany(assets map[string]*interfaces.MediaAsset) map[string]*Attachment {
	result := make(map[string]*Attachment, len(assets))
	for key, asset := range assets {
		result[key] = Normalize(asset)
	}
	return result
}

func normalizeResource(res *interfaces.MediaResource) *Resource {
	if res == nil {
		return nil
	}
	resource := &Resource{
		URL:       res.URL,
		MimeType:  res.MimeType,
		Size:      res.Size,
		Width:     res.Width,
		Height:    res.Height,
		Hash:      res.Hash,
		SignedURL: res.SignedURL,
	}
	resource.Duration = res.Duration
	resource.LastModified = res.LastModified
	if len(res.Metadata) > 0 {
		resource.Metadata = maps.Clone(res.Metadata)
	}
	return resource
}

func cloneMetadata(meta interfaces.MediaMetadata) interfaces.MediaMetadata {
	cloned := meta
	if len(meta.Tags) > 0 {
		cloned.Tags = append([]string(nil), meta.Tags...)
	}
	if len(meta.Attributes) > 0 {
		cloned.Attributes = maps.Clone(meta.Attributes)
	}
	if len(meta.Checksums) > 0 {
		cloned.Checksums = maps.Clone(meta.Checksums)
	}
	return cloned
}
