package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	localizationgorm "github.com/korwlik23/go-api-starter/internal/modules/localization/adapters/gorm"
	localizationapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

func Migrate(ctx context.Context, lookup config.LookupFunc) (result error) {
	cfg, err := config.Load(lookup)
	if err != nil {
		return err
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return database.ErrUnavailable
	}
	defer func() {
		result = errors.Join(result, sqlDB.Close())
	}()

	migrator, err := database.NewMigrator(sqlDB, cfg.Database, MigrationModules())
	if err != nil {
		return err
	}
	localeRepository, err := localizationgorm.NewLocaleRepository(db)
	if err != nil {
		return err
	}
	localeInitializer, err := localizationapp.NewInitializeLocales(
		localizationapp.InitializeLocaleDependencies{
			Repository: localeRepository,
			NewID: func() (string, error) {
				value, idErr := uuid.NewV7()
				return value.String(), idErr
			},
			Now: time.Now,
		},
	)
	if err != nil {
		return err
	}
	return runMigrationStages(
		ctx,
		migrator.Up,
		func(ctx context.Context) error {
			return localeInitializer.Execute(
				ctx,
				cfg.Localization.InitialLocales,
				cfg.Localization.DefaultLocale,
			)
		},
	)
}

func runMigrationStages(
	ctx context.Context,
	migrateSchema func(context.Context) error,
	initializeLocales func(context.Context) error,
) error {
	if ctx == nil || migrateSchema == nil || initializeLocales == nil {
		return database.ErrMigrationFailed
	}
	if err := migrateSchema(ctx); err != nil {
		return err
	}
	return initializeLocales(ctx)
}
