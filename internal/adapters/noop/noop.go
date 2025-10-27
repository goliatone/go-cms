package noop

import (
	"context"
	"io"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

// Cache returns an interfaces.CacheProvider that does nothing.
func Cache() interfaces.CacheProvider {
	return cacheAdapter{}
}

type cacheAdapter struct{}

func (cacheAdapter) Get(context.Context, string) (any, error) {
	return nil, nil
}

func (cacheAdapter) Set(context.Context, string, any, time.Duration) error {
	return nil
}

func (cacheAdapter) Delete(context.Context, string) error {
	return nil
}

func (cacheAdapter) Clear(context.Context) error {
	return nil
}

// Template returns a template renderer that bypasses rendering.
func Template() interfaces.TemplateRenderer {
	return templateAdapter{}
}

type templateAdapter struct{}

func (templateAdapter) Render(string, any, ...io.Writer) (string, error) {
	return "", nil
}

func (templateAdapter) RenderTemplate(string, any, ...io.Writer) (string, error) {
	return "", nil
}

func (templateAdapter) RenderString(string, any, ...io.Writer) (string, error) {
	return "", nil
}

func (templateAdapter) RegisterFilter(string, func(any, any) (any, error)) error {
	return nil
}

func (templateAdapter) GlobalContext(any) error {
	return nil
}

// Media returns a media provider that cannot resolve assets.
func Media() interfaces.MediaProvider {
	return mediaAdapter{}
}

type mediaAdapter struct{}

func (mediaAdapter) Resolve(_ context.Context, req interfaces.MediaResolveRequest) (*interfaces.MediaAsset, error) {
	return &interfaces.MediaAsset{
		Reference:  req.Reference,
		Source:     nil,
		Renditions: map[string]*interfaces.MediaResource{},
		Metadata: interfaces.MediaMetadata{
			ID: req.Reference.ID,
		},
	}, nil
}

func (mediaAdapter) ResolveBatch(ctx context.Context, reqs []interfaces.MediaResolveRequest) (map[string]*interfaces.MediaAsset, error) {
	result := make(map[string]*interfaces.MediaAsset, len(reqs))
	for _, req := range reqs {
		ref := req.Reference
		asset, _ := (mediaAdapter{}).Resolve(ctx, req)
		key := ref.ID
		if key == "" {
			key = ref.Path
		}
		result[key] = asset
	}
	return result, nil
}

func (mediaAdapter) Invalidate(context.Context, ...interfaces.MediaReference) error {
	return nil
}

// Auth returns a no-op auth service compatible with go-auth interfaces.
func Auth() interfaces.AuthService {
	return authService{}
}

type authService struct{}

func (authService) Authenticator() interfaces.Authenticator {
	return noopAuthenticator{}
}

func (authService) TokenService() interfaces.TokenService {
	return noopTokenService{}
}

func (authService) TemplateHelpers() map[string]any {
	return map[string]any{}
}

type noopAuthenticator struct{}

func (noopAuthenticator) Login(context.Context, string, string) (string, error) {
	return "", nil
}

func (noopAuthenticator) Impersonate(context.Context, string) (string, error) {
	return "", nil
}

func (noopAuthenticator) SessionFromToken(string) (interfaces.Session, error) {
	return noopSession{}, nil
}

func (noopAuthenticator) IdentityFromSession(context.Context, interfaces.Session) (interfaces.Identity, error) {
	return noopIdentity{}, nil
}

func (noopAuthenticator) TokenService() interfaces.TokenService {
	return noopTokenService{}
}

type noopSession struct{}

func (noopSession) GetUserID() string { return "" }

func (noopSession) GetUserUUID() (uuid.UUID, error) { return uuid.Nil, nil }

func (noopSession) GetAudience() []string { return nil }

func (noopSession) GetIssuer() string { return "" }

func (noopSession) GetIssuedAt() *time.Time { return nil }

func (noopSession) GetData() map[string]any { return nil }

type noopIdentity struct{}

func (noopIdentity) ID() string       { return "" }
func (noopIdentity) Username() string { return "" }
func (noopIdentity) Email() string    { return "" }
func (noopIdentity) Role() string     { return "" }

type noopTokenService struct{}

func (noopTokenService) Generate(interfaces.Identity, map[string]string) (string, error) {
	return "", nil
}

func (noopTokenService) Validate(string) (interfaces.AuthClaims, error) {
	return noopClaims{}, nil
}

type noopClaims struct{}

func (noopClaims) Subject() string       { return "" }
func (noopClaims) UserID() string        { return "" }
func (noopClaims) Role() string          { return "" }
func (noopClaims) CanRead(string) bool   { return false }
func (noopClaims) CanEdit(string) bool   { return false }
func (noopClaims) CanCreate(string) bool { return false }
func (noopClaims) CanDelete(string) bool { return false }
func (noopClaims) HasRole(string) bool   { return false }
func (noopClaims) IsAtLeast(string) bool { return false }
func (noopClaims) Expires() time.Time    { return time.Time{} }
func (noopClaims) IssuedAt() time.Time   { return time.Time{} }
