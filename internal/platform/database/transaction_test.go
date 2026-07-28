package database

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestD5TransactionCommitsOnSuccessAndResolvesBoundConnection(t *testing.T) {
	t.Parallel()

	base, basePool := d5GormHandle(t)
	transaction, transactionPool := d5GormHandle(t)
	session := &d5TransactionSession{db: transaction}
	begin := &d5BeginRecorder{session: session}
	manager, err := newTransactionManager(base, begin.Begin)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), d5ContextMarkerKey{}, "marker")
	outside, err := manager.Connection(ctx)
	require.NoError(t, err)
	require.Same(t, basePool, outside.Statement.ConnPool)
	require.Equal(t, "marker", outside.Statement.Context.Value(d5ContextMarkerKey{}))

	err = manager.WithinTransaction(ctx, func(transactionContext context.Context) error {
		connection, connectionError := manager.Connection(transactionContext)
		require.NoError(t, connectionError)
		require.Same(t, transactionPool, connection.Statement.ConnPool)
		require.Equal(t, "marker", connection.Statement.Context.Value(d5ContextMarkerKey{}))
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 1, begin.calls)
	require.Equal(t, 1, session.commitCalls)
	require.Equal(t, 0, session.rollbackCalls)
}

func TestD5TransactionRollsBackAndPreservesOperationError(t *testing.T) {
	t.Parallel()

	expectedError := errors.New("domain operation failed")
	manager, begin, session := d5TransactionManager(t)

	err := manager.WithinTransaction(context.Background(), func(context.Context) error {
		return expectedError
	})

	require.ErrorIs(t, err, expectedError)
	require.Equal(t, 1, begin.calls)
	require.Equal(t, 0, session.commitCalls)
	require.Equal(t, 1, session.rollbackCalls)
}

func TestD5TransactionRedactsInfrastructureFailures(t *testing.T) {
	t.Parallel()

	const privateMarker = "private-transaction-driver-error"
	domainError := errors.New("safe domain error")
	tests := []struct {
		name           string
		beginError     error
		commitError    error
		rollbackError  error
		operationError error
		operationRuns  bool
	}{
		{name: "begin failure", beginError: errors.New(privateMarker)},
		{name: "commit failure", commitError: errors.New(privateMarker), operationRuns: true},
		{
			name:           "rollback failure",
			rollbackError:  errors.New(privateMarker),
			operationError: domainError,
			operationRuns:  true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			base, _ := d5GormHandle(t)
			transaction, _ := d5GormHandle(t)
			session := &d5TransactionSession{
				db:            transaction,
				commitError:   test.commitError,
				rollbackError: test.rollbackError,
			}
			begin := &d5BeginRecorder{session: session, err: test.beginError}
			manager, err := newTransactionManager(base, begin.Begin)
			require.NoError(t, err)
			operationCalls := 0

			err = manager.WithinTransaction(context.Background(), func(context.Context) error {
				operationCalls++
				return test.operationError
			})

			require.ErrorIs(t, err, ErrTransactionFailed)
			if test.operationError != nil {
				require.ErrorIs(t, err, test.operationError)
			}
			require.NotContains(t, err.Error(), privateMarker)
			require.Equal(t, test.operationRuns, operationCalls == 1)
		})
	}
}

func TestD5TransactionRollsBackAndRepanics(t *testing.T) {
	t.Parallel()

	manager, _, session := d5TransactionManager(t)

	require.PanicsWithValue(t, "operation panic", func() {
		_ = manager.WithinTransaction(context.Background(), func(context.Context) error {
			panic("operation panic")
		})
	})
	require.Equal(t, 0, session.commitCalls)
	require.Equal(t, 1, session.rollbackCalls)
}

func TestD5TransactionRollsBackWhenOperationExitsItsGoroutine(t *testing.T) {
	t.Parallel()

	manager, _, session := d5TransactionManager(t)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_ = manager.WithinTransaction(context.Background(), func(context.Context) error {
			runtime.Goexit()
			return nil
		})
	}()
	<-finished

	require.Equal(t, 0, session.commitCalls)
	require.Equal(t, 1, session.rollbackCalls)
}

