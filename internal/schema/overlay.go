package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

const overlaySchemaVersion = "x-ui-overlay/v1"

// OverlayDocument defines UI metadata overrides applied to a schema.
type OverlayDocument struct {
	Schema    string            `json:"$schema"`
	Overrides []OverlayOverride `json:"overrides"`
}

// OverlayOverride targets a schema path and applies UI metadata.
type OverlayOverride struct {
	Path     string         `json:"path"`
	XFormgen map[string]any `json:"x-formgen,omitempty"`
	XAdmin   map[string]any `json:"x-admin,omitempty"`
	UI       map[string]any `json:"ui,omitempty"`
}

// OverlayResolver resolves overlay references into documents.
type OverlayResolver interface {
	Resolve(ctx context.Context, ref string) (OverlayDocument, error)
}

// FSOverlayResolver resolves overlays from an fs.FS.
type FSOverlayResolver struct {
	FS fs.FS
}

// Resolve loads and parses an overlay document from the provided reference.
func (r FSOverlayResolver) Resolve(_ context.Context, ref string) (OverlayDocument, error) {
	if r.FS == nil {
		return OverlayDocument{}, fmt.Errorf("schema: overlay resolver fs not configured")
	}
	data, err := fs.ReadFile(r.FS, ref)
	if err != nil {
		return OverlayDocument{}, err
	}
	var doc OverlayDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return OverlayDocument{}, err
	}
	if strings.TrimSpace(doc.Schema) == "" {
		doc.Schema = overlaySchemaVersion
	}
	return doc, nil
}

func applyOverlays(schema map[string]any, overlays []OverlayDocument) (map[string]any, error) {
	if schema == nil || len(overlays) == 0 {
		return schema, nil
	}
	target := cloneMap(schema)
	for _, overlay := range overlays {
		if len(overlay.Overrides) == 0 {
			continue
		}
		for _, override := range overlay.Overrides {
			updated, err := applyOverlayOverride(target, override)
			if err != nil {
				return nil, err
			}
			target = updated
		}
	}
	return target, nil
}

func applyOverlayOverride(schema map[string]any, override OverlayOverride) (map[string]any, error) {
	if schema == nil {
		return nil, nil
	}
	path := strings.TrimSpace(override.Path)
	if path == "" {
		return schema, nil
	}
	target, err := resolvePointer(schema, path)
	if err != nil {
		return nil, err
	}
	targetMap, ok := target.(map[string]any)
	if !ok {
		return nil, ErrOverlayPathNotFound
	}
	if len(override.UI) > 0 {
		existing, _ := targetMap["x-formgen"].(map[string]any)
		targetMap["x-formgen"] = mergeMap(existing, override.UI, true)
	}
	if len(override.XFormgen) > 0 {
		existing, _ := targetMap["x-formgen"].(map[string]any)
		targetMap["x-formgen"] = mergeMap(existing, override.XFormgen, true)
	}
	if len(override.XAdmin) > 0 {
		existing, _ := targetMap["x-admin"].(map[string]any)
		targetMap["x-admin"] = mergeMap(existing, override.XAdmin, true)
	}
	return schema, nil
}
