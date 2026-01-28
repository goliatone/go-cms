package schema

import (
	"context"
	"fmt"
	"strings"
)

// NormalizeOptions configures schema normalization.
type NormalizeOptions struct {
	Slug              string
	OverlayResolver   OverlayResolver
	OverlayDocuments  []OverlayDocument
	FailOnUnsupported bool
}

// NormalizedSchema captures the normalized schema and associated metadata.
type NormalizedSchema struct {
	Schema   map[string]any
	Metadata Metadata
	Overlays []OverlayDocument
	Version  Version
}

// NormalizeContentSchema normalizes a content type schema and merges UI overlays.
func NormalizeContentSchema(ctx context.Context, schema map[string]any, opts NormalizeOptions) (NormalizedSchema, error) {
	if schema == nil {
		return NormalizedSchema{}, fmt.Errorf("schema: content schema required")
	}
	working := cloneMap(schema)
	meta := ExtractMetadata(working)
	if opts.Slug != "" && meta.Slug == "" {
		meta.Slug = strings.TrimSpace(opts.Slug)
	}
	normalizedSchema, version, err := EnsureSchemaVersion(working, meta.Slug)
	if err != nil {
		return NormalizedSchema{}, err
	}
	meta = ExtractMetadata(normalizedSchema)

	if opts.FailOnUnsupported {
		if err := ValidateSchemaSubset(normalizedSchema); err != nil {
			return NormalizedSchema{}, err
		}
	}

	normalizedSchema = normalizeUIMetadata(normalizedSchema)

	overlays, err := resolveOverlays(ctx, meta.UIOverlays, opts)
	if err != nil {
		return NormalizedSchema{}, err
	}
	if len(overlays) > 0 {
		normalizedSchema, err = applyOverlays(normalizedSchema, overlays)
		if err != nil {
			return NormalizedSchema{}, err
		}
	}

	return NormalizedSchema{
		Schema:   normalizedSchema,
		Metadata: meta,
		Overlays: overlays,
		Version:  version,
	}, nil
}

func resolveOverlays(ctx context.Context, refs []string, opts NormalizeOptions) ([]OverlayDocument, error) {
	if len(opts.OverlayDocuments) > 0 {
		return opts.OverlayDocuments, nil
	}
	if opts.OverlayResolver == nil || len(refs) == 0 {
		return nil, nil
	}
	overlays := make([]OverlayDocument, 0, len(refs))
	for _, ref := range refs {
		doc, err := opts.OverlayResolver.Resolve(ctx, ref)
		if err != nil {
			return nil, err
		}
		overlays = append(overlays, doc)
	}
	return overlays, nil
}

func normalizeUIMetadata(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	normalizeNode(schema)
	return schema
}

func normalizeNode(node map[string]any) {
	if node == nil {
		return
	}
	if ui, ok := node["ui"].(map[string]any); ok {
		existing, _ := node["x-formgen"].(map[string]any)
		node["x-formgen"] = mergeMap(existing, ui, false)
		delete(node, "ui")
	}
	if props, ok := node["properties"].(map[string]any); ok {
		for _, value := range props {
			if child, ok := value.(map[string]any); ok {
				normalizeNode(child)
			}
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		normalizeNode(items)
	}
	if oneOf, ok := node["oneOf"].([]any); ok {
		for _, entry := range oneOf {
			if child, ok := entry.(map[string]any); ok {
				normalizeNode(child)
			}
		}
	}
	if allOf, ok := node["allOf"].([]any); ok {
		for _, entry := range allOf {
			if child, ok := entry.(map[string]any); ok {
				normalizeNode(child)
			}
		}
	}
	if defs, ok := node["$defs"].(map[string]any); ok {
		for _, value := range defs {
			if child, ok := value.(map[string]any); ok {
				normalizeNode(child)
			}
		}
	}
}
