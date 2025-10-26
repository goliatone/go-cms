package themes

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"
	"time"

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

var (
	ErrFeatureDisabled            = errors.New("themes: feature disabled")
	ErrThemeRepositoryRequired    = errors.New("themes: theme repository required")
	ErrTemplateRepositoryRequired = errors.New("themes: template repository required")

	ErrThemeNameRequired    = errors.New("themes: name required")
	ErrThemeVersionRequired = errors.New("themes: version required")
	ErrThemePathRequired    = errors.New("themes: theme path required")
	ErrThemeExists          = errors.New("themes: theme already exists")
	ErrThemeNotFound        = errors.New("themes: theme not found")

	ErrThemeActivationMissingTemplates = errors.New("themes: activation requires at least one template")
	ErrThemeActivationPathInvalid      = errors.New("themes: activation requires theme_path")
	ErrThemeWidgetAreaInvalid          = errors.New("themes: widget area missing code or name")

	ErrTemplateNotFound = errors.New("themes: template not found")
)

// IDGenerator produces unique identifiers.
type IDGenerator func() uuid.UUID

// ServiceOption configures service behaviour.
type ServiceOption func(*service)

// WithThemeIDGenerator overrides the default ID generator.
func WithThemeIDGenerator(generator IDGenerator) ServiceOption {
	return func(s *service) {
		if generator != nil {
			s.id = generator
		}
	}
}

// WithNow overrides the time source (primarily for tests).
func WithNow(now func() time.Time) ServiceOption {
	return func(s *service) {
		if now != nil {
			s.now = now
		}
	}
}

type service struct {
	themes    ThemeRepository
	templates TemplateRepository
	id        IDGenerator
	now       func() time.Time
}

// NewService constructs a theme service instance.
func NewService(themeRepo ThemeRepository, templateRepo TemplateRepository, opts ...ServiceOption) Service {
	if themeRepo == nil {
		panic(ErrThemeRepositoryRequired)
	}
	if templateRepo == nil {
		panic(ErrTemplateRepositoryRequired)
	}

	s := &service{
		themes:    themeRepo,
		templates: templateRepo,
		id:        uuid.New,
		now:       time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *service) RegisterTheme(ctx context.Context, input RegisterThemeInput) (*Theme, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, ErrThemeNameRequired
	}
	version := strings.TrimSpace(input.Version)
	if version == "" {
		return nil, ErrThemeVersionRequired
	}
	themePath := strings.TrimSpace(input.ThemePath)
	if themePath == "" {
		return nil, ErrThemePathRequired
	}

	if err := validateThemeConfig(input.Config); err != nil {
		return nil, err
	}

	if existing, err := s.themes.GetByName(ctx, name); err == nil && existing != nil {
		return nil, ErrThemeExists
	} else if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
	}

	record := &Theme{
		ID:          s.id(),
		Name:        name,
		Description: cloneString(input.Description),
		Version:     version,
		Author:      cloneString(input.Author),
		IsActive:    false,
		ThemePath:   themePath,
		Config:      cloneThemeConfig(input.Config),
		CreatedAt:   s.now().UTC(),
		UpdatedAt:   s.now().UTC(),
	}

	created, err := s.themes.Create(ctx, record)
	if err != nil {
		return nil, err
	}
	return cloneTheme(created), nil
}

func (s *service) GetTheme(ctx context.Context, id uuid.UUID) (*Theme, error) {
	if id == uuid.Nil {
		return nil, ErrThemeNotFound
	}
	theme, err := s.themes.GetByID(ctx, id)
	if err != nil {
		return nil, translateRepoError(err, ErrThemeNotFound)
	}
	return cloneTheme(theme), nil
}

func (s *service) GetThemeByName(ctx context.Context, name string) (*Theme, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrThemeNotFound
	}
	theme, err := s.themes.GetByName(ctx, name)
	if err != nil {
		return nil, translateRepoError(err, ErrThemeNotFound)
	}
	return cloneTheme(theme), nil
}

func (s *service) ListThemes(ctx context.Context) ([]*Theme, error) {
	records, err := s.themes.List(ctx)
	if err != nil {
		return nil, err
	}
	return cloneThemeSlice(records), nil
}

func (s *service) ListActiveThemes(ctx context.Context) ([]*Theme, error) {
	records, err := s.themes.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	return cloneThemeSlice(records), nil
}

func (s *service) ActivateTheme(ctx context.Context, id uuid.UUID) (*Theme, error) {
	theme, err := s.themes.GetByID(ctx, id)
	if err != nil {
		return nil, translateRepoError(err, ErrThemeNotFound)
	}

	if strings.TrimSpace(theme.ThemePath) == "" {
		return nil, ErrThemeActivationPathInvalid
	}

	templates, err := s.templates.ListByTheme(ctx, theme.ID)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, ErrThemeActivationMissingTemplates
	}
	for _, area := range theme.Config.WidgetAreas {
		if strings.TrimSpace(area.Code) == "" || strings.TrimSpace(area.Name) == "" {
			return nil, ErrThemeWidgetAreaInvalid
		}
	}

	theme.IsActive = true
	theme.UpdatedAt = s.now().UTC()
	updated, err := s.themes.Update(ctx, theme)
	if err != nil {
		return nil, err
	}
	return cloneTheme(updated), nil
}

func (s *service) DeactivateTheme(ctx context.Context, id uuid.UUID) (*Theme, error) {
	theme, err := s.themes.GetByID(ctx, id)
	if err != nil {
		return nil, translateRepoError(err, ErrThemeNotFound)
	}
	theme.IsActive = false
	theme.UpdatedAt = s.now().UTC()
	updated, err := s.themes.Update(ctx, theme)
	if err != nil {
		return nil, err
	}
	return cloneTheme(updated), nil
}

