package widgets

import "errors"

var (
	ErrFeatureDisabled = errors.New("widgets: feature disabled")

	ErrDefinitionNameRequired          = errors.New("widgets: definition name required")
	ErrDefinitionSchemaRequired        = errors.New("widgets: definition schema required")
	ErrDefinitionSchemaInvalid         = errors.New("widgets: definition schema invalid")
	ErrDefinitionExists                = errors.New("widgets: definition already exists")
	ErrDefinitionDefaultsInvalid       = errors.New("widgets: defaults contain unknown fields")
	ErrDefinitionInUse                 = errors.New("widgets: definition has active instances")
	ErrDefinitionSoftDeleteUnsupported = errors.New("widgets: soft delete not supported for definitions")

	ErrInstanceDefinitionRequired    = errors.New("widgets: definition id required")
	ErrInstanceCreatorRequired       = errors.New("widgets: created_by is required")
	ErrInstanceUpdaterRequired       = errors.New("widgets: updated_by is required")
	ErrInstanceIDRequired            = errors.New("widgets: instance id required")
	ErrInstancePositionInvalid       = errors.New("widgets: position cannot be negative")
	ErrInstanceConfigurationInvalid  = errors.New("widgets: configuration contains unknown fields")
	ErrInstanceScheduleInvalid       = errors.New("widgets: publish_on must be before unpublish_on")
	ErrVisibilityRulesInvalid        = errors.New("widgets: visibility_rules contains unsupported keys")
	ErrVisibilityScheduleInvalid     = errors.New("widgets: visibility schedule timestamps must be RFC3339")
	ErrInstanceSoftDeleteUnsupported = errors.New("widgets: soft delete not supported for instances")

	ErrTranslationContentRequired = errors.New("widgets: translation content required")
	ErrTranslationLocaleRequired  = errors.New("widgets: translation locale required")
	ErrTranslationExists          = errors.New("widgets: translation already exists for locale")
	ErrTranslationNotFound        = errors.New("widgets: translation not found")

	ErrAreaCodeRequired           = errors.New("widgets: area code required")
	ErrAreaCodeInvalid            = errors.New("widgets: area code must contain letters, numbers, dot, or underscore")
	ErrAreaNameRequired           = errors.New("widgets: area name required")
	ErrAreaDefinitionExists       = errors.New("widgets: area code already exists")
	ErrAreaDefinitionNotFound     = errors.New("widgets: area definition not found")
	ErrAreaFeatureDisabled        = errors.New("widgets: area repositories not configured")
	ErrAreaInstanceRequired       = errors.New("widgets: instance id required")
	ErrAreaPlacementExists        = errors.New("widgets: widget already assigned to area for locale")
	ErrAreaPlacementPosition      = errors.New("widgets: placement position must be zero or positive")
	ErrAreaPlacementNotFound      = errors.New("widgets: placement not found")
	ErrAreaWidgetOrderMismatch    = errors.New("widgets: reorder input must include every placement")
	ErrVisibilityLocaleRestricted = errors.New("widgets: locale not permitted for widget")
)
