package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestM3A8RunnerExecutesEachRegisteredJobAndReportsFailure(t *testing.T) {
	t.Parallel()

	called := 0
	runner, err := NewRunner(
		FuncJob{Name: "publishing.schedule.process", Handler: func(context.Context) error { called++; return nil }},
		FuncJob{Name: "cleanup", Handler: func(context.Context) error { return errors.New("failed") }},
	)
	require.NoError(t, err)
	results, err := runner.RunOnce(context.Background())
	require.Error(t, err)
	require.Len(t, results, 2)
	require.Equal(t, 1, called)
}

func TestM3A8RunnerRejectsDuplicateIDs(t *testing.T) {
	t.Parallel()

	_, err := NewRunner(FuncJob{Name: "same"}, FuncJob{Name: "same"})
	require.ErrorIs(t, err, ErrDuplicateJob)
}
