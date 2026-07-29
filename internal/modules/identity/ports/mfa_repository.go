package ports

import "context"

type MFAStatusReader interface {
	IsEnabled(ctx context.Context, userID string) (bool, error)
}
