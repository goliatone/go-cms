package cms

import (
	"github.com/goliatone/go-cms/blocks"
	"github.com/goliatone/go-cms/content"
	adminblocks "github.com/goliatone/go-cms/internal/admin/blocks"
	adminstorage "github.com/goliatone/go-cms/internal/admin/storage"
	admintranslations "github.com/goliatone/go-cms/internal/admin/translations"
	"github.com/goliatone/go-cms/internal/di"
	"github.com/goliatone/go-cms/internal/generator"
	"github.com/goliatone/go-cms/internal/media"
	"github.com/goliatone/go-cms/internal/themes"
	"github.com/goliatone/go-cms/pages"
	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/goliatone/go-cms/widgets"
)

// ContentService exports the content service contract for consumers of the cms package.
type ContentService = content.Service

// ContentTranslationCreator exports the additive translation-create capability.
type ContentTranslationCreator = content.TranslationCreator

// PageService exports the pages service contract.
type PageService = pages.Service

// ContentTypeService exports the content-type service contract.
type ContentTypeService = content.ContentTypeService

// AdminPageReadService exports the admin page read service contract.
type AdminPageReadService = interfaces.AdminPageReadService

// AdminContentReadService exports the admin content read service contract.
type AdminContentReadService = interfaces.AdminContentReadService

// AdminContentWriteService exports the admin content write service contract.
type AdminContentWriteService = interfaces.AdminContentWriteService

// AdminBlockReadService exports the admin block read service contract.
type AdminBlockReadService = interfaces.AdminBlockReadService

// AdminBlockWriteService exports the admin block write service contract.
type AdminBlockWriteService = interfaces.AdminBlockWriteService

// AdminPageRecord exports the admin page read record DTO.
type AdminPageRecord = interfaces.AdminPageRecord

// AdminContentRecord exports the admin content read record DTO.
type AdminContentRecord = interfaces.AdminContentRecord

// AdminBlockDefinitionRecord exports the admin block definition DTO.
type AdminBlockDefinitionRecord = interfaces.AdminBlockDefinitionRecord

// AdminBlockDefinitionVersionRecord exports the admin block definition version DTO.
type AdminBlockDefinitionVersionRecord = interfaces.AdminBlockDefinitionVersionRecord

// AdminBlockRecord exports the admin block record DTO.
type AdminBlockRecord = interfaces.AdminBlockRecord

// AdminPageListOptions exports the admin page list options.
type AdminPageListOptions = interfaces.AdminPageListOptions

// AdminContentListOptions exports the admin content list options.
type AdminContentListOptions = interfaces.AdminContentListOptions

// AdminPageGetOptions exports the admin page get options.
type AdminPageGetOptions = interfaces.AdminPageGetOptions

// AdminContentGetOptions exports the admin content get options.
type AdminContentGetOptions = interfaces.AdminContentGetOptions

// AdminPageIncludeOptions exports admin include options.
type AdminPageIncludeOptions = interfaces.AdminPageIncludeOptions

// AdminContentIncludeOptions exports admin content include options.
type AdminContentIncludeOptions = interfaces.AdminContentIncludeOptions

// AdminPageIncludeDefaults exports admin include defaults.
type AdminPageIncludeDefaults = interfaces.AdminPageIncludeDefaults

// AdminContentIncludeDefaults exports admin content include defaults.
type AdminContentIncludeDefaults = interfaces.AdminContentIncludeDefaults

// AdminContentCreateRequest exports the admin content create request.
type AdminContentCreateRequest = interfaces.AdminContentCreateRequest

// AdminContentUpdateRequest exports the admin content update request.
type AdminContentUpdateRequest = interfaces.AdminContentUpdateRequest

// AdminContentDeleteRequest exports the admin content delete request.
type AdminContentDeleteRequest = interfaces.AdminContentDeleteRequest

// AdminContentCreateTranslationRequest exports the admin content translation clone request.
type AdminContentCreateTranslationRequest = interfaces.AdminContentCreateTranslationRequest

// AdminBlockDefinitionListOptions exports the admin block definition list options.
type AdminBlockDefinitionListOptions = interfaces.AdminBlockDefinitionListOptions

// AdminBlockDefinitionGetOptions exports the admin block definition get options.
type AdminBlockDefinitionGetOptions = interfaces.AdminBlockDefinitionGetOptions

// AdminBlockListOptions exports the admin block instance list options.
type AdminBlockListOptions = interfaces.AdminBlockListOptions

// AdminBlockDefinitionCreateRequest exports the admin block definition create request.
type AdminBlockDefinitionCreateRequest = interfaces.AdminBlockDefinitionCreateRequest

// AdminBlockDefinitionUpdateRequest exports the admin block definition update request.
type AdminBlockDefinitionUpdateRequest = interfaces.AdminBlockDefinitionUpdateRequest

