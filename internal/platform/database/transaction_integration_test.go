package database

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestD5TransactionIntegration(t *testing.T) {
	if !databaseIntegrationEnabled() {
		t.Skip("database integration test is disabled")
	}

	cfg := databaseIntegrationConfig(t)
	requireDisposableIntegrationDatabase(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := Open(ctx, cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	probeTable := "d5_transaction_probe_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	require.NoError(
		t,
		db.Exec("CREATE TABLE "+probeTable+" (id BIGINT PRIMARY KEY)").Error,
	)
	t.Cleanup(func() {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+probeTable).Error)
	})

	manager, err := NewTransactionManager(db)
	require.NoError(t, err)

	t.Run("commit", func(t *testing.T) {
		require.NoError(t, manager.WithinTransaction(ctx, func(transactionContext context.Context) error {
			return d5InsertTransactionProbe(manager, transactionContext, probeTable, 1)
		}))
		require.Equal(t, int64(1), d5TransactionProbeCount(t, db, probeTable, 1))
	})

	t.Run("operation error rollback", func(t *testing.T) {
		operationError := errors.New("safe operation failure")
		err := manager.WithinTransaction(ctx, func(transactionContext context.Context) error {
			require.NoError(t, d5InsertTransactionProbe(manager, transactionContext, probeTable, 2))
			return operationError
		})
		require.ErrorIs(t, err, operationError)
		require.Equal(t, int64(0), d5TransactionProbeCount(t, db, probeTable, 2))
	})

	t.Run("panic rollback", func(t *testing.T) {
		require.PanicsWithValue(t, "transaction panic", func() {
			_ = manager.WithinTransaction(ctx, func(transactionContext context.Context) error {
				require.NoError(t, d5InsertTransactionProbe(manager, transactionContext, probeTable, 3))
				panic("transaction panic")
			})
		})
		require.Equal(t, int64(0), d5TransactionProbeCount(t, db, probeTable, 3))
	})

	t.Run("nested success", func(t *testing.T) {
		err := manager.WithinTransaction(ctx, func(outerContext context.Context) error {
			outer, connectionError := manager.Connection(outerContext)
			require.NoError(t, connectionError)
			require.NoError(t, d5InsertTransactionProbe(manager, outerContext, probeTable, 4))

			return manager.WithinTransaction(outerContext, func(innerContext context.Context) error {
				inner, innerConnectionError := manager.Connection(innerContext)
				require.NoError(t, innerConnectionError)
				require.Same(t, outer.Statement.ConnPool, inner.Statement.ConnPool)
				return d5InsertTransactionProbe(manager, innerContext, probeTable, 5)
			})
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), d5TransactionProbeCount(t, db, probeTable, 4))
		require.Equal(t, int64(1), d5TransactionProbeCount(t, db, probeTable, 5))
	})

	t.Run("swallowed nested error rollback", func(t *testing.T) {
		nestedError := errors.New("safe nested failure")
		err := manager.WithinTransaction(ctx, func(outerContext context.Context) error {
			require.NoError(t, d5InsertTransactionProbe(manager, outerContext, probeTable, 6))
			require.ErrorIs(t, manager.WithinTransaction(
				outerContext,
				func(innerContext context.Context) error {
					require.NoError(t, d5InsertTransactionProbe(manager, innerContext, probeTable, 7))
					return nestedError
				},
			), nestedError)
			return nil
		})
		require.ErrorIs(t, err, ErrTransactionFailed)
		require.Equal(t, int64(0), d5TransactionProbeCount(t, db, probeTable, 6))
		require.Equal(t, int64(0), d5TransactionProbeCount(t, db, probeTable, 7))
	})
}

func d5InsertTransactionProbe(
	manager *TransactionManager,
	ctx context.Context,
	table string,
	id int64,
) error {
	connection, err := manager.Connection(ctx)
	if err != nil {
		return err
	}
	return connection.Exec(
		"INSERT INTO "+table+" (id) VALUES (?)",
		id,
	).Error
}

func d5TransactionProbeCount(
	t *testing.T,
	db *gorm.DB,
	table string,
	id int64,
) int64 {
	t.Helper()

	var count int64
	require.NoError(t, db.Raw(
		"SELECT COUNT(*) FROM "+table+" WHERE id = ?",
		id,
	).Scan(&count).Error)
	return count
}
