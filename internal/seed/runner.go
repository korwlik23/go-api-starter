package seed

import (
	"context"
	"errors"
	"fmt"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	gormdb "gorm.io/gorm"
)

type Summary struct {
	Profile   Profile
	AccountID string
}

// Run applies the non-foundation portion of a cumulative local profile. The
// seed command invokes app.RunBootstrap first so foundation creation keeps the
// existing audited bootstrap path and its atomicity guarantees.
func Run(ctx context.Context, lookup LookupFunc) (Summary, error) {
	if ctx == nil || lookup == nil {
		return Summary{}, ErrInvalidConfig
	}
	seedConfig, err := ParseConfig(lookup)
	if err != nil {
		return Summary{}, err
	}
	runtimeConfig, err := config.Load(config.LookupFunc(lookup))
	if err != nil {
		return Summary{}, err
	}
	if err := config.Validate(runtimeConfig); err != nil {
		return Summary{}, err
	}
	db, err := database.Open(ctx, runtimeConfig.Database)
	if err != nil {
		return Summary{}, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return Summary{}, database.ErrUnavailable
	}
	defer func() { _ = sqlDB.Close() }()

	ids := NewIDs()
	ids.Account = seedConfig.AccountID
	ids.OwnerUser = ids.User("owner")
	seedStore, err := newStore(db, ids)
	if err != nil {
		return Summary{}, err
	}
	var userIDs map[string]string
	var localeIDs map[string]string
	err = db.WithContext(ctx).Transaction(func(txContext *gormdb.DB) error {
		seedStore.db = txContext
		transactionContext := txContext.Statement.Context
		if transactionContext == nil {
			transactionContext = ctx
		}
		if _, err := seedStore.ensureFoundation(transactionContext); err != nil {
			return err
		}
		if !seedConfig.Profile.Includes(ProfilePermissionMatrix) {
			return nil
		}
		userIDs, err = seedStore.seedIdentities(transactionContext, seedConfig)
		if err != nil {
			return err
		}
		if err := seedStore.seedAuthorization(transactionContext, seedConfig, userIDs); err != nil {
			return err
		}
		if !seedConfig.Profile.Includes(ProfileDemoCMS) {
			return nil
		}
		localeIDs, err = seedStore.seedLocalization(transactionContext)
		if err != nil {
			return err
		}
		if err := seedStore.seedContent(transactionContext, seedConfig, localeIDs, userIDs); err != nil {
			return err
		}
		if err := seedStore.seedNavigation(transactionContext, seedConfig, localeIDs); err != nil {
			return err
		}
		if err := seedStore.seedMedia(transactionContext, seedConfig); err != nil {
			return err
		}
		return seedStore.seedOperations(transactionContext, seedConfig, userIDs)
	})
	if err != nil {
		return Summary{}, errors.Join(fmt.Errorf("seed: transaction failed"), err)
	}
	return Summary{Profile: seedConfig.Profile, AccountID: seedConfig.AccountID}, nil
}
