package gorm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidRoleAssignmentRepository = errors.New(
	"invalid role assignment repository",
)

type RoleAssignmentRepository struct {
	connections database.ConnectionProvider
	dialect     string
	now         func() time.Time
	newID       func() (string, error)
}

func NewTransactionalRoleAssignmentRepository(
	connections database.ConnectionProvider,
	dialect string,
) (*RoleAssignmentRepository, error) {
	if connections == nil || (dialect != "postgres" && dialect != "mysql") {
		return nil, ErrInvalidRoleAssignmentRepository
	}
	return &RoleAssignmentRepository{
		connections: connections,
		dialect:     dialect,
		now:         time.Now,
		newID: func() (string, error) {
			id, err := uuid.NewV7()
			return id.String(), err
		},
	}, nil
}

type roleAssignmentRole struct {
	ID        databaseid.ID  `gorm:"column:id"`
	OwnerType string         `gorm:"column:owner_type"`
	AccountID *databaseid.ID `gorm:"column:account_id"`
	Version   int64          `gorm:"column:version"`
}

type systemRoleAssignmentRecord struct {
	AssignmentID databaseid.ID `gorm:"column:assignment_id"`
	UserID       databaseid.ID `gorm:"column:user_id"`
	RoleID       databaseid.ID `gorm:"column:role_id"`
}

type accountRoleAssignmentRecord struct {
	AssignmentID databaseid.ID `gorm:"column:assignment_id"`
	AccountID    databaseid.ID `gorm:"column:account_id"`
	MembershipID databaseid.ID `gorm:"column:membership_id"`
	RoleID       databaseid.ID `gorm:"column:role_id"`
	UserID       databaseid.ID `gorm:"column:user_id"`
}

type activeMembershipRecord struct {
	ID databaseid.ID `gorm:"column:id"`
}

type systemPermissionRecord struct {
	UserID        databaseid.ID `gorm:"column:user_id"`
	PermissionKey string        `gorm:"column:permission_key"`
}

func (repository *RoleAssignmentRepository) AssignRole(
	ctx context.Context,
	command authorizationapp.AssignRoleCommand,
) (authorizationapp.RoleAssignmentResult, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return authorizationapp.RoleAssignmentResult{}, err
	}
	roleID, err := databaseid.Encode(repository.dialect, command.RoleID)
	if err != nil {
		return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentConflict
	}
	targetUserID, err := databaseid.Encode(repository.dialect, command.TargetUserID)
	if err != nil {
		return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentDenied
	}
	role, err := repository.lockRole(ctx, db, roleID)
	if err != nil {
		return authorizationapp.RoleAssignmentResult{}, err
	}
	if role.Version != command.ExpectedVersion {
		return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentConflict
	}
	roleOwner := authorizationapp.RoleOwner(role.OwnerType)
	roleAccountID := roleAccountString(role.AccountID)
	if roleOwner != authorizationapp.RoleOwnerSystem && roleOwner != authorizationapp.RoleOwnerAccount {
		return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentDenied
	}
	if !activeUser(db, ctx, targetUserID) ||
		!moduleEnabled(db, ctx, "authorization") ||
		!grantorHasPermission(db, ctx, command.GrantorUserID, command.AccountID, authorizationapp.PermissionRoleAssignmentAssign) {
		return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentDenied
	}
	targetAccountID := ""
	var membershipID any
	if command.Scope == authorizationapp.AssignmentScopeAccount {
		if command.AccountID == "" {
			return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentDenied
		}
		targetAccountID = command.AccountID
		encodedAccount, encodeErr := databaseid.Encode(repository.dialect, command.AccountID)
		if encodeErr != nil {
			return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentDenied
		}
		var membership activeMembershipRecord
		if queryErr := db.WithContext(ctx).Table("memberships").
			Where("account_id = ? AND user_id = ? AND status = 'active'", encodedAccount, targetUserID).
			First(&membership).Error; queryErr != nil {
			return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentDenied
		}
		membershipID, err = databaseid.Encode(repository.dialect, membership.ID.String())
		if err != nil {
			return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentDenied
		}
	}
	if err := authorizationapp.ValidateRoleAssignment(authorizationapp.RoleAssignmentPolicyInput{
		GrantorPermissions: map[string]bool{authorizationapp.PermissionRoleAssignmentAssign: true},
		Scope:              command.Scope,
		RoleOwner:          roleOwner,
		RoleAccountID:      roleAccountID,
		TargetAccountID:    targetAccountID,
		TargetActive:       true,
		ModuleEnabled:      true,
		AlreadyAssigned:    assignmentExists(db, ctx, command.Scope, targetUserID, membershipID, roleID),
	}); err != nil {
		return authorizationapp.RoleAssignmentResult{}, err
	}
	assignmentID, err := repository.newID()
	if err != nil {
		return authorizationapp.RoleAssignmentResult{}, fmt.Errorf("create assignment ID: %w", err)
	}
	encodedAssignmentID, err := databaseid.Encode(repository.dialect, assignmentID)
	if err != nil {
		return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentConflict
	}
	values := map[string]any{
		"assignment_id": encodedAssignmentID,
		"role_id":       roleID,
		"created_at":    repository.now(),
	}
	if command.Scope == authorizationapp.AssignmentScopeSystem {
		values["user_id"] = targetUserID
		values["role_owner_type"] = string(authorizationdomain.RoleOwnerSystem)
		if err := db.WithContext(ctx).Table("system_role_assignments").Create(values).Error; err != nil {
			return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentConflict
		}
	} else {
		values["account_id"], _ = databaseid.Encode(repository.dialect, command.AccountID)
		values["membership_id"] = membershipID
		if err := db.WithContext(ctx).Table("membership_roles").Create(values).Error; err != nil {
			return authorizationapp.RoleAssignmentResult{}, authorizationapp.ErrRoleAssignmentConflict
		}
	}
	if err := bumpRoleVersion(db, ctx, roleID, command.ExpectedVersion); err != nil {
		return authorizationapp.RoleAssignmentResult{}, err
	}
	if err := bumpUserAuthorizationVersion(db, ctx, targetUserID); err != nil {
		return authorizationapp.RoleAssignmentResult{}, err
	}
	return authorizationapp.RoleAssignmentResult{
		AssignmentID: assignmentID, RoleID: command.RoleID,
		Scope: command.Scope, AccountID: command.AccountID,
		AffectedUserID: command.TargetUserID,
		Version:        command.ExpectedVersion + 1, AffectedPrincipals: 1,
	}, nil
}

