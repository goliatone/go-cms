package noop

import (
	"context"
	"io"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
	"github.com/google/uuid"
)

func Cache() interfaces.CacheProvider {
	return cacheAdapter{}
}

type cacheAdapter struct{}

func (cacheAdapter) Get(context.Context, string) (any, error)              { return nil, nil }
func (cacheAdapter) Set(context.Context, string, any, time.Duration) error { return nil }
func (cacheAdapter) Delete(context.Context, string) error                  { return nil }
func (cacheAdapter) Clear(context.Context) error                           { return nil }

func Template() interfaces.TemplateRenderer { return templateAdapter{} }

type templateAdapter struct{}

func (templateAdapter) Render(string, any, ...io.Writer) (string, error)         { return "", nil }
func (templateAdapter) RenderTemplate(string, any, ...io.Writer) (string, error) { return "", nil }
func (templateAdapter) RenderString(string, any, ...io.Writer) (string, error)   { return "", nil }
func (templateAdapter) RegisterFilter(string, func(any, any) (any, error)) error { return nil }
func (templateAdapter) GlobalContext(any) error                                  { return nil }

func Media() interfaces.MediaProvider { return mediaAdapter{} }

type mediaAdapter struct{}

func (mediaAdapter) GetURL(context.Context, string) (string, error) { return "", nil }
func (mediaAdapter) GetMetadata(context.Context, string) (interfaces.MediaMetadata, error) {
	return interfaces.MediaMetadata{}, nil
}

func Auth() interfaces.AuthService { return authService{} }

type authService struct{}

func (authService) Authenticator() interfaces.Authenticator { return noopAuthenticator{} }
func (authService) TokenService() interfaces.TokenService   { return noopTokenService{} }
func (authService) TemplateHelpers() map[string]any         { return map[string]any{} }

type noopAuthenticator struct{}

func (noopAuthenticator) Login(context.Context, string, string) (string, error) { return "", nil }
func (noopAuthenticator) Impersonate(context.Context, string) (string, error)   { return "", nil }
func (noopAuthenticator) SessionFromToken(string) (interfaces.Session, error) {
	return noopSession{}, nil
}
func (noopAuthenticator) IdentityFromSession(context.Context, interfaces.Session) (interfaces.Identity, error) {
	return noopIdentity{}, nil
}
func (noopAuthenticator) TokenService() interfaces.TokenService { return noopTokenService{} }

type noopSession struct{}

func (noopSession) GetUserID() string               { return "" }
func (noopSession) GetUserUUID() (uuid.UUID, error) { return uuid.Nil, nil }
func (noopSession) GetAudience() []string           { return nil }
func (noopSession) GetIssuer() string               { return "" }
func (noopSession) GetIssuedAt() *time.Time         { return nil }
func (noopSession) GetData() map[string]any         { return nil }

type noopIdentity struct{}

func (noopIdentity) ID() string       { return "" }
func (noopIdentity) Username() string { return "" }
func (noopIdentity) Email() string    { return "" }
func (noopIdentity) Role() string     { return "" }

type noopTokenService struct{}

func (noopTokenService) Generate(interfaces.Identity, map[string]string) (string, error) {
	return "", nil
}

func (noopTokenService) Validate(string) (interfaces.AuthClaims, error) { return noopClaims{}, nil }

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
