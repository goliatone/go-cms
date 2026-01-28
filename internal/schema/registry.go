package schema

import "context"

// Registry represents a destination for OpenAPI schema documents.
type Registry interface {
	Register(ctx context.Context, name string, doc map[string]any) error
}

// RegisterProjections registers projections in the provided registry.
func RegisterProjections(ctx context.Context, registry Registry, projections []*Projection) error {
	if registry == nil || len(projections) == 0 {
		return nil
	}
	for _, projection := range projections {
		if projection == nil || projection.Document == nil {
			continue
		}
		if err := registry.Register(ctx, projection.Name, projection.Document.AsMap()); err != nil {
			return err
		}
	}
	return nil
}
