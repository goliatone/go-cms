package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AuthClaims matches go-auth AuthClaims behaviour.
type AuthClaims interface {
	Subject() string
	UserID() string
	Role() string
	CanRead(resource string) bool
	CanEdit(resource string) bool
	CanCreate(resource string) bool
	CanDelete(resource string) bool
	HasRole(role string) bool
	IsAtLeast(minRole string) bool
	Expires() time.Time
	IssuedAt() time.Time
}

// Session mirrors go-auth Session interface.
type Session interface {
	GetUserID() string
	GetUserUUID() (uuid.UUID, error)
	GetAudience() []string
	GetIssuer() string
	GetIssuedAt() *time.Time
	GetData() map[string]any
}

// RoleCapableSession extends Session with role helpers.
type RoleCapableSession interface {
	Session
	CanRead(resource string) bool
	CanEdit(resource string) bool
	CanCreate(resource string) bool
	CanDelete(resource string) bool
	HasRole(role string) bool
	IsAtLeast(minRole UserRole) bool
}

// TokenService provides JWT operations.
type TokenService interface {
	Generate(identity Identity, resourceRoles map[string]string) (string, error)
	Validate(token string) (AuthClaims, error)
}

type Authenticator interface {
	Login(ctx context.Context, identifier, password string) (string, error)
	Impersonate(ctx context.Context, identifier string) (string, error)
	SessionFromToken(token string) (Session, error)
	IdentityFromSession(ctx context.Context, session Session) (Identity, error)
	TokenService() TokenService
}

type Identity interface {
	ID() string
	Username() string
	Email() string
	Role() string
}

type Config interface {
	GetSigningKey() string
	GetSigningMethod() string
	GetContextKey() string
	GetTokenExpiration() int
	GetExtendedTokenDuration() int
	GetTokenLookup() string
	GetAuthScheme() string
	GetIssuer() string
	GetAudience() []string
	GetRejectedRouteKey() string
	GetRejectedRouteDefault() string
}

type IdentityProvider interface {
	VerifyIdentity(ctx context.Context, identifier, password string) (Identity, error)
	FindIdentityByIdentifier(ctx context.Context, identifier string) (Identity, error)
}

type ResourceRoleProvider interface {
	FindResourceRoles(ctx context.Context, identity Identity) (map[string]string, error)
}

type LoginPayload interface {
	GetIdentifier() string
	GetPassword() string
	GetExtendedSession() bool
}

type UserRole string

const (
	RoleGuest  UserRole = "guest"
	RoleMember UserRole = "member"
	RoleAdmin  UserRole = "admin"
	RoleOwner  UserRole = "owner"
)

func (r UserRole) IsValid() bool {
	switch r {
	case RoleGuest, RoleMember, RoleAdmin, RoleOwner:
		return true
	default:
		return false
	}
}

func (r UserRole) CanRead() bool {
	switch r {
	case RoleGuest, RoleMember, RoleAdmin, RoleOwner:
		return true
	default:
		return false
	}
}

func (r UserRole) CanEdit() bool {
	switch r {
	case RoleMember, RoleAdmin, RoleOwner:
		return true
	default:
		return false
	}
}

func (r UserRole) CanCreate() bool {
	switch r {
	case RoleAdmin, RoleOwner:
		return true
	default:
		return false
	}
}

func (r UserRole) CanDelete() bool {
	switch r {
	case RoleOwner:
		return true
	default:
		return false
	}
}

func (r UserRole) IsAtLeast(minRole UserRole) bool {
	roleHierarchy := map[UserRole]int{
		RoleGuest:  0,
		RoleMember: 1,
		RoleAdmin:  2,
		RoleOwner:  3,
	}

	currentLevel, ok := roleHierarchy[r]
	if !ok {
		return false
	}

	minLevel, ok := roleHierarchy[minRole]
	if !ok {
		return false
	}

	return currentLevel >= minLevel
}

type AuthService interface {
	Authenticator() Authenticator
	TokenService() TokenService
	TemplateHelpers() map[string]any
}
