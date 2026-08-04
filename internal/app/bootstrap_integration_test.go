package app

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	auditgorm "github.com/korwlik23/go-api-starter/internal/modules/audit/adapters/gorm"
	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/tests/testsupport"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBootstrapIntegrationIsAtomicAndIdempotent(t *testing.T) {
	if os.Getenv("RUN_DATABASE_INTEGRATION_TESTS") != "1" {
		t.Skip("database integration test is disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	lookup := bootstrapIntegrationLookup(t)
	cfg, err := config.Load(lookup)
	require.NoError(t, err)
	require.NoError(t, config.Validate(cfg))
	require.NoError(t, testsupport.RequireDisposableDatabase(
		cfg,
		os.Getenv("ALLOW_TEST_DATABASE_RESET"),
	))
	require.NoError(t, Migrate(ctx, lookup))

	db, err := database.Open(ctx, cfg.Database)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	cleanBootstrapIntegration(t, ctx, db)

	manager, err := database.NewTransactionManager(db)
	require.NoError(t, err)
	store, err := NewGormBootstrapStore(db, manager)
	require.NoError(t, err)
	auditRepository, err := auditgorm.NewTransactionalEventRepository(
		manager,
		db.Dialector.Name(),
	)
	require.NoError(t, err)
	auditor, err := auditapp.NewAppendEvent(auditapp.AppendDependencies{
		Repository: auditRepository,
		NewID:      newBootstrapUUID,
		Now:        time.Now,
	})
	require.NoError(t, err)
	passwords, err := platformcrypto.NewPasswordHasher(platformcrypto.PasswordPolicy{
		MemoryKiB:   64 * 1024,
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})
	require.NoError(t, err)
	bootstrap, err := NewBootstrap(BootstrapDependencies{
		Store: store, Auditor: auditor, Passwords: passwords,
		Modules: Modules, NewID: newBootstrapUUID,
	})
	require.NoError(t, err)
	command := BootstrapCommand{
		Email:           "owner@example.test",
		Password:        "correct horse battery staple",
		AccountSlug:     "personal",
		AccountRoleName: "Account Manager",
		SystemRoleName:  "System Manager",
	}

	first, err := bootstrap.Execute(ctx, command)
	require.NoError(t, err)
	require.True(t, first.Created)
	second, err := bootstrap.Execute(ctx, command)
	require.NoError(t, err)
	require.False(t, second.Created)
	require.Equal(t, first.AccountID, second.AccountID)
	require.Equal(t, first.UserID, second.UserID)

	expectedCounts := map[string]int64{
		"accounts":                1,
		"users":                   1,
		"password_credentials":    1,
		"user_profiles":           1,
		"memberships":             1,
		"roles":                   2,
		"role_permissions":        44,
		"membership_roles":        1,
		"system_role_assignments": 1,
		"audit_events":            1,
	}
	for table, expected := range expectedCounts {
		require.Equal(t, expected, bootstrapTableCount(t, ctx, db, table), table)
	}
}

func newBootstrapUUID() (string, error) {
	value, err := uuid.NewV7()
	return value.String(), err
}

func bootstrapIntegrationLookup(t *testing.T) config.LookupFunc {
	t.Helper()

	file, err := os.Open("../../.env.example")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })
	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		require.True(t, ok)
		values[key] = value
	}
	require.NoError(t, scanner.Err())
	return func(key string) (string, bool) {
		if value, exists := os.LookupEnv(key); exists {
			return value, true
		}
		value, exists := values[key]
		return value, exists
	}
}

func cleanBootstrapIntegration(
	t *testing.T,
	ctx context.Context,
	db *gorm.DB,
) {
	t.Helper()

	for _, table := range []string{
		"audit_events",
		"system_role_assignments",
		"membership_roles",
		"role_permissions",
		"roles",
		"memberships",
		"password_credentials",
		"user_profiles",
		"users",
		"accounts",
	} {
		require.NoError(t, db.WithContext(ctx).Exec("DELETE FROM "+table).Error)
	}
}

func bootstrapTableCount(
	t *testing.T,
	ctx context.Context,
	db *gorm.DB,
	table string,
) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.WithContext(ctx).Table(table).Count(&count).Error)
	return count
}
