package ports

import (
	"testing"

	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	"github.com/stretchr/testify/require"
)

func TestMembershipQueryContractHasExplicitAccountScope(t *testing.T) {
	t.Parallel()
	filter := MembershipListFilter{AccountID: "account", Status: accountsdomain.MembershipStatusActive, Page: 1, PerPage: 25}
	require.NotEmpty(t, filter.AccountID)
	require.Equal(t, accountsdomain.MembershipStatusActive, filter.Status)
}
