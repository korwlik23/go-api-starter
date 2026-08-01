package gorm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var ErrInvalidRoleCommandRepository = errors.New("invalid role command repository")

type RoleCommandRepository struct {
	connections database.ConnectionProvider
	dialect     string
}

type roleCommandRecord struct {
	ID        databaseid.ID  `gorm:"column:id"`
	OwnerType string         `gorm:"column:owner_type"`
	AccountID *databaseid.ID `gorm:"column:account_id"`
	Name      string         `gorm:"column:name"`
	Version   int64          `gorm:"column:version"`
	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
}

func (roleCommandRecord) TableName() string {
	return "roles"
}

func NewRoleCommandRepository(db *gormdb.DB) (*RoleCommandRepository, error) {
	if db == nil || db.Dialector == nil {
		return nil, ErrInvalidRoleCommandRepository
	}
	return &RoleCommandRepository{
		connections: directRoleCommandConnection{db: db},
		dialect:     db.Dialector.Name(),
	}, nil
}

func NewTransactionalRoleCommandRepository(
	connections database.ConnectionProvider,
	dialect string,
) (*RoleCommandRepository, error) {
	if connections == nil || (dialect != "postgres" && dialect != "mysql") {
		return nil, ErrInvalidRoleCommandRepository
	}
	return &RoleCommandRepository{connections: connections, dialect: dialect}, nil
}

func (repository *RoleCommandRepository) CreateAccountRole(
	ctx context.Context,
	accountID string,
	name string,
) (authorizationdomain.Role, error) {
	db, connectionErr := repository.connection(ctx)
	if connectionErr != nil {
		return authorizationdomain.Role{}, connectionErr
	}
	if strings.TrimSpace(accountID) == "" || !validRoleCommandName(name) {
		return authorizationdomain.Role{}, ErrInvalidRoleCommandRepository
	}
	encodedAccountID, err := databaseid.Encode(repository.dialect, accountID)
	if err != nil {
		return authorizationdomain.Role{}, ErrInvalidRoleCommandRepository
	}
	roleUUID, err := uuid.NewV7()
	if err != nil {
		return authorizationdomain.Role{}, fmt.Errorf("generate role id: %w", err)
	}
	roleID := roleUUID.String()
	encodedRoleID, err := databaseid.Encode(repository.dialect, roleID)
	if err != nil {
		return authorizationdomain.Role{}, ErrInvalidRoleCommandRepository
	}
	now := time.Now().UTC()
	result := db.Exec(
		`INSERT INTO roles (id, owner_type, account_id, name, version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		encodedRoleID,
		string(authorizationdomain.RoleOwnerAccount),
		encodedAccountID,
		name,
		int64(1),
		now,
		now,
	)
	if result.Error != nil {
		return authorizationdomain.Role{}, fmt.Errorf("create account role: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return authorizationdomain.Role{}, ports.ErrRoleConflict
	}
	return authorizationdomain.Role{
		ID: roleID, OwnerType: authorizationdomain.RoleOwnerAccount,
		AccountID: accountID, Name: name, Version: 1,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (repository *RoleCommandRepository) UpdateAccountRoleName(
	ctx context.Context,
	accountID string,
	roleID string,
	expectedVersion int64,
	name string,
) (authorizationdomain.Role, error) {
	db, connectionErr := repository.connection(ctx)
	if connectionErr != nil {
		return authorizationdomain.Role{}, connectionErr
	}
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(roleID) == "" ||
		expectedVersion < 1 || !validRoleCommandName(name) {
		return authorizationdomain.Role{}, ErrInvalidRoleCommandRepository
	}
	encodedAccountID, encodedRoleID, err := repository.encodeOwnedIDs(accountID, roleID)
	if err != nil {
		return authorizationdomain.Role{}, ports.ErrRoleNotFound
	}
	var record roleCommandRecord
	findErr := db.
		Where(
			"id = ? AND account_id = ? AND owner_type = ?",
			encodedRoleID,
			encodedAccountID,
			string(authorizationdomain.RoleOwnerAccount),
		).
		First(&record).Error
	if errors.Is(findErr, gormdb.ErrRecordNotFound) {
		return authorizationdomain.Role{}, ports.ErrRoleNotFound
	}
	if findErr != nil {
		return authorizationdomain.Role{}, fmt.Errorf("find account role for update: %w", findErr)
	}
	now := time.Now().UTC()
	result := db.Exec(
		`UPDATE roles
SET name = ?, version = ?, updated_at = ?
WHERE id = ? AND account_id = ? AND owner_type = ? AND version = ?`,
		name,
		expectedVersion+1,
		now,
		encodedRoleID,
		encodedAccountID,
		string(authorizationdomain.RoleOwnerAccount),
		expectedVersion,
	)
	if result.Error != nil {
		return authorizationdomain.Role{}, fmt.Errorf("update account role: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return authorizationdomain.Role{}, ports.ErrRoleConflict
	}
	return authorizationdomain.Role{
		ID: record.ID.String(), OwnerType: authorizationdomain.RoleOwnerAccount,
		AccountID: accountID, Name: name, Version: expectedVersion + 1,
		CreatedAt: record.CreatedAt, UpdatedAt: now,
	}, nil
}

func (repository *RoleCommandRepository) encodeOwnedIDs(
	accountID string,
	roleID string,
) (any, any, error) {
	encodedAccountID, err := databaseid.Encode(repository.dialect, accountID)
	if err != nil {
		return nil, nil, err
	}
	encodedRoleID, err := databaseid.Encode(repository.dialect, roleID)
	return encodedAccountID, encodedRoleID, err
}

func (repository *RoleCommandRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidRoleCommandRepository
	}
	connection, err := repository.connections.Connection(ctx)
	if err != nil || connection == nil {
		if err != nil {
			return nil, fmt.Errorf("resolve role command connection: %w", err)
		}
		return nil, ErrInvalidRoleCommandRepository
	}
	return connection, nil
}

type directRoleCommandConnection struct {
	db *gormdb.DB
}

func (provider directRoleCommandConnection) Connection(ctx context.Context) (*gormdb.DB, error) {
	if provider.db == nil || ctx == nil {
		return nil, ErrInvalidRoleCommandRepository
	}
	return provider.db.WithContext(ctx), nil
}

func validRoleCommandName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" && trimmed == name && len(trimmed) <= 128
}

var _ ports.RoleCommandRepository = (*RoleCommandRepository)(nil)
