package interfaces

import "context"

// AdminBlockReadService exposes admin-oriented block definition and instance reads.
type AdminBlockReadService interface {
	ListDefinitions(ctx context.Context, opts AdminBlockDefinitionListOptions) ([]AdminBlockDefinitionRecord, int, error)
	GetDefinition(ctx context.Context, id string, opts AdminBlockDefinitionGetOptions) (*AdminBlockDefinitionRecord, error)
	ListDefinitionVersions(ctx context.Context, definitionID string) ([]AdminBlockDefinitionVersionRecord, error)
	ListContentBlocks(ctx context.Context, contentID string, opts AdminBlockListOptions) ([]AdminBlockRecord, error)
}

// AdminBlockWriteService exposes admin-oriented block definition and instance mutations.
type AdminBlockWriteService interface {
	CreateDefinition(ctx context.Context, req AdminBlockDefinitionCreateRequest) (*AdminBlockDefinitionRecord, error)
	UpdateDefinition(ctx context.Context, req AdminBlockDefinitionUpdateRequest) (*AdminBlockDefinitionRecord, error)
	DeleteDefinition(ctx context.Context, req AdminBlockDefinitionDeleteRequest) error
	SaveBlock(ctx context.Context, req AdminBlockSaveRequest) (*AdminBlockRecord, error)
	DeleteBlock(ctx context.Context, req AdminBlockDeleteRequest) error
}
