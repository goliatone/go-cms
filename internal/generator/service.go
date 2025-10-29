package generator

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/i18n"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

var (
	// ErrNotImplemented indicates that a generator operation has not been implemented yet.
	ErrNotImplemented = errors.New("generator: operation not implemented")
	// ErrServiceDisabled indicates the generator feature is disabled.
	ErrServiceDisabled           = errors.New("generator: service disabled")
	errRendererRequired          = errors.New("generator: template renderer is required")
	errTemplateRequired          = errors.New("generator: template is required for rendering")
	errTemplateIdentifierMissing = errors.New("generator: template identifier is required for rendering")
)

// Service describes the static site generator contract.
type Service interface {
	Build(ctx context.Context, opts BuildOptions) (*BuildResult, error)
	BuildPage(ctx context.Context, pageID uuid.UUID, locale string) error
	BuildAssets(ctx context.Context) error
	BuildSitemap(ctx context.Context) error
	Clean(ctx context.Context) error
}

// Config captures runtime behaviour toggles for the generator.
type Config struct {
	OutputDir       string
	BaseURL         string
	CleanBuild      bool
	Incremental     bool
	CopyAssets      bool
	GenerateSitemap bool
	GenerateRobots  bool
	GenerateFeeds   bool
	Workers         int
	DefaultLocale   string
	Locales         []string
	Menus           map[string]string
}

// BuildOptions narrows the scope of a generator run.
type BuildOptions struct {
	Locales []string
	PageIDs []uuid.UUID
	DryRun  bool
}

// BuildResult reports aggregated build metadata.
type BuildResult struct {
	PagesBuilt  int
	AssetsBuilt int
	Locales     []string
	Duration    time.Duration
	Rendered    []RenderedPage
	Diagnostics []RenderDiagnostic
	Errors      []error
	DryRun      bool
}

// Dependencies lists the services required by the generator.
type Dependencies struct {
	Pages    pages.Service
	Content  content.Service
	Blocks   blocks.Service
	Widgets  widgets.Service
	Menus    menus.Service
	Themes   themes.Service
	I18N     i18n.Service
	Renderer interfaces.TemplateRenderer
	Storage  interfaces.StorageProvider
	Locales  LocaleLookup
}

// LocaleLookup resolves locales from configured repositories.
type LocaleLookup interface {
	GetByCode(ctx context.Context, code string) (*content.Locale, error)
}

// NewService wires a generator implementation with the provided configuration and dependencies.
func NewService(cfg Config, deps Dependencies) Service {
	return &service{
		cfg:  cfg,
		deps: deps,
		now:  time.Now,
	}
}

// NewDisabledService returns a Service that fails all operations with ErrServiceDisabled.
func NewDisabledService() Service {
	return disabledService{}
}

type service struct {
	cfg  Config
	deps Dependencies
	now  func() time.Time
}

type disabledService struct{}

func (s *service) Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.deps.Renderer == nil {
		return nil, errRendererRequired
	}

	start := time.Now()
	buildCtx, err := s.loadContext(ctx, opts)
	if err != nil {
		return nil, err
	}

	result := &BuildResult{
		Locales:     make([]string, 0, len(buildCtx.Locales)),
		DryRun:      opts.DryRun,
		Diagnostics: make([]RenderDiagnostic, 0, len(buildCtx.Pages)),
	}
	for _, spec := range buildCtx.Locales {
		result.Locales = append(result.Locales, spec.Code)
	}

	siteMeta := SiteMetadata{
		BaseURL:       strings.TrimRight(s.cfg.BaseURL, "/"),
		DefaultLocale: buildCtx.DefaultLocale,
		Locales:       append([]LocaleSpec(nil), buildCtx.Locales...),
		MenuAliases:   maps.Clone(buildCtx.MenuAliases),
		Metadata:      map[string]any{},
	}
	if siteMeta.MenuAliases == nil {
		siteMeta.MenuAliases = map[string]string{}
	}

	var (
		mu          sync.Mutex
		rendered    = make([]RenderedPage, 0, len(buildCtx.Pages))
		errorsSlice []error
	)

	collect := func(outcome renderOutcome) {
		mu.Lock()
		defer mu.Unlock()
		result.Diagnostics = append(result.Diagnostics, outcome.diagnostic)
		if outcome.err != nil {
			errorsSlice = append(errorsSlice, outcome.err)
			return
		}
		result.PagesBuilt++
		if !opts.DryRun {
			rendered = append(rendered, outcome.page)
		}
	}

	workerCount := s.effectiveWorkerCount(len(buildCtx.Locales))
	if workerCount <= 1 || len(buildCtx.Pages) <= 1 {
		for _, page := range buildCtx.Pages {
			select {
			case <-ctx.Done():
				collect(renderOutcome{
					diagnostic: RenderDiagnostic{
						PageID: page.Page.ID,
						Locale: page.Locale.Code,
						Path:   safeTranslationPath(page.Translation),
						Err:    ctx.Err(),
					},
					err: ctx.Err(),
				})
				return result, ctx.Err()
			default:
				outcome := s.renderPage(ctx, siteMeta, buildCtx, page)
				collect(outcome)
			}
		}
	} else {
		if err := s.renderConcurrently(ctx, siteMeta, buildCtx, workerCount, collect); err != nil {
			errorsSlice = append(errorsSlice, err)
		}
	}

	result.Rendered = rendered
	result.Duration = time.Since(start)
	if len(errorsSlice) > 0 {
		result.Errors = append(result.Errors, errorsSlice...)
		return result, errors.Join(errorsSlice...)
	}
	return result, nil
}

