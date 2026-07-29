package config

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateAuthAttemptConfig(t *testing.T) {
	valid := AuthAttemptConfig{
		Threshold:    5,
		Window:       15 * time.Minute,
		LockDuration: 30 * time.Minute,
		CleanupBatch: 500,
	}
	require.NoError(t, ValidateAuthAttemptConfig(valid))

	testCases := []AuthAttemptConfig{
		{Threshold: 1, Window: valid.Window, LockDuration: valid.LockDuration, CleanupBatch: 500},
		{Threshold: 5, Window: 0, LockDuration: valid.LockDuration, CleanupBatch: 500},
		{Threshold: 5, Window: valid.Window, LockDuration: 0, CleanupBatch: 500},
		{Threshold: 5, Window: valid.Window, LockDuration: valid.LockDuration, CleanupBatch: 0},
		{Threshold: 5, Window: valid.Window, LockDuration: valid.LockDuration, CleanupBatch: 10_001},
		{
			Threshold:    uint64(math.MaxInt64) + 1,
			Window:       valid.Window,
			LockDuration: valid.LockDuration,
			CleanupBatch: 500,
		},
	}
	for _, testCase := range testCases {
		require.Error(t, ValidateAuthAttemptConfig(testCase))
	}
}
