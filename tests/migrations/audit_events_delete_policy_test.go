package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func m17AssertAuditEventDeletePolicy(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	cfg config.DatabaseConfig,
	seedOffset byte,
) {
	t.Helper()

	referencedAccount := m1AccountID(t, cfg, 150+seedOffset)
	unusedAccount := m1AccountID(t, cfg, 151+seedOffset)
	referencedUser := m3UserID(t, cfg, 150+seedOffset)
	unusedUser := m3UserID(t, cfg, 151+seedOffset)
	m2InsertAccount(t, ctx, db, cfg, referencedAccount, fmt.Sprintf("m17-referenced-account-%d", seedOffset))
	m2InsertAccount(t, ctx, db, cfg, unusedAccount, fmt.Sprintf("m17-unused-account-%d", seedOffset))
	m2InsertUser(t, ctx, db, cfg, referencedUser, fmt.Sprintf("m17-referenced-%d@example.test", seedOffset))
	m2InsertUser(t, ctx, db, cfg, unusedUser, fmt.Sprintf("m17-unused-%d@example.test", seedOffset))

	event := m17NewAuditEvent(t, cfg, 152+seedOffset, "account", referencedAccount, "user", referencedUser, "audit.retention", nil, time.Now().UTC())
	m17InsertAuditEvent(t, ctx, db, cfg, event)

	updatedAccount := m1AccountID(t, cfg, 153+seedOffset)
	_, err := db.ExecContext(ctx,
		"UPDATE accounts SET id = "+migrationPlaceholder(cfg, 1)+" WHERE id = "+migrationPlaceholder(cfg, 2),
		updatedAccount, referencedAccount,
	)
	require.Error(t, err, "referenced account identity must remain immutable")
	_, err = db.ExecContext(ctx, "DELETE FROM accounts WHERE id = "+migrationPlaceholder(cfg, 1), referencedAccount)
	require.Error(t, err, "referenced account cannot be hard-deleted before audit retention purge")

	updatedUser := m3UserID(t, cfg, 153+seedOffset)
	_, err = db.ExecContext(ctx,
		"UPDATE users SET id = "+migrationPlaceholder(cfg, 1)+" WHERE id = "+migrationPlaceholder(cfg, 2),
		updatedUser, referencedUser,
	)
	require.Error(t, err, "referenced actor identity must remain immutable")
	_, err = db.ExecContext(ctx, "DELETE FROM users WHERE id = "+migrationPlaceholder(cfg, 1), referencedUser)
	require.Error(t, err, "referenced actor cannot be hard-deleted before audit retention purge")
	require.Equal(t, 1, m17AuditEventCount(t, ctx, db, cfg, event.id))

	_, err = db.ExecContext(ctx, "DELETE FROM accounts WHERE id = "+migrationPlaceholder(cfg, 1), unusedAccount)
	require.NoError(t, err, "unused account remains independently deletable")
	_, err = db.ExecContext(ctx, "DELETE FROM users WHERE id = "+migrationPlaceholder(cfg, 1), unusedUser)
	require.NoError(t, err, "unused user remains independently deletable")
}
