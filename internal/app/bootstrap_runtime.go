package app

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	auditgorm "github.com/korwlik23/go-api-starter/internal/modules/audit/adapters/gorm"
	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	operationsgorm "github.com/korwlik23/go-api-starter/internal/modules/operations/adapters/gorm"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

func RunBootstrap(
	ctx context.Context,
	lookup config.LookupFunc,
) (result BootstrapResult, resultErr error) {
	if ctx == nil || lookup == nil {
		return BootstrapResult{}, ErrInvalidBootstrap
	}
	cfg, err := config.Load(lookup)
	if err != nil {
		return BootstrapResult{}, err
	}
	if err := config.Validate(cfg); err != nil {
		return BootstrapResult{}, err
	}
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return BootstrapResult{}, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return BootstrapResult{}, ErrBootstrapFailed
	}
	defer func() {
		resultErr = errors.Join(resultErr, sqlDB.Close())
	}()

	registry, err := Modules()
	if err != nil {
		return BootstrapResult{}, ErrBootstrapFailed
	}
	moduleRepository, err := operationsgorm.NewModuleStateRepository(
		db,
		cfg.Database.MigrationLockTimeout,
	)
	if err != nil {
		return BootstrapResult{}, ErrBootstrapFailed
	}
	moduleProbe, err := newModuleProjectionProbe(
		registry,
		enabledModuleConfiguration(registry),
		moduleRepository,
	)
	if err != nil || moduleProbe(ctx) != nil {
		return BootstrapResult{}, ErrBootstrapFailed
	}

	manager, err := database.NewTransactionManager(db)
	if err != nil {
		return BootstrapResult{}, ErrBootstrapFailed
	}
	store, err := NewGormBootstrapStore(db, manager)
	if err != nil {
		return BootstrapResult{}, err
	}
	auditRepository, err := auditgorm.NewTransactionalEventRepository(
		manager,
		db.Dialector.Name(),
	)
	if err != nil {
		return BootstrapResult{}, err
	}
	auditor, err := auditapp.NewAppendEvent(auditapp.AppendDependencies{
		Repository: auditRepository,
		NewID:      newRuntimeID,
		Now:        time.Now,
	})
	if err != nil {
		return BootstrapResult{}, err
	}
	passwords, err := platformcrypto.NewPasswordHasher(defaultPasswordPolicy())
	if err != nil {
		return BootstrapResult{}, err
	}
	bootstrap, err := NewBootstrap(BootstrapDependencies{
		Store: store, Auditor: auditor, Passwords: passwords,
		Modules: Modules, NewID: newRuntimeID,
	})
	if err != nil {
		return BootstrapResult{}, err
	}
	command := BootstrapCommand{
		Email:           lookupValue(lookup, "BOOTSTRAP_EMAIL"),
		Password:        lookupValue(lookup, "BOOTSTRAP_PASSWORD"),
		AccountSlug:     lookupValue(lookup, "BOOTSTRAP_ACCOUNT_SLUG"),
		AccountRoleName: lookupValue(lookup, "BOOTSTRAP_ACCOUNT_ROLE_NAME"),
		SystemRoleName:  lookupValue(lookup, "BOOTSTRAP_SYSTEM_ROLE_NAME"),
	}
	return bootstrap.Execute(ctx, command)
}

func defaultPasswordPolicy() platformcrypto.PasswordPolicy {
	return platformcrypto.PasswordPolicy{
		MemoryKiB:   64 * 1024,
		Iterations:  2,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
}

func newRuntimeID() (string, error) {
	value, err := uuid.NewV7()
	return value.String(), err
}

func lookupValue(lookup config.LookupFunc, key string) string {
	value, _ := lookup(key)
	return value
}
