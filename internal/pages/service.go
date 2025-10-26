package pages

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/google/uuid"
)

// Service describes page management capabilities.
type Service interface {
	Create(ctx context.Context, req CreatePageRequest) (*Page, error)
	Get(ctx context.Context, id uuid.UUID) (*Page, error)
	List(ctx context.Context) ([]*Page, error)
}

// CreatePageRequest captures the payload required to create a page.
type CreatePageRequest struct {
	ContentID    uuid.UUID
	TemplateID   uuid.UUID
	ParentID     *uuid.UUID
	Slug         string
	Status       string
	CreatedBy    uuid.UUID
	UpdatedBy    uuid.UUID
	Translations []PageTranslationInput
}

// PageTranslationInput represents localized routing information.
type PageTranslationInput struct {
	Locale  string
	Title   string
	Path    string
	Summary *string
}

var (
	ErrContentRequired    = errors.New("pages: content does not exist")
	ErrTemplateRequired   = errors.New("pages: template is required")
	ErrSlugRequired       = errors.New("pages: slug is required")
	ErrSlugInvalid        = errors.New("pages: slug contains invalid characters")
	ErrSlugExists         = errors.New("pages: slug already exists")
	ErrPathExists         = errors.New("pages: translation path already exists")
	ErrUnknownLocale      = errors.New("pages: unknown locale")
	ErrDuplicateLocale    = errors.New("pages: duplicate locale provided")
	ErrParentNotFound     = errors.New("pages: parent page not found")
	ErrNoPageTranslations = errors.New("pages: at least one translation is required")
	ErrTemplateUnknown    = errors.New("pages: template not found")
)

// PageRepository abstracts storage operations for pages.
type PageRepository interface {
	Create(ctx context.Context, record *Page) (*Page, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Page, error)
	GetBySlug(ctx context.Context, slug string) (*Page, error)
	List(ctx context.Context) ([]*Page, error)
	CreateVersion(ctx context.Context, version *PageVersion) (*PageVersion, error)
	ListVersions(ctx context.Context, pageID uuid.UUID) ([]*PageVersion, error)
	GetVersion(ctx context.Context, pageID uuid.UUID, number int) (*PageVersion, error)
	GetLatestVersion(ctx context.Context, pageID uuid.UUID) (*PageVersion, error)
}

// LocaleRepository resolves locales.
type LocaleRepository interface {
	GetByCode(ctx context.Context, code string) (*content.Locale, error)
}

// ContentRepository allows lookups for existing content.
type ContentRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*content.Content, error)
}

// PageNotFoundError indicates missing page records.
type PageNotFoundError struct {
	Key string
}

func (e *PageNotFoundError) Error() string {
	return fmt.Sprintf("page %q not found", e.Key)
}

type PageVersionNotFoundError struct {
	PageID  uuid.UUID
	Version int
}

func (e *PageVersionNotFoundError) Error() string {
	if e.PageID == uuid.Nil {
		return "page version not found"
	}
	if e.Version > 0 {
		return fmt.Sprintf("page version %s@%d not found", e.PageID.String(), e.Version)
	}
	return fmt.Sprintf("page version %s not found", e.PageID.String())
}

// ServiceOption mutates the service configuration.
type ServiceOption func(*pageService)

// WithPageClock overrides the internal clock.
func WithPageClock(clock func() time.Time) ServiceOption {
	return func(s *pageService) {
		s.now = clock
	}
}

// IDGenerator5 produces unique identifier for page entiteis
type IDGenerator func() uuid.UUID

func WithIDGenerator(generator IDGenerator) ServiceOption {
	return func(ps *pageService) {
		if generator != nil {
			ps.id = generator
		}
	}
}

type pageService struct {
	pages   PageRepository
	content ContentRepository
	locales LocaleRepository
	blocks  blocks.Service
	widgets widgets.Service
	themes  themes.Service
	now     func() time.Time
	id      IDGenerator
}

