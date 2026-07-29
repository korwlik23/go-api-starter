package config

import (
	"fmt"
	"math"
	"time"
)

const maximumAuthAttemptCleanupBatch = 10_000

type AuthAttemptConfig struct {
	Threshold    uint64
	Window       time.Duration
	LockDuration time.Duration
	CleanupBatch int
}

func ValidateAuthAttemptConfig(config AuthAttemptConfig) error {
	if config.Threshold < 2 || config.Threshold > math.MaxInt64 {
		return fmt.Errorf(
			"config: AUTH_ATTEMPT_THRESHOLD must be between 2 and %d",
			int64(math.MaxInt64),
		)
	}
	if config.Window <= 0 {
		return fmt.Errorf("config: AUTH_ATTEMPT_WINDOW must be greater than zero")
	}
	if config.LockDuration <= 0 {
		return fmt.Errorf("config: AUTH_ATTEMPT_LOCK_DURATION must be greater than zero")
	}
	if config.CleanupBatch < 1 || config.CleanupBatch > maximumAuthAttemptCleanupBatch {
		return fmt.Errorf(
			"config: AUTH_ATTEMPT_CLEANUP_BATCH must be between 1 and %d",
			maximumAuthAttemptCleanupBatch,
		)
	}
	return nil
}
