package application

import (
	"context"
	"testing"

	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	"github.com/stretchr/testify/require"
)

func TestResolveAccountUsesAuthenticatedUserWithoutClientAccountSelection(
	t *testing.T,
) {
	repository := &accountRepositoryStub{resolved: ports.ResolvedAccount{
		Account: accountsdomain.Account{
			ID:     "account-id",
			Slug:   "personal",
			Status: accountsdomain.AccountStatusActive,
		},
		Membership: accountsdomain.Membership{
			ID:                   "membership-id",
			AccountID:            "account-id",
			UserID:               "user-id",
			Status:               accountsdomain.MembershipStatusActive,
			AuthorizationVersion: 1,
		},
	}}
	resolver, err := NewResolveAccount(repository)
	require.NoError(t, err)

	resolved, err := resolver.Execute(context.Background(), "user-id")

	require.NoError(t, err)
	require.Equal(t, "user-id", repository.userID)
	require.Equal(t, "account-id", resolved.Account.ID)
}

func TestResolveAccountFailsClosedOnCrossUserMembership(t *testing.T) {
	repository := &accountRepositoryStub{resolved: ports.ResolvedAccount{
		Account: accountsdomain.Account{
			ID:     "account-id",
			Status: accountsdomain.AccountStatusActive,
		},
		Membership: accountsdomain.Membership{
			ID:                   "membership-id",
			AccountID:            "account-id",
			UserID:               "other-user",
			Status:               accountsdomain.MembershipStatusActive,
			AuthorizationVersion: 1,
		},
	}}
	resolver, err := NewResolveAccount(repository)
	require.NoError(t, err)

	_, err = resolver.Execute(context.Background(), "user-id")

	require.ErrorIs(t, err, ErrAccountResolutionDenied)
}

type accountRepositoryStub struct {
	resolved ports.ResolvedAccount
	userID   string
	err      error
}

func (stub *accountRepositoryStub) ResolveActiveForUser(
	_ context.Context,
	userID string,
) (ports.ResolvedAccount, error) {
	stub.userID = userID
	return stub.resolved, stub.err
}
