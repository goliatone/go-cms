package themes

import (
	"context"

	"github.com/google/uuid"
)

type noopService struct{}

func NewNoOpService() Service {
	return noopService{}
}

func (noopService) RegisterTheme(context.Context, RegisterThemeInput) (*Theme, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) GetTheme(context.Context, uuid.UUID) (*Theme, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) GetThemeByName(context.Context, string) (*Theme, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) ListThemes(context.Context) ([]*Theme, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) ListActiveThemes(context.Context) ([]*Theme, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) ActivateTheme(context.Context, uuid.UUID) (*Theme, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) DeactivateTheme(context.Context, uuid.UUID) (*Theme, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) RegisterTemplate(context.Context, RegisterTemplateInput) (*Template, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) UpdateTemplate(context.Context, UpdateTemplateInput) (*Template, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) DeleteTemplate(context.Context, uuid.UUID) error {
	return ErrFeatureDisabled
}

func (noopService) GetTemplate(context.Context, uuid.UUID) (*Template, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) ListTemplates(context.Context, uuid.UUID) ([]*Template, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) TemplateRegions(context.Context, uuid.UUID) ([]RegionInfo, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) ThemeRegionIndex(context.Context, uuid.UUID) (map[string][]RegionInfo, error) {
	return nil, ErrFeatureDisabled
}

func (noopService) ListActiveSummaries(context.Context) ([]ThemeSummary, error) {
	return nil, ErrFeatureDisabled
}
