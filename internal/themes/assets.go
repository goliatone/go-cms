package themes

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// AssetResolver abstracts theme asset lookups for tests and production.
type AssetResolver interface {
	Open(asset string) (io.ReadCloser, error)
	ResolvePath(asset string) (string, error)
}

// FileSystemAssetResolver resolves assets from an fs.FS implementation.
type FileSystemAssetResolver struct {
	FS       fs.FS
	BasePath string
}

// Open returns a reader for the requested asset relative to BasePath.
func (r FileSystemAssetResolver) Open(asset string) (io.ReadCloser, error) {
	clean, err := r.cleanAssetPath(asset)
	if err != nil {
		return nil, err
	}
	file, err := r.FS.Open(clean)
	if err != nil {
		return nil, fmt.Errorf("themes: open asset %s: %w", asset, err)
	}
	reader, ok := file.(io.ReadCloser)
	if !ok {
		return nil, fmt.Errorf("themes: asset %s is not readable", asset)
	}
	return reader, nil
}

// ResolvePath returns the on-disk path suitable for HTTP serving.
func (r FileSystemAssetResolver) ResolvePath(asset string) (string, error) {
	clean, err := r.cleanAssetPath(asset)
	if err != nil {
		return "", err
	}
	return clean, nil
}

func (r FileSystemAssetResolver) cleanAssetPath(asset string) (string, error) {
	if r.FS == nil {
		return "", fmt.Errorf("themes: filesystem resolver not configured")
	}
	asset = strings.TrimSpace(asset)
	if asset == "" {
		return "", fmt.Errorf("themes: asset path required")
	}
	base := r.BasePath
	if strings.TrimSpace(base) == "" {
		base = "."
	}
	joined := filepath.Join(base, asset)
	clean := filepath.Clean(joined)
	baseClean := filepath.Clean(base)
	if baseClean != "." && !strings.HasPrefix(clean, baseClean) {
		return "", fmt.Errorf("themes: asset traversal detected")
	}
	return clean, nil
}
