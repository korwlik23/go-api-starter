package application

import (
	"context"
	"errors"
	"testing"

	platformcache "github.com/korwlik23/go-api-starter/internal/platform/cache"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/stretchr/testify/require"
)

type m3InvalidationCacheFake struct {
	events []CacheInvalidationEvent
	err    error
}

func (cache *m3InvalidationCacheFake) InvalidateAfterCommit(_ context.Context, event CacheInvalidationEvent) error {
	cache.events = append(cache.events, event)
	return cache.err
}

type m3Transactor struct {
	err     error
	commits int
}

func (transaction *m3Transactor) WithinTransaction(ctx context.Context, operation func(context.Context) error) error {
	if transaction.err != nil {
		return transaction.err
	}
	if err := operation(ctx); err != nil {
		return err
	}
	transaction.commits++
	return nil
}

func TestM3C3InvalidationRunsOnlyAfterSuccessfulCommit(t *testing.T) {
	t.Parallel()

	event := CacheInvalidationEvent{Namespace: "publishing.content", AccountID: "account-1", ContentID: "content-1", LocaleID: "th", Path: "/home"}
	cache := &m3InvalidationCacheFake{}
	transaction := &m3Transactor{}
	operationCalls := 0
	err := RunMutationWithInvalidation(context.Background(), transaction, cache, event, func(context.Context) error {
		operationCalls++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, operationCalls)
	require.Equal(t, 1, transaction.commits)
	require.Len(t, cache.events, 1)
	require.Equal(t, event, cache.events[0])

	cache.events = nil
	transaction.err = errors.New("rollback")
	err = RunMutationWithInvalidation(context.Background(), transaction, cache, event, func(context.Context) error {
		return nil
	})
	require.Error(t, err)
	require.Empty(t, cache.events)
}

func TestM3C3InvalidationDoesNotRunAfterOperationFailure(t *testing.T) {
	t.Parallel()

	cache := &m3InvalidationCacheFake{}
	transaction := &m3Transactor{}
	errOperation := errors.New("write failed")
	err := RunMutationWithInvalidation(context.Background(), transaction, cache, CacheInvalidationEvent{
		Namespace: "navigation.menu", AccountID: "account-1", LocaleID: "th",
	}, func(context.Context) error { return errOperation })
	require.ErrorIs(t, err, errOperation)
	require.Empty(t, cache.events)
}

func TestM3C3NewCacheInvalidationServiceRejectsMissingCache(t *testing.T) {
	t.Parallel()

	service, err := NewCacheInvalidationService(nil)
	require.Nil(t, service)
	require.ErrorIs(t, err, ErrInvalidCacheInvalidation)
	_, err = NewCacheInvalidationService(&platformCacheFake{})
	require.NoError(t, err)
	var _ database.Transactor = (*m3Transactor)(nil)
}

type platformCacheFake struct{}

func (*platformCacheFake) Get(context.Context, platformcache.Key) (platformcache.Lookup, error) {
	return platformcache.Lookup{}, nil
}
func (*platformCacheFake) Set(context.Context, platformcache.Key, []byte, platformcache.SetOptions) error {
	return nil
}
func (*platformCacheFake) Invalidate(context.Context, platformcache.Invalidation) (int, error) {
	return 0, nil
}
