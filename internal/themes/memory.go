package themes

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// MemoryThemeRepository provides an in-memory implementation of ThemeRepository.
type MemoryThemeRepository struct {
	mu     sync.RWMutex
	byID   map[uuid.UUID]*Theme
	byName map[string]uuid.UUID
}

// NewMemoryThemeRepository constructs an empty memory-backed theme repository.
func NewMemoryThemeRepository() *MemoryThemeRepository {
	return &MemoryThemeRepository{
		byID:   make(map[uuid.UUID]*Theme),
		byName: make(map[string]uuid.UUID),
	}
}

func (r *MemoryThemeRepository) Create(_ context.Context, theme *Theme) (*Theme, error) {
	if theme == nil {
		return nil, nil
	}
	cloned := cloneTheme(theme)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.byID[cloned.ID] = cloned
	r.byName[cloned.Name] = cloned.ID

	return cloneTheme(cloned), nil
}

func (r *MemoryThemeRepository) Update(_ context.Context, theme *Theme) (*Theme, error) {
	if theme == nil {
		return nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[theme.ID]; !ok {
		return nil, &NotFoundError{Resource: "theme", Key: theme.ID.String()}
	}

	cloned := cloneTheme(theme)
	r.byID[cloned.ID] = cloned
	r.byName[cloned.Name] = cloned.ID

	return cloneTheme(cloned), nil
}

func (r *MemoryThemeRepository) GetByID(_ context.Context, id uuid.UUID) (*Theme, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.byID[id]
	if !ok {
		return nil, &NotFoundError{Resource: "theme", Key: id.String()}
	}
	return cloneTheme(record), nil
}

func (r *MemoryThemeRepository) GetByName(_ context.Context, name string) (*Theme, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byName[name]
	if !ok {
		return nil, &NotFoundError{Resource: "theme", Key: name}
	}
	return cloneTheme(r.byID[id]), nil
}

func (r *MemoryThemeRepository) List(_ context.Context) ([]*Theme, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*Theme, 0, len(r.byID))
	for _, theme := range r.byID {
		out = append(out, cloneTheme(theme))
	}
	return out, nil
}

func (r *MemoryThemeRepository) ListActive(_ context.Context) ([]*Theme, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []*Theme
	for _, theme := range r.byID {
		if theme.IsActive {
			out = append(out, cloneTheme(theme))
		}
	}
	return out, nil
}

// MemoryTemplateRepository provides an in-memory implementation of TemplateRepository.
type MemoryTemplateRepository struct {
	mu        sync.RWMutex
	byID      map[uuid.UUID]*Template
	byThemeID map[uuid.UUID]map[string]uuid.UUID // themeID -> slug -> templateID
}

// NewMemoryTemplateRepository constructs an empty memory-backed template repository.
func NewMemoryTemplateRepository() *MemoryTemplateRepository {
	return &MemoryTemplateRepository{
		byID:      make(map[uuid.UUID]*Template),
		byThemeID: make(map[uuid.UUID]map[string]uuid.UUID),
	}
}

func (r *MemoryTemplateRepository) Create(_ context.Context, template *Template) (*Template, error) {
	if template == nil {
		return nil, nil
	}

	cloned := cloneTemplate(template)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byThemeID[cloned.ThemeID]; !ok {
		r.byThemeID[cloned.ThemeID] = make(map[string]uuid.UUID)
	}
	r.byID[cloned.ID] = cloned
	r.byThemeID[cloned.ThemeID][cloned.Slug] = cloned.ID

	return cloneTemplate(cloned), nil
}

