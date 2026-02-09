package widgets

import cmswidgets "github.com/goliatone/go-cms/widgets"

type (
	Service                     = cmswidgets.Service
	RegisterDefinitionInput     = cmswidgets.RegisterDefinitionInput
	DeleteDefinitionRequest     = cmswidgets.DeleteDefinitionRequest
	CreateInstanceInput         = cmswidgets.CreateInstanceInput
	UpdateInstanceInput         = cmswidgets.UpdateInstanceInput
	DeleteInstanceRequest       = cmswidgets.DeleteInstanceRequest
	AddTranslationInput         = cmswidgets.AddTranslationInput
	UpdateTranslationInput      = cmswidgets.UpdateTranslationInput
	DeleteTranslationRequest    = cmswidgets.DeleteTranslationRequest
	RegisterAreaDefinitionInput = cmswidgets.RegisterAreaDefinitionInput
	AssignWidgetToAreaInput     = cmswidgets.AssignWidgetToAreaInput
	RemoveWidgetFromAreaInput   = cmswidgets.RemoveWidgetFromAreaInput
	ReorderAreaWidgetsInput     = cmswidgets.ReorderAreaWidgetsInput
	AreaWidgetOrder             = cmswidgets.AreaWidgetOrder
	ResolveAreaInput            = cmswidgets.ResolveAreaInput
	VisibilityContext           = cmswidgets.VisibilityContext
)

var (
	ErrFeatureDisabled = cmswidgets.ErrFeatureDisabled

	ErrDefinitionNameRequired          = cmswidgets.ErrDefinitionNameRequired
	ErrDefinitionSchemaRequired        = cmswidgets.ErrDefinitionSchemaRequired
	ErrDefinitionSchemaInvalid         = cmswidgets.ErrDefinitionSchemaInvalid
	ErrDefinitionExists                = cmswidgets.ErrDefinitionExists
	ErrDefinitionDefaultsInvalid       = cmswidgets.ErrDefinitionDefaultsInvalid
	ErrDefinitionInUse                 = cmswidgets.ErrDefinitionInUse
	ErrDefinitionSoftDeleteUnsupported = cmswidgets.ErrDefinitionSoftDeleteUnsupported

	ErrInstanceDefinitionRequired    = cmswidgets.ErrInstanceDefinitionRequired
	ErrInstanceCreatorRequired       = cmswidgets.ErrInstanceCreatorRequired
	ErrInstanceUpdaterRequired       = cmswidgets.ErrInstanceUpdaterRequired
	ErrInstanceIDRequired            = cmswidgets.ErrInstanceIDRequired
	ErrInstancePositionInvalid       = cmswidgets.ErrInstancePositionInvalid
	ErrInstanceConfigurationInvalid  = cmswidgets.ErrInstanceConfigurationInvalid
	ErrInstanceScheduleInvalid       = cmswidgets.ErrInstanceScheduleInvalid
	ErrVisibilityRulesInvalid        = cmswidgets.ErrVisibilityRulesInvalid
	ErrVisibilityScheduleInvalid     = cmswidgets.ErrVisibilityScheduleInvalid
	ErrInstanceSoftDeleteUnsupported = cmswidgets.ErrInstanceSoftDeleteUnsupported

	ErrTranslationContentRequired = cmswidgets.ErrTranslationContentRequired
	ErrTranslationLocaleRequired  = cmswidgets.ErrTranslationLocaleRequired
	ErrTranslationExists          = cmswidgets.ErrTranslationExists
	ErrTranslationNotFound        = cmswidgets.ErrTranslationNotFound

	ErrAreaCodeRequired           = cmswidgets.ErrAreaCodeRequired
	ErrAreaCodeInvalid            = cmswidgets.ErrAreaCodeInvalid
	ErrAreaNameRequired           = cmswidgets.ErrAreaNameRequired
	ErrAreaDefinitionExists       = cmswidgets.ErrAreaDefinitionExists
	ErrAreaDefinitionNotFound     = cmswidgets.ErrAreaDefinitionNotFound
	ErrAreaFeatureDisabled        = cmswidgets.ErrAreaFeatureDisabled
	ErrAreaInstanceRequired       = cmswidgets.ErrAreaInstanceRequired
	ErrAreaPlacementExists        = cmswidgets.ErrAreaPlacementExists
	ErrAreaPlacementPosition      = cmswidgets.ErrAreaPlacementPosition
	ErrAreaPlacementNotFound      = cmswidgets.ErrAreaPlacementNotFound
	ErrAreaWidgetOrderMismatch    = cmswidgets.ErrAreaWidgetOrderMismatch
	ErrVisibilityLocaleRestricted = cmswidgets.ErrVisibilityLocaleRestricted
)