// AdminBlockDefinitionDeleteRequest exports the admin block definition delete request.
type AdminBlockDefinitionDeleteRequest = interfaces.AdminBlockDefinitionDeleteRequest

// AdminBlockSaveRequest exports the admin block save request.
type AdminBlockSaveRequest = interfaces.AdminBlockSaveRequest

// AdminBlockDeleteRequest exports the admin block delete request.
type AdminBlockDeleteRequest = interfaces.AdminBlockDeleteRequest

// BlockService exports the blocks service contract.
type BlockService = blocks.Service

// WidgetService exports the widgets service contract.
type WidgetService = widgets.Service

// ThemeService exports the themes service contract.
type ThemeService = themes.Service

// MediaService exports the media helper contract.
type MediaService = media.Service

// GeneratorService exports the static site generator contract.
type GeneratorService = generator.Service

// StorageAdminService exports the storage admin helper contract.
type StorageAdminService = *adminstorage.Service

// TranslationAdminService exports the translation settings admin helper contract.
type TranslationAdminService = *admintranslations.Service

// BlockAdminService exports the embedded blocks admin helper contract.
type BlockAdminService = *adminblocks.Service

// Module represents the top level CMS runtime façade.
type Module struct {
	container *di.Container
}

// New constructs a CMS module using the provided configuration and optional DI overrides.
func New(cfg Config, opts ...Option) (*Module, error) {
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

// ContentTranslations returns the optional translation-create capability for content services.
func (m *Module) ContentTranslations() ContentTranslationCreator {
	if m == nil || m.container == nil {
		return nil
	}
	creator, _ := m.container.ContentService().(content.TranslationCreator)
	return creator
}

// ContentTypes returns the configured content type service.
func (m *Module) ContentTypes() ContentTypeService {
	return m.container.ContentTypeService()
}

// StorageAdmin returns the storage admin helper service.
func (m *Module) StorageAdmin() StorageAdminService {
	if m == nil || m.container == nil {
		return nil
	}
	return m.container.StorageAdminService()
}

// TranslationAdmin returns the translations admin helper service.
func (m *Module) TranslationAdmin() TranslationAdminService {
	if m == nil || m.container == nil {
		return nil
	}
	return m.container.TranslationAdminService()
}

// BlocksAdmin returns the embedded blocks admin helper service.
func (m *Module) BlocksAdmin() BlockAdminService {
	if m == nil || m.container == nil {
		return nil
	}
	return m.container.BlockAdminService()
}

// Pages returns the configured page service.
func (m *Module) Pages() PageService {
	return m.container.PageService()
}

// AdminPageRead returns the configured admin page read service.
func (m *Module) AdminPageRead() AdminPageReadService {
	if m == nil || m.container == nil {
		return nil
	}
	return m.container.AdminPageReadService()
}

// AdminContentRead returns the configured admin content read service.
func (m *Module) AdminContentRead() AdminContentReadService {
	if m == nil || m.container == nil {
		return nil
	}
	return m.container.AdminContentReadService()
}

// AdminContentWrite returns the configured admin content write service.
func (m *Module) AdminContentWrite() AdminContentWriteService {
	if m == nil || m.container == nil {
		return nil
	}
	return m.container.AdminContentWriteService()
}

// AdminBlockRead returns the configured admin block read service.
func (m *Module) AdminBlockRead() AdminBlockReadService {
	if m == nil || m.container == nil {
		return nil
	}
	return m.container.AdminBlockReadService()
}

// AdminBlockWrite returns the configured admin block write service.
func (m *Module) AdminBlockWrite() AdminBlockWriteService {
	if m == nil || m.container == nil {
		return nil
	}
	return m.container.AdminBlockWriteService()
}

// Blocks returns the configured block service.
func (m *Module) Blocks() BlockService {
	return m.container.BlockService()
}

// Menus returns the configured menu service.
func (m *Module) Menus() MenuService {
	return newMenuService(m)
}

// Locales returns the configured locale resolver service.
func (m *Module) Locales() LocaleService {
	return newLocaleService(m)
}

// Widgets returns the configured widget service.
func (m *Module) Widgets() WidgetService {
	return m.container.WidgetService()
}

// Shortcodes returns the configured shortcode service.
func (m *Module) Shortcodes() interfaces.ShortcodeService {
	if m == nil || m.container == nil {
		return nil
	}
	return m.container.ShortcodeService()
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

// WorkflowEngine returns the configured workflow engine.
func (m *Module) WorkflowEngine() interfaces.WorkflowEngine {
	return m.container.WorkflowEngine()
}

// TranslationsEnabled reports whether translations are globally enabled.
func (m *Module) TranslationsEnabled() bool {
	if m == nil || m.container == nil {
		return false
	}
	return m.container.TranslationsEnabled()
}

// TranslationsRequired reports whether translations are required when enabled.
func (m *Module) TranslationsRequired() bool {
	if m == nil || m.container == nil {
		return false
	}
	return m.container.TranslationsRequired()
}
