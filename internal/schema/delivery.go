package schema

import "context"

// DeliveryPayload captures the schema output returned to admin clients.
type DeliveryPayload struct {
	Schema   map[string]any    `json:"schema"`
	Overlays []OverlayDocument `json:"overlays,omitempty"`
	Version  string            `json:"version"`
	Metadata Metadata          `json:"metadata"`
}

// BuildDeliveryPayload normalizes a schema and bundles overlays for delivery.
func BuildDeliveryPayload(ctx context.Context, schema map[string]any, opts NormalizeOptions) (DeliveryPayload, error) {
	normalized, err := NormalizeContentSchema(ctx, schema, opts)
	if err != nil {
		return DeliveryPayload{}, err
	}
	return DeliveryPayload{
		Schema:   normalized.Schema,
		Overlays: normalized.Overlays,
		Version:  normalized.Version.String(),
		Metadata: normalized.Metadata,
	}, nil
}
