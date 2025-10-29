package generator

import (
	"context"
	"errors"
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
	ErrServiceDisabled = errors.New("generator: service disabled")
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

func (service) Build(context.Context, BuildOptions) (*BuildResult, error) {
	return nil, ErrNotImplemented
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
