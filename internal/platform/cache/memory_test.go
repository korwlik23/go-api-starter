package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type m3Clock struct {
	mu  sync.Mutex
	now time.Time
}

func (clock *m3Clock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *m3Clock) Advance(delta time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(delta)
	clock.mu.Unlock()
}

type m3Metrics struct {
	mu     sync.Mutex
	counts map[string]int
}

func (metrics *m3Metrics) Record(metric Metric) {
	metrics.mu.Lock()
	if metrics.counts == nil {
		metrics.counts = make(map[string]int)
	}
	metrics.counts[metric.Name]++
	metrics.mu.Unlock()
}

func (metrics *m3Metrics) Count(name string) int {
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	return metrics.counts[name]
}

func TestM3C3MemoryCacheBoundsTTLAndStaleLookup(t *testing.T) {
	t.Parallel()

	clock := &m3Clock{now: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)}
	memory, err := NewMemory(MemoryOptions{
		MaxEntries: 4, MaxBytes: 1024, DefaultTTL: time.Minute,
		DefaultStaleIfError: 2 * time.Minute, Now: clock.Now,
	})
	require.NoError(t, err)
	key := Key{Version: 1, Namespace: "publishing.content", AccountID: "account-1", ContentID: "content-1", Locale: "th", Path: "/home"}
	require.NoError(t, memory.Set(context.Background(), key, []byte("fresh"), SetOptions{}))

	lookup, err := memory.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, []byte("fresh"), lookup.Value)
	require.False(t, lookup.Stale)

	clock.Advance(time.Minute + time.Second)
	lookup, err = memory.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, []byte("fresh"), lookup.Value)
	require.True(t, lookup.Stale)

	clock.Advance(2 * time.Minute)
	lookup, err = memory.Get(context.Background(), key)
	require.NoError(t, err)
	require.False(t, lookup.Found)
	metrics := &m3Metrics{}
	_ = metrics
}

func TestM3C3MemoryCacheEvictsOldestAndCopiesValues(t *testing.T) {
	t.Parallel()

	clock := &m3Clock{now: time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)}
	memory, err := NewMemory(MemoryOptions{
		MaxEntries: 2, MaxBytes: 8, DefaultTTL: time.Minute, Now: clock.Now,
	})
	require.NoError(t, err)
	key := func(content string) Key {
		return Key{Version: 1, Namespace: "publishing.content", AccountID: "account-1", ContentID: content, Locale: "th", Path: "/" + content}
	}
	value := []byte("one")
	require.NoError(t, memory.Set(context.Background(), key("one"), value, SetOptions{}))
	value[0] = 'x'
	clock.Advance(time.Second)
	require.NoError(t, memory.Set(context.Background(), key("two"), []byte("two"), SetOptions{}))
	clock.Advance(time.Second)
	require.NoError(t, memory.Set(context.Background(), key("three"), []byte("three"), SetOptions{}))

	lookup, err := memory.Get(context.Background(), key("one"))
	require.NoError(t, err)
	require.False(t, lookup.Found)
	lookup, err = memory.Get(context.Background(), key("two"))
	require.NoError(t, err)
	require.Equal(t, []byte("two"), lookup.Value)
	lookup, err = memory.Get(context.Background(), key("three"))
	require.NoError(t, err)
	require.Equal(t, []byte("three"), lookup.Value)
}

func TestM3C3MemoryCacheInvalidatesByContentPathAndLocale(t *testing.T) {
	t.Parallel()

	memory, err := NewMemory(MemoryOptions{
		MaxEntries: 8, MaxBytes: 2048, DefaultTTL: time.Minute,
		Now: time.Now,
	})
	require.NoError(t, err)
	keys := []Key{
		{Version: 1, Namespace: "publishing.content", AccountID: "account-1", ContentID: "content-1", Locale: "th", Path: "/home"},
		{Version: 1, Namespace: "publishing.content", AccountID: "account-1", ContentID: "content-2", Locale: "th", Path: "/about"},
		{Version: 1, Namespace: "publishing.content", AccountID: "account-1", ContentID: "content-3", Locale: "en", Path: "/home"},
	}
	for _, key := range keys {
		require.NoError(t, memory.Set(context.Background(), key, []byte(key.ContentID), SetOptions{}))
	}
	removed, err := memory.Invalidate(context.Background(), Invalidation{
		Namespace: "publishing.content", AccountID: "account-1", Path: "/home", Locale: "th",
	})
	require.NoError(t, err)
	require.Equal(t, 1, removed)
	removed, err = memory.Invalidate(context.Background(), Invalidation{
		Namespace: "publishing.content", AccountID: "account-1", ContentID: "content-2",
	})
	require.NoError(t, err)
	require.Equal(t, 1, removed)
	lookup, err := memory.Get(context.Background(), keys[2])
	require.NoError(t, err)
	require.True(t, lookup.Found)
}
