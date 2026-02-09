package content

import cmscontent "github.com/goliatone/go-cms/content"

type (
	Service                         = cmscontent.Service
	TranslationCheckOptions         = cmscontent.TranslationCheckOptions
	CreateContentRequest            = cmscontent.CreateContentRequest
	ContentTranslationInput         = cmscontent.ContentTranslationInput
	UpdateContentRequest            = cmscontent.UpdateContentRequest
	DeleteContentRequest            = cmscontent.DeleteContentRequest
	UpdateContentTranslationRequest = cmscontent.UpdateContentTranslationRequest
	DeleteContentTranslationRequest = cmscontent.DeleteContentTranslationRequest
	CreateContentDraftRequest       = cmscontent.CreateContentDraftRequest
	PublishContentDraftRequest      = cmscontent.PublishContentDraftRequest
	PreviewContentDraftRequest      = cmscontent.PreviewContentDraftRequest
	RestoreContentVersionRequest    = cmscontent.RestoreContentVersionRequest
	ContentPreview                  = cmscontent.ContentPreview
	ScheduleContentRequest          = cmscontent.ScheduleContentRequest

	ContentTypeService       = cmscontent.ContentTypeService
	CreateContentTypeRequest = cmscontent.CreateContentTypeRequest
	UpdateContentTypeRequest = cmscontent.UpdateContentTypeRequest
	DeleteContentTypeRequest = cmscontent.DeleteContentTypeRequest
)

var (
	ErrContentTypeRequired                 = cmscontent.ErrContentTypeRequired
	ErrSlugRequired                        = cmscontent.ErrSlugRequired
	ErrSlugInvalid                         = cmscontent.ErrSlugInvalid
	ErrSlugExists                          = cmscontent.ErrSlugExists
	ErrNoTranslations                      = cmscontent.ErrNoTranslations
	ErrDefaultLocaleRequired               = cmscontent.ErrDefaultLocaleRequired
	ErrDuplicateLocale                     = cmscontent.ErrDuplicateLocale
	ErrUnknownLocale                       = cmscontent.ErrUnknownLocale
	ErrContentSchemaInvalid                = cmscontent.ErrContentSchemaInvalid
	ErrContentSoftDeleteUnsupported        = cmscontent.ErrContentSoftDeleteUnsupported
	ErrContentIDRequired                   = cmscontent.ErrContentIDRequired
	ErrContentMetadataInvalid              = cmscontent.ErrContentMetadataInvalid
	ErrVersioningDisabled                  = cmscontent.ErrVersioningDisabled
	ErrContentVersionRequired              = cmscontent.ErrContentVersionRequired
	ErrContentVersionConflict              = cmscontent.ErrContentVersionConflict
	ErrContentVersionAlreadyPublished      = cmscontent.ErrContentVersionAlreadyPublished
	ErrContentVersionRetentionExceeded     = cmscontent.ErrContentVersionRetentionExceeded
	ErrSchedulingDisabled                  = cmscontent.ErrSchedulingDisabled
	ErrScheduleWindowInvalid               = cmscontent.ErrScheduleWindowInvalid
	ErrScheduleTimestampInvalid            = cmscontent.ErrScheduleTimestampInvalid
	ErrContentTranslationsDisabled         = cmscontent.ErrContentTranslationsDisabled
	ErrContentTranslationNotFound          = cmscontent.ErrContentTranslationNotFound
	ErrContentSchemaMigrationRequired      = cmscontent.ErrContentSchemaMigrationRequired
	ErrContentTranslationLookupUnsupported = cmscontent.ErrContentTranslationLookupUnsupported
	ErrEmbeddedBlocksResolverMissing       = cmscontent.ErrEmbeddedBlocksResolverMissing

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
