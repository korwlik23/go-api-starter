package database

import (
	"context"
	"errors"
	"sync/atomic"

	"gorm.io/gorm"
)

var (
	// ErrInvalidTransactionBoundary reports an invalid manager, context, or
	// operation without exposing database implementation details.
	ErrInvalidTransactionBoundary = errors.New("database: invalid transaction boundary")
	// ErrTransactionFailed reports an infrastructure or rollback-only failure
	// without exposing a raw driver error.
	ErrTransactionFailed = errors.New("database: transaction failed")
)

// Transactor is the transaction boundary required by application use cases.
type Transactor interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}

// ConnectionProvider resolves either the root connection or the transaction
// connection carried by the supplied context.
type ConnectionProvider interface {
	Connection(context.Context) (*gorm.DB, error)
}

type beginTransaction func(context.Context) (transactionSession, error)

type transactionSession interface {
	Connection(context.Context) *gorm.DB
	Commit() error
	Rollback() error
}

type transactionContextKey struct{}

type transactionScope struct {
	owner        *TransactionManager
	session      transactionSession
	rollbackOnly atomic.Bool
	state        atomic.Uint32
}

const (
	transactionScopeActive uint32 = iota
	transactionScopeClosing
	transactionScopeClosed
)

// TransactionManager owns one root GORM connection and propagates a bound
// transaction through context. Nested calls reuse the existing boundary.
type TransactionManager struct {
	root  *gorm.DB
	begin beginTransaction
}

// NewTransactionManager creates a transaction boundary backed by GORM's
// explicit Begin, Commit, and Rollback operations.
func NewTransactionManager(root *gorm.DB) (*TransactionManager, error) {
	if root == nil {
		return nil, ErrInvalidTransactionBoundary
	}

	return newTransactionManager(root, func(ctx context.Context) (transactionSession, error) {
		transaction := root.WithContext(ctx).Begin()
		if transaction.Error != nil {
			return nil, transaction.Error
		}
		return &gormTransactionSession{transaction: transaction}, nil
	})
}

func newTransactionManager(
	root *gorm.DB,
	begin beginTransaction,
) (*TransactionManager, error) {
	if root == nil || begin == nil {
		return nil, ErrInvalidTransactionBoundary
	}
	return &TransactionManager{root: root, begin: begin}, nil
}

// Connection returns the GORM handle bound to ctx. A context owned by another
// manager is rejected so transactions cannot cross database boundaries.
func (manager *TransactionManager) Connection(ctx context.Context) (*gorm.DB, error) {
	if !manager.valid() || ctx == nil {
		return nil, ErrInvalidTransactionBoundary
	}

	scope, err := transactionScopeFrom(ctx)
	if err != nil {
		return nil, err
	}
	if scope == nil {
		return manager.root.WithContext(ctx), nil
	}
	if scope.owner != manager || scope.session == nil || !scope.active() {
		markTransactionRollbackOnly(scope)
		return nil, ErrInvalidTransactionBoundary
	}
	return scope.session.Connection(ctx), nil
}

// WithinTransaction executes operation atomically. A nested call made with the
// same context and manager reuses the outer session instead of using a savepoint.
func (manager *TransactionManager) WithinTransaction(
	ctx context.Context,
	operation func(context.Context) error,
) (result error) {
	if !manager.valid() || ctx == nil {
		return ErrInvalidTransactionBoundary
	}

	scope, err := transactionScopeFrom(ctx)
	if err != nil {
		return err
	}
	if operation == nil {
		markTransactionRollbackOnly(scope)
		return ErrInvalidTransactionBoundary
	}
	if err := ctx.Err(); err != nil {
		markTransactionRollbackOnly(scope)
		return err
	}
	if scope != nil {
		if scope.owner != manager || scope.session == nil || !scope.active() {
			markTransactionRollbackOnly(scope)
			return ErrInvalidTransactionBoundary
		}
		return withinExistingTransaction(ctx, scope, operation)
	}

	session, err := manager.begin(ctx)
	if err != nil || session == nil {
		return stableTransactionFailure(ctx, err)
	}

	scope = &transactionScope{owner: manager, session: session}
	transactionContext := context.WithValue(ctx, transactionContextKey{}, scope)
	operationReturned := false
	defer func() {
		if !operationReturned {
			scope.state.Store(transactionScopeClosing)
			scope.rollbackOnly.Store(true)
			_ = session.Rollback()
		}
		scope.state.Store(transactionScopeClosed)
	}()

	operationError := operation(transactionContext)
	operationReturned = true
	scope.state.Store(transactionScopeClosing)
	if operationError != nil {
		scope.rollbackOnly.Store(true)
		return operationRollbackResult(ctx, session, operationError)
	}
	if contextError := ctx.Err(); contextError != nil {
		scope.rollbackOnly.Store(true)
		return operationRollbackResult(ctx, session, contextError)
	}
	if scope.rollbackOnly.Load() {
		if rollbackError := session.Rollback(); rollbackError != nil {
			return errors.Join(
				ErrTransactionFailed,
				stableTransactionFailure(ctx, rollbackError),
			)
		}
		return ErrTransactionFailed
	}
	if commitError := session.Commit(); commitError != nil {
		return stableTransactionFailure(ctx, commitError)
	}
	return nil
}

func (manager *TransactionManager) valid() bool {
	return manager != nil && manager.root != nil && manager.begin != nil
}

func (scope *transactionScope) active() bool {
	return scope != nil && scope.state.Load() == transactionScopeActive
}

func markTransactionRollbackOnly(scope *transactionScope) {
	if scope != nil && scope.active() {
		scope.rollbackOnly.Store(true)
	}
}

func transactionScopeFrom(ctx context.Context) (*transactionScope, error) {
	value := ctx.Value(transactionContextKey{})
	if value == nil {
		return nil, nil
	}
	scope, ok := value.(*transactionScope)
	if !ok || scope == nil {
		return nil, ErrInvalidTransactionBoundary
	}
	return scope, nil
}

func withinExistingTransaction(
	ctx context.Context,
	scope *transactionScope,
	operation func(context.Context) error,
) (result error) {
	operationReturned := false
	defer func() {
		if !operationReturned {
			scope.rollbackOnly.Store(true)
		}
	}()

	result = operation(ctx)
	operationReturned = true
	if result != nil {
		scope.rollbackOnly.Store(true)
		return result
	}
	if contextError := ctx.Err(); contextError != nil {
		scope.rollbackOnly.Store(true)
		return contextError
	}
	return result
}

func operationRollbackResult(
	ctx context.Context,
	session transactionSession,
	operationError error,
) error {
	if rollbackError := session.Rollback(); rollbackError != nil {
		return errors.Join(
			operationError,
			stableTransactionFailure(ctx, rollbackError),
		)
	}
	return operationError
}

func stableTransactionFailure(ctx context.Context, failure error) error {
	if errors.Is(failure, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(failure, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return ErrTransactionFailed
}

type gormTransactionSession struct {
	transaction *gorm.DB
}

func (session *gormTransactionSession) Connection(ctx context.Context) *gorm.DB {
	return session.transaction.WithContext(ctx)
}

func (session *gormTransactionSession) Commit() error {
	return session.transaction.Commit().Error
}

func (session *gormTransactionSession) Rollback() error {
	return session.transaction.Rollback().Error
}
