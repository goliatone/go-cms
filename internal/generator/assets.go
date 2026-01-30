package generator

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/goliatone/go-cms/internal/themes"
	gotheme "github.com/goliatone/go-theme"
)

// AssetResolver resolves theme assets for copying into static outputs.
type AssetResolver interface {
	Open(ctx context.Context, theme *themes.Theme, asset string) (io.ReadCloser, error)
	ResolvePath(theme *themes.Theme, asset string) (string, error)
}

// NoOpAssetResolver skips asset resolution.
type NoOpAssetResolver struct{}

func (NoOpAssetResolver) Open(context.Context, *themes.Theme, string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("generator: asset resolver not configured")
}

func (NoOpAssetResolver) ResolvePath(*themes.Theme, string) (string, error) {
	return "", fmt.Errorf("generator: asset resolver not configured")
}

func collectThemeAssets(theme *themes.Theme, selection *gotheme.Selection) []string {
	if selection != nil && selection.Manifest != nil {
		assets := collectManifestAssets(selection)
		if len(assets) > 0 {
			return assets
		}
	}
	if theme == nil || theme.Config.Assets == nil {
		return nil
	}

	var assets []string
	base := ""
	if theme.Config.Assets.BasePath != nil {
		base = strings.TrimSpace(*theme.Config.Assets.BasePath)
	}

	appendAssets := func(list []string) {
		for _, item := range list {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if base != "" {
				assets = append(assets, path.Join(base, filepath.ToSlash(item)))
			} else {
				assets = append(assets, filepath.ToSlash(item))
			}
		}
	}

	appendAssets(theme.Config.Assets.Styles)
	appendAssets(theme.Config.Assets.Scripts)
	appendAssets(theme.Config.Assets.Images)

	return assets
}

func collectManifestAssets(selection *gotheme.Selection) []string {
	if selection == nil || selection.Manifest == nil {
		return nil
	}

	assets := selection.Manifest.Assets.Files
	if variant := strings.TrimSpace(selection.Variant); variant != "" {
		if v, ok := selection.Manifest.Variants[variant]; ok && len(v.Assets.Files) > 0 {
			merged := make(map[string]string, len(selection.Manifest.Assets.Files)+len(v.Assets.Files))
			for key, path := range selection.Manifest.Assets.Files {
				merged[key] = path
			}
			for key, path := range v.Assets.Files {
				merged[key] = path
			}
			assets = merged
		}
	}

	seen := map[string]struct{}{}
	var out []string
	for _, asset := range assets {
		asset = strings.TrimPrefix(strings.TrimSpace(asset), "/")
		if asset == "" {
			continue
		}
		if _, ok := seen[asset]; ok {
			continue
		}
		seen[asset] = struct{}{}
		out = append(out, filepath.ToSlash(asset))
	}
	return out
}

func detectAssetContentType(asset string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(asset), "."))
	switch ext {
	case "css":
		return "text/css"
	case "js":
		return "application/javascript"
	case "json":
		return "application/json"
	case "svg":
		return "image/svg+xml"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}
