package gorm

import (
	"context"
	"errors"
	"fmt"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var (
	ErrInvalidPermissionRepository = errors.New(
		"invalid permission repository",
	)
	ErrInvalidEffectiveGrant = errors.New("invalid effective permission grant")
)

type PermissionRepository struct {
	db *gormdb.DB
}

type effectiveGrantRecord struct {
	RoleID          databaseid.ID  `gorm:"column:role_id"`
	AccountID       *databaseid.ID `gorm:"column:account_id"`
	PermissionID    databaseid.ID  `gorm:"column:permission_id"`
	PermissionKey   string         `gorm:"column:permission_key"`
	ModuleID        string         `gorm:"column:module_id"`
	Scope           string         `gorm:"column:scope"`
	Source          string         `gorm:"column:source"`
	CatalogEpoch    int64          `gorm:"column:catalog_epoch"`
	CatalogChecksum []byte         `gorm:"column:catalog_checksum"`
}

func NewPermissionRepository(
	db *gormdb.DB,
) (*PermissionRepository, error) {
	if db == nil {
		return nil, ErrInvalidPermissionRepository
	}
	return &PermissionRepository{db: db}, nil
}

func (repository *PermissionRepository) EffectiveGrants(
	ctx context.Context,
	userID string,
	accountID string,
) ([]ports.EffectiveGrant, error) {
	encodedUserID, err := databaseid.Encode(
		repository.db.Dialector.Name(),
		userID,
	)
	if err != nil {
		return nil, ErrInvalidEffectiveGrant
	}
	var encodedAccountID any
	if accountID != "" {
		encodedAccountID, err = databaseid.Encode(
			repository.db.Dialector.Name(),
			accountID,
		)
		if err != nil {
			return nil, ErrInvalidEffectiveGrant
		}
	}
	var records []effectiveGrantRecord
	err = repository.db.WithContext(ctx).Raw(
		effectiveGrantsQuery,
		encodedUserID,
		encodedAccountID,
		encodedUserID,
	).Scan(&records).Error
	if err != nil {
		return nil, fmt.Errorf("read effective grants: %w", err)
	}
	grants := make([]ports.EffectiveGrant, 0, len(records))
	for _, record := range records {
		grant, mapErr := mapEffectiveGrant(record, accountID)
		if mapErr != nil {
			return nil, mapErr
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

func mapEffectiveGrant(
	record effectiveGrantRecord,
	requestedAccountID string,
) (ports.EffectiveGrant, error) {
	key, err := authorizationdomain.ParsePermissionKey(record.PermissionKey)
	if err != nil ||
		key.Module != record.ModuleID ||
		string(key.Scope) != record.Scope ||
		record.RoleID.String() == "" ||
		record.PermissionID.String() == "" ||
		record.CatalogEpoch < 1 ||
		len(record.CatalogChecksum) != 32 {
		return ports.EffectiveGrant{}, ErrInvalidEffectiveGrant
	}
	source := ports.GrantSource(record.Source)
	accountID := ""
	if record.AccountID != nil {
		accountID = record.AccountID.String()
	}
	switch source {
	case ports.GrantSourceAccountRole:
		if accountID == "" ||
			accountID != requestedAccountID ||
			key.Scope == authorizationdomain.PermissionScopeSystem {
			return ports.EffectiveGrant{}, ErrInvalidEffectiveGrant
		}
	case ports.GrantSourceSystemRole:
		if accountID != "" ||
			key.Scope != authorizationdomain.PermissionScopeSystem {
			return ports.EffectiveGrant{}, ErrInvalidEffectiveGrant
		}
	default:
		return ports.EffectiveGrant{}, ErrInvalidEffectiveGrant
	}
	var checksum [32]byte
	copy(checksum[:], record.CatalogChecksum)
	return ports.EffectiveGrant{
		RoleID:          record.RoleID.String(),
		AccountID:       accountID,
		PermissionID:    record.PermissionID.String(),
		Key:             key,
		Source:          source,
		CatalogEpoch:    record.CatalogEpoch,
		CatalogChecksum: checksum,
	}, nil
}

const effectiveGrantsQuery = `
SELECT
	roles.id AS role_id,
	roles.account_id AS account_id,
	permissions.id AS permission_id,
	permissions.permission_key,
	permissions.module_id,
	permissions.scope,
	'account_role' AS source,
	module_states.catalog_epoch,
	module_states.catalog_checksum
FROM memberships
JOIN membership_roles
	ON membership_roles.account_id = memberships.account_id
	AND membership_roles.membership_id = memberships.id
JOIN roles
	ON roles.id = membership_roles.role_id
	AND roles.account_id = membership_roles.account_id
	AND roles.owner_type = 'account'
JOIN role_permissions ON role_permissions.role_id = roles.id
JOIN permissions ON permissions.id = role_permissions.permission_id
JOIN module_states
	ON module_states.module_id = permissions.module_id
	AND module_states.enabled = TRUE
WHERE memberships.user_id = ?
	AND memberships.account_id = ?
	AND memberships.status = 'active'
	AND permissions.active = TRUE
UNION ALL
SELECT
	roles.id AS role_id,
	roles.account_id AS account_id,
	permissions.id AS permission_id,
	permissions.permission_key,
	permissions.module_id,
	permissions.scope,
	'system_role' AS source,
	module_states.catalog_epoch,
	module_states.catalog_checksum
FROM system_role_assignments
JOIN roles
	ON roles.id = system_role_assignments.role_id
	AND roles.owner_type = 'system'
	AND roles.account_id IS NULL
JOIN role_permissions ON role_permissions.role_id = roles.id
JOIN permissions ON permissions.id = role_permissions.permission_id
JOIN module_states
	ON module_states.module_id = permissions.module_id
	AND module_states.enabled = TRUE
WHERE system_role_assignments.user_id = ?
	AND system_role_assignments.role_owner_type = 'system'
	AND permissions.active = TRUE
`

var _ ports.PermissionRepository = (*PermissionRepository)(nil)
