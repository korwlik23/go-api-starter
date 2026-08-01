package app

import (
	"context"
	"testing"

	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	accountsports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	"github.com/stretchr/testify/require"
)

func TestPrincipalOwnershipNeverWidensAcrossAccounts(t *testing.T) {
	ownership := principalOwnership{accounts: accountResolverStub{
		resolved: accountsports.ResolvedAccount{
			Account: accountsdomain.Account{
				ID: "account-a", Status: accountsdomain.AccountStatusActive,
			},
			Membership: accountsdomain.Membership{
				ID: "membership-a", AccountID: "account-a", UserID: "user-a",
				Status:               accountsdomain.MembershipStatusActive,
				AuthorizationVersion: 1,
			},
		},
	}}

	owner, found, err := ownership.OwnerWithinAccount(
		context.Background(),
		"account-b",
		"user-a",
	)
	require.NoError(t, err)
	require.False(t, found)
	require.Empty(t, owner)
}

type accountResolverStub struct {
	resolved accountsports.ResolvedAccount
	err      error
}

func (stub accountResolverStub) Execute(
	context.Context,
	string,
) (accountsports.ResolvedAccount, error) {
	return stub.resolved, stub.err
}
