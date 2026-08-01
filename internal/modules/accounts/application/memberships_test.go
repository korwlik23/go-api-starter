package application

import (
	"context"
	"testing"
	"time"

	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/stretchr/testify/require"
)

func TestMembershipStatusRequiresSystemPermission(t *testing.T) {
	commands := &membershipCommandStub{view: ports.MembershipView{ID: "membership-1", AccountID: "account-1", UserID: "user-1", Status: accountsdomain.MembershipStatusDisabled, AuthorizationVersion: 2, CreatedAt: time.Now(), UpdatedAt: time.Now()}}
	service, err := NewMembershipService(&membershipTransactorStub{}, membershipAuthorizerStub{allowed: true}, membershipQueryStub{}, commands)
	require.NoError(t, err)
	view, err := service.UpdateStatus(context.Background(), UpdateMembershipStatusCommand{ActorUserID: "actor", AccountID: "account-1", MembershipID: "membership-1", ExpectedVersion: 1, Status: accountsdomain.MembershipStatusDisabled})
	require.NoError(t, err)
	require.Equal(t, commands.view, view)
}

type membershipAuthorizerStub struct{ allowed bool }

func (stub membershipAuthorizerStub) Allowed(context.Context, string, authdomain.PermissionKey) (bool, error) {
	return stub.allowed, nil
}

type membershipTransactorStub struct{}

func (*membershipTransactorStub) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type membershipQueryStub struct{}

func (membershipQueryStub) ListMemberships(context.Context, ports.MembershipListFilter) (ports.MembershipPage, error) {
	return ports.MembershipPage{}, nil
}

type membershipCommandStub struct{ view ports.MembershipView }

func (stub *membershipCommandStub) UpdateMembershipStatus(context.Context, string, string, int64, accountsdomain.MembershipStatus, time.Time) (ports.MembershipView, error) {
	return stub.view, nil
}
