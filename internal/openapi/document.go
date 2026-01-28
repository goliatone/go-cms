package openapi

// Document represents a minimal OpenAPI document.
type Document struct {
	OpenAPI    string         `json:"openapi"`
	Info       Info           `json:"info"`
	Paths      map[string]any `json:"paths,omitempty"`
	Components Components     `json:"components,omitempty"`
	Extensions map[string]any `json:"-"`
}

// Info captures OpenAPI metadata.
type Info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// Components aggregates schema components.
type Components struct {
	Schemas map[string]any `json:"schemas,omitempty"`
}

// NewDocument constructs a minimal OpenAPI document.
func NewDocument(title, version string) *Document {
	return &Document{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:   title,
			Version: version,
		},
		Paths:      map[string]any{},
		Components: Components{Schemas: map[string]any{}},
		Extensions: map[string]any{},
	}
}

// AddSchema registers a component schema.
func (d *Document) AddSchema(name string, schema map[string]any) {
	if d == nil || name == "" || schema == nil {
		return
	}
	if d.Components.Schemas == nil {
		d.Components.Schemas = map[string]any{}
	}
	d.Components.Schemas[name] = schema
}

// SetExtension sets a vendor extension on the document.
func (d *Document) SetExtension(key string, value any) {
	if d == nil || key == "" {
		return
	}
	if d.Extensions == nil {
		d.Extensions = map[string]any{}
	}
	d.Extensions[key] = value
}

// AsMap returns the document as a map for registry consumers.
func (d *Document) AsMap() map[string]any {
	if d == nil {
		return nil
	}
	out := map[string]any{
		"openapi": d.OpenAPI,
		"info": map[string]any{
			"title":   d.Info.Title,
			"version": d.Info.Version,
		},
	}
	if len(d.Paths) > 0 {
		out["paths"] = d.Paths
	} else {
		out["paths"] = map[string]any{}
	}
	if len(d.Components.Schemas) > 0 {
		out["components"] = map[string]any{
			"schemas": d.Components.Schemas,
		}
	}
	for key, value := range d.Extensions {
		out[key] = value
	}
	return out
}
