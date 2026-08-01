package gorm

import (
	"context"
	"errors"
	"fmt"

	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidGrantRolePermissionRepository = errors.New(
	"invalid grant role permission repository",
)

type GrantRolePermissionRepository struct {
	connections database.ConnectionProvider
	dialect     string
}

func NewTransactionalGrantRolePermissionRepository(
	connections database.ConnectionProvider,
	dialect string,
) (*GrantRolePermissionRepository, error) {
	if connections == nil || (dialect != "postgres" && dialect != "mysql") {
		return nil, ErrInvalidGrantRolePermissionRepository
	}
	return &GrantRolePermissionRepository{connections: connections, dialect: dialect}, nil
}

type lockedGrantRole struct {
	ID        databaseid.ID  `gorm:"column:id"`
	OwnerType string         `gorm:"column:owner_type"`
	AccountID *databaseid.ID `gorm:"column:account_id"`
	Version   int64          `gorm:"column:version"`
}

type grantPermission struct {
	ID        databaseid.ID `gorm:"column:id"`
	ModuleID  string        `gorm:"column:module_id"`
	Scope     string        `gorm:"column:scope"`
	Active    bool          `gorm:"column:active"`
	Delegable bool          `gorm:"column:delegable"`
}

func (repository *GrantRolePermissionRepository) GrantRolePermission(
	ctx context.Context,
	command authorizationapp.GrantRolePermissionCommand,
) (authorizationapp.GrantRolePermissionResult, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return authorizationapp.GrantRolePermissionResult{}, ErrInvalidGrantRolePermissionRepository
	}
	db, err := repository.connections.Connection(ctx)
	if err != nil {
		return authorizationapp.GrantRolePermissionResult{}, ErrInvalidGrantRolePermissionRepository
	}
	roleID, err := databaseid.Encode(repository.dialect, command.RoleID)
	if err != nil {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantConflict
	}
	var role lockedGrantRole
	if err := db.WithContext(ctx).
		Table("roles").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", roleID).
		First(&role).Error; err != nil {
		if errors.Is(err, gormdb.ErrRecordNotFound) {
			return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantConflict
		}
		return authorizationapp.GrantRolePermissionResult{}, fmt.Errorf("lock role: %w", err)
	}
	if role.Version != command.ExpectedVersion {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantConflict
	}

	var permission grantPermission
	if err := db.WithContext(ctx).
		Table("permissions").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("permissions.id, permissions.module_id, permissions.scope, permissions.active, permissions.delegable").
		Where("permissions.permission_key = ?", command.PermissionKey).
		First(&permission).Error; err != nil {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantDenied
	}
	parsedPermission, err := authorizationdomain.ParsePermissionKey(command.PermissionKey)
	if err != nil || parsedPermission.Module != permission.ModuleID ||
		string(parsedPermission.Scope) != permission.Scope {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantDenied
	}
	if !permission.Active || !permission.Delegable || !moduleEnabled(db, ctx, permission.ModuleID) {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantDenied
	}
	roleOwner := authorizationapp.RoleOwner(role.OwnerType)
	if role.AccountID == nil {
		roleOwner = authorizationapp.RoleOwnerSystem
	}
	if roleOwner == authorizationapp.RoleOwnerAccount && role.AccountID == nil {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantDenied
	}
	if roleOwner == authorizationapp.RoleOwnerSystem && permission.Scope != string(authorizationdomain.PermissionScopeSystem) {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantDenied
	}
	if roleOwner == authorizationapp.RoleOwnerAccount && permission.Scope == string(authorizationdomain.PermissionScopeSystem) {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantDenied
	}
	if !grantorHasPermission(db, ctx, command.GrantorUserID, command.AccountID, authorizationapp.PermissionRolePermissionGrant) ||
		!grantorHasPermission(db, ctx, command.GrantorUserID, command.AccountID, command.PermissionKey) {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantDenied
	}

	permissionID, err := databaseid.Encode(repository.dialect, permission.ID.String())
	if err != nil {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantConflict
	}
	result := db.WithContext(ctx).Table("role_permissions").Create(map[string]any{
		"role_id":       roleID,
		"permission_id": permissionID,
	})
	if result.Error != nil {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantConflict
	}
	versionUpdate := db.WithContext(ctx).Table("roles").
		Where("id = ? AND version = ?", roleID, command.ExpectedVersion).
		Update("version", command.ExpectedVersion+1)
	if versionUpdate.Error != nil || versionUpdate.RowsAffected != 1 {
		return authorizationapp.GrantRolePermissionResult{}, authorizationapp.ErrGrantConflict
	}
	affected, err := bumpAffectedUsers(db, ctx, role, roleID)
	if err != nil {
		return authorizationapp.GrantRolePermissionResult{}, err
	}
	return authorizationapp.GrantRolePermissionResult{
		RoleID:             command.RoleID,
		PermissionKey:      command.PermissionKey,
		Version:            command.ExpectedVersion + 1,
		AffectedPrincipals: affected,
	}, nil
}

