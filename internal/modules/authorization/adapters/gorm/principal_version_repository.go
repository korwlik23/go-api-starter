package gorm

import (
	"context"
	"errors"
	"fmt"

	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidPrincipalVersionRepository = errors.New(
	"invalid principal version repository",
)

type PrincipalVersionRepository struct {
	db *gormdb.DB
}

func NewPrincipalVersionRepository(
	db *gormdb.DB,
) (*PrincipalVersionRepository, error) {
	if db == nil {
		return nil, ErrInvalidPrincipalVersionRepository
	}
	return &PrincipalVersionRepository{db: db}, nil
}

func (repository *PrincipalVersionRepository) BumpUserVersion(
	ctx context.Context,
	userID string,
	expectedVersion int64,
) (int64, error) {
	return repository.bump(
		ctx,
		"users",
		"id = ? AND authorization_version = ?",
		[]string{userID},
		expectedVersion,
	)
}

func (repository *PrincipalVersionRepository) BumpMembershipVersion(
	ctx context.Context,
	accountID string,
	membershipID string,
	expectedVersion int64,
) (int64, error) {
	return repository.bump(
		ctx,
		"memberships",
		"account_id = ? AND id = ? AND authorization_version = ?",
		[]string{accountID, membershipID},
		expectedVersion,
	)
}

func (repository *PrincipalVersionRepository) bump(
	ctx context.Context,
	table string,
	predicate string,
	ids []string,
	expectedVersion int64,
) (int64, error) {
	if expectedVersion < 1 {
		return 0, ErrInvalidPrincipalVersionRepository
	}
	arguments := make([]any, 0, len(ids)+1)
	for _, value := range ids {
		encoded, err := databaseid.Encode(
			repository.db.Dialector.Name(),
			value,
		)
		if err != nil {
			return 0, ports.ErrPrincipalVersionConflict
		}
		arguments = append(arguments, encoded)
	}
	arguments = append(arguments, expectedVersion)
	result := repository.db.WithContext(ctx).
		Table(table).
		Where(predicate, arguments...).
		Update("authorization_version", expectedVersion+1)
	if result.Error != nil {
		return 0, fmt.Errorf("bump principal authorization version: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return 0, ports.ErrPrincipalVersionConflict
	}
	return expectedVersion + 1, nil
}

var _ ports.PrincipalVersionRepository = (*PrincipalVersionRepository)(nil)
