package ports

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestM3A6RevisionListQueryRequiresAccountContentAndBoundedCursor(t *testing.T) {
	t.Parallel()

	valid := RevisionListQuery{
		AccountID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		ContentID:     "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		TranslationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		Limit:         50,
		After: &RevisionCursor{
			CreatedAt: time.Now().UTC(),
			ID:        "018f1f64-7b2a-7c10-8bf2-3c40d5f67588",
		},
	}
	require.NoError(t, ValidateRevisionListQuery(valid))
	valid.Limit = 101
	require.ErrorIs(t, ValidateRevisionListQuery(valid), ErrInvalidRevisionQuery)
}
