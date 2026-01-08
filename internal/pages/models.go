package pages

import cmspages "github.com/goliatone/go-cms/pages"

type (
	Page                    = cmspages.Page
	PageVersion             = cmspages.PageVersion
	PageTranslation         = cmspages.PageTranslation
	PageVersionSnapshot     = cmspages.PageVersionSnapshot
	PageBlockPlacement      = cmspages.PageBlockPlacement
	WidgetPlacementSnapshot = cmspages.WidgetPlacementSnapshot
)

var PageVersionSnapshotSchema = cmspages.PageVersionSnapshotSchema