func (repository *RoleAssignmentRepository) RevokeRole(
	ctx context.Context,
	command authorizationapp.RevokeRoleCommand,
) (authorizationapp.RoleRevokeResult, error) {
	db, err := repository.connection(ctx)
	if err != nil {
		return authorizationapp.RoleRevokeResult{}, err
	}
	assignmentID, err := databaseid.Encode(repository.dialect, command.AssignmentID)
	if err != nil {
		return authorizationapp.RoleRevokeResult{}, authorizationapp.ErrRoleAssignmentConflict
	}
	scope, userID, accountID, roleID, found := findAssignment(db, ctx, assignmentID)
	if !found {
		return authorizationapp.RoleRevokeResult{}, authorizationapp.ErrRoleAssignmentConflict
	}
	encodedRoleID, err := databaseid.Encode(repository.dialect, roleIDString(roleID))
	if err != nil {
		return authorizationapp.RoleRevokeResult{}, authorizationapp.ErrRoleAssignmentConflict
	}
	encodedUserID, err := databaseid.Encode(repository.dialect, roleIDString(userID))
	if err != nil {
		return authorizationapp.RoleRevokeResult{}, authorizationapp.ErrRoleAssignmentConflict
	}
	role, err := repository.lockRole(ctx, db, encodedRoleID)
	if err != nil {
		return authorizationapp.RoleRevokeResult{}, err
	}
	if role.Version != command.ExpectedVersion {
		return authorizationapp.RoleRevokeResult{}, authorizationapp.ErrRoleAssignmentConflict
	}
	if !grantorHasPermission(db, ctx, command.GrantorUserID, accountID, authorizationapp.PermissionRoleAssignmentRevoke) {
		return authorizationapp.RoleRevokeResult{}, authorizationapp.ErrRoleAssignmentDenied
	}
	if roleOwner := authorizationapp.RoleOwner(role.OwnerType); roleOwner != authorizationapp.RoleOwnerSystem && roleOwner != authorizationapp.RoleOwnerAccount {
		return authorizationapp.RoleRevokeResult{}, authorizationapp.ErrRoleAssignmentDenied
	}
	if scope == authorizationapp.AssignmentScopeSystem {
		if err := lockAuthorizationModule(db, ctx); err != nil {
			return authorizationapp.RoleRevokeResult{}, err
		}
		currentManager, managerErr := hasSystemManagerPermissionsForUser(db, ctx, encodedUserID, nil)
		if managerErr != nil {
			return authorizationapp.RoleRevokeResult{}, fmt.Errorf("check current authorization manager: %w", managerErr)
		}
		if currentManager {
			remainingManager, remainingErr := hasAnySystemManager(db, ctx, assignmentID)
			if remainingErr != nil {
				return authorizationapp.RoleRevokeResult{}, fmt.Errorf("check remaining authorization managers: %w", remainingErr)
			}
			if !remainingManager {
				return authorizationapp.RoleRevokeResult{}, authorizationapp.ErrLastAuthorizationManager
			}
		}
	}
	var result *gormdb.DB
	if scope == authorizationapp.AssignmentScopeSystem {
		result = db.WithContext(ctx).Exec(
			"DELETE FROM system_role_assignments WHERE assignment_id = ? AND role_id = ? AND user_id = ?",
			assignmentID,
			encodedRoleID,
			encodedUserID,
		)
	} else {
		encodedAccountID, encodeErr := databaseid.Encode(repository.dialect, accountID)
		if encodeErr != nil {
			return authorizationapp.RoleRevokeResult{}, authorizationapp.ErrRoleAssignmentConflict
		}
		result = db.WithContext(ctx).Exec(
			"DELETE FROM membership_roles WHERE assignment_id = ? AND role_id = ? AND account_id = ?",
			assignmentID,
			encodedRoleID,
			encodedAccountID,
		)
	}
	if result.Error != nil || result.RowsAffected != 1 {
		return authorizationapp.RoleRevokeResult{}, authorizationapp.ErrRoleAssignmentConflict
	}
	if err := bumpRoleVersion(db, ctx, encodedRoleID, command.ExpectedVersion); err != nil {
		return authorizationapp.RoleRevokeResult{}, err
	}
	if err := bumpUserAuthorizationVersion(db, ctx, encodedUserID); err != nil {
		return authorizationapp.RoleRevokeResult{}, err
	}
	return authorizationapp.RoleRevokeResult{
		AssignmentID:       command.AssignmentID,
		RoleID:             roleIDString(roleID),
		Scope:              scope,
		AccountID:          accountID,
		AffectedUserID:     roleIDString(userID),
		Version:            command.ExpectedVersion + 1,
		Revoked:            true,
		AffectedPrincipals: 1,
	}, nil
}

