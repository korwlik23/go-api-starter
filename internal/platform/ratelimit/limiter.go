package ratelimit

import (
	"container/list"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

var (
	errClientKeyRequired          = errors.New("ratelimit: client key is required")
	errClockRequired              = errors.New("ratelimit: clock is required")
	errMaxEntriesPerPolicyInvalid = errors.New(
		"ratelimit: max entries per policy must be greater than zero",
	)
)

// DenialReason classifies a normal limiter denial without turning it into an
// operational error.
type DenialReason uint8

const (
	DenialNone DenialReason = iota
	DenialRateExceeded
	DenialCapacitySaturated
)

// Policy defines the burst capacity and continuous-refill window of a bucket.
type Policy struct {
	Capacity int
	Window   time.Duration
}

// Options contains the required process-local limiter dependencies and bounds.
type Options struct {
	Clock               func() time.Time
	MaxEntriesPerPolicy int
}

// Decision describes whether a request may proceed and, when denied, how long
// a caller should wait before retrying.
type Decision struct {
	Allowed    bool
	Reason     DenialReason
	RetryAfter time.Duration
}

type bucket struct {
	tokens     float64
	lastRefill time.Time
	lastAccess time.Time
	recency    *list.Element
}

type policyState struct {
	mu      sync.Mutex
	policy  Policy
	buckets map[string]*bucket
	recency list.List
}

type Limiter struct {
	clock               func() time.Time
	maxEntriesPerPolicy int
	policies            map[string]*policyState
}

// NewLimiter creates isolated, bounded state for each named policy.
func NewLimiter(policies map[string]Policy, options Options) (*Limiter, error) {
	if options.Clock == nil {
		return nil, errClockRequired
	}
	if options.MaxEntriesPerPolicy <= 0 {
		return nil, errMaxEntriesPerPolicyInvalid
	}

	states := make(map[string]*policyState, len(policies))
	for name, policy := range policies {
		if strings.TrimSpace(name) == "" {
			return nil, errors.New("ratelimit: policy name is required")
		}
		if policy.Capacity <= 0 || policy.Window <= 0 {
			return nil, fmt.Errorf(
				"ratelimit: policy %q must have positive capacity and window",
				name,
			)
		}
		states[name] = &policyState{
			policy:  policy,
			buckets: make(map[string]*bucket),
		}
	}

	return &Limiter{
		clock:               options.Clock,
		maxEntriesPerPolicy: options.MaxEntriesPerPolicy,
		policies:            states,
	}, nil
}

// Allow consumes one token for a policy and opaque client key. When a policy's
// client table is full, active buckets are retained and new clients fail closed
// until a bucket is safe to evict.
func (limiter *Limiter) Allow(policyName, clientKey string) (Decision, error) {
	state, exists := limiter.policies[policyName]
	if !exists {
		return Decision{}, fmt.Errorf("ratelimit: unknown policy %q", policyName)
	}
	if clientKey == "" {
		return Decision{}, errClientKeyRequired
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	now := limiter.clock()
	currentBucket, exists := state.buckets[clientKey]
	if !exists {
		if len(state.buckets) >= limiter.maxEntriesPerPolicy {
			retryAfter, evicted := evictSafeBucket(state, now)
			if !evicted {
				return Decision{
					Reason:     DenialCapacitySaturated,
					RetryAfter: retryAfter,
				}, nil
			}
		}

		currentBucket = &bucket{
			tokens:     float64(state.policy.Capacity),
			lastRefill: now,
			lastAccess: now,
		}
		currentBucket.recency = state.recency.PushBack(clientKey)
		state.buckets[clientKey] = currentBucket
	} else {
		refill(currentBucket, state.policy, now)
		touchBucket(state, currentBucket, now)
	}

	if currentBucket.tokens >= 1 {
		currentBucket.tokens--
		return Decision{Allowed: true}, nil
	}

	return Decision{
		Reason:     DenialRateExceeded,
		RetryAfter: retryAfterForToken(currentBucket, state.policy),
	}, nil
}

func (limiter *Limiter) size() int {
	total := 0
	for _, state := range limiter.policies {
		state.mu.Lock()
		total += len(state.buckets)
		state.mu.Unlock()
	}
	return total
}

func evictSafeBucket(state *policyState, now time.Time) (time.Duration, bool) {
	var shortestWait time.Duration

	for element := state.recency.Front(); element != nil; element = element.Next() {
		clientKey := element.Value.(string)
		currentBucket := state.buckets[clientKey]
		safeAt := currentBucket.lastAccess.Add(state.policy.Window)
		if !now.Before(safeAt) {
			delete(state.buckets, clientKey)
			state.recency.Remove(element)
			return 0, true
		}

		wait := safeAt.Sub(now)
		if shortestWait == 0 || wait < shortestWait {
			shortestWait = wait
		}
	}

	if shortestWait <= 0 {
		shortestWait = time.Nanosecond
	}
	return shortestWait, false
}

func touchBucket(state *policyState, currentBucket *bucket, now time.Time) {
	if now.After(currentBucket.lastAccess) {
		currentBucket.lastAccess = now
	}
	state.recency.MoveToBack(currentBucket.recency)
}

func refill(currentBucket *bucket, policy Policy, now time.Time) {
	if !now.After(currentBucket.lastRefill) {
		return
	}

	elapsed := now.Sub(currentBucket.lastRefill)
	currentBucket.tokens = min(
		float64(policy.Capacity),
		currentBucket.tokens+
			float64(elapsed)*float64(policy.Capacity)/float64(policy.Window),
	)
	currentBucket.lastRefill = now
}

func retryAfterForToken(currentBucket *bucket, policy Policy) time.Duration {
	missingTokens := 1 - currentBucket.tokens
	retryAfter := time.Duration(math.Ceil(
		missingTokens * float64(policy.Window) / float64(policy.Capacity),
	))
	if retryAfter <= 0 {
		return time.Nanosecond
	}
	return retryAfter
}
