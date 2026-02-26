package menus

import cmsmenus "github.com/goliatone/go-cms/menus"

type (
	Menu                = cmsmenus.Menu
	MenuItem            = cmsmenus.MenuItem
	MenuItemTranslation = cmsmenus.MenuItemTranslation
	MenuLocationBinding = cmsmenus.MenuLocationBinding
	MenuViewProfile     = cmsmenus.MenuViewProfile
)

const (
	MenuItemTypeItem      = cmsmenus.MenuItemTypeItem
	MenuItemTypeGroup     = cmsmenus.MenuItemTypeGroup
	MenuItemTypeSeparator = cmsmenus.MenuItemTypeSeparator
	MenuStatusDraft       = cmsmenus.MenuStatusDraft
	MenuStatusPublished   = cmsmenus.MenuStatusPublished
)