func (repository *RoleAssignmentRepository) connection(ctx context.Context) (*gormdb.DB, error) {
	if repository == nil || repository.connections == nil || ctx == nil {
		return nil, ErrInvalidRoleAssignmentRepository
	}
	db, err := repository.connections.Connection(ctx)
	if err != nil || db == nil {
		return nil, ErrInvalidRoleAssignmentRepository
	}
	return db, nil
}

func (repository *RoleAssignmentRepository) lockRole(
	ctx context.Context,
	db *gormdb.DB,
	roleID any,
) (roleAssignmentRole, error) {
	var role roleAssignmentRole
	err := db.WithContext(ctx).Table("roles").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", roleID).First(&role).Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return roleAssignmentRole{}, authorizationapp.ErrRoleAssignmentConflict
	}
	if err != nil {
		return roleAssignmentRole{}, fmt.Errorf("lock role: %w", err)
	}
	return role, nil
}

func activeUser(db *gormdb.DB, ctx context.Context, userID any) bool {
	var count int64
	err := db.WithContext(ctx).Table("users").Where("id = ? AND status = 'active'", userID).Count(&count).Error
	return err == nil && count == 1
}

func assignmentExists(db *gormdb.DB, ctx context.Context, scope string, userID, membershipID, roleID any) bool {
	var count int64
	if scope == authorizationapp.AssignmentScopeSystem {
		db = db.WithContext(ctx).Table("system_role_assignments").Where("user_id = ? AND role_id = ?", userID, roleID)
	} else {
		db = db.WithContext(ctx).Table("membership_roles").Where("membership_id = ? AND role_id = ?", membershipID, roleID)
	}
	return db.Count(&count).Error == nil && count > 0
}

