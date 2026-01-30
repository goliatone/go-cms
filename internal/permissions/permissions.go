package permissions

import (
	"context"
	"errors"
	"strings"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

type Action string

const (
	ActionRead    Action = "read"
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionDelete  Action = "delete"
	ActionPublish Action = "publish"
)

const (
	ResourceContentTypes = "content_types"
	ResourceBlocks       = "blocks"
	ResourceEnvironments = "environments"
	ResourceContent      = "content"
	ResourcePages        = "pages"
	ResourceMenus        = "menus"
)

const (
	ContentTypesRead    = "content_types:read"
	ContentTypesCreate  = "content_types:create"
	ContentTypesUpdate  = "content_types:update"
	ContentTypesDelete  = "content_types:delete"
	ContentTypesPublish = "content_types:publish"

	BlocksRead   = "blocks:read"
	BlocksCreate = "blocks:create"
	BlocksUpdate = "blocks:update"
	BlocksDelete = "blocks:delete"

	EnvironmentsRead   = "environments:read"
	EnvironmentsCreate = "environments:create"
	EnvironmentsUpdate = "environments:update"
	EnvironmentsDelete = "environments:delete"
)

var ErrPermissionDenied = errors.New("permissions: denied")

type Error struct {
	Permission string
}

func (e Error) Error() string {
	if strings.TrimSpace(e.Permission) == "" {
		return "permission denied"
	}
	return "permission denied: " + e.Permission
}

func (e Error) Unwrap() error {
	return ErrPermissionDenied
}

// PermissionSet captures common CRUD permission tokens.
type PermissionSet struct {
	Read    string `json:"read,omitempty"`
	Create  string `json:"create,omitempty"`
	Update  string `json:"update,omitempty"`
	Delete  string `json:"delete,omitempty"`
	Publish string `json:"publish,omitempty"`
}

// BuilderContentTypePermissions returns permissions for content type builder actions.
func BuilderContentTypePermissions() PermissionSet {
	return ResourcePermissions(ResourceContentTypes, true)
}

// BlockLibraryPermissions returns permissions for block library actions.
func BlockLibraryPermissions() PermissionSet {
	return ResourcePermissions(ResourceBlocks, false)
}

// ContentTypePermissions returns the permission set for a specific content type slug.
func ContentTypePermissions(slug string) PermissionSet {
	return ResourcePermissions(slug, true)
}

// ResourcePermissions creates a permission set for a resource.
func ResourcePermissions(resource string, includePublish bool) PermissionSet {
	normalized := normalizeToken(resource)
	perms := PermissionSet{
		Read:   Join(normalized, ActionRead),
		Create: Join(normalized, ActionCreate),
		Update: Join(normalized, ActionUpdate),
		Delete: Join(normalized, ActionDelete),
	}
	if includePublish {
		perms.Publish = Join(normalized, ActionPublish)
	}
	return perms
}

// Join builds a permission token from resource and action.
func Join(resource string, action Action) string {
	res := normalizeToken(resource)
	act := normalizeToken(string(action))
	if res == "" || act == "" {
		return ""
	}
	return res + ":" + act
}

// List returns the non-empty permissions in the set.
func (p PermissionSet) List() []string {
	out := make([]string, 0, 5)
	if p.Read != "" {
		out = append(out, p.Read)
	}
	if p.Create != "" {
		out = append(out, p.Create)
	}
	if p.Update != "" {
		out = append(out, p.Update)
	}
	if p.Delete != "" {
		out = append(out, p.Delete)
	}
	if p.Publish != "" {
		out = append(out, p.Publish)
	}
	return out
}

type Checker interface {
	Allowed(permission string) bool
}

type CheckerFunc func(permission string) bool

func (fn CheckerFunc) Allowed(permission string) bool {
	return fn(permission)
}

type Set map[string]struct{}

func NewSet(perms ...string) Set {
	set := Set{}
	for _, perm := range perms {
		normalized := normalizePermission(perm)
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	return set
}

func (s Set) Allowed(permission string) bool {
	if len(s) == 0 {
		return false
	}
	normalized := normalizePermission(permission)
	if normalized == "" {
		return false
	}
	if _, ok := s[normalized]; ok {
		return true
	}
	resource, _ := splitPermission(normalized)
	if resource != "" {
		if _, ok := s[resource+":*"]; ok {
			return true
		}
	}
	if _, ok := s["*"]; ok {
		return true
	}
	return false
}

type Permissioner interface {
	HasPermission(permission string) bool
}

type contextKey string

const checkerKey contextKey = "cms.permissions.checker"

// WithChecker stores a permission checker on the context.
func WithChecker(ctx context.Context, checker Checker) context.Context {
	if ctx == nil || checker == nil {
		return ctx
	}
	return context.WithValue(ctx, checkerKey, checker)
}

// WithPermissions stores a static permission set on the context.
func WithPermissions(ctx context.Context, perms ...string) context.Context {
	if ctx == nil || len(perms) == 0 {
		return ctx
	}
	return WithChecker(ctx, NewSet(perms...))
}

// WithClaims stores auth claims on the context for permission checks.
func WithClaims(ctx context.Context, claims interfaces.AuthClaims) context.Context {
	if ctx == nil || claims == nil {
		return ctx
	}
	return context.WithValue(ctx, checkerKey, claims)
}

// WithSession stores a role-capable session on the context for permission checks.
func WithSession(ctx context.Context, session interfaces.RoleCapableSession) context.Context {
	if ctx == nil || session == nil {
		return ctx
	}
	return context.WithValue(ctx, checkerKey, session)
}

// CheckerFromContext returns the configured permission checker if available.
func CheckerFromContext(ctx context.Context) Checker {
	if ctx == nil {
		return nil
	}
	value := ctx.Value(checkerKey)
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case Checker:
		return typed
	case Permissioner:
		return CheckerFunc(typed.HasPermission)
	case interfaces.AuthClaims:
		return checkerFromClaims(typed)
	case interfaces.RoleCapableSession:
		return checkerFromSession(typed)
	case []string:
		return NewSet(typed...)
	case map[string]struct{}:
		return Set(typed)
	case map[string]bool:
		set := Set{}
		for key, allowed := range typed {
			if !allowed {
				continue
			}
			if normalized := normalizePermission(key); normalized != "" {
				set[normalized] = struct{}{}
			}
		}
		return set
	default:
		return nil
	}
}

// Allowed reports whether the provided permission is allowed for the context.
func Allowed(ctx context.Context, permission string) bool {
	checker := CheckerFromContext(ctx)
	if checker == nil {
		return true
	}
	normalized := normalizePermission(permission)
	if normalized == "" {
		return true
	}
	return checker.Allowed(normalized)
}

// Require enforces a permission requirement when a checker is available on the context.
func Require(ctx context.Context, permission string) error {
	normalized := normalizePermission(permission)
	if normalized == "" {
		return nil
	}
	checker := CheckerFromContext(ctx)
	if checker == nil {
		return nil
	}
	if checker.Allowed(normalized) {
		return nil
	}
	return Error{Permission: normalized}
}

func checkerFromClaims(claims interfaces.AuthClaims) Checker {
	return CheckerFunc(func(permission string) bool {
		if claims == nil {
			return false
		}
		resource, action := splitPermission(permission)
		switch action {
		case ActionRead:
			return claims.CanRead(resource)
		case ActionCreate:
			return claims.CanCreate(resource)
		case ActionUpdate:
			return claims.CanEdit(resource)
		case ActionDelete:
			return claims.CanDelete(resource)
		case ActionPublish:
			return claims.CanEdit(resource)
		default:
			return false
		}
	})
}

func checkerFromSession(session interfaces.RoleCapableSession) Checker {
	return CheckerFunc(func(permission string) bool {
		if session == nil {
			return false
		}
		resource, action := splitPermission(permission)
		switch action {
		case ActionRead:
			return session.CanRead(resource)
		case ActionCreate:
			return session.CanCreate(resource)
		case ActionUpdate:
			return session.CanEdit(resource)
		case ActionDelete:
			return session.CanDelete(resource)
		case ActionPublish:
			return session.CanEdit(resource)
		default:
			return false
		}
	})
}

func splitPermission(permission string) (string, Action) {
	normalized := normalizePermission(permission)
	if normalized == "" {
		return "", ""
	}
	parts := strings.SplitN(normalized, ":", 2)
	resource := normalizeToken(parts[0])
	if len(parts) == 1 {
		return resource, ""
	}
	return resource, Action(normalizeToken(parts[1]))
}

func normalizePermission(permission string) string {
	trimmed := strings.TrimSpace(permission)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

func normalizeToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
