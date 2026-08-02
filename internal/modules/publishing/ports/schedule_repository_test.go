package ports

import (
	"testing"
	"time"

	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/stretchr/testify/require"
)

func TestM3A8ScheduleFinishOnlyAllowsTerminalOrRetryStates(t *testing.T) {
	t.Parallel()

	valid := ScheduleFinishCommand{
		AccountID:        "018f1f64-7b2a-7c10-8bf2-3c40d5f67580",
		ScheduleID:       "018f1f64-7b2a-7c10-8bf2-3c40d5f67581",
		ExpectedAttempts: 1, State: publishingdomain.ScheduleSucceeded,
		FinishedAt: time.Now().UTC(),
	}
	require.NoError(t, ValidateScheduleFinish(valid))
	valid.State = publishingdomain.ScheduleLeased
	require.ErrorIs(t, ValidateScheduleFinish(valid), ErrInvalidScheduleRepository)
}
