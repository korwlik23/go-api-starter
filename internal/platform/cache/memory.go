package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalidMemory        = errors.New("cache: invalid memory cache")
	ErrInvalidMemoryOptions = errors.New("cache: invalid memory options")
)

const (
	metricHit          = "hit"
	metricMiss         = "miss"
	metricStale        = "stale"
	metricEviction     = "eviction"
	metricInvalidation = "invalidation"
)

type MemoryOptions struct {
	MaxEntries          int
	MaxBytes            int64
	DefaultTTL          time.Duration
	DefaultStaleIfError time.Duration
	Now                 func() time.Time
	Metrics             MetricsHook
}

type memoryEntry struct {
	key        Key
	value      []byte
	storedAt   time.Time
	expiresAt  time.Time
	staleUntil time.Time
	sequence   uint64
}

type Memory struct {
	mu           sync.Mutex
	entries      map[Key]memoryEntry
	bytes        int64
	sequence     uint64
	maxEntries   int
	maxBytes     int64
	defaultTTL   time.Duration
	defaultStale time.Duration
	now          func() time.Time
	metrics      MetricsHook
}

func NewMemory(options MemoryOptions) (*Memory, error) {
	if options.MaxEntries < 1 || options.MaxBytes < 1 || options.DefaultTTL <= 0 ||
		options.DefaultStaleIfError < 0 || options.Now == nil {
		return nil, ErrInvalidMemoryOptions
	}
	return &Memory{
		entries: make(map[Key]memoryEntry), maxEntries: options.MaxEntries,
		maxBytes: options.MaxBytes, defaultTTL: options.DefaultTTL,
		defaultStale: options.DefaultStaleIfError, now: options.Now,
		metrics: options.Metrics,
	}, nil
}

func (memory *Memory) Get(ctx context.Context, key Key) (Lookup, error) {
	if memory == nil || memory.entries == nil {
		return Lookup{}, ErrInvalidMemory
	}
	if err := validateContext(ctx); err != nil {
		return Lookup{}, err
	}
	if err := key.Validate(); err != nil {
		return Lookup{}, err
	}
	memory.mu.Lock()
	defer memory.mu.Unlock()
	entry, ok := memory.entries[key]
	if !ok {
		memory.record(metricMiss, key, 1)
		return Lookup{}, nil
	}
	now := memory.now().UTC()
	if now.Before(entry.expiresAt) {
		memory.record(metricHit, key, 1)
		return memory.lookup(entry, false), nil
	}
	if now.Before(entry.staleUntil) {
		memory.record(metricStale, key, 1)
		return memory.lookup(entry, true), nil
	}
	delete(memory.entries, key)
	memory.bytes -= int64(len(entry.value))
	memory.record(metricMiss, key, 1)
	return Lookup{}, nil
}

func (memory *Memory) Set(ctx context.Context, key Key, value []byte, options SetOptions) error {
	if memory == nil || memory.entries == nil {
		return ErrInvalidMemory
	}
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := key.Validate(); err != nil {
		return err
	}
	if options.TTL <= 0 {
		options.TTL = memory.defaultTTL
	}
	if options.StaleIfError < 0 {
		return ErrInvalidCacheOptions
	}
	if options.StaleIfError == 0 {
		options.StaleIfError = memory.defaultStale
	}
	if options.TTL <= 0 || len(value) == 0 {
		return ErrInvalidCacheOptions
	}
	if int64(len(value)) > memory.maxBytes {
		return ErrCacheEntryTooLarge
	}
	now := memory.now().UTC()
	memory.mu.Lock()
	defer memory.mu.Unlock()
	if existing, ok := memory.entries[key]; ok {
		memory.bytes -= int64(len(existing.value))
		delete(memory.entries, key)
	}
	for len(memory.entries) >= memory.maxEntries || memory.bytes+int64(len(value)) > memory.maxBytes {
		if !memory.evictOldest() {
			return ErrInvalidCacheOptions
		}
	}
	memory.sequence++
	copyValue := append([]byte(nil), value...)
	memory.entries[key] = memoryEntry{
		key: key, value: copyValue, storedAt: now,
		expiresAt: now.Add(options.TTL), staleUntil: now.Add(options.TTL + options.StaleIfError),
		sequence: memory.sequence,
	}
	memory.bytes += int64(len(copyValue))
	return nil
}

func (memory *Memory) Invalidate(ctx context.Context, invalidation Invalidation) (int, error) {
	if memory == nil || memory.entries == nil {
		return 0, ErrInvalidMemory
	}
	if err := validateContext(ctx); err != nil {
		return 0, err
	}
	if err := invalidation.Validate(); err != nil {
		return 0, err
	}
	memory.mu.Lock()
	defer memory.mu.Unlock()
	removed := 0
	for key, entry := range memory.entries {
		if !invalidation.Matches(key) {
			continue
		}
		delete(memory.entries, key)
		memory.bytes -= int64(len(entry.value))
		removed++
	}
	memory.record(metricInvalidation, Key{}, removed)
	return removed, nil
}

func (memory *Memory) lookup(entry memoryEntry, stale bool) Lookup {
	return Lookup{
		Found: true, Value: append([]byte(nil), entry.value...),
		StoredAt: entry.storedAt, ExpiresAt: entry.expiresAt,
		StaleUntil: entry.staleUntil, Stale: stale,
	}
}

func (memory *Memory) evictOldest() bool {
	var oldestKey Key
	var oldest memoryEntry
	found := false
	for key, entry := range memory.entries {
		if !found || entry.sequence < oldest.sequence {
			oldestKey, oldest, found = key, entry, true
		}
	}
	if !found {
		return false
	}
	delete(memory.entries, oldestKey)
	memory.bytes -= int64(len(oldest.value))
	memory.record(metricEviction, oldestKey, 1)
	return true
}

func (memory *Memory) record(name string, key Key, count int) {
	if memory.metrics != nil && count > 0 {
		memory.metrics.Record(Metric{Name: name, Key: key, Count: count})
	}
}

func validateContext(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidCache
	}
	return ctx.Err()
}
