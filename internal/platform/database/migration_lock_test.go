package database

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestD3MigrationLockIdentityIsStableAndRedacted(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(-8051857392887072362), postgresMigrationLockID)

	const databaseName = "Go_Lang_Starter_Test_MariaDB"
	lockName := mysqlMigrationLockName(databaseName)
	require.Equal(
		t,
		"go-api-starter:migrations:v1:4d6c19fa7681c14572f23397bc1f16ac",
		lockName,
	)
	require.LessOrEqual(t, len(lockName), 64)
	require.NotContains(t, lockName, databaseName)
	require.NotContains(t, lockName, "go_lang_starter_test_mariadb")
}

func TestD3SessionMigrationLockLifecycle(t *testing.T) {
	t.Parallel()

	session := &fakeMigrationSession{}
	var events []string
	var eventsMu sync.Mutex
	addEvent := func(event string) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}
	locker := &sessionMigrationLocker{
		openSession: func(context.Context) (migrationSession, error) {
			addEvent("open")
			return session, nil
		},
		tryAcquire: func(context.Context, migrationSession) (bool, error) {
			addEvent("acquire")
			return true, nil
		},
		release: func(context.Context, migrationSession) error {
			addEvent("release")
			return nil
		},
		lockTimeout:   100 * time.Millisecond,
		pollInterval:  time.Millisecond,
		unlockTimeout: 100 * time.Millisecond,
	}

	require.NoError(t, locker.WithLock(context.Background(), func(context.Context) error {
		addEvent("run")
		return nil
	}))

	require.True(t, session.Closed())
	require.False(t, session.Discarded())
	require.Equal(t, []string{"open", "acquire", "run", "release"}, events)
}

func TestD3SessionMigrationLockDiscardsAmbiguousConnections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		tryAcquire tryMigrationLock
		release    releaseMigrationLock
		wantRun    bool
	}{
		{
			name: "acquire query fails",
			tryAcquire: func(context.Context, migrationSession) (bool, error) {
				return false, errors.New("private-acquire-driver-error")
			},
			release: func(context.Context, migrationSession) error {
				return nil
			},
		},
		{
			name: "release query fails",
			tryAcquire: func(context.Context, migrationSession) (bool, error) {
				return true, nil
			},
			release: func(context.Context, migrationSession) error {
				return errors.New("private-release-driver-error")
			},
			wantRun: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			session := &fakeMigrationSession{}
			runCalled := false
			locker := &sessionMigrationLocker{
				openSession: func(context.Context) (migrationSession, error) {
					return session, nil
				},
				tryAcquire:    test.tryAcquire,
				release:       test.release,
				lockTimeout:   100 * time.Millisecond,
				pollInterval:  time.Millisecond,
				unlockTimeout: 100 * time.Millisecond,
			}

			err := locker.WithLock(context.Background(), func(context.Context) error {
				runCalled = true
				return nil
			})

			require.ErrorIs(t, err, ErrMigrationLockUnavailable)
			require.NotContains(t, err.Error(), "private-")
			require.Equal(t, test.wantRun, runCalled)
			require.True(t, session.Discarded())
			require.False(t, session.Closed())
		})
	}
}

func TestD3SessionMigrationLockCombinesMigrationAndReleaseFailures(t *testing.T) {
	t.Parallel()

	session := &fakeMigrationSession{}
	locker := &sessionMigrationLocker{
		openSession: func(context.Context) (migrationSession, error) {
			return session, nil
		},
		tryAcquire: func(context.Context, migrationSession) (bool, error) {
			return true, nil
		},
		release: func(context.Context, migrationSession) error {
			return errors.New("private-release-driver-error")
		},
		lockTimeout:   100 * time.Millisecond,
		pollInterval:  time.Millisecond,
		unlockTimeout: 100 * time.Millisecond,
	}

	err := locker.WithLock(context.Background(), func(context.Context) error {
		return ErrMigrationFailed
	})

	require.ErrorIs(t, err, ErrMigrationFailed)
	require.ErrorIs(t, err, ErrMigrationLockUnavailable)
	require.NotContains(t, err.Error(), "private-release-driver-error")
	require.True(t, session.Discarded())
}

func TestD3SessionMigrationLockReleasesAfterPanic(t *testing.T) {
	t.Parallel()

	session := &fakeMigrationSession{}
	released := false
	locker := &sessionMigrationLocker{
		openSession: func(context.Context) (migrationSession, error) {
			return session, nil
		},
		tryAcquire: func(context.Context, migrationSession) (bool, error) {
			return true, nil
		},
		release: func(context.Context, migrationSession) error {
			released = true
			return nil
		},
		lockTimeout:   100 * time.Millisecond,
		pollInterval:  time.Millisecond,
		unlockTimeout: 100 * time.Millisecond,
	}

	require.PanicsWithValue(t, "migration callback panic", func() {
		_ = locker.WithLock(context.Background(), func(context.Context) error {
			panic("migration callback panic")
		})
	})
	require.True(t, released)
	require.True(t, session.Closed())
	require.False(t, session.Discarded())
}

