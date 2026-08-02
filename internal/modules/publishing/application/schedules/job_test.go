package schedules

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3A8WorkerJobRejectsMissingConfiguredService(t *testing.T) {
	t.Parallel()

	err := (WorkerJob{Limit: 10}).Run(context.Background())
	require.ErrorIs(t, err, ErrInvalidScheduleService)
}
