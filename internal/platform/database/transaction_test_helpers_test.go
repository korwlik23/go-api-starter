package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	postgresgorm "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type d5TransactionSession struct {
	db            *gorm.DB
	commitError   error
	rollbackError error
	commitCalls   int
	rollbackCalls int
}

func (session *d5TransactionSession) Connection(ctx context.Context) *gorm.DB {
	return session.db.WithContext(ctx)
}

func (session *d5TransactionSession) Commit() error {
	session.commitCalls++
	return session.commitError
}

func (session *d5TransactionSession) Rollback() error {
	session.rollbackCalls++
	return session.rollbackError
}

type d5BeginRecorder struct {
	session transactionSession
	err     error
	calls   int
}

func (begin *d5BeginRecorder) Begin(context.Context) (transactionSession, error) {
	begin.calls++
	return begin.session, begin.err
}

func d5TransactionManager(
	t *testing.T,
) (*TransactionManager, *d5BeginRecorder, *d5TransactionSession) {
	t.Helper()

	base, _ := d5GormHandle(t)
	transaction, _ := d5GormHandle(t)
	session := &d5TransactionSession{db: transaction}
	begin := &d5BeginRecorder{session: session}
	manager, err := newTransactionManager(base, begin.Begin)
	require.NoError(t, err)
	return manager, begin, session
}

func d5GormHandle(t *testing.T) (*gorm.DB, *recordingPool) {
	t.Helper()

	pool := &recordingPool{}
	db, err := gorm.Open(
		postgresgorm.New(postgresgorm.Config{Conn: pool}),
		&gorm.Config{DisableAutomaticPing: true},
	)
	require.NoError(t, err)
	return db, pool
}
