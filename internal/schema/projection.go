package schema

import (
	"fmt"
	"strings"

	"github.com/goliatone/go-cms/internal/openapi"
	"github.com/goliatone/go-slug"
)

// BlockSchema captures a named block schema for projection.
type BlockSchema struct {
	Name   string
	Schema map[string]any
}

// Projection contains an OpenAPI document projection.
type Projection struct {
	Name     string
	Document *openapi.Document
}

// ProjectToOpenAPI builds an OpenAPI document for the content type and blocks.
func ProjectToOpenAPI(contentSlug string, contentName string, schema map[string]any, version Version, blocks []BlockSchema) (*Projection, error) {
	slugValue := strings.TrimSpace(contentSlug)
	if slugValue == "" {
		return nil, fmt.Errorf("schema: content slug required for projection")
	}
	title := strings.TrimSpace(contentName)
	if title == "" {
		title = slugValue
	}
	doc := openapi.NewDocument(title, strings.TrimPrefix(version.SemVer, "v"))
	doc.AddSchema(componentName(slugValue), cloneMap(schema))
	for _, block := range blocks {
		if block.Schema == nil || strings.TrimSpace(block.Name) == "" {
			continue
		}
		doc.AddSchema(componentName(block.Name), cloneMap(block.Schema))
	}
	doc.SetExtension("x-cms", map[string]any{
		"content_type": slugValue,
		"schema":       version.String(),
	})
	return &Projection{
		Name:     slugValue,
		Document: doc,
	}, nil
}

func componentName(value string) string {
	normalized, err := slug.Normalize(value)
	if err != nil || normalized == "" {
		normalized = value
	}
	return strings.ReplaceAll(normalized, "-", "_")
}
