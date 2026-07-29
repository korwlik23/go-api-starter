package ports

import (
	"context"
	"errors"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
)

var (
	ErrRoleNotFound = errors.New("role not found")
	ErrRoleConflict = errors.New("role version conflict")
)

type RoleRepository interface {
	FindAccountRole(
		ctx context.Context,
		accountID string,
		roleID string,
	) (authorizationdomain.Role, error)
	UpdateAccountRoleName(
		ctx context.Context,
		accountID string,
		roleID string,
		expectedVersion int64,
		name string,
	) (newVersion int64, err error)
}
