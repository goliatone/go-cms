package blocks

import cmsblocks "github.com/goliatone/go-cms/blocks"

type (
	Service                       = cmsblocks.Service
	RegisterDefinitionInput       = cmsblocks.RegisterDefinitionInput
	UpdateDefinitionInput         = cmsblocks.UpdateDefinitionInput
	CreateDefinitionVersionInput  = cmsblocks.CreateDefinitionVersionInput
	DeleteDefinitionRequest       = cmsblocks.DeleteDefinitionRequest
	CreateInstanceInput           = cmsblocks.CreateInstanceInput
	UpdateInstanceInput           = cmsblocks.UpdateInstanceInput
	DeleteInstanceRequest         = cmsblocks.DeleteInstanceRequest
	AddTranslationInput           = cmsblocks.AddTranslationInput
	UpdateTranslationInput        = cmsblocks.UpdateTranslationInput
	DeleteTranslationRequest      = cmsblocks.DeleteTranslationRequest
	CreateInstanceDraftRequest    = cmsblocks.CreateInstanceDraftRequest
	PublishInstanceDraftRequest   = cmsblocks.PublishInstanceDraftRequest
	RestoreInstanceVersionRequest = cmsblocks.RestoreInstanceVersionRequest
)

var (
	ErrDefinitionNameRequired          = cmsblocks.ErrDefinitionNameRequired
	ErrDefinitionSlugRequired          = cmsblocks.ErrDefinitionSlugRequired
	ErrDefinitionSlugInvalid           = cmsblocks.ErrDefinitionSlugInvalid
	ErrDefinitionSlugExists            = cmsblocks.ErrDefinitionSlugExists
	ErrDefinitionSchemaRequired        = cmsblocks.ErrDefinitionSchemaRequired
	ErrDefinitionSchemaInvalid         = cmsblocks.ErrDefinitionSchemaInvalid
	ErrDefinitionSchemaVersionInvalid  = cmsblocks.ErrDefinitionSchemaVersionInvalid
	ErrDefinitionExists                = cmsblocks.ErrDefinitionExists
	ErrDefinitionIDRequired            = cmsblocks.ErrDefinitionIDRequired
	ErrDefinitionInUse                 = cmsblocks.ErrDefinitionInUse
	ErrDefinitionSoftDeleteUnsupported = cmsblocks.ErrDefinitionSoftDeleteUnsupported
	ErrDefinitionVersionRequired       = cmsblocks.ErrDefinitionVersionRequired
	ErrDefinitionVersionExists         = cmsblocks.ErrDefinitionVersionExists
	ErrDefinitionVersioningDisabled    = cmsblocks.ErrDefinitionVersioningDisabled

	ErrInstanceDefinitionRequired    = cmsblocks.ErrInstanceDefinitionRequired
	ErrInstanceRegionRequired        = cmsblocks.ErrInstanceRegionRequired
	ErrInstancePositionInvalid       = cmsblocks.ErrInstancePositionInvalid
	ErrInstanceUpdaterRequired       = cmsblocks.ErrInstanceUpdaterRequired
	ErrInstanceSoftDeleteUnsupported = cmsblocks.ErrInstanceSoftDeleteUnsupported

	ErrTranslationContentRequired       = cmsblocks.ErrTranslationContentRequired
	ErrTranslationExists                = cmsblocks.ErrTranslationExists
	ErrTranslationLocaleRequired        = cmsblocks.ErrTranslationLocaleRequired
	ErrTranslationSchemaInvalid         = cmsblocks.ErrTranslationSchemaInvalid
	ErrTranslationNotFound              = cmsblocks.ErrTranslationNotFound
	ErrTranslationMinimum               = cmsblocks.ErrTranslationMinimum
	ErrTranslationsDisabled             = cmsblocks.ErrTranslationsDisabled
	ErrInstanceIDRequired               = cmsblocks.ErrInstanceIDRequired
	ErrVersioningDisabled               = cmsblocks.ErrVersioningDisabled
	ErrInstanceVersionRequired          = cmsblocks.ErrInstanceVersionRequired
	ErrInstanceVersionConflict          = cmsblocks.ErrInstanceVersionConflict
	ErrInstanceVersionAlreadyPublished  = cmsblocks.ErrInstanceVersionAlreadyPublished
	ErrInstanceVersionRetentionExceeded = cmsblocks.ErrInstanceVersionRetentionExceeded
	ErrMediaReferenceRequired           = cmsblocks.ErrMediaReferenceRequired
	ErrBlockSchemaMigrationRequired     = cmsblocks.ErrBlockSchemaMigrationRequired
	ErrBlockSchemaValidationFailed      = cmsblocks.ErrBlockSchemaValidationFailed
)