func moduleEnabled(db *gormdb.DB, ctx context.Context, moduleID string) bool {
	var enabled bool
	err := db.WithContext(ctx).Table("module_states").Select("enabled").
		Where("module_id = ?", moduleID).Scan(&enabled).Error
	return err == nil && enabled
}

func grantorHasPermission(
	db *gormdb.DB,
	ctx context.Context,
	userID string,
	accountID string,
	permissionKey string,
) bool {
	encodedUser, err := databaseid.Encode(db.Dialector.Name(), userID)
	if err != nil {
		return false
	}
	var encodedAccount any
	if accountID != "" {
		encodedAccount, err = databaseid.Encode(db.Dialector.Name(), accountID)
		if err != nil {
			return false
		}
	}
	var count int64
	err = db.WithContext(ctx).Table("role_permissions").
		Joins("JOIN roles ON roles.id = role_permissions.role_id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Joins("JOIN module_states ON module_states.module_id = permissions.module_id").
		Where("permissions.permission_key = ? AND permissions.active = TRUE AND module_states.enabled = TRUE", permissionKey).
		Where("((roles.owner_type = 'system' AND roles.account_id IS NULL AND EXISTS (SELECT 1 FROM system_role_assignments sra WHERE sra.user_id = ? AND sra.role_id = roles.id AND sra.role_owner_type = 'system')) OR (roles.owner_type = 'account' AND roles.account_id = ? AND EXISTS (SELECT 1 FROM memberships m JOIN membership_roles mr ON mr.account_id = m.account_id AND mr.membership_id = m.id WHERE m.user_id = ? AND m.account_id = roles.account_id AND m.status = 'active' AND mr.role_id = roles.id)))", encodedUser, encodedAccount, encodedUser).
		Count(&count).Error
	return err == nil && count > 0
}

func bumpAffectedUsers(
	db *gormdb.DB,
	ctx context.Context,
	role lockedGrantRole,
	roleID any,
) (int, error) {
	var usersQuery *gormdb.DB
	if role.OwnerType == string(authorizationdomain.RoleOwnerSystem) {
		usersQuery = db.Table("system_role_assignments").Select("user_id").Where("role_id = ? AND role_owner_type = 'system'", roleID)
	} else {
		usersQuery = db.Table("memberships").Select("memberships.user_id").
			Joins("JOIN membership_roles ON membership_roles.account_id = memberships.account_id AND membership_roles.membership_id = memberships.id").
			Where("membership_roles.account_id = ? AND membership_roles.role_id = ? AND memberships.status = 'active'", role.AccountID, roleID)
	}
	result := db.WithContext(ctx).Table("users").
		Where("id IN (?)", usersQuery).
		UpdateColumn("authorization_version", gormdb.Expr("authorization_version + 1"))
	if result.Error != nil {
		return 0, fmt.Errorf("bump affected authorization versions: %w", result.Error)
	}
	return int(result.RowsAffected), nil
}

var _ authorizationapp.GrantRolePermissionRepository = (*GrantRolePermissionRepository)(nil)
