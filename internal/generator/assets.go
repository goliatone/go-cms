package generator

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/goliatone/go-cms/internal/themes"
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

func collectThemeAssets(theme *themes.Theme) []string {
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
