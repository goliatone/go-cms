package generator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"path"
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
	Assets   AssetResolver
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
						Route:  safeTranslationPath(page.Translation),
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

	if opts.DryRun {
		result.Rendered = rendered
		result.Duration = time.Since(start)
		if len(errorsSlice) > 0 {
			result.Errors = append(result.Errors, errorsSlice...)
			return result, errors.Join(errorsSlice...)
		}
		return result, nil
	}

	writer := newArtifactWriter(s.deps.Storage)
	if err := s.persistPages(ctx, writer, buildCtx, rendered); err != nil {
		errorsSlice = append(errorsSlice, err)
	}

	assetsBuilt, err := s.copyAssets(ctx, writer, buildCtx)
	if err != nil {
		errorsSlice = append(errorsSlice, err)
	} else {
		result.AssetsBuilt += assetsBuilt
	}

	if s.cfg.GenerateSitemap {
		if err := s.writeSitemap(ctx, writer, siteMeta, buildCtx, rendered); err != nil {
			errorsSlice = append(errorsSlice, err)
		}
	}

	if s.cfg.GenerateRobots {
		if err := s.writeRobots(ctx, writer, siteMeta); err != nil {
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
								Route:  safeTranslationPath(page.Translation),
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
			Route:  safeTranslationPath(data.Translation),
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
		Route:    safeTranslationPath(data.Translation),
		Template: templateName,
		HTML:     rendered,
		Metadata: data.Metadata,
		Duration: duration,
	}
	return outcome
}

