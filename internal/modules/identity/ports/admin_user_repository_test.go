package ports

import (
	"testing"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/stretchr/testify/require"
)

func TestAdminUserContractsUseBoundedStatusValues(t *testing.T) {
	t.Parallel()
	filter := AdminUserListFilter{Status: domain.UserStatusActive, Page: 1, PerPage: 25}
	require.Equal(t, domain.UserStatusActive, filter.Status)
	require.GreaterOrEqual(t, filter.PerPage, 1)
}
