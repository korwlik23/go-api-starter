package gorm_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/korwlik23/go-api-starter/internal/app"
	auditgorm "github.com/korwlik23/go-api-starter/internal/modules/audit/adapters/gorm"
	auditdomain "github.com/korwlik23/go-api-starter/internal/modules/audit/domain"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/tests/testsupport"
	"github.com/stretchr/testify/require"
)

func TestEventRepositoryAppendsAndPagesSystemEvents(t *testing.T) {
	if os.Getenv("RUN_DATABASE_INTEGRATION_TESTS") != "1" {
		t.Skip("database integration test is disabled")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	cfg := auditDatabaseConfig(t)
	require.NoError(t, testsupport.RequireDisposableDatabase(
		config.Config{
			App:      config.AppConfig{Environment: config.Environment(os.Getenv("APP_ENV"))},
			Database: cfg,
		},
		os.Getenv("ALLOW_TEST_DATABASE_RESET"),
	))
	db, err := database.Open(ctx, cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	migrator, err := database.NewMigrator(sqlDB, cfg, app.MigrationModules())
	require.NoError(t, err)
	require.NoError(t, migrator.Up(ctx))
	require.NoError(t, db.WithContext(ctx).Exec("DELETE FROM audit_events").Error)

	repository, err := auditgorm.NewEventRepository(db)
	require.NoError(t, err)
	occurredAt := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	for index, id := range []string{
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67586",
	} {
		event, eventErr := auditdomain.NewEvent(auditdomain.EventInput{
			ID:          id,
			Scope:       auditdomain.ScopeSystem,
			Actor:       auditdomain.Actor{Type: auditdomain.ActorSystem},
			Action:      "operations.modules.reconciled",
			OperationID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67587",
			Data:        map[string]any{"revision": index + 1},
			OccurredAt:  occurredAt.Add(time.Duration(index) * time.Second),
		})
		require.NoError(t, eventErr)
		require.NoError(t, repository.Append(ctx, event))
		time.Sleep(2 * time.Millisecond)
	}

	first, err := repository.ListSystem(ctx, nil, 1)
	require.NoError(t, err)
	require.Len(t, first.Items, 1)
	require.NotNil(t, first.NextCursor)
	require.False(t, first.Items[0].RecordedAt.IsZero())

	second, err := repository.ListSystem(ctx, first.NextCursor, 1)
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	require.NotEqual(t, first.Items[0].ID, second.Items[0].ID)

	accountPage, err := repository.ListAccount(
		ctx,
		"018f1f64-7b2a-7c10-8bf2-3c40d5f67588",
		nil,
		10,
	)
	require.NoError(t, err)
	require.Empty(t, accountPage.Items)
}