func (s *service) RegisterTemplate(ctx context.Context, input RegisterTemplateInput) (*Template, error) {
	if _, err := s.themes.GetByID(ctx, input.ThemeID); err != nil {
		return nil, translateRepoError(err, ErrThemeNotFound)
	}

	if err := ValidateRegisterTemplate(ctx, s.templates, input); err != nil {
		return nil, err
	}

	record := PrepareTemplateRecord(input, s.id)
	record.CreatedAt = s.now().UTC()
	record.UpdatedAt = s.now().UTC()

	created, err := s.templates.Create(ctx, record)
	if err != nil {
		return nil, err
	}
	return cloneTemplate(created), nil
}

func (s *service) UpdateTemplate(ctx context.Context, input UpdateTemplateInput) (*Template, error) {
	if err := ValidateUpdateTemplate(input); err != nil {
		return nil, err
	}

	template, err := s.templates.GetByID(ctx, input.TemplateID)
	if err != nil {
		return nil, translateRepoError(err, ErrTemplateNotFound)
	}

	if input.Name != nil {
		template.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		template.Description = cloneString(input.Description)
	}
	if input.TemplatePath != nil {
		template.TemplatePath = strings.TrimSpace(*input.TemplatePath)
	}
	if input.Metadata != nil {
		template.Metadata = deepCloneMap(input.Metadata)
	}
	if input.Regions != nil {
		if err := validateRegions(input.Regions); err != nil {
			return nil, err
		}
		template.Regions = cloneTemplateRegions(input.Regions)
	}
	template.UpdatedAt = s.now().UTC()

	updated, err := s.templates.Update(ctx, template)
	if err != nil {
		return nil, err
	}
	return cloneTemplate(updated), nil
}

func (s *service) DeleteTemplate(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return ErrTemplateNotFound
	}
	if err := s.templates.Delete(ctx, id); err != nil {
		return translateRepoError(err, ErrTemplateNotFound)
	}
	return nil
}

func (s *service) GetTemplate(ctx context.Context, id uuid.UUID) (*Template, error) {
	if id == uuid.Nil {
		return nil, ErrTemplateNotFound
	}
	template, err := s.templates.GetByID(ctx, id)
	if err != nil {
		return nil, translateRepoError(err, ErrTemplateNotFound)
	}
	return cloneTemplate(template), nil
}

func (s *service) ListTemplates(ctx context.Context, themeID uuid.UUID) ([]*Template, error) {
	if themeID == uuid.Nil {
		return nil, ErrThemeNotFound
	}
	records, err := s.templates.ListByTheme(ctx, themeID)
	if err != nil {
		return nil, err
	}
	return cloneTemplateSlice(records), nil
}

func (s *service) TemplateRegions(ctx context.Context, templateID uuid.UUID) ([]RegionInfo, error) {
	template, err := s.templates.GetByID(ctx, templateID)
	if err != nil {
		return nil, translateRepoError(err, ErrTemplateNotFound)
	}
	return InspectTemplateRegions(template), nil
}

func (s *service) ThemeRegionIndex(ctx context.Context, themeID uuid.UUID) (map[string][]RegionInfo, error) {
	templates, err := s.templates.ListByTheme(ctx, themeID)
	if err != nil {
		return nil, err
	}
	return InspectThemeRegions(templates), nil
}

func (s *service) ListActiveSummaries(ctx context.Context) ([]ThemeSummary, error) {
	themes, err := s.themes.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(themes, func(i, j int) bool {
		return strings.ToLower(themes[i].Name) < strings.ToLower(themes[j].Name)
	})

	summaries := make([]ThemeSummary, 0, len(themes))
	for _, theme := range themes {
		summary := ThemeSummary{
			Theme:  cloneTheme(theme),
			Assets: resolveThemeAssets(theme.Config.Assets),
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func translateRepoError(err error, fallback error) error {
	if err == nil {
		return nil
	}
	var nf *NotFoundError
	if errors.As(err, &nf) {
		return fallback
	}
	return err
}

func cloneThemeSlice(src []*Theme) []*Theme {
	if len(src) == 0 {
		return nil
	}
	out := make([]*Theme, len(src))
	for i, theme := range src {
		out[i] = cloneTheme(theme)
	}
	return out
}

func cloneTemplateSlice(src []*Template) []*Template {
	if len(src) == 0 {
		return nil
	}
	out := make([]*Template, len(src))
	for i, template := range src {
		out[i] = cloneTemplate(template)
	}
	return out
}

func resolveThemeAssets(assets *ThemeAssets) ThemeAssetsSummary {
	if assets == nil {
		return ThemeAssetsSummary{}
	}
	base := ""
	if assets.BasePath != nil {
		base = strings.TrimSpace(*assets.BasePath)
	}
	return ThemeAssetsSummary{
		Styles:  resolveAssetList(base, assets.Styles),
		Scripts: resolveAssetList(base, assets.Scripts),
		Images:  resolveAssetList(base, assets.Images),
	}
}

func resolveAssetList(base string, assets []string) []string {
	if len(assets) == 0 {
		return nil
	}
	result := make([]string, 0, len(assets))
	for _, asset := range assets {
		asset = strings.TrimSpace(asset)
		if asset == "" {
			continue
		}
		if base == "" {
			result = append(result, asset)
		} else {
			result = append(result, path.Join(base, asset))
		}
	}
	return result
}
