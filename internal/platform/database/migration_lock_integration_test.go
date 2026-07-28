package database

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestD3MigrationLockIntegration(t *testing.T) {
	if !databaseIntegrationEnabled() {
		t.Skip("database integration test is disabled")
	}

	cfg := databaseIntegrationConfig(t)
	firstConfig := cfg
	firstConfig.MigrationLockTimeout = 2 * time.Second
	contenderConfig := cfg
	contenderConfig.MigrationLockTimeout = 300 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := Open(ctx, cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
	})

	firstLocker, err := newMigrationLocker(sqlDB, firstConfig)
	require.NoError(t, err)
	secondLocker, err := newMigrationLocker(sqlDB, contenderConfig)
	require.NoError(t, err)

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	var releaseFirstOnce sync.Once
	t.Cleanup(func() {
		releaseFirstOnce.Do(func() {
			close(releaseFirst)
		})
	})
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- firstLocker.WithLock(ctx, func(lockContext context.Context) error {
			close(firstEntered)
			select {
			case <-releaseFirst:
				return nil
			case <-lockContext.Done():
				return lockContext.Err()
			}
		})
	}()

	select {
	case <-firstEntered:
	case <-ctx.Done():
		t.Fatal("first migration lock was not acquired")
	}

	var secondCallbackCalled atomic.Bool
	secondResult := make(chan error, 1)
	go func() {
		secondResult <- secondLocker.WithLock(
			ctx,
			func(context.Context) error {
				secondCallbackCalled.Store(true)
				return nil
			},
		)
	}()
	err = awaitMigrationLockResult(t, ctx, secondResult, "contending lock")
	require.ErrorIs(t, err, ErrMigrationLockUnavailable)
	require.False(t, secondCallbackCalled.Load())

	releaseFirstOnce.Do(func() {
		close(releaseFirst)
	})
	require.NoError(
		t,
		awaitMigrationLockResult(t, ctx, firstResult, "first lock release"),
	)

	retryResult := make(chan error, 1)
	go func() {
		retryResult <- secondLocker.WithLock(
			ctx,
			func(context.Context) error {
				secondCallbackCalled.Store(true)
				return nil
			},
		)
	}()
	require.NoError(t, awaitMigrationLockResult(t, ctx, retryResult, "lock retry"))
	require.True(t, secondCallbackCalled.Load())
}

func awaitMigrationLockResult(
	t *testing.T,
	ctx context.Context,
	result <-chan error,
	operation string,
) error {
	t.Helper()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		t.Fatalf("%s did not finish before the test deadline", operation)
		return ctx.Err()
	}
}
