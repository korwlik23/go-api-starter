package gorm

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"gorm.io/gorm"
)

var (
	ErrInvalidModuleStateRepository = errors.New(
		"operations: invalid module state repository",
	)
	ErrModuleStateRepositoryFailed = errors.New(
		"operations: module state repository failed",
	)
)

const (
	postgresReconcileLockID int64  = 0x676f6d6f64756c65
	mysqlReconcileLockName  string = "go_api_starter_module_reconcile"
)

type reconcileTransaction struct {
	db *gorm.DB
}

func (repository *ModuleStateRepository) withinLockedTransaction(
	ctx context.Context,
	operation func(context.Context, *reconcileTransaction) error,
) (result error) {
	connection, err := repository.sqlDB.Conn(ctx)
	if err != nil {
		return ErrModuleStateRepositoryFailed
	}
	defer func() {
		result = errors.Join(result, stableCloseError(connection.Close()))
	}()

	release, err := repository.acquireLock(ctx, connection)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, release())
	}()

	transaction, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return ErrModuleStateRepositoryFailed
	}
	committed := false
	defer func() {
		if !committed {
			_ = transaction.Rollback()
		}
	}()

	bound := repository.db.Session(&gorm.Session{
		NewDB:   true,
		Context: ctx,
	})
	bound.Statement.ConnPool = transaction
	if repository.dialect == "postgres" {
		if err := bound.WithContext(ctx).
			Exec("SELECT pg_advisory_xact_lock(?)", postgresReconcileLockID).
			Error; err != nil {
			return ErrModuleStateRepositoryFailed
		}
	}
	if err := operation(ctx, &reconcileTransaction{db: bound}); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return ErrModuleStateRepositoryFailed
	}
	committed = true
	return nil
}

func (repository *ModuleStateRepository) acquireLock(
	ctx context.Context,
	connection *sql.Conn,
) (func() error, error) {
	if repository.dialect == "postgres" {
		return func() error { return nil }, nil
	}
	timeoutSeconds := int64(math.Ceil(repository.lockTimeout.Seconds()))
	var acquired sql.NullInt64
	if err := connection.QueryRowContext(
		ctx,
		"SELECT GET_LOCK(?, ?)",
		mysqlReconcileLockName,
		timeoutSeconds,
	).Scan(&acquired); err != nil || !acquired.Valid || acquired.Int64 != 1 {
		return nil, ErrModuleStateRepositoryFailed
	}
	return func() error {
		releaseContext, cancel := context.WithTimeout(
			context.Background(),
			min(repository.lockTimeout, 5*time.Second),
		)
		defer cancel()
		var released sql.NullInt64
		if err := connection.QueryRowContext(
			releaseContext,
			"SELECT RELEASE_LOCK(?)",
			mysqlReconcileLockName,
		).Scan(&released); err != nil || !released.Valid || released.Int64 != 1 {
			return ErrModuleStateRepositoryFailed
		}
		return nil
	}, nil
}

func stableCloseError(err error) error {
	if err != nil {
		return ErrModuleStateRepositoryFailed
	}
	return nil
}
