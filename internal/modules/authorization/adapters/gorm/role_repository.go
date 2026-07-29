package gorm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidRoleRepository = errors.New("invalid role repository")

type RoleRepository struct {
	db *gormdb.DB
}

type roleRecord struct {
	ID        databaseid.ID  `gorm:"column:id"`
	OwnerType string         `gorm:"column:owner_type"`
	AccountID *databaseid.ID `gorm:"column:account_id"`
	Name      string         `gorm:"column:name"`
	Version   int64          `gorm:"column:version"`
}

func (roleRecord) TableName() string {
	return "roles"
}

func NewRoleRepository(db *gormdb.DB) (*RoleRepository, error) {
	if db == nil {
		return nil, ErrInvalidRoleRepository
	}
	return &RoleRepository{db: db}, nil
}

func (repository *RoleRepository) FindAccountRole(
	ctx context.Context,
	accountID string,
	roleID string,
) (authorizationdomain.Role, error) {
	encodedAccountID, encodedRoleID, err := repository.encodeOwnedIDs(
		accountID,
		roleID,
	)
	if err != nil {
		return authorizationdomain.Role{}, ports.ErrRoleNotFound
	}
	var record roleRecord
	err = repository.db.WithContext(ctx).
		Where(
			"id = ? AND account_id = ? AND owner_type = ?",
			encodedRoleID,
			encodedAccountID,
			string(authorizationdomain.RoleOwnerAccount),
		).
		First(&record).
		Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return authorizationdomain.Role{}, ports.ErrRoleNotFound
	}
	if err != nil {
		return authorizationdomain.Role{}, fmt.Errorf("find account role: %w", err)
	}
	if record.AccountID == nil || record.Version < 1 {
		return authorizationdomain.Role{}, ErrInvalidRoleRepository
	}
	return authorizationdomain.Role{
		ID:        record.ID.String(),
		OwnerType: authorizationdomain.RoleOwnerType(record.OwnerType),
		AccountID: record.AccountID.String(),
		Name:      record.Name,
		Version:   record.Version,
	}, nil
}

func (repository *RoleRepository) UpdateAccountRoleName(
	ctx context.Context,
	accountID string,
	roleID string,
	expectedVersion int64,
	name string,
) (int64, error) {
	if expectedVersion < 1 ||
		strings.TrimSpace(name) == "" ||
		name != strings.TrimSpace(name) {
		return 0, ErrInvalidRoleRepository
	}
	encodedAccountID, encodedRoleID, err := repository.encodeOwnedIDs(
		accountID,
		roleID,
	)
	if err != nil {
		return 0, ports.ErrRoleNotFound
	}
	result := repository.db.WithContext(ctx).
		Model(&roleRecord{}).
		Where(
			"id = ? AND account_id = ? AND owner_type = ? AND version = ?",
			encodedRoleID,
			encodedAccountID,
			string(authorizationdomain.RoleOwnerAccount),
			expectedVersion,
		).
		Updates(map[string]any{
			"name":    name,
			"version": expectedVersion + 1,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("update account role: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return 0, ports.ErrRoleConflict
	}
	return expectedVersion + 1, nil
}

func (repository *RoleRepository) encodeOwnedIDs(
	accountID string,
	roleID string,
) (any, any, error) {
	encodedAccountID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		accountID,
	)
	if err != nil {
		return nil, nil, err
	}
	encodedRoleID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		roleID,
	)
	return encodedAccountID, encodedRoleID, err
}

var _ ports.RoleRepository = (*RoleRepository)(nil)
