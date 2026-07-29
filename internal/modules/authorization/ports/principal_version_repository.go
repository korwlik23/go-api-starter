package ports

import (
	"context"
	"errors"
)

var ErrPrincipalVersionConflict = errors.New(
	"principal authorization version conflict",
)

type PrincipalVersionRepository interface {
	BumpUserVersion(
		ctx context.Context,
		userID string,
		expectedVersion int64,
	) (int64, error)
	BumpMembershipVersion(
		ctx context.Context,
		accountID string,
		membershipID string,
		expectedVersion int64,
	) (int64, error)
}