func TestD3SessionMigrationLockUsesDetachedBoundedUnlockContext(t *testing.T) {
	t.Parallel()

	parentContext, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	session := &fakeMigrationSession{}
	releaseContextLive := false
	releaseContextBounded := false
	locker := &sessionMigrationLocker{
		openSession: func(context.Context) (migrationSession, error) {
			return session, nil
		},
		tryAcquire: func(context.Context, migrationSession) (bool, error) {
			return true, nil
		},
		release: func(releaseContext context.Context, _ migrationSession) error {
			releaseContextLive = releaseContext.Err() == nil
			deadline, hasDeadline := releaseContext.Deadline()
			releaseContextBounded = hasDeadline &&
				time.Until(deadline) > 0 &&
				time.Until(deadline) <= 100*time.Millisecond
			return nil
		},
		lockTimeout:   100 * time.Millisecond,
		pollInterval:  time.Millisecond,
		unlockTimeout: 100 * time.Millisecond,
	}

	err := locker.WithLock(parentContext, func(context.Context) error {
		cancelParent()
		return nil
	})

	require.NoError(t, err)
	require.True(t, releaseContextLive)
	require.True(t, releaseContextBounded)
	require.True(t, session.Closed())
	require.False(t, session.Discarded())
}

func TestD3SessionMigrationLockTimeoutAndCallerCancellation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		context              func() (context.Context, context.CancelFunc)
		lockTimeout          time.Duration
		expectedError        error
		expectedTriesAtLeast int
	}{
		{
			name: "internal timeout is stable",
			context: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			lockTimeout:          15 * time.Millisecond,
			expectedError:        ErrMigrationLockUnavailable,
			expectedTriesAtLeast: 1,
		},
		{
			name: "caller deadline is preserved",
			context: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 15*time.Millisecond)
			},
			lockTimeout:          time.Second,
			expectedError:        context.DeadlineExceeded,
			expectedTriesAtLeast: 1,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			session := &fakeMigrationSession{}
			tries := 0
			locker := &sessionMigrationLocker{
				openSession: func(context.Context) (migrationSession, error) {
					return session, nil
				},
				tryAcquire: func(context.Context, migrationSession) (bool, error) {
					tries++
					return false, nil
				},
				release: func(context.Context, migrationSession) error {
					return nil
				},
				lockTimeout:   test.lockTimeout,
				pollInterval:  time.Millisecond,
				unlockTimeout: 100 * time.Millisecond,
			}
			ctx, cancel := test.context()
			defer cancel()
			runCalled := false

			err := locker.WithLock(ctx, func(context.Context) error {
				runCalled = true
				return nil
			})

			require.ErrorIs(t, err, test.expectedError)
			require.False(t, runCalled)
			require.GreaterOrEqual(t, tries, test.expectedTriesAtLeast)
			require.True(t, session.Discarded())
		})
	}
}

func TestD3SessionMigrationLockOpenFailureIsRedacted(t *testing.T) {
	t.Parallel()

	locker := &sessionMigrationLocker{
		openSession: func(context.Context) (migrationSession, error) {
			return nil, errors.New("private-open-driver-error")
		},
		tryAcquire: func(context.Context, migrationSession) (bool, error) {
			return true, nil
		},
		release: func(context.Context, migrationSession) error {
			return nil
		},
		lockTimeout:   100 * time.Millisecond,
		pollInterval:  time.Millisecond,
		unlockTimeout: 100 * time.Millisecond,
	}

	err := locker.WithLock(context.Background(), func(context.Context) error {
		return nil
	})

	require.ErrorIs(t, err, ErrMigrationLockUnavailable)
	require.NotContains(t, err.Error(), "private-open-driver-error")
}

type fakeMigrationSession struct {
	mu        sync.Mutex
	closed    bool
	discarded bool
}

func (*fakeMigrationSession) QueryRowContext(
	context.Context,
	string,
	...any,
) migrationRowScanner {
	return errorMigrationRow{err: fmt.Errorf("unexpected query")}
}

func (session *fakeMigrationSession) Close() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.closed = true
	return nil
}

func (session *fakeMigrationSession) Discard() {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.discarded = true
}

func (session *fakeMigrationSession) Closed() bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.closed
}

func (session *fakeMigrationSession) Discarded() bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.discarded
}

type errorMigrationRow struct {
	err error
}

func (row errorMigrationRow) Scan(...any) error {
	return row.err
}