func findAssignment(db *gormdb.DB, ctx context.Context, assignmentID any) (string, any, string, any, bool) {
	var system systemRoleAssignmentRecord
	if err := db.WithContext(ctx).Table("system_role_assignments").Where("assignment_id = ?", assignmentID).First(&system).Error; err == nil {
		return authorizationapp.AssignmentScopeSystem, system.UserID, "", system.RoleID, true
	}
	var account accountRoleAssignmentRecord
	if err := db.WithContext(ctx).Table("membership_roles").
		Select("membership_roles.assignment_id, membership_roles.account_id, membership_roles.membership_id, membership_roles.role_id, memberships.user_id").
		Joins("JOIN memberships ON memberships.account_id = membership_roles.account_id AND memberships.id = membership_roles.membership_id").
		Where("membership_roles.assignment_id = ?", assignmentID).First(&account).Error; err == nil {
		return authorizationapp.AssignmentScopeAccount, account.UserID, account.AccountID.String(), account.RoleID, true
	}
	return "", nil, "", nil, false
}

func lockAuthorizationModule(db *gormdb.DB, ctx context.Context) error {
	var state struct {
		ModuleID string `gorm:"column:module_id"`
	}
	err := db.WithContext(ctx).Table("module_states").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("module_id").
		Where("module_id = ?", "authorization").
		First(&state).Error
	if err != nil {
		return fmt.Errorf("lock authorization module: %w", err)
	}
	return nil
}

func hasSystemManagerPermissionsForUser(
	db *gormdb.DB,
	ctx context.Context,
	userID any,
	excludedAssignmentID any,
) (bool, error) {
	var rows []string
	query := db.WithContext(ctx).Table("system_role_assignments sra").
		Select("DISTINCT permissions.permission_key").
		Joins("JOIN role_permissions ON role_permissions.role_id = sra.role_id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Joins("JOIN module_states ON module_states.module_id = permissions.module_id").
		Where("sra.user_id = ? AND sra.role_owner_type = 'system'", userID).
		Where("permissions.active = TRUE AND module_states.enabled = TRUE")
	if excludedAssignmentID != nil {
		query = query.Where("sra.assignment_id <> ?", excludedAssignmentID)
	}
	if err := query.Pluck("permissions.permission_key", &rows).Error; err != nil {
		return false, err
	}
	return hasAllSystemManagerPermissions(rows), nil
}

func hasAnySystemManager(db *gormdb.DB, ctx context.Context, excludedAssignmentID any) (bool, error) {
	var rows []systemPermissionRecord
	query := db.WithContext(ctx).Table("system_role_assignments sra").
		Select("sra.user_id, permissions.permission_key").
		Joins("JOIN role_permissions ON role_permissions.role_id = sra.role_id").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Joins("JOIN module_states ON module_states.module_id = permissions.module_id").
		Where("sra.role_owner_type = 'system'").
		Where("permissions.active = TRUE AND module_states.enabled = TRUE")
	if excludedAssignmentID != nil {
		query = query.Where("sra.assignment_id <> ?", excludedAssignmentID)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return false, err
	}
	permissionsByUser := make(map[string][]string, len(rows))
	for _, row := range rows {
		permissionsByUser[row.UserID.String()] = append(
			permissionsByUser[row.UserID.String()],
			row.PermissionKey,
		)
	}
	for _, permissions := range permissionsByUser {
		if hasAllSystemManagerPermissions(permissions) {
			return true, nil
		}
	}
	return false, nil
}

func hasAllSystemManagerPermissions(keys []string) bool {
	set := make(map[string]bool, len(keys))
	for _, key := range keys {
		set[key] = true
	}
	return set[authorizationapp.PermissionRoleAssignmentAssign] &&
		set[authorizationapp.PermissionRoleAssignmentRevoke]
}

func bumpRoleVersion(db *gormdb.DB, ctx context.Context, roleID any, expected int64) error {
	result := db.WithContext(ctx).Table("roles").Where("id = ? AND version = ?", roleID, expected).
		Update("version", expected+1)
	if result.Error != nil || result.RowsAffected != 1 {
		return authorizationapp.ErrRoleAssignmentConflict
	}
	return nil
}

func bumpUserAuthorizationVersion(db *gormdb.DB, ctx context.Context, userID any) error {
	result := db.WithContext(ctx).Table("users").Where("id = ?", userID).
		UpdateColumn("authorization_version", gormdb.Expr("authorization_version + 1"))
	if result.Error != nil || result.RowsAffected != 1 {
		return fmt.Errorf("bump authorization version: %w", result.Error)
	}
	return nil
}

func roleAccountString(id *databaseid.ID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func roleIDString(id any) string {
	if value, ok := id.(databaseid.ID); ok {
		return value.String()
	}
	return fmt.Sprint(id)
}

var _ authorizationapp.RoleAssignmentRepository = (*RoleAssignmentRepository)(nil)
