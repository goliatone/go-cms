package widgets

import (
	"context"
	"errors"
)

// BootstrapConfig bundles the registration payloads applied during startup.
type BootstrapConfig struct {
	Definitions []RegisterDefinitionInput
	Areas       []RegisterAreaDefinitionInput
}

// Bootstrap registers widget definitions and areas, tolerating duplicates.
func Bootstrap(ctx context.Context, svc Service, cfg BootstrapConfig) error {
	if err := EnsureDefinitions(ctx, svc, cfg.Definitions); err != nil {
		return err
	}
	return EnsureAreaDefinitions(ctx, svc, cfg.Areas)
}

// EnsureDefinitions idempotently registers widget definitions with the provided service.
func EnsureDefinitions(ctx context.Context, svc Service, definitions []RegisterDefinitionInput) error {
	if svc == nil {
		return nil
	}

	for _, definition := range definitions {
		if definition.Name == "" {
			continue
		}
		if _, err := svc.RegisterDefinition(ctx, definition); err != nil {
			if errors.Is(err, ErrDefinitionExists) || errors.Is(err, ErrFeatureDisabled) {
				continue
			}
			return err
		}
	}
	return nil
}

// EnsureAreaDefinitions idempotently registers widget area definitions.
func EnsureAreaDefinitions(ctx context.Context, svc Service, areas []RegisterAreaDefinitionInput) error {
	if svc == nil {
		return nil
	}
	for _, area := range areas {
		if area.Code == "" {
			continue
		}
		if _, err := svc.RegisterAreaDefinition(ctx, area); err != nil {
			if errors.Is(err, ErrAreaDefinitionExists) ||
				errors.Is(err, ErrFeatureDisabled) ||
				errors.Is(err, ErrAreaFeatureDisabled) {
				continue
			}
			return err
		}
	}
	return nil
}
