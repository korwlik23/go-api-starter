package ratelimit

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestA12(t *testing.T) {
	t.Parallel()

	t.Run("token buckets are isolated by policy and client", func(t *testing.T) {
		t.Parallel()

		clock := newFakeClock(time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC))
		limiter, err := NewLimiter(
			map[string]Policy{
				"baseline": {Capacity: 2, Window: time.Minute},
				"login":    {Capacity: 1, Window: time.Minute},
			},
			Options{
				Clock:               clock.Now,
				MaxEntriesPerPolicy: 10,
			},
		)
		require.NoError(t, err)

		decision, err := limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.True(t, decision.Allowed)
		decision, err = limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.True(t, decision.Allowed)
		decision, err = limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialRateExceeded, decision.Reason)
		require.Equal(t, 30*time.Second, decision.RetryAfter)

		decision, err = limiter.Allow("baseline", "client-b")
		require.NoError(t, err)
		require.True(t, decision.Allowed)
		decision, err = limiter.Allow("login", "client-a")
		require.NoError(t, err)
		require.True(t, decision.Allowed)

		clock.Advance(30 * time.Second)
		decision, err = limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.True(t, decision.Allowed)

		secondLimiter, err := NewLimiter(
			map[string]Policy{
				"baseline": {Capacity: 2, Window: time.Minute},
			},
			Options{
				Clock:               clock.Now,
				MaxEntriesPerPolicy: 10,
			},
		)
		require.NoError(t, err)
		decision, err = secondLimiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.True(t, decision.Allowed)

		clock.Advance(10 * time.Minute)
		for range 2 {
			decision, err = limiter.Allow("baseline", "client-a")
			require.NoError(t, err)
			require.True(t, decision.Allowed)
		}
		decision, err = limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialRateExceeded, decision.Reason)
	})

	t.Run("active buckets cannot be evicted to reset quota", func(t *testing.T) {
		t.Parallel()

		clock := newFakeClock(time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC))
		limiter, err := NewLimiter(
			map[string]Policy{
				"baseline": {Capacity: 1, Window: time.Hour},
			},
			Options{
				Clock:               clock.Now,
				MaxEntriesPerPolicy: 1,
			},
		)
		require.NoError(t, err)

		decision, err := limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.True(t, decision.Allowed)

		decision, err = limiter.Allow("baseline", "client-b")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialCapacitySaturated, decision.Reason)
		require.Equal(t, time.Hour, decision.RetryAfter)
		require.Equal(t, 1, limiter.size())

		clock.Advance(59 * time.Minute)
		decision, err = limiter.Allow("baseline", "client-b")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialCapacitySaturated, decision.Reason)
		require.Equal(t, time.Minute, decision.RetryAfter)
		require.Equal(t, 1, limiter.size())

		clock.Advance(time.Minute)
		decision, err = limiter.Allow("baseline", "client-b")
		require.NoError(t, err)
		require.True(t, decision.Allowed)
		require.Equal(t, 1, limiter.size())

		decision, err = limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialCapacitySaturated, decision.Reason)
		require.Equal(t, time.Hour, decision.RetryAfter)
		require.Equal(t, 1, limiter.size())
	})

	t.Run("policy bounds prevent baseline churn from resetting login quota", func(t *testing.T) {
		t.Parallel()

		clock := newFakeClock(time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC))
		limiter, err := NewLimiter(
			map[string]Policy{
				"baseline": {Capacity: 1, Window: time.Minute},
				"login":    {Capacity: 1, Window: time.Hour},
			},
			Options{
				Clock:               clock.Now,
				MaxEntriesPerPolicy: 1,
			},
		)
		require.NoError(t, err)

		decision, err := limiter.Allow("login", "target")
		require.NoError(t, err)
		require.True(t, decision.Allowed)

		decision, err = limiter.Allow("baseline", "churn-a")
		require.NoError(t, err)
		require.True(t, decision.Allowed)
		decision, err = limiter.Allow("baseline", "churn-b")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialCapacitySaturated, decision.Reason)
		require.Equal(t, time.Minute, decision.RetryAfter)

		decision, err = limiter.Allow("login", "target")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialRateExceeded, decision.Reason)
		require.Equal(t, time.Hour, decision.RetryAfter)
		require.Equal(t, 2, limiter.size())
	})

	t.Run("least recently used safe bucket is evicted", func(t *testing.T) {
		t.Parallel()

		clock := newFakeClock(time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC))
		limiter, err := NewLimiter(
			map[string]Policy{
				"baseline": {Capacity: 1, Window: time.Hour},
			},
			Options{
				Clock:               clock.Now,
				MaxEntriesPerPolicy: 2,
			},
		)
		require.NoError(t, err)

		decision, err := limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.True(t, decision.Allowed)
		clock.Advance(time.Second)
		decision, err = limiter.Allow("baseline", "client-b")
		require.NoError(t, err)
		require.True(t, decision.Allowed)

		clock.Advance(30*time.Minute - time.Second)
		decision, err = limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialRateExceeded, decision.Reason)

		clock.Advance(30*time.Minute + time.Second)
		decision, err = limiter.Allow("baseline", "client-c")
		require.NoError(t, err)
		require.True(t, decision.Allowed)
		require.Equal(t, 2, limiter.size())

		decision, err = limiter.Allow("baseline", "client-b")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialCapacitySaturated, decision.Reason)
		require.Equal(t, 30*time.Minute-time.Second, decision.RetryAfter)
	})

	t.Run("concurrent callers share one race-safe bucket", func(t *testing.T) {
		t.Parallel()

		clock := newFakeClock(time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC))
		limiter, err := NewLimiter(
			map[string]Policy{
				"baseline": {Capacity: 100, Window: time.Hour},
			},
			Options{
				Clock:               clock.Now,
				MaxEntriesPerPolicy: 10,
			},
		)
		require.NoError(t, err)

		const requestCount = 200
		start := make(chan struct{})
		results := make(chan Decision, requestCount)
		errors := make(chan error, requestCount)
		var callers sync.WaitGroup
		callers.Add(requestCount)

		for range requestCount {
			go func() {
				defer callers.Done()
				<-start
				decision, allowErr := limiter.Allow("baseline", "shared-client")
				results <- decision
				errors <- allowErr
			}()
		}

		close(start)
		callers.Wait()
		close(results)
		close(errors)

		for allowErr := range errors {
			require.NoError(t, allowErr)
		}
		allowedCount := 0
		for decision := range results {
			if decision.Allowed {
				allowedCount++
			}
		}
		require.Equal(t, 100, allowedCount)
		require.Equal(t, 1, limiter.size())
	})

	t.Run("concurrent unique clients remain bounded", func(t *testing.T) {
		t.Parallel()

		clock := newFakeClock(time.Date(2026, time.July, 28, 0, 0, 0, 0, time.UTC))
		limiter, err := NewLimiter(
			map[string]Policy{
				"baseline": {Capacity: 1, Window: time.Hour},
			},
			Options{
				Clock:               clock.Now,
				MaxEntriesPerPolicy: 10,
			},
		)
		require.NoError(t, err)

		const requestCount = 100
		start := make(chan struct{})
		results := make(chan Decision, requestCount)
		errors := make(chan error, requestCount)
		var callers sync.WaitGroup
		callers.Add(requestCount)

		for clientNumber := range requestCount {
			go func() {
				defer callers.Done()
				<-start
				decision, allowErr := limiter.Allow(
					"baseline",
					strconv.Itoa(clientNumber),
				)
				errors <- allowErr
				results <- decision
			}()
		}

		close(start)
		callers.Wait()
		close(results)
		close(errors)

		for allowErr := range errors {
			require.NoError(t, allowErr)
		}
		allowedCount := 0
		saturatedCount := 0
		for decision := range results {
			if decision.Allowed {
				allowedCount++
			}
			if decision.Reason == DenialCapacitySaturated {
				saturatedCount++
			}
		}
		require.Equal(t, 10, allowedCount)
		require.Equal(t, 90, saturatedCount)
		require.Equal(t, 10, limiter.size())
	})

	t.Run("constructor and lookup errors are defensive", func(t *testing.T) {
		t.Parallel()

		validPolicies := map[string]Policy{
			"baseline": {Capacity: 1, Window: time.Minute},
		}

		_, err := NewLimiter(validPolicies, Options{MaxEntriesPerPolicy: 1})
		require.ErrorIs(t, err, errClockRequired)

		_, err = NewLimiter(
			validPolicies,
			Options{Clock: time.Now, MaxEntriesPerPolicy: 0},
		)
		require.ErrorIs(t, err, errMaxEntriesPerPolicyInvalid)

		_, err = NewLimiter(
			map[string]Policy{"": {Capacity: 1, Window: time.Minute}},
			Options{Clock: time.Now, MaxEntriesPerPolicy: 1},
		)
		require.Error(t, err)

		limiter, err := NewLimiter(
			validPolicies,
			Options{Clock: time.Now, MaxEntriesPerPolicy: 1},
		)
		require.NoError(t, err)

		_, err = limiter.Allow("unknown", "opaque-client-key")
		require.Error(t, err)
		require.NotContains(t, err.Error(), "opaque-client-key")

		_, err = limiter.Allow("baseline", "")
		require.ErrorIs(t, err, errClientKeyRequired)
	})

	t.Run("clock rollback neither refills nor evicts", func(t *testing.T) {
		t.Parallel()

		clock := newFakeClock(time.Date(2026, time.July, 28, 1, 0, 0, 0, time.UTC))
		limiter, err := NewLimiter(
			map[string]Policy{
				"baseline": {Capacity: 1, Window: time.Hour},
			},
			Options{
				Clock:               clock.Now,
				MaxEntriesPerPolicy: 1,
			},
		)
		require.NoError(t, err)

		decision, err := limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.True(t, decision.Allowed)

		clock.Advance(-time.Hour)
		decision, err = limiter.Allow("baseline", "client-a")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialRateExceeded, decision.Reason)

		decision, err = limiter.Allow("baseline", "client-b")
		require.NoError(t, err)
		require.False(t, decision.Allowed)
		require.Equal(t, DenialCapacitySaturated, decision.Reason)
		require.Equal(t, 2*time.Hour, decision.RetryAfter)
		require.Equal(t, 1, limiter.size())
	})
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(now time.Time) *fakeClock {
	return &fakeClock{now: now}
}

func (clock *fakeClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *fakeClock) Advance(duration time.Duration) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.now = clock.now.Add(duration)
}
