package interfaces

import "context"

// AdminContentReadService exposes list and detail reads for admin content records.
type AdminContentReadService interface {
	List(ctx context.Context, opts AdminContentListOptions) ([]AdminContentRecord, int, error)
	Get(ctx context.Context, id string, opts AdminContentGetOptions) (*AdminContentRecord, error)
}

// AdminContentWriteService exposes admin-oriented content mutations.
type AdminContentWriteService interface {
	Create(ctx context.Context, req AdminContentCreateRequest) (*AdminContentRecord, error)
	Update(ctx context.Context, req AdminContentUpdateRequest) (*AdminContentRecord, error)
	Delete(ctx context.Context, req AdminContentDeleteRequest) error
	CreateTranslation(ctx context.Context, req AdminContentCreateTranslationRequest) (*AdminContentRecord, error)
}
