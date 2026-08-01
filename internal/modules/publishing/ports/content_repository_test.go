package ports

import (
	"testing"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/stretchr/testify/require"
)

func TestM3A4ListQueryRequiresAccountAndBoundsCursorPage(t *testing.T) {
	t.Parallel()

	valid := ContentListQuery{
		AccountID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		LocaleID:  "018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
		Limit:     25,
		After: &ContentCursor{
			UpdatedAt: time.Now().UTC(),
			ID:        "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
		},
	}
	require.NoError(t, ValidateContentListQuery(valid))

	for _, mutate := range []func(*ContentListQuery){
		func(query *ContentListQuery) { query.AccountID = "" },
		func(query *ContentListQuery) { query.LocaleID = "" },
		func(query *ContentListQuery) { query.Limit = 0 },
		func(query *ContentListQuery) { query.Limit = 101 },
		func(query *ContentListQuery) {
			query.After = &ContentCursor{ID: "not-a-uuid", UpdatedAt: time.Now()}
		},
		func(query *ContentListQuery) {
			kind := publishingdomain.Kind("landing")
			query.Kind = &kind
		},
	} {
		candidate := valid
		mutate(&candidate)
		require.ErrorIs(t, ValidateContentListQuery(candidate), ErrInvalidContentQuery)
	}
}
