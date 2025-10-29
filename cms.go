package cms

import (
	"github.com/goliatone/go-cms/internal/blocks"
	"github.com/goliatone/go-cms/internal/content"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/internal/menus"
	"github.com/goliatone/go-cms/internal/pages"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/internal/widgets"
	"github.com/goliatone/go-cms/pkg/interfaces"
)

// ContentService exports the content service contract for consumers of the cms package.
type ContentService = content.Service

// PageService exports the pages service contract.
type PageService = pages.Service

// BlockService exports the blocks service contract.
type BlockService = blocks.Service

// MenuService exports the menus service contract.
type MenuService = menus.Service

// WidgetService exports the widgets service contract.
type WidgetService = widgets.Service

// ThemeService exports the themes service contract.
type ThemeService = themes.Service

// MediaService exports the media helper contract.
type MediaService = media.Service

// GeneratorService exports the static site generator contract.
type GeneratorService = generator.Service

// Module represents the top level CMS runtime façade.
type Module struct {
	container *di.Container
}

// New constructs a CMS module using the provided configuration and optional DI overrides.
func New(cfg Config, opts ...di.Option) (*Module, error) {
	container, err := di.NewContainer(cfg, opts...)
	if err != nil {
		return nil, err
	}
	return &Module{container: container}, nil
}

// Container exposes the underlying DI container for advanced integrations.
func (m *Module) Container() *di.Container {
	return m.container
}

// Content returns the configured content service.
func (m *Module) Content() ContentService {
	return m.container.ContentService()
}

// Pages returns the configured page service.
func (m *Module) Pages() PageService {
	return m.container.PageService()
}

// Blocks returns the configured block service.
func (m *Module) Blocks() BlockService {
	return m.container.BlockService()
}

// Menus returns the configured menu service.
func (m *Module) Menus() MenuService {
	return m.container.MenuService()
}

// Widgets returns the configured widget service.
func (m *Module) Widgets() WidgetService {
	return m.container.WidgetService()
}

// Themes returns the configured theme service.
func (m *Module) Themes() ThemeService {
	return m.container.ThemeService()
}

// Media returns the media helper used by the module.
func (m *Module) Media() MediaService {
	return m.container.MediaService()
}

// Generator returns the configured generator service.
func (m *Module) Generator() GeneratorService {
	return m.container.GeneratorService()
}

// Markdown returns the markdown service when configured.
func (m *Module) Markdown() interfaces.MarkdownService {
	return m.container.MarkdownService()
}

// Scheduler returns the scheduler used for publish automation.
func (m *Module) Scheduler() interfaces.Scheduler {
	return m.container.Scheduler()
}

// CommandHandlers returns copies of the registered command handlers, allowing callers
// to wire them into custom dispatchers when automatic registration is disabled.
func (m *Module) CommandHandlers() []any {
	if m == nil || m.container == nil {
		return nil
	}
	return m.container.CommandHandlers()
}
