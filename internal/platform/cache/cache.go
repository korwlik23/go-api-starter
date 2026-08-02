// Package cache contains bounded, process-local cache primitives used by
// public read models. Cache entries are never a source of truth: callers must
// be able to rebuild a value from the database when a lookup misses.
package cache

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidCache             = errors.New("cache: invalid cache")
	ErrInvalidCacheKey          = errors.New("cache: invalid key")
	ErrInvalidCacheInvalidation = errors.New("cache: invalid invalidation")
	ErrInvalidCacheOptions      = errors.New("cache: invalid options")
	ErrCacheEntryTooLarge       = errors.New("cache: entry too large")
)

// Key is the complete public-cache identity. Version is deliberately part of
// the key so changing serialization or the response contract cannot reuse an
// old payload.
type Key struct {
	Version   uint16
	Namespace string
	AccountID string
	ContentID string
	Locale    string
	Path      string
}

func (key Key) Validate() error {
	if key.Version < 1 || !validSegment(key.Namespace, 128) ||
		!validSegment(key.AccountID, 128) || !validSegment(key.ContentID, 128) ||
		!validSegment(key.Locale, 64) || !validSegment(key.Path, 1024) {
		return ErrInvalidCacheKey
	}
	return nil
}

// Canonical returns a stable, delimiter-safe representation suitable for
// logs, metrics and external cache implementations. The path is escaped so
// slash and separator characters cannot create ambiguous keys.
func (key Key) Canonical() string {
	return fmt.Sprintf(
		"cache:v%d:%s:%s:%s:%s:%s",
		key.Version,
		url.PathEscape(key.Namespace), url.PathEscape(key.AccountID),
		url.PathEscape(key.ContentID), url.PathEscape(key.Locale),
		url.PathEscape(key.Path),
	)
}

// Invalidation selects a set of cache entries. Empty selectors are wildcards;
// at least one selector is required to prevent accidental global flushes.
type Invalidation struct {
	Namespace string
	AccountID string
	ContentID string
	Locale    string
	Path      string
}

func (invalidation Invalidation) Validate() error {
	if invalidation.Namespace == "" && invalidation.AccountID == "" &&
		invalidation.ContentID == "" && invalidation.Locale == "" &&
		invalidation.Path == "" {
		return ErrInvalidCacheInvalidation
	}
	if (invalidation.Namespace != "" && !validSegment(invalidation.Namespace, 128)) ||
		(invalidation.AccountID != "" && !validSegment(invalidation.AccountID, 128)) ||
		(invalidation.ContentID != "" && !validSegment(invalidation.ContentID, 128)) ||
		(invalidation.Locale != "" && !validSegment(invalidation.Locale, 64)) ||
		(invalidation.Path != "" && !validSegment(invalidation.Path, 1024)) {
		return ErrInvalidCacheInvalidation
	}
	return nil
}

func (invalidation Invalidation) Matches(key Key) bool {
	if invalidation.Namespace != "" && invalidation.Namespace != key.Namespace {
		return false
	}
	if invalidation.AccountID != "" && invalidation.AccountID != key.AccountID {
		return false
	}
	if invalidation.ContentID != "" && invalidation.ContentID != key.ContentID {
		return false
	}
	if invalidation.Locale != "" && invalidation.Locale != key.Locale {
		return false
	}
	if invalidation.Path != "" && invalidation.Path != key.Path {
		return false
	}
	return true
}

type Lookup struct {
	Found      bool
	Value      []byte
	StoredAt   time.Time
	ExpiresAt  time.Time
	StaleUntil time.Time
	Stale      bool
}

type SetOptions struct {
	TTL          time.Duration
	StaleIfError time.Duration
}

type Metric struct {
	Name  string
	Key   Key
	Count int
}

type MetricsHook interface {
	Record(Metric)
}

type Cache interface {
	Get(context.Context, Key) (Lookup, error)
	Set(context.Context, Key, []byte, SetOptions) error
	Invalidate(context.Context, Invalidation) (int, error)
}

func validSegment(value string, maximum int) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && len(trimmed) <= maximum &&
		!strings.ContainsAny(trimmed, "\x00\r\n")
}
