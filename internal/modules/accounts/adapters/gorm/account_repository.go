package gorm

import (
	"context"
	"errors"
	"fmt"

	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	gormdb "gorm.io/gorm"
)

var (
	ErrInvalidAccountRepository = errors.New("invalid account repository")
	ErrAmbiguousAccount         = errors.New("multiple active accounts found")
)

type AccountRepository struct {
	db *gormdb.DB
}

type resolvedAccountRecord struct {
	AccountID            databaseID `gorm:"column:account_id"`
	AccountSlug          string     `gorm:"column:account_slug"`
	AccountStatus        string     `gorm:"column:account_status"`
	MembershipID         databaseID `gorm:"column:membership_id"`
	UserID               databaseID `gorm:"column:user_id"`
	MembershipStatus     string     `gorm:"column:membership_status"`
	AuthorizationVersion int64      `gorm:"column:authorization_version"`
}

func NewAccountRepository(
	db *gormdb.DB,
) (*AccountRepository, error) {
	if db == nil {
		return nil, ErrInvalidAccountRepository
	}
	return &AccountRepository{db: db}, nil
}

func (repository *AccountRepository) ResolveActiveForUser(
	ctx context.Context,
	userID string,
) (ports.ResolvedAccount, error) {
	encodedUserID, err := encodeDatabaseID(
		repository.db.Dialector.Name(),
		userID,
	)
	if err != nil {
		return ports.ResolvedAccount{}, ports.ErrAccountNotFound
	}
	var records []resolvedAccountRecord
	err = repository.db.WithContext(ctx).
		Table("memberships").
		Select(
			"accounts.id AS account_id, accounts.slug AS account_slug, "+
				"accounts.status AS account_status, "+
				"memberships.id AS membership_id, memberships.user_id, "+
				"memberships.status AS membership_status, "+
				"memberships.authorization_version",
		).
		Joins("JOIN accounts ON accounts.id = memberships.account_id").
		Where(
			"memberships.user_id = ? AND memberships.status = ? "+
				"AND accounts.status = ?",
			encodedUserID,
			string(accountsdomain.MembershipStatusActive),
			string(accountsdomain.AccountStatusActive),
		).
		Order("memberships.created_at ASC").
		Limit(2).
		Find(&records).
		Error
	if err != nil {
		return ports.ResolvedAccount{}, fmt.Errorf("resolve account: %w", err)
	}
	if len(records) == 0 {
		return ports.ResolvedAccount{}, ports.ErrAccountNotFound
	}
	if len(records) != 1 {
		return ports.ResolvedAccount{}, ErrAmbiguousAccount
	}
	record := records[0]
	return ports.ResolvedAccount{
		Account: accountsdomain.Account{
			ID:     record.AccountID.String(),
			Slug:   record.AccountSlug,
			Status: accountsdomain.AccountStatus(record.AccountStatus),
		},
		Membership: accountsdomain.Membership{
			ID:                   record.MembershipID.String(),
			AccountID:            record.AccountID.String(),
			UserID:               record.UserID.String(),
			Status:               accountsdomain.MembershipStatus(record.MembershipStatus),
			AuthorizationVersion: record.AuthorizationVersion,
		},
	}, nil
}

var _ ports.AccountRepository = (*AccountRepository)(nil)
