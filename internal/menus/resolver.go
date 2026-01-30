package menus

import (
	"context"

	"github.com/google/uuid"
)

// ResolveRequest carries the context required for URL resolvers to build links.
type ResolveRequest struct {
	MenuCode string
	Item     *MenuItem
	Locale   string
	LocaleID uuid.UUID
}

// URLResolver allows callers to override how menu URLs are generated.
type URLResolver interface {
	Resolve(ctx context.Context, req ResolveRequest) (string, error)
}

type defaultURLResolver struct {
	service *service
}

func (r *defaultURLResolver) Resolve(ctx context.Context, req ResolveRequest) (string, error) {
	if req.Item == nil {
		return "", nil
	}
	return r.service.resolveURLForTarget(ctx, req.Item.Target, req.Item.EnvironmentID, req.LocaleID)
}

// WithURLResolver overrides the default URL resolver used by the menu service.
func WithURLResolver(resolver URLResolver) ServiceOption {
	return func(s *service) {
		if resolver != nil {
			s.urlResolver = resolver
		}
	}
}
