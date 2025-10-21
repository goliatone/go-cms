package interfaces

import "context"

type AuthProvider interface {
	CurrentUserID(ctx context.Context) (string, error)
	HasPermission(ctx context.Context, permission string) (bool, error)
}
