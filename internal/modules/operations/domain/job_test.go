package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJobFailureCanBeRetriedWithoutLosingAttemptHistory(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	job, err := NewJob("job-1", "backup.verify", []byte(`{"path":"backup.zip"}`), now)
	require.NoError(t, err)
	require.Equal(t, StateQueued, job.State)

	require.NoError(t, job.Fail("restore checksum mismatch", now.Add(time.Minute)))
	require.Equal(t, StateFailed, job.State)
	require.Equal(t, 1, job.Attempts)
	require.Equal(t, "restore checksum mismatch", job.LastError)

	require.NoError(t, job.Retry(now.Add(2*time.Minute)))
	require.Equal(t, StateQueued, job.State)
	require.Equal(t, 1, job.Attempts)
	require.Empty(t, job.LastError)
}

func TestJobRejectsInvalidPayload(t *testing.T) {
	_, err := NewJob("job-1", "", nil, time.Now())
	require.ErrorIs(t, err, ErrInvalidJob)
}
