package content

import cmscontent "github.com/goliatone/go-cms/content"

type (
	Service                            = cmscontent.Service
	TranslationCreator                 = cmscontent.TranslationCreator
	TranslationCheckOptions            = cmscontent.TranslationCheckOptions
	ProjectionTranslationMode          = cmscontent.ProjectionTranslationMode
	TranslationConflictStrategy        = cmscontent.TranslationConflictStrategy
	CreateContentRequest               = cmscontent.CreateContentRequest
	ContentTranslationInput            = cmscontent.ContentTranslationInput
	UpdateContentRequest               = cmscontent.UpdateContentRequest
	DeleteContentRequest               = cmscontent.DeleteContentRequest
	CreateContentTranslationRequest    = cmscontent.CreateContentTranslationRequest
	UpdateContentTranslationRequest    = cmscontent.UpdateContentTranslationRequest
	DeleteContentTranslationRequest    = cmscontent.DeleteContentTranslationRequest
	CreateContentDraftRequest          = cmscontent.CreateContentDraftRequest
	PublishContentDraftRequest         = cmscontent.PublishContentDraftRequest
	PreviewContentDraftRequest         = cmscontent.PreviewContentDraftRequest
	RestoreContentVersionRequest       = cmscontent.RestoreContentVersionRequest
	ContentPreview                     = cmscontent.ContentPreview
	ScheduleContentRequest             = cmscontent.ScheduleContentRequest
	TranslationAlreadyExistsError      = cmscontent.TranslationAlreadyExistsError
	InvalidLocaleError                 = cmscontent.InvalidLocaleError
	SourceNotFoundError                = cmscontent.SourceNotFoundError
	SlugConflictError                  = cmscontent.SlugConflictError
	TranslationInvariantViolationError = cmscontent.TranslationInvariantViolationError

	ContentTypeService       = cmscontent.ContentTypeService
	CreateContentTypeRequest = cmscontent.CreateContentTypeRequest
	UpdateContentTypeRequest = cmscontent.UpdateContentTypeRequest
	DeleteContentTypeRequest = cmscontent.DeleteContentTypeRequest
)

const (
	ContentProjectionAdmin         = cmscontent.ContentProjectionAdmin
	ContentProjectionDerivedFields = cmscontent.ContentProjectionDerivedFields

	ProjectionTranslationModeAutoLoad = cmscontent.ProjectionTranslationModeAutoLoad
	ProjectionTranslationModeNoop     = cmscontent.ProjectionTranslationModeNoop
	ProjectionTranslationModeError    = cmscontent.ProjectionTranslationModeError

	TranslationConflictStrict = cmscontent.TranslationConflictStrict
)

var (
	ErrContentTypeRequired                   = cmscontent.ErrContentTypeRequired
	ErrSlugRequired                          = cmscontent.ErrSlugRequired
	ErrSlugInvalid                           = cmscontent.ErrSlugInvalid
	ErrSlugExists                            = cmscontent.ErrSlugExists
	ErrSlugConflict                          = cmscontent.ErrSlugConflict
	ErrNoTranslations                        = cmscontent.ErrNoTranslations
	ErrDefaultLocaleRequired                 = cmscontent.ErrDefaultLocaleRequired
	ErrDuplicateLocale                       = cmscontent.ErrDuplicateLocale
	ErrUnknownLocale                         = cmscontent.ErrUnknownLocale
	ErrInvalidLocale                         = cmscontent.ErrInvalidLocale
	ErrSourceNotFound                        = cmscontent.ErrSourceNotFound
	ErrTranslationAlreadyExists              = cmscontent.ErrTranslationAlreadyExists
	ErrTranslationInvariantViolation         = cmscontent.ErrTranslationInvariantViolation
	ErrContentSchemaInvalid                  = cmscontent.ErrContentSchemaInvalid
	ErrContentSoftDeleteUnsupported          = cmscontent.ErrContentSoftDeleteUnsupported
	ErrContentIDRequired                     = cmscontent.ErrContentIDRequired
	ErrContentMetadataInvalid                = cmscontent.ErrContentMetadataInvalid
	ErrVersioningDisabled                    = cmscontent.ErrVersioningDisabled
	ErrContentVersionRequired                = cmscontent.ErrContentVersionRequired
	ErrContentVersionConflict                = cmscontent.ErrContentVersionConflict
	ErrContentVersionAlreadyPublished        = cmscontent.ErrContentVersionAlreadyPublished
	ErrContentVersionRetentionExceeded       = cmscontent.ErrContentVersionRetentionExceeded
	ErrSchedulingDisabled                    = cmscontent.ErrSchedulingDisabled
	ErrScheduleWindowInvalid                 = cmscontent.ErrScheduleWindowInvalid
	ErrScheduleTimestampInvalid              = cmscontent.ErrScheduleTimestampInvalid
	ErrContentTranslationsDisabled           = cmscontent.ErrContentTranslationsDisabled
	ErrContentTranslationNotFound            = cmscontent.ErrContentTranslationNotFound
	ErrContentSchemaMigrationRequired        = cmscontent.ErrContentSchemaMigrationRequired
	ErrContentTranslationLookupUnsupported   = cmscontent.ErrContentTranslationLookupUnsupported
	ErrContentProjectionModeInvalid          = cmscontent.ErrContentProjectionModeInvalid
	ErrContentProjectionUnsupported          = cmscontent.ErrContentProjectionUnsupported
	ErrContentProjectionRequiresTranslations = cmscontent.ErrContentProjectionRequiresTranslations
	ErrEmbeddedBlocksResolverMissing         = cmscontent.ErrEmbeddedBlocksResolverMissing

	ErrContentTypeNameRequired   = cmscontent.ErrContentTypeNameRequired
	ErrContentTypeSchemaRequired = cmscontent.ErrContentTypeSchemaRequired
	ErrContentTypeSchemaInvalid  = cmscontent.ErrContentTypeSchemaInvalid
	ErrContentTypeIDRequired     = cmscontent.ErrContentTypeIDRequired
	ErrContentTypeSlugInvalid    = cmscontent.ErrContentTypeSlugInvalid
	ErrContentTypeSchemaVersion  = cmscontent.ErrContentTypeSchemaVersion
	ErrContentTypeSchemaBreaking = cmscontent.ErrContentTypeSchemaBreaking
	ErrContentTypeStatusInvalid  = cmscontent.ErrContentTypeStatusInvalid
	ErrContentTypeStatusChange   = cmscontent.ErrContentTypeStatusChange
)