func WithBlockService(service blocks.Service) ServiceOption {
	return func(ps *pageService) {
		ps.blocks = service
	}
}

func WithWidgetService(svc widgets.Service) ServiceOption {
	return func(ps *pageService) {
		ps.widgets = svc
	}
}

func WithThemeService(svc themes.Service) ServiceOption {
	return func(ps *pageService) {
		ps.themes = svc
	}
}

// NewService constructs a page service with the required dependencies.
func NewService(pages PageRepository, contentRepo ContentRepository, locales LocaleRepository, opts ...ServiceOption) Service {
	s := &pageService{
		pages:   pages,
		content: contentRepo,
		locales: locales,
		now:     time.Now,
		id:      uuid.New,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Create registers a new page with translations and hierarchy rules.
func (s *pageService) Create(ctx context.Context, req CreatePageRequest) (*Page, error) {
	if (req.ContentID == uuid.UUID{}) {
		return nil, ErrContentRequired
	}

	if (req.TemplateID == uuid.UUID{}) {
		return nil, ErrTemplateRequired
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		return nil, ErrSlugRequired
	}
	if !isValidSlug(slug) {
		return nil, ErrSlugInvalid
	}

	if len(req.Translations) == 0 {
		return nil, ErrNoPageTranslations
	}

	if _, err := s.content.GetByID(ctx, req.ContentID); err != nil {
		return nil, ErrContentRequired
	}

	if s.themes != nil {
		if _, err := s.themes.GetTemplate(ctx, req.TemplateID); err != nil {
			if errors.Is(err, themes.ErrFeatureDisabled) {
				// feature disabled, skip template validation
			} else if errors.Is(err, themes.ErrTemplateNotFound) {
				return nil, ErrTemplateUnknown
			} else {
				return nil, err
			}
		}
	}

	if existing, err := s.pages.GetBySlug(ctx, slug); err == nil && existing != nil {
		return nil, ErrSlugExists
	} else if err != nil {
		var notFound *PageNotFoundError
		if !errors.As(err, &notFound) {
			return nil, err
		}
	}

	now := s.now()
	page := &Page{
		ID:           s.id(),
		ContentID:    req.ContentID,
		ParentID:     req.ParentID,
		TemplateID:   req.TemplateID,
		Slug:         slug,
		Status:       chooseStatus(req.Status),
		CreatedBy:    req.CreatedBy,
		UpdatedBy:    req.UpdatedBy,
		CreatedAt:    now,
		UpdatedAt:    now,
		Translations: []*PageTranslation{},
	}

	if req.ParentID != nil {
		if _, err := s.pages.GetByID(ctx, *req.ParentID); err != nil {
			return nil, ErrParentNotFound
		}
	}

	existingPages, err := s.pages.List(ctx)
	if err != nil {
		return nil, err
	}

	seenLocales := map[string]struct{}{}
	for _, tr := range req.Translations {
		code := strings.TrimSpace(tr.Locale)
		if code == "" {
			return nil, ErrUnknownLocale
		}
		if _, ok := seenLocales[code]; ok {
			return nil, ErrDuplicateLocale
		}

		locale, err := s.locales.GetByCode(ctx, code)
		if err != nil {
			return nil, ErrUnknownLocale
		}

		path := sanitizePath(tr.Path)
		if path == "" {
			return nil, ErrPathExists
		}
		if pathExists(existingPages, locale.ID, path) {
			return nil, ErrPathExists
		}

		translation := &PageTranslation{
			ID:        s.id(),
			PageID:    page.ID,
			LocaleID:  locale.ID,
			Title:     tr.Title,
			Path:      path,
			Summary:   tr.Summary,
			CreatedAt: now,
			UpdatedAt: now,
		}

		page.Translations = append(page.Translations, translation)
		seenLocales[code] = struct{}{}
	}

	created, err := s.pages.Create(ctx, page)
	if err != nil {
		return nil, err
	}

	return created, nil
}

// Get fetches a page by identifier.
func (s *pageService) Get(ctx context.Context, id uuid.UUID) (*Page, error) {
	page, err := s.pages.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	enriched, err := s.enrichPages(ctx, []*Page{page})
	if err != nil {
		return nil, err
	}
	return enriched[0], nil
}

// List returns all pages.
func (s *pageService) List(ctx context.Context) ([]*Page, error) {
	pages, err := s.pages.List(ctx)
	if err != nil {
		return nil, err
	}
	return s.enrichPages(ctx, pages)
}

func (s *pageService) enrichPages(ctx context.Context, pages []*Page) ([]*Page, error) {
	withBlocks, err := s.attachBlocks(ctx, pages)
	if err != nil {
		return nil, err
	}
	return s.attachWidgets(ctx, withBlocks)
}

func (s *pageService) attachBlocks(ctx context.Context, pages []*Page) ([]*Page, error) {
	if s.blocks == nil || len(pages) == 0 {
		return pages, nil
	}

	global := []*blocks.Instance{}
	if inst, err := s.blocks.ListGlobalInstances(ctx); err == nil {
		global = inst
	} else {
		var nf *blocks.NotFoundError
		if !errors.As(err, &nf) {
			return nil, err
		}
	}

	enriched := make([]*Page, 0, len(pages))
	for _, page := range pages {
		if page == nil {
			enriched = append(enriched, nil)
			continue
		}
		clone := *page
		pageBlocks, err := s.blocks.ListPageInstances(ctx, page.ID)
		if err != nil {
			var nf *blocks.NotFoundError
			if !errors.As(err, &nf) {
				return nil, err
			}
			pageBlocks = nil
		}
		combined := append(cloneBlockInstances(pageBlocks), cloneBlockInstances(global)...)
		clone.Blocks = combined
		enriched = append(enriched, &clone)
	}
	return enriched, nil
}

func (s *pageService) attachWidgets(ctx context.Context, pages []*Page) ([]*Page, error) {
	if s.widgets == nil || len(pages) == 0 {
		return pages, nil
	}

	definitions, err := s.widgets.ListAreaDefinitions(ctx)
	if err != nil {
		if errors.Is(err, widgets.ErrFeatureDisabled) || errors.Is(err, widgets.ErrAreaFeatureDisabled) {
			return pages, nil
		}
		return nil, err
	}
	if len(definitions) == 0 {
		return pages, nil
	}

	templateCache := make(map[uuid.UUID]*themes.Template)
	regionCache := make(map[uuid.UUID]map[string]struct{})

	now := s.now()
	enriched := make([]*Page, 0, len(pages))
	for _, page := range pages {
		if page == nil {
			enriched = append(enriched, nil)
			continue
		}

		clone := *page
		var areaWidgets map[string][]*widgets.ResolvedWidget

		var template *themes.Template
		allowedRegions := map[string]struct{}{}
		if s.themes != nil && page.TemplateID != uuid.Nil {
			if cached, ok := templateCache[page.TemplateID]; ok {
				template = cached
			} else {
				tpl, tplErr := s.themes.GetTemplate(ctx, page.TemplateID)
				if tplErr != nil {
					if errors.Is(tplErr, themes.ErrFeatureDisabled) || errors.Is(tplErr, themes.ErrTemplateNotFound) {
						templateCache[page.TemplateID] = nil
					} else {
						return nil, tplErr
					}
				} else {
					templateCache[page.TemplateID] = tpl
					template = tpl
				}
			}
			if template != nil {
				if cachedRegions, ok := regionCache[template.ID]; ok {
					if cachedRegions != nil {
						allowedRegions = cachedRegions
					}
				} else {
					infos, regionErr := s.themes.TemplateRegions(ctx, template.ID)
					if regionErr != nil {
						if errors.Is(regionErr, themes.ErrFeatureDisabled) || errors.Is(regionErr, themes.ErrTemplateNotFound) {
							regionCache[template.ID] = nil
						} else {
							return nil, regionErr
						}
					} else {
						regionMap := make(map[string]struct{})
						for _, info := range infos {
							if info.AcceptsWidgets {
								regionMap[info.Key] = struct{}{}
							}
						}
						regionCache[template.ID] = regionMap
						allowedRegions = regionMap
					}
				}
			}
		}
		for _, definition := range definitions {
			if definition == nil {
				continue
			}
			code := strings.TrimSpace(definition.Code)
			if code == "" {
				continue
			}
			if template != nil && !areaDefinitionApplies(definition, template) {
				continue
			}
			if len(allowedRegions) > 0 {
				if _, ok := allowedRegions[code]; !ok {
					continue
				}
			}

			resolved, err := s.widgets.ResolveArea(ctx, widgets.ResolveAreaInput{
				AreaCode: code,
				Now:      now,
			})
			if err != nil {
				if errors.Is(err, widgets.ErrFeatureDisabled) || errors.Is(err, widgets.ErrAreaFeatureDisabled) {
					areaWidgets = nil
					break
				}
				return nil, err
			}
			if len(resolved) == 0 {
				continue
			}
			if areaWidgets == nil {
				areaWidgets = make(map[string][]*widgets.ResolvedWidget)
			}
			areaWidgets[code] = cloneResolvedWidgetSlice(resolved)
		}

		if len(areaWidgets) > 0 {
			clone.Widgets = areaWidgets
		}
		enriched = append(enriched, &clone)
	}
	return enriched, nil
}

func cloneResolvedWidgetSlice(input []*widgets.ResolvedWidget) []*widgets.ResolvedWidget {
	if len(input) == 0 {
		return nil
	}
	cloned := make([]*widgets.ResolvedWidget, len(input))
	copy(cloned, input)
	return cloned
}

func areaDefinitionApplies(definition *widgets.AreaDefinition, template *themes.Template) bool {
	if definition == nil {
		return false
	}
	scope := definition.Scope
	switch scope {
	case widgets.AreaScopeTheme:
		if template == nil || definition.ThemeID == nil {
			return false
		}
		return template.ThemeID == *definition.ThemeID
	case widgets.AreaScopeTemplate:
		if template == nil || definition.TemplateID == nil {
			return false
		}
		return template.ID == *definition.TemplateID
	default:
		// Treat global/unspecified scopes as applicable everywhere.
		return true
	}
}

func cloneBlockInstances(instances []*blocks.Instance) []*blocks.Instance {
	if len(instances) == 0 {
		return nil
	}

	cloned := make([]*blocks.Instance, len(instances))
	for i, inst := range instances {
		if inst == nil {
			continue
		}
		copyInst := *inst
		if inst.Configuration != nil {
			copyInst.Configuration = maps.Clone(inst.Configuration)
		}
		if len(inst.Translations) > 0 {
			copyInst.Translations = make([]*blocks.Translation, len(inst.Translations))
			for j, tr := range inst.Translations {
				if tr == nil {
					continue
				}
				copyTr := *tr
				if tr.Content != nil {
					copyTr.Content = maps.Clone(tr.Content)
				}
				if tr.AttributeOverride != nil {
					copyTr.AttributeOverride = maps.Clone(tr.AttributeOverride)
				}
				copyInst.Translations[j] = &copyTr
			}
		}
		cloned[i] = &copyInst
	}
	return cloned
}

func sanitizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func pathExists(pages []*Page, localeID uuid.UUID, path string) bool {
	for _, p := range pages {
		for _, tr := range p.Translations {
			if tr == nil {
				continue
			}
			if tr.LocaleID == localeID && strings.EqualFold(tr.Path, path) {
				return true
			}
		}
	}
	return false
}

func chooseStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "draft"
	}
	return strings.ToLower(status)
}

func isValidSlug(slug string) bool {
	const pattern = "^[a-z0-9\\-]+$"
	matched, _ := regexp.MatchString(pattern, slug)
	return matched
}
