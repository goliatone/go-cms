package widgets

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrFeatureDisabled indicates the widget feature is disabled via configuration.
var ErrFeatureDisabled = errors.New("widgets: feature disabled")

type noOpService struct{}

// NewNoOpService returns a Service implementation that always reports the feature as disabled.
func NewNoOpService() Service {
	return noOpService{}
}

func (noOpService) RegisterDefinition(context.Context, RegisterDefinitionInput) (*Definition, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) GetDefinition(context.Context, uuid.UUID) (*Definition, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) ListDefinitions(context.Context) ([]*Definition, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) CreateInstance(context.Context, CreateInstanceInput) (*Instance, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) UpdateInstance(context.Context, UpdateInstanceInput) (*Instance, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) GetInstance(context.Context, uuid.UUID) (*Instance, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) ListInstancesByDefinition(context.Context, uuid.UUID) ([]*Instance, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) ListInstancesByArea(context.Context, string) ([]*Instance, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) ListAllInstances(context.Context) ([]*Instance, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) AddTranslation(context.Context, AddTranslationInput) (*Translation, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) UpdateTranslation(context.Context, UpdateTranslationInput) (*Translation, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) GetTranslation(context.Context, uuid.UUID, uuid.UUID) (*Translation, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) RegisterAreaDefinition(context.Context, RegisterAreaDefinitionInput) (*AreaDefinition, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) ListAreaDefinitions(context.Context) ([]*AreaDefinition, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) AssignWidgetToArea(context.Context, AssignWidgetToAreaInput) ([]*AreaPlacement, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) RemoveWidgetFromArea(context.Context, RemoveWidgetFromAreaInput) error {
	return ErrFeatureDisabled
}

func (noOpService) ReorderAreaWidgets(context.Context, ReorderAreaWidgetsInput) ([]*AreaPlacement, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) ResolveArea(context.Context, ResolveAreaInput) ([]*ResolvedWidget, error) {
	return nil, ErrFeatureDisabled
}

func (noOpService) EvaluateVisibility(context.Context, *Instance, VisibilityContext) (bool, error) {
	return false, ErrFeatureDisabled
}