func TestD5NestedTransactionReusesTheExistingBoundary(t *testing.T) {
	t.Parallel()

	manager, begin, session := d5TransactionManager(t)
	transactionPool := session.db.Statement.ConnPool

	err := manager.WithinTransaction(context.Background(), func(outerContext context.Context) error {
		derivedContext, cancel := context.WithTimeout(outerContext, time.Second)
		defer cancel()
		return manager.WithinTransaction(derivedContext, func(innerContext context.Context) error {
			connection, connectionError := manager.Connection(innerContext)
			require.NoError(t, connectionError)
			require.Same(t, transactionPool, connection.Statement.ConnPool)
			require.Equal(t, innerContext, connection.Statement.Context)
			return nil
		})
	})

	require.NoError(t, err)
	require.Equal(t, 1, begin.calls)
	require.Equal(t, 1, session.commitCalls)
	require.Equal(t, 0, session.rollbackCalls)
}

func TestD5NestedErrorMarksTheOuterBoundaryRollbackOnly(t *testing.T) {
	t.Parallel()

	nestedError := errors.New("nested operation failed")
	manager, begin, session := d5TransactionManager(t)

	err := manager.WithinTransaction(context.Background(), func(outerContext context.Context) error {
		require.ErrorIs(t, manager.WithinTransaction(
			outerContext,
			func(context.Context) error { return nestedError },
		), nestedError)
		return nil
	})

	require.ErrorIs(t, err, ErrTransactionFailed)
	require.Equal(t, 1, begin.calls)
	require.Equal(t, 0, session.commitCalls)
	require.Equal(t, 1, session.rollbackCalls)
}

func TestD5CaughtNestedPanicMarksTheOuterBoundaryRollbackOnly(t *testing.T) {
	t.Parallel()

	manager, _, session := d5TransactionManager(t)

	err := manager.WithinTransaction(context.Background(), func(outerContext context.Context) error {
		func() {
			defer func() { require.Equal(t, "nested panic", recover()) }()
			_ = manager.WithinTransaction(outerContext, func(context.Context) error {
				panic("nested panic")
			})
		}()
		return nil
	})

	require.ErrorIs(t, err, ErrTransactionFailed)
	require.Equal(t, 0, session.commitCalls)
	require.Equal(t, 1, session.rollbackCalls)
}

func TestD5RejectedNestedCallMarksTheOuterBoundaryRollbackOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(*TransactionManager, context.Context) error
	}{
		{
			name: "nil operation",
			call: func(manager *TransactionManager, ctx context.Context) error {
				return manager.WithinTransaction(ctx, nil)
			},
		},
		{
			name: "canceled child context",
			call: func(manager *TransactionManager, ctx context.Context) error {
				childContext, cancel := context.WithCancel(ctx)
				cancel()
				return manager.WithinTransaction(
					childContext,
					func(context.Context) error { return nil },
				)
			},
		},
		{
			name: "deadline expires during nested operation",
			call: func(manager *TransactionManager, ctx context.Context) error {
				childContext, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
				defer cancel()
				return manager.WithinTransaction(
					childContext,
					func(operationContext context.Context) error {
						<-operationContext.Done()
						return nil
					},
				)
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manager, _, session := d5TransactionManager(t)
			err := manager.WithinTransaction(
				context.Background(),
				func(outerContext context.Context) error {
					require.Error(t, test.call(manager, outerContext))
					return nil
				},
			)

			require.ErrorIs(t, err, ErrTransactionFailed)
			require.Equal(t, 0, session.commitCalls)
			require.Equal(t, 1, session.rollbackCalls)
		})
	}
}

func TestD5TransactionContextCannotBeReusedAfterCompletion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		operationError error
	}{
		{name: "after commit"},
		{name: "after rollback", operationError: errors.New("operation failed")},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			manager, begin, _ := d5TransactionManager(t)
			var escapedContext context.Context
			err := manager.WithinTransaction(
				context.Background(),
				func(transactionContext context.Context) error {
					escapedContext = transactionContext
					return test.operationError
				},
			)
			if test.operationError == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, test.operationError)
			}

			_, err = manager.Connection(escapedContext)
			require.ErrorIs(t, err, ErrInvalidTransactionBoundary)
			err = manager.WithinTransaction(
				escapedContext,
				func(context.Context) error { return nil },
			)
			require.ErrorIs(t, err, ErrInvalidTransactionBoundary)
			require.Equal(t, 1, begin.calls)
		})
	}
}

