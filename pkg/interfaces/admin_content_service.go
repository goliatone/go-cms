package interfaces

import (
	"context"
	"errors"
	"strings"
)

// ErrAdminContentFamilyReadUnsupported marks unavailable optimized family reads.
var ErrAdminContentFamilyReadUnsupported = errors.New("admin content family read unsupported")

// AdminContentFamilyReadUnsupportedError carries structured unsupported details.
type AdminContentFamilyReadUnsupportedError struct {
	Reason   string
	Metadata map[string]any
}

func (e AdminContentFamilyReadUnsupportedError) Error() string {
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		return ErrAdminContentFamilyReadUnsupported.Error()
	}
	return ErrAdminContentFamilyReadUnsupported.Error() + ": " + reason
}

func (e AdminContentFamilyReadUnsupportedError) Unwrap() error {
	return ErrAdminContentFamilyReadUnsupported
}

// AdminContentReadService exposes list and detail reads for admin content records.
type AdminContentReadService interface {
	List(ctx context.Context, opts AdminContentListOptions) ([]AdminContentRecord, int, error)
	Get(ctx context.Context, id string, opts AdminContentGetOptions) (*AdminContentRecord, error)
}

// AdminContentFamilyReadService exposes optimized grouped-family reads.
type AdminContentFamilyReadService interface {
	ListFamilies(ctx context.Context, opts AdminContentFamilyListOptions) (AdminContentFamilyListResult, error)
}

// AdminContentWriteService exposes admin-oriented content mutations.
type AdminContentWriteService interface {
	Create(ctx context.Context, req AdminContentCreateRequest) (*AdminContentRecord, error)
	Update(ctx context.Context, req AdminContentUpdateRequest) (*AdminContentRecord, error)
	Delete(ctx context.Context, req AdminContentDeleteRequest) error
	CreateTranslation(ctx context.Context, req AdminContentCreateTranslationRequest) (*AdminContentRecord, error)
}