func (s *service) renderConcurrently(
	ctx context.Context,
	siteMeta SiteMetadata,
	buildCtx *BuildContext,
	workers int,
	collect func(renderOutcome),
) error {
	grouped := groupPagesByLocale(buildCtx.Pages)
	if len(grouped) == 0 {
		return nil
	}

	jobs := make(chan []*PageData)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobs {
				for _, page := range batch {
					select {
					case <-ctx.Done():
						collect(renderOutcome{
							diagnostic: RenderDiagnostic{
								PageID: page.Page.ID,
								Locale: page.Locale.Code,
								Path:   safeTranslationPath(page.Translation),
								Err:    ctx.Err(),
							},
							err: ctx.Err(),
						})
						return
					default:
						outcome := s.renderPage(ctx, siteMeta, buildCtx, page)
						collect(outcome)
					}
				}
			}
		}()
	}

	for _, locale := range buildCtx.Locales {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- grouped[locale.Code]:
		}
	}
	close(jobs)
	wg.Wait()
	return nil
}

func (s *service) renderPage(
	ctx context.Context,
	siteMeta SiteMetadata,
	buildCtx *BuildContext,
	data *PageData,
) renderOutcome {
	outcome := renderOutcome{
		diagnostic: RenderDiagnostic{
			PageID: data.Page.ID,
			Locale: data.Locale.Code,
			Path:   safeTranslationPath(data.Translation),
		},
	}

	select {
	case <-ctx.Done():
		outcome.err = ctx.Err()
		outcome.diagnostic.Err = ctx.Err()
		return outcome
	default:
	}

	if data.Template == nil {
		err := fmt.Errorf("generator: page %s locale %s missing template: %w", data.Page.ID, data.Locale.Code, errTemplateRequired)
		outcome.err = err
		outcome.diagnostic.Err = err
		return outcome
	}

	templateName := strings.TrimSpace(data.Template.TemplatePath)
	if templateName == "" {
		templateName = strings.TrimSpace(data.Template.Slug)
	}
	if templateName == "" {
		err := fmt.Errorf("generator: page %s locale %s template missing identifier: %w", data.Page.ID, data.Locale.Code, errTemplateIdentifierMissing)
		outcome.err = err
		outcome.diagnostic.Err = err
		return outcome
	}
	outcome.diagnostic.Template = templateName

	templateCtx := TemplateContext{
		Site: siteMeta,
		Page: PageRenderingContext{
			Page:               data.Page,
			Content:            data.Content,
			Translation:        data.Translation,
			ContentTranslation: data.ContentTranslation,
			Blocks:             data.Blocks,
			Widgets:            data.Widgets,
			Menus:              data.Menus,
			Template:           data.Template,
			Theme:              data.Theme,
			Locale:             data.Locale,
			Metadata:           data.Metadata,
		},
		Build: BuildMetadata{
			GeneratedAt: buildCtx.GeneratedAt,
			Options:     buildCtx.Options,
		},
		Helpers: newTemplateHelpers(siteMeta.DefaultLocale, data.Locale, siteMeta.BaseURL),
	}

	start := time.Now()
	rendered, err := s.deps.Renderer.RenderTemplate(templateName, templateCtx)
	duration := time.Since(start)
	outcome.diagnostic.Duration = duration
	if err != nil {
		wrapped := fmt.Errorf("generator: render template %q for page %s (%s): %w", templateName, data.Page.ID, data.Locale.Code, err)
		outcome.err = wrapped
		outcome.diagnostic.Err = wrapped
		return outcome
	}

	outcome.page = RenderedPage{
		PageID:   data.Page.ID,
		Locale:   data.Locale.Code,
		Path:     safeTranslationPath(data.Translation),
		Template: templateName,
		HTML:     rendered,
		Metadata: data.Metadata,
		Duration: duration,
	}
	return outcome
}

func (s *service) effectiveWorkerCount(localeCount int) int {
	workers := s.cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers < 1 {
		workers = 1
	}
	if localeCount > 0 && workers > localeCount {
		return localeCount
	}
	return workers
}

func groupPagesByLocale(pages []*PageData) map[string][]*PageData {
	grouped := make(map[string][]*PageData, len(pages))
	for _, page := range pages {
		if page == nil {
			continue
		}
		code := page.Locale.Code
		grouped[code] = append(grouped[code], page)
	}
	return grouped
}

func safeTranslationPath(translation *pages.PageTranslation) string {
	if translation == nil {
		return ""
	}
	return translation.Path
}

func (service) BuildPage(context.Context, uuid.UUID, string) error {
	return ErrNotImplemented
}

func (service) BuildAssets(context.Context) error {
	return ErrNotImplemented
}

func (service) BuildSitemap(context.Context) error {
	return ErrNotImplemented
}

func (service) Clean(context.Context) error {
	return ErrNotImplemented
}

func (disabledService) Build(context.Context, BuildOptions) (*BuildResult, error) {
	return nil, ErrServiceDisabled
}

func (disabledService) BuildPage(context.Context, uuid.UUID, string) error {
	return ErrServiceDisabled
}

func (disabledService) BuildAssets(context.Context) error {
	return ErrServiceDisabled
}

func (disabledService) BuildSitemap(context.Context) error {
	return ErrServiceDisabled
}

func (disabledService) Clean(context.Context) error {
	return ErrServiceDisabled
}
