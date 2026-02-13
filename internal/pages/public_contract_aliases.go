package pages

import cmspages "github.com/goliatone/go-cms/pages"

type (
	Service                            = cmspages.Service
	TranslationCheckOptions            = cmspages.TranslationCheckOptions
	CreatePageRequest                  = cmspages.CreatePageRequest
	PageTranslationInput               = cmspages.PageTranslationInput
	UpdatePageRequest                  = cmspages.UpdatePageRequest
	DeletePageRequest                  = cmspages.DeletePageRequest
	UpdatePageTranslationRequest       = cmspages.UpdatePageTranslationRequest
	DeletePageTranslationRequest       = cmspages.DeletePageTranslationRequest
	MovePageRequest                    = cmspages.MovePageRequest
	DuplicatePageRequest               = cmspages.DuplicatePageRequest
	CreatePageDraftRequest             = cmspages.CreatePageDraftRequest
	PublishPageDraftRequest            = cmspages.PublishPageDraftRequest
	PublishPagePublishRequest          = cmspages.PublishPageDraftRequest
	PreviewPageDraftRequest            = cmspages.PreviewPageDraftRequest
	RestorePageVersionRequest          = cmspages.RestorePageVersionRequest
	PagePreview                        = cmspages.PagePreview
	SchedulePageRequest                = cmspages.SchedulePageRequest
	TranslationAlreadyExistsError      = cmspages.TranslationAlreadyExistsError
	InvalidLocaleError                 = cmspages.InvalidLocaleError
	SourceNotFoundError                = cmspages.SourceNotFoundError
	PathConflictError                  = cmspages.PathConflictError
	TranslationInvariantViolationError = cmspages.TranslationInvariantViolationError
)

var (
	ErrContentRequired               = cmspages.ErrContentRequired
	ErrTemplateRequired              = cmspages.ErrTemplateRequired
	ErrSlugRequired                  = cmspages.ErrSlugRequired
	ErrSlugInvalid                   = cmspages.ErrSlugInvalid
	ErrSlugExists                    = cmspages.ErrSlugExists
	ErrPathExists                    = cmspages.ErrPathExists
	ErrPathConflict                  = cmspages.ErrPathConflict
	ErrUnknownLocale                 = cmspages.ErrUnknownLocale
	ErrInvalidLocale                 = cmspages.ErrInvalidLocale
	ErrSourceNotFound                = cmspages.ErrSourceNotFound
	ErrTranslationAlreadyExists      = cmspages.ErrTranslationAlreadyExists
	ErrTranslationInvariantViolation = cmspages.ErrTranslationInvariantViolation
	ErrDuplicateLocale               = cmspages.ErrDuplicateLocale
	ErrParentNotFound                = cmspages.ErrParentNotFound
	ErrNoPageTranslations            = cmspages.ErrNoPageTranslations
	ErrDefaultLocaleRequired         = cmspages.ErrDefaultLocaleRequired
	ErrTemplateUnknown               = cmspages.ErrTemplateUnknown
	ErrPageRequired                  = cmspages.ErrPageRequired
	ErrVersioningDisabled            = cmspages.ErrVersioningDisabled
	ErrPageVersionRequired           = cmspages.ErrPageVersionRequired
	ErrVersionAlreadyPublished       = cmspages.ErrVersionAlreadyPublished
	ErrVersionRetentionExceeded      = cmspages.ErrVersionRetentionExceeded
	ErrVersionConflict               = cmspages.ErrVersionConflict
	ErrSchedulingDisabled            = cmspages.ErrSchedulingDisabled
	ErrScheduleWindowInvalid         = cmspages.ErrScheduleWindowInvalid
	ErrScheduleTimestampInvalid      = cmspages.ErrScheduleTimestampInvalid
	ErrPageMediaReferenceRequired    = cmspages.ErrPageMediaReferenceRequired
	ErrPageSoftDeleteUnsupported     = cmspages.ErrPageSoftDeleteUnsupported
	ErrPageTranslationsDisabled      = cmspages.ErrPageTranslationsDisabled
	ErrPageTranslationNotFound       = cmspages.ErrPageTranslationNotFound
	ErrPageParentCycle               = cmspages.ErrPageParentCycle
	ErrPageDuplicateSlug             = cmspages.ErrPageDuplicateSlug
)
