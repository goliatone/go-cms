package themes

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// Service exposes theme management capabilities.
type Service interface {
	RegisterTheme(ctx context.Context, input RegisterThemeInput) (*Theme, error)
	GetTheme(ctx context.Context, id uuid.UUID) (*Theme, error)
	GetThemeByName(ctx context.Context, name string) (*Theme, error)
	ListThemes(ctx context.Context) ([]*Theme, error)
	ListActiveThemes(ctx context.Context) ([]*Theme, error)

	ActivateTheme(ctx context.Context, id uuid.UUID) (*Theme, error)
	DeactivateTheme(ctx context.Context, id uuid.UUID) (*Theme, error)

	RegisterTemplate(ctx context.Context, input RegisterTemplateInput) (*Template, error)
	UpdateTemplate(ctx context.Context, input UpdateTemplateInput) (*Template, error)
	DeleteTemplate(ctx context.Context, id uuid.UUID) error
	GetTemplate(ctx context.Context, id uuid.UUID) (*Template, error)
	ListTemplates(ctx context.Context, themeID uuid.UUID) ([]*Template, error)

	TemplateRegions(ctx context.Context, templateID uuid.UUID) ([]RegionInfo, error)
	ThemeRegionIndex(ctx context.Context, themeID uuid.UUID) (map[string][]RegionInfo, error)

	ListActiveSummaries(ctx context.Context) ([]ThemeSummary, error)
}

// ThemeSummary aggregates a theme with resolved asset paths.
type ThemeSummary struct {
	Theme  *Theme
	Assets ThemeAssetsSummary
}

// ThemeAssetsSummary lists resolved static assets per theme.
type ThemeAssetsSummary struct {
	Styles  []string
	Scripts []string
	Images  []string
}

type RegisterThemeInput struct {
	Name        string
	Description *string
	Version     string
	Author      *string
	ThemePath   string
	Config      ThemeConfig
	Activate    bool
}

type RegisterTemplateInput struct {
	ThemeID      uuid.UUID
	Name         string
	Slug         string
	Description  *string
	TemplatePath string
	Regions      map[string]TemplateRegion
	Metadata     map[string]any
}

type UpdateTemplateInput struct {
	TemplateID   uuid.UUID
	Name         *string
	Description  *string
	TemplatePath *string
	Regions      map[string]TemplateRegion
	Metadata     map[string]any
}

var (
	// ErrTemplateThemeRequired indicates the theme ID is missing.
	ErrTemplateThemeRequired = errors.New("themes: theme id required")
	// ErrTemplateNameRequired indicates the template name is missing.
	ErrTemplateNameRequired = errors.New("themes: template name required")
	// ErrTemplateSlugRequired indicates the slug is missing.
	ErrTemplateSlugRequired = errors.New("themes: template slug required")
	// ErrTemplatePathRequired indicates the file path is missing.
	ErrTemplatePathRequired = errors.New("themes: template path required")
	// ErrTemplateSlugConflict indicates a duplicate slug within a theme.
	ErrTemplateSlugConflict = errors.New("themes: template slug already exists for theme")
	// ErrTemplateRegionsInvalid indicates malformed region metadata.
	ErrTemplateRegionsInvalid = errors.New("themes: template regions invalid")
)

// RegionInfo summarises template region capabilities for consumers.
type RegionInfo struct {
	Key            string
	Name           string
	AcceptsBlocks  bool
	AcceptsWidgets bool
	Fallbacks      []string
}
