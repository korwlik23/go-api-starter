package ports

import "context"

type AuthorizationVersionReader interface {
	CurrentAuthorizationVersion(ctx context.Context, userID string) (int64, error)
}