func (s *service) persistPages(
	ctx context.Context,
	writer artifactWriter,
	buildCtx *BuildContext,
	pages []RenderedPage,
) error {
	if len(pages) == 0 {
		return nil
	}
	baseDir := strings.Trim(strings.TrimSpace(s.cfg.OutputDir), "/")
	dirCache := map[string]struct{}{}
	if baseDir != "" {
		dirCache[baseDir] = struct{}{}
		if err := writer.EnsureDir(ctx, baseDir); err != nil {
			return err
		}
	}
	for i := range pages {
		route := pages[i].Route
		destRel := buildOutputPath(route, pages[i].Locale, buildCtx.DefaultLocale)
		if strings.TrimSpace(destRel) == "" {
			destRel = "index.html"
		}
		fullPath := joinOutputPath(baseDir, destRel)
		if err := ensureDir(ctx, writer, dirCache, path.Dir(fullPath)); err != nil {
			return err
		}
		checksum := computeHashFromString(pages[i].HTML)
		pages[i].Output = fullPath
		pages[i].Checksum = checksum

		metadata := map[string]string{
			"page_id":  pages[i].PageID.String(),
			"route":    route,
			"template": pages[i].Template,
		}
		if s.cfg.Incremental {
			metadata["incremental"] = "true"
		}
		req := writeFileRequest{
			Path:        fullPath,
			Content:     strings.NewReader(pages[i].HTML),
			Size:        int64(len(pages[i].HTML)),
			Locale:      pages[i].Locale,
			Category:    categoryPage,
			ContentType: "text/html; charset=utf-8",
			Checksum:    checksum,
			Metadata:    metadata,
		}
		if err := writer.WriteFile(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) copyAssets(
	ctx context.Context,
	writer artifactWriter,
	buildCtx *BuildContext,
) (int, error) {
	if s.deps.Assets == nil {
		return 0, nil
	}
	baseDir := strings.Trim(strings.TrimSpace(s.cfg.OutputDir), "/")
	dirCache := map[string]struct{}{}
	if baseDir != "" {
		dirCache[baseDir] = struct{}{}
		if err := writer.EnsureDir(ctx, baseDir); err != nil {
			return 0, err
		}
	}
	written := 0
	seen := map[string]struct{}{}
	for _, page := range buildCtx.Pages {
		theme := page.Theme
		if theme == nil {
			continue
		}
		assets := collectThemeAssets(theme)
		for _, asset := range assets {
			key := theme.ID.String() + "::" + asset
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			reader, err := s.deps.Assets.Open(ctx, theme, asset)
			if err != nil {
				return written, err
			}
			data, err := io.ReadAll(reader)
			_ = reader.Close()
			if err != nil {
				return written, err
			}
			resolved, err := s.deps.Assets.ResolvePath(theme, asset)
			if err != nil {
				return written, err
			}
			resolved = strings.TrimLeft(strings.TrimSpace(resolved), "/")
			if resolved == "" {
				resolved = strings.TrimLeft(strings.TrimSpace(asset), "/")
			}
			destRel := path.Join("assets", resolved)
			fullPath := joinOutputPath(baseDir, destRel)
			if err := ensureDir(ctx, writer, dirCache, path.Dir(fullPath)); err != nil {
				return written, err
			}
			checksum := computeHash(data)
			metadata := map[string]string{
				"theme_id": theme.ID.String(),
				"asset":    asset,
			}
			req := writeFileRequest{
				Path:        fullPath,
				Content:     bytes.NewReader(data),
				Size:        int64(len(data)),
				Locale:      "",
				Category:    categoryAsset,
				ContentType: detectAssetContentType(destRel),
				Checksum:    checksum,
				Metadata:    metadata,
			}
			if err := writer.WriteFile(ctx, req); err != nil {
				return written, err
			}
			written++
		}
	}
	return written, nil
}

func (s *service) writeSitemap(
	ctx context.Context,
	writer artifactWriter,
	siteMeta SiteMetadata,
	buildCtx *BuildContext,
	pages []RenderedPage,
) error {
	content := buildSitemap(siteMeta.BaseURL, pages, buildCtx.GeneratedAt)
	baseDir := strings.Trim(strings.TrimSpace(s.cfg.OutputDir), "/")
	destRel := "sitemap.xml"
	fullPath := joinOutputPath(baseDir, destRel)
	if err := ensureDir(ctx, writer, map[string]struct{}{}, path.Dir(fullPath)); err != nil {
		return err
	}
	checksum := computeHashFromString(content)
	req := writeFileRequest{
		Path:        fullPath,
		Content:     strings.NewReader(content),
		Size:        int64(len(content)),
		Category:    categorySitemap,
		ContentType: "application/xml",
		Checksum:    checksum,
		Metadata: map[string]string{
			"generated_at": buildCtx.GeneratedAt.UTC().Format(time.RFC3339),
		},
	}
	return writer.WriteFile(ctx, req)
}

func (s *service) writeRobots(
	ctx context.Context,
	writer artifactWriter,
	siteMeta SiteMetadata,
) error {
	content := buildRobots(siteMeta.BaseURL, s.cfg.GenerateSitemap)
	baseDir := strings.Trim(strings.TrimSpace(s.cfg.OutputDir), "/")
	destRel := "robots.txt"
	fullPath := joinOutputPath(baseDir, destRel)
	if err := ensureDir(ctx, writer, map[string]struct{}{}, path.Dir(fullPath)); err != nil {
		return err
	}
	checksum := computeHashFromString(content)
	req := writeFileRequest{
		Path:        fullPath,
		Content:     strings.NewReader(content),
		Size:        int64(len(content)),
		Category:    categoryRobots,
		ContentType: "text/plain; charset=utf-8",
		Checksum:    checksum,
		Metadata: map[string]string{
			"generated_at": s.now().UTC().Format(time.RFC3339),
		},
	}
	return writer.WriteFile(ctx, req)
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

func ensureDir(ctx context.Context, writer artifactWriter, cache map[string]struct{}, dir string) error {
	dir = strings.Trim(dir, " ")
	if dir == "" || dir == "." {
		return nil
	}
	if cache != nil {
		if _, ok := cache[dir]; ok {
			return nil
		}
		cache[dir] = struct{}{}
	}
	return writer.EnsureDir(ctx, dir)
}

func joinOutputPath(base string, rel string) string {
	if strings.TrimSpace(base) == "" {
		return strings.TrimLeft(rel, "/")
	}
	return path.Join(strings.Trim(base, "/"), rel)
}

func computeHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func computeHashFromString(content string) string {
	return computeHash([]byte(content))
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
