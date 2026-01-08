package blocks

import cmsblocks "github.com/goliatone/go-cms/blocks"

type (
	Definition                      = cmsblocks.Definition
	Instance                        = cmsblocks.Instance
	Translation                     = cmsblocks.Translation
	InstanceVersion                 = cmsblocks.InstanceVersion
	BlockVersionSnapshot            = cmsblocks.BlockVersionSnapshot
	BlockVersionTranslationSnapshot = cmsblocks.BlockVersionTranslationSnapshot
)

var BlockVersionSnapshotSchema = cmsblocks.BlockVersionSnapshotSchema
