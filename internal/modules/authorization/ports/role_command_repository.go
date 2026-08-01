package ports

import (
	"context"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
)

type RoleCommandRepository interface {
	CreateAccountRole(context.Context, string, string) (authorizationdomain.Role, error)
	UpdateAccountRoleName(
		context.Context,
		string,
		string,
		int64,
		string,
	) (authorizationdomain.Role, error)
}
