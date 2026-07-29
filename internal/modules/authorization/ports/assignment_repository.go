package ports

import (
	"context"
	"errors"
	"time"
)

var ErrAssignmentConflict = errors.New("role assignment conflict")

type AssignmentRepository interface {
	AssignMembershipRole(
		ctx context.Context,
		accountID string,
		membershipID string,
		roleID string,
		createdAt time.Time,
	) error
	RevokeMembershipRole(
		ctx context.Context,
		accountID string,
		membershipID string,
		roleID string,
	) error
	AssignSystemRole(
		ctx context.Context,
		userID string,
		roleID string,
		createdAt time.Time,
	) error
	RevokeSystemRole(
		ctx context.Context,
		userID string,
		roleID string,
	) error
}
