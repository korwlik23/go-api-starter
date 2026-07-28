package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
)

const (
	migrationLockNamespace  = "go-api-starter:migrations:v1"
	postgresMigrationLockID = int64(-8051857392887072362)

	postgresTryMigrationLockSQL = "SELECT pg_try_advisory_lock($1)"
	postgresReleaseLockSQL      = "SELECT pg_advisory_unlock($1)"
	mysqlTryMigrationLockSQL    = "SELECT GET_LOCK(?, 0)"
	mysqlReleaseLockSQL         = "SELECT RELEASE_LOCK(?)"

	defaultMigrationLockPollInterval = 50 * time.Millisecond
	defaultMigrationUnlockTimeout    = 5 * time.Second
)

type migrationRowScanner interface {
	Scan(...any) error
}

type migrationSession interface {
	QueryRowContext(
		context.Context,
		string,
		...any,
	) migrationRowScanner
	Close() error
	Discard()
}

type openMigrationSession func(context.Context) (migrationSession, error)
type tryMigrationLock func(context.Context, migrationSession) (bool, error)
type releaseMigrationLock func(context.Context, migrationSession) error

type sessionMigrationLocker struct {
	openSession   openMigrationSession
	tryAcquire    tryMigrationLock
	release       releaseMigrationLock
	lockTimeout   time.Duration
	pollInterval  time.Duration
	unlockTimeout time.Duration
}

func newMigrationLocker(
	db *sql.DB,
	cfg config.DatabaseConfig,
) (migrationLocker, error) {
	if db == nil || cfg.MigrationLockTimeout <= 0 {
		return nil, ErrInvalidMigrationPlan
	}

	locker := &sessionMigrationLocker{
		openSession: func(ctx context.Context) (migrationSession, error) {
			connection, err := db.Conn(ctx)
			if err != nil {
				return nil, err
			}
			return &sqlMigrationSession{connection: connection}, nil
		},
		lockTimeout:   cfg.MigrationLockTimeout,
		pollInterval:  defaultMigrationLockPollInterval,
		unlockTimeout: defaultMigrationUnlockTimeout,
	}

	switch {
	case cfg.Driver == "postgres" && cfg.Flavor == "":
		locker.tryAcquire = tryPostgresMigrationLock
		locker.release = releasePostgresMigrationLock
	case cfg.Driver == "mysql" &&
		(cfg.Flavor == "mariadb" || cfg.Flavor == "mysql"):
		lockName := mysqlMigrationLockName(cfg.Name)
		locker.tryAcquire = tryMySQLMigrationLock(lockName)
		locker.release = releaseMySQLMigrationLock(lockName)
	default:
		return nil, ErrInvalidMigrationPlan
	}
	return locker, nil
}

func (locker *sessionMigrationLocker) WithLock(
	ctx context.Context,
	run func(context.Context) error,
) (result error) {
	if ctx == nil ||
		run == nil ||
		locker == nil ||
		locker.openSession == nil ||
		locker.tryAcquire == nil ||
		locker.release == nil ||
		locker.lockTimeout <= 0 ||
		locker.pollInterval <= 0 ||
		locker.unlockTimeout <= 0 {
		return ErrMigrationLockUnavailable
	}

	session, err := locker.openSession(ctx)
	if err != nil || session == nil {
		if contextError := ctx.Err(); contextError != nil {
			return contextError
		}
		return ErrMigrationLockUnavailable
	}

	acquireContext, cancelAcquire := context.WithTimeout(
		ctx,
		locker.lockTimeout,
	)
	acquired := locker.acquire(acquireContext, session)
	cancelAcquire()
	if !acquired {
		session.Discard()
		if contextError := ctx.Err(); contextError != nil {
			return contextError
		}
		return ErrMigrationLockUnavailable
	}

	defer func() {
		recovered := recover()
		unlockContext, cancelUnlock := context.WithTimeout(
			context.WithoutCancel(ctx),
			locker.unlockTimeout,
		)
		releaseError := locker.release(unlockContext, session)
		cancelUnlock()

		if releaseError != nil {
			session.Discard()
			result = errors.Join(result, ErrMigrationLockUnavailable)
		} else if closeError := session.Close(); closeError != nil {
			session.Discard()
			result = errors.Join(result, ErrMigrationLockUnavailable)
		}
		if recovered != nil {
			panic(recovered)
		}
	}()

	return run(ctx)
}

func (locker *sessionMigrationLocker) acquire(
	ctx context.Context,
	session migrationSession,
) bool {
	for {
		acquired, err := locker.tryAcquire(ctx, session)
		if err != nil {
			return false
		}
		if acquired {
			return true
		}

		timer := time.NewTimer(locker.pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return false
		case <-timer.C:
		}
	}
}

type sqlMigrationSession struct {
	connection *sql.Conn
}

func (session *sqlMigrationSession) QueryRowContext(
	ctx context.Context,
	query string,
	args ...any,
) migrationRowScanner {
	return session.connection.QueryRowContext(ctx, query, args...)
}

func (session *sqlMigrationSession) Close() error {
	return session.connection.Close()
}

func (session *sqlMigrationSession) Discard() {
	_ = session.connection.Raw(func(any) error {
		return driver.ErrBadConn
	})
	_ = session.connection.Close()
}

func tryPostgresMigrationLock(
	ctx context.Context,
	session migrationSession,
) (bool, error) {
	var acquired bool
	err := session.QueryRowContext(
		ctx,
		postgresTryMigrationLockSQL,
		postgresMigrationLockID,
	).Scan(&acquired)
	return acquired, err
}

func releasePostgresMigrationLock(
	ctx context.Context,
	session migrationSession,
) error {
	var released bool
	if err := session.QueryRowContext(
		ctx,
		postgresReleaseLockSQL,
		postgresMigrationLockID,
	).Scan(&released); err != nil {
		return err
	}
	if !released {
		return fmt.Errorf("postgres migration lock was not owned")
	}
	return nil
}

func tryMySQLMigrationLock(lockName string) tryMigrationLock {
	return func(
		ctx context.Context,
		session migrationSession,
	) (bool, error) {
		var result sql.NullInt64
		if err := session.QueryRowContext(
			ctx,
			mysqlTryMigrationLockSQL,
			lockName,
		).Scan(&result); err != nil {
			return false, err
		}
		if !result.Valid {
			return false, fmt.Errorf("mysql migration lock result was null")
		}
		return result.Int64 == 1, nil
	}
}

func releaseMySQLMigrationLock(lockName string) releaseMigrationLock {
	return func(
		ctx context.Context,
		session migrationSession,
	) error {
		var result sql.NullInt64
		if err := session.QueryRowContext(
			ctx,
			mysqlReleaseLockSQL,
			lockName,
		).Scan(&result); err != nil {
			return err
		}
		if !result.Valid || result.Int64 != 1 {
			return fmt.Errorf("mysql migration lock was not owned")
		}
		return nil
	}
}

func mysqlMigrationLockName(databaseName string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(databaseName)))
	return fmt.Sprintf("%s:%x", migrationLockNamespace, digest[:16])
}
