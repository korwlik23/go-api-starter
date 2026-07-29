package application

import (
	"context"
	"errors"
	"strings"

	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
)

var ErrAccountResolutionDenied = errors.New("account resolution denied")

type ResolveAccount struct {
	repository ports.AccountRepository
}

func NewResolveAccount(
	repository ports.AccountRepository,
) (*ResolveAccount, error) {
	if repository == nil {
		return nil, ErrAccountResolutionDenied
	}
	return &ResolveAccount{repository: repository}, nil
}

func (useCase *ResolveAccount) Execute(
	ctx context.Context,
	userID string,
) (ports.ResolvedAccount, error) {
	if strings.TrimSpace(userID) == "" {
		return ports.ResolvedAccount{}, ErrAccountResolutionDenied
	}
	resolved, err := useCase.repository.ResolveActiveForUser(ctx, userID)
	if err != nil {
		return ports.ResolvedAccount{}, err
	}
	if resolved.Account.ID == "" ||
		resolved.Account.Status != accountsdomain.AccountStatusActive ||
		resolved.Membership.ID == "" ||
		resolved.Membership.AccountID != resolved.Account.ID ||
		resolved.Membership.UserID != userID ||
		resolved.Membership.Status != accountsdomain.MembershipStatusActive ||
		resolved.Membership.AuthorizationVersion < 1 {
		return ports.ResolvedAccount{}, ErrAccountResolutionDenied
	}
	return resolved, nil
}
