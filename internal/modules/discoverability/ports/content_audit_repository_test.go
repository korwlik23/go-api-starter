package ports

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3B6ValidateContentAuditListQueryRequiresAccountScope(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, ValidateContentAuditListQuery(ContentAuditListQuery{Limit: 25}), ErrInvalidContentAuditQuery)
	require.NoError(t, ValidateContentAuditListQuery(ContentAuditListQuery{AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580", Limit: 25}))
}
