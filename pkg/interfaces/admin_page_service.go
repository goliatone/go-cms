package interfaces

import "context"

// AdminPageReadService exposes list and detail reads for admin page records.
type AdminPageReadService interface {
	List(ctx context.Context, opts AdminPageListOptions) ([]AdminPageRecord, int, error)
	Get(ctx context.Context, id string, opts AdminPageGetOptions) (*AdminPageRecord, error)
}