func TestD5TransactionContextCannotBeReusedAfterPanic(t *testing.T) {
	t.Parallel()

	manager, begin, _ := d5TransactionManager(t)
	var escapedContext context.Context
	require.PanicsWithValue(t, "operation panic", func() {
		_ = manager.WithinTransaction(
			context.Background(),
			func(transactionContext context.Context) error {
				escapedContext = transactionContext
				panic("operation panic")
			},
		)
	})

	_, err := manager.Connection(escapedContext)
	require.ErrorIs(t, err, ErrInvalidTransactionBoundary)
	operationCalls := 0
	err = manager.WithinTransaction(escapedContext, func(context.Context) error {
		operationCalls++
		return nil
	})
	require.ErrorIs(t, err, ErrInvalidTransactionBoundary)
	require.Equal(t, 0, operationCalls)
	require.Equal(t, 1, begin.calls)
}

func TestD5NestedTransactionRejectsAnotherManager(t *testing.T) {
	t.Parallel()

	firstManager, _, firstSession := d5TransactionManager(t)
	secondManager, secondBegin, _ := d5TransactionManager(t)

	err := firstManager.WithinTransaction(context.Background(), func(firstContext context.Context) error {
		_, connectionError := secondManager.Connection(firstContext)
		require.ErrorIs(t, connectionError, ErrInvalidTransactionBoundary)
		return secondManager.WithinTransaction(
			firstContext,
			func(context.Context) error { return nil },
		)
	})

	require.ErrorIs(t, err, ErrInvalidTransactionBoundary)
	require.Equal(t, 0, secondBegin.calls)
	require.Equal(t, 0, firstSession.commitCalls)
	require.Equal(t, 1, firstSession.rollbackCalls)
}

func TestD5TransactionValidatesInputsAndPreservesCancellation(t *testing.T) {
	t.Parallel()

	base, _ := d5GormHandle(t)
	transaction, _ := d5GormHandle(t)
	begin := &d5BeginRecorder{session: &d5TransactionSession{db: transaction}}

	manager, err := NewTransactionManager(nil)
	require.Nil(t, manager)
	require.ErrorIs(t, err, ErrInvalidTransactionBoundary)
	manager, err = newTransactionManager(base, nil)
	require.Nil(t, manager)
	require.ErrorIs(t, err, ErrInvalidTransactionBoundary)
	manager, err = newTransactionManager(base, begin.Begin)
	require.NoError(t, err)

	err = manager.WithinTransaction(nil, func(context.Context) error { return nil })
	require.ErrorIs(t, err, ErrInvalidTransactionBoundary)
	err = manager.WithinTransaction(context.Background(), nil)
	require.ErrorIs(t, err, ErrInvalidTransactionBoundary)
	_, err = manager.Connection(nil)
	require.ErrorIs(t, err, ErrInvalidTransactionBoundary)

	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	err = manager.WithinTransaction(canceledContext, func(context.Context) error { return nil })
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 0, begin.calls)
}

func TestD5TransactionPreservesCancellationFromOperation(t *testing.T) {
	t.Parallel()

	manager, _, session := d5TransactionManager(t)
	canceledContext, cancel := context.WithCancel(context.Background())

	err := manager.WithinTransaction(canceledContext, func(context.Context) error {
		cancel()
		return context.Canceled
	})

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, session.rollbackCalls)
}

func TestD5TransactionRejectsNilSession(t *testing.T) {
	t.Parallel()

	base, _ := d5GormHandle(t)
	begin := &d5BeginRecorder{}
	manager, err := newTransactionManager(base, begin.Begin)
	require.NoError(t, err)
	operationCalls := 0

	err = manager.WithinTransaction(context.Background(), func(context.Context) error {
		operationCalls++
		return nil
	})

	require.ErrorIs(t, err, ErrTransactionFailed)
	require.Equal(t, 0, operationCalls)
}

type d5ContextMarkerKey struct{}
