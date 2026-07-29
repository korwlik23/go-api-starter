package ports

import (
	"context"
	"errors"

	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
)

var ErrAccountNotFound = errors.New("account not found")

type ResolvedAccount struct {
	Account    accountsdomain.Account
	Membership accountsdomain.Membership
}

type AccountRepository interface {
	ResolveActiveForUser(
		ctx context.Context,
		userID string,
	) (ResolvedAccount, error)
}