func (r *MemoryTemplateRepository) Update(_ context.Context, template *Template) (*Template, error) {
	if template == nil {
		return nil, nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.byID[template.ID]
	if !ok {
		return nil, &NotFoundError{Resource: "template", Key: template.ID.String()}
	}
	cloned := cloneTemplate(template)

	if _, ok := r.byThemeID[cloned.ThemeID]; !ok {
		r.byThemeID[cloned.ThemeID] = make(map[string]uuid.UUID)
	}
	// Remove previous slug mapping if it changed.
	if current.Slug != cloned.Slug {
		if themeTemplates, ok := r.byThemeID[current.ThemeID]; ok {
			delete(themeTemplates, current.Slug)
		}
	}
	r.byID[cloned.ID] = cloned
	r.byThemeID[cloned.ThemeID][cloned.Slug] = cloned.ID

	return cloneTemplate(cloned), nil
}

func (r *MemoryTemplateRepository) GetByID(_ context.Context, id uuid.UUID) (*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.byID[id]
	if !ok {
		return nil, &NotFoundError{Resource: "template", Key: id.String()}
	}
	return cloneTemplate(record), nil
}

func (r *MemoryTemplateRepository) GetBySlug(_ context.Context, themeID uuid.UUID, slug string) (*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	themeTemplates, ok := r.byThemeID[themeID]
	if !ok {
		return nil, &NotFoundError{Resource: "template", Key: slug}
	}
	templateID, ok := themeTemplates[slug]
	if !ok {
		return nil, &NotFoundError{Resource: "template", Key: slug}
	}
	return cloneTemplate(r.byID[templateID]), nil
}

func (r *MemoryTemplateRepository) ListByTheme(_ context.Context, themeID uuid.UUID) ([]*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	themeTemplates, ok := r.byThemeID[themeID]
	if !ok {
		return []*Template{}, nil
	}

	out := make([]*Template, 0, len(themeTemplates))
	for _, id := range themeTemplates {
		out = append(out, cloneTemplate(r.byID[id]))
	}
	return out, nil
}

func (r *MemoryTemplateRepository) ListAll(_ context.Context) ([]*Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*Template, 0, len(r.byID))
	for _, tpl := range r.byID {
		out = append(out, cloneTemplate(tpl))
	}
	return out, nil
}

func (r *MemoryTemplateRepository) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	template, ok := r.byID[id]
	if !ok {
		return &NotFoundError{Resource: "template", Key: id.String()}
	}
	delete(r.byID, id)
	if themeTemplates, ok := r.byThemeID[template.ThemeID]; ok {
		delete(themeTemplates, template.Slug)
		if len(themeTemplates) == 0 {
			delete(r.byThemeID, template.ThemeID)
		}
	}
	return nil
}

func cloneTheme(theme *Theme) *Theme {
	if theme == nil {
		return nil
	}
	cloned := *theme
	cloned.Config = ThemeConfig{
		WidgetAreas:   cloneWidgetAreas(theme.Config.WidgetAreas),
		MenuLocations: cloneMenuLocations(theme.Config.MenuLocations),
		Assets:        cloneAssets(theme.Config.Assets),
		Metadata:      deepCloneMap(theme.Config.Metadata),
	}
	cloned.Description = cloneString(theme.Description)
	cloned.Author = cloneString(theme.Author)
	if theme.Templates != nil {
		cloned.Templates = make([]*Template, len(theme.Templates))
		for i, tpl := range theme.Templates {
			cloned.Templates[i] = cloneTemplate(tpl)
		}
	}
	return &cloned
}

func cloneTemplate(tpl *Template) *Template {
	if tpl == nil {
		return nil
	}
	cloned := *tpl
	cloned.Description = cloneString(tpl.Description)
	cloned.Metadata = deepCloneMap(tpl.Metadata)
	cloned.Regions = cloneTemplateRegions(tpl.Regions)
	if tpl.Theme != nil {
		cloned.Theme = cloneTheme(tpl.Theme)
	}
	return &cloned
}

func cloneTemplateRegions(regions map[string]TemplateRegion) map[string]TemplateRegion {
	if regions == nil {
		return nil
	}
	cloned := make(map[string]TemplateRegion, len(regions))
	for key, region := range regions {
		rCopy := region
		rCopy.Description = cloneString(region.Description)
		if region.FallbackRegions != nil {
			rCopy.FallbackRegions = append([]string{}, region.FallbackRegions...)
		}
		cloned[key] = rCopy
	}
	return cloned
}

func deepCloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	cloned := make(map[string]any, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func cloneAssets(src *ThemeAssets) *ThemeAssets {
	if src == nil {
		return nil
	}
	cloned := *src
	if src.Styles != nil {
		cloned.Styles = append([]string{}, src.Styles...)
	}
	if src.Scripts != nil {
		cloned.Scripts = append([]string{}, src.Scripts...)
	}
	if src.Images != nil {
		cloned.Images = append([]string{}, src.Images...)
	}
	if src.BasePath != nil {
		cloned.BasePath = cloneString(src.BasePath)
	}
	return &cloned
}
