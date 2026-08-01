package app

import (
	"context"

	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

type GormBootstrapStore struct {
	connections  database.ConnectionProvider
	transactions database.Transactor
	dialect      string
}

type bootstrapSession struct {
	connections database.ConnectionProvider
	dialect     string
}

type bootstrapIdentityRecord struct {
	ID databaseid.ID `gorm:"column:id"`
}

type bootstrapPermissionRecord struct {
	ID     databaseid.ID `gorm:"column:id"`
	Key    string        `gorm:"column:permission_key"`
	Scope  string        `gorm:"column:scope"`
	Active bool          `gorm:"column:active"`
}

func NewGormBootstrapStore(
	db *gormdb.DB,
	manager *database.TransactionManager,
) (*GormBootstrapStore, error) {
	if db == nil || manager == nil {
		return nil, ErrInvalidBootstrap
	}
	dialect := db.Dialector.Name()
	if dialect != "postgres" && dialect != "mysql" {
		return nil, ErrInvalidBootstrap
	}
	return &GormBootstrapStore{
		connections:  manager,
		transactions: manager,
		dialect:      dialect,
	}, nil
}

func (store *GormBootstrapStore) WithinBootstrap(
	ctx context.Context,
	operation func(context.Context, BootstrapSession) error,
) error {
	if store == nil ||
		store.connections == nil ||
		store.transactions == nil ||
		ctx == nil ||
		operation == nil {
		return ErrInvalidBootstrap
	}
	return store.transactions.WithinTransaction(
		ctx,
		func(transactionContext context.Context) error {
			return operation(transactionContext, &bootstrapSession{
				connections: store.connections,
				dialect:     store.dialect,
			})
		},
	)
}

func (session *bootstrapSession) Inspect(
	ctx context.Context,
	email string,
	accountSlug string,
) (BootstrapState, error) {
	db, err := session.connection(ctx)
	if err != nil {
		return BootstrapState{}, err
	}
	accountID, accountFound, err := bootstrapIdentity(
		db,
		"accounts",
		"slug = ?",
		accountSlug,
	)
	if err != nil {
		return BootstrapState{}, err
	}
	userID, userFound, err := bootstrapIdentity(
		db,
		"users",
		"normalized_email = ?",
		email,
	)
	if err != nil {
		return BootstrapState{}, err
	}
	switch {
	case !accountFound && !userFound:
		return BootstrapState{Status: BootstrapAbsent}, nil
	case accountFound && userFound:
		return BootstrapState{
			Status:    BootstrapPresent,
			AccountID: accountID,
			UserID:    userID,
		}, nil
	default:
		return BootstrapState{Status: BootstrapPartial}, nil
	}
}

func (session *bootstrapSession) Create(
	ctx context.Context,
	record BootstrapRecord,
) error {
	db, err := session.connection(ctx)
	if err != nil {
		return err
	}
	ids, err := session.encodedRecordIDs(record)
	if err != nil {
		return ErrInvalidBootstrap
	}
	permissions, err := session.permissions(ctx, db, record.Permissions)
	if err != nil {
		return err
	}
	statements := []struct {
		table  string
		values map[string]any
	}{
		{
			table: "accounts",
			values: map[string]any{
				"id":     ids.account,
				"slug":   record.AccountSlug,
				"status": "active",
			},
		},
		{
			table: "users",
			values: map[string]any{
				"id":                    ids.user,
				"normalized_email":      record.Email,
				"status":                "active",
				"authorization_version": 1,
			},
		},
		{
			table: "password_credentials",
			values: map[string]any{
				"user_id":       ids.user,
				"password_hash": record.PasswordHash,
			},
		},
		{
			table: "memberships",
			values: map[string]any{
				"id":                    ids.membership,
				"account_id":            ids.account,
				"user_id":               ids.user,
				"status":                "active",
				"authorization_version": 1,
			},
		},
		{
			table: "roles",
			values: map[string]any{
				"id":         ids.accountRole,
				"owner_type": "account",
				"account_id": ids.account,
				"name":       record.AccountRoleName,
				"version":    1,
			},
		},
		{
			table: "roles",
			values: map[string]any{
				"id":         ids.systemRole,
				"owner_type": "system",
				"account_id": nil,
				"name":       record.SystemRoleName,
				"version":    1,
			},
		},
		{
			table: "membership_roles",
			values: map[string]any{
				"account_id":    ids.account,
				"membership_id": ids.membership,
				"role_id":       ids.accountRole,
				"assignment_id": ids.membershipRoleAssignment,
			},
		},
		{
			table: "system_role_assignments",
			values: map[string]any{
				"user_id":         ids.user,
				"role_owner_type": "system",
				"role_id":         ids.systemRole,
				"assignment_id":   ids.systemRoleAssignment,
			},
		},
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).
			Table(statement.table).
			Create(statement.values).Error; err != nil {
			return ErrBootstrapFailed
		}
	}
	for _, permission := range permissions {
		roleID := ids.accountRole
		if permission.Scope == stringScopeSystem {
			roleID = ids.systemRole
		}
		permissionID, encodeErr := databaseid.Encode(
			session.dialect,
			permission.ID.String(),
		)
		if encodeErr != nil {
			return ErrBootstrapFailed
		}
		if err := db.WithContext(ctx).
			Table("role_permissions").
			Create(map[string]any{
				"role_id":       roleID,
				"permission_id": permissionID,
			}).Error; err != nil {
			return ErrBootstrapFailed
		}
	}
	return nil
}

func (session *bootstrapSession) Verify(
	ctx context.Context,
	record BootstrapRecord,
) error {
	db, err := session.connection(ctx)
	if err != nil {
		return err
	}
	ids, err := session.encodedRecordIDs(record)
	if err != nil {
		return ErrBootstrapConflict
	}
	var membershipCount int64
	if err := db.WithContext(ctx).
		Table("memberships").
		Where(
			"account_id = ? AND user_id = ? AND status = ?",
			ids.account,
			ids.user,
			"active",
		).
		Count(&membershipCount).Error; err != nil || membershipCount != 1 {
		return ErrBootstrapConflict
	}
	if !bootstrapRoleExists(db, ids.accountRole, "account", ids.account) ||
		!bootstrapRoleExists(db, ids.systemRole, "system", nil) {
		return ErrBootstrapConflict
	}
	var accountAssignmentCount int64
	if err := db.Table("membership_roles").
		Joins(
			"JOIN memberships ON memberships.account_id = membership_roles.account_id "+
				"AND memberships.id = membership_roles.membership_id",
		).
		Where(
			"membership_roles.account_id = ? AND memberships.user_id = ? "+
				"AND membership_roles.role_id = ?",
			ids.account,
			ids.user,
			ids.accountRole,
		).
		Count(&accountAssignmentCount).Error; err != nil || accountAssignmentCount != 1 {
		return ErrBootstrapConflict
	}
	if !bootstrapAssignmentExists(
		db,
		"system_role_assignments",
		"user_id = ? AND role_owner_type = ? AND role_id = ?",
		ids.user,
		"system",
		ids.systemRole,
	) {
		return ErrBootstrapConflict
	}
	permissions, err := session.permissions(ctx, db, record.Permissions)
	if err != nil {
		return ErrBootstrapConflict
	}
	accountExpected, systemExpected := partitionPermissionKeys(permissions)
	accountActual, err := bootstrapRolePermissionKeys(db, ids.accountRole)
	if err != nil {
		return ErrBootstrapConflict
	}
	systemActual, err := bootstrapRolePermissionKeys(db, ids.systemRole)
	if err != nil {
		return ErrBootstrapConflict
	}
	if !equalStrings(accountExpected, accountActual) ||
		!equalStrings(systemExpected, systemActual) {
		return ErrBootstrapConflict
	}
	return nil
}

func (session *bootstrapSession) connection(
	ctx context.Context,
) (*gormdb.DB, error) {
	if session == nil || session.connections == nil || ctx == nil {
		return nil, ErrInvalidBootstrap
	}
	db, err := session.connections.Connection(ctx)
	if err != nil {
		return nil, ErrBootstrapFailed
	}
	return db, nil
}

type encodedBootstrapIDs struct {
	account                  any
	user                     any
	membership               any
	accountRole              any
	systemRole               any
	membershipRoleAssignment any
	systemRoleAssignment     any
}

func (session *bootstrapSession) encodedRecordIDs(
	record BootstrapRecord,
) (encodedBootstrapIDs, error) {
	values := []string{
		record.AccountID,
		record.UserID,
		record.MembershipID,
		record.AccountRoleID,
		record.SystemRoleID,
		record.MembershipRoleAssignmentID,
		record.SystemRoleAssignmentID,
	}
	encoded := make([]any, len(values))
	for index, value := range values {
		if value == "" && (index == 2 || index == 5 || index == 6) {
			continue
		}
		converted, err := databaseid.Encode(session.dialect, value)
		if err != nil {
			return encodedBootstrapIDs{}, err
		}
		encoded[index] = converted
	}
	return encodedBootstrapIDs{
		account:                  encoded[0],
		user:                     encoded[1],
		membership:               encoded[2],
		accountRole:              encoded[3],
		systemRole:               encoded[4],
		membershipRoleAssignment: encoded[5],
		systemRoleAssignment:     encoded[6],
	}, nil
}

func (session *bootstrapSession) permissions(
	ctx context.Context,
	db *gormdb.DB,
	expected []BootstrapPermission,
) ([]bootstrapPermissionRecord, error) {
	if len(expected) == 0 {
		return nil, ErrBootstrapConflict
	}
	keys := make([]string, 0, len(expected))
	for _, permission := range expected {
		keys = append(keys, permission.Key)
	}
	var records []bootstrapPermissionRecord
	if err := db.WithContext(ctx).
		Table("permissions").
		Select("id", "permission_key", "scope", "active").
		Where("permission_key IN ?", keys).
		Order("permission_key").
		Find(&records).Error; err != nil || len(records) != len(expected) {
		return nil, ErrBootstrapConflict
	}
	for index, record := range records {
		if !record.Active ||
			record.Key != expected[index].Key ||
			record.Scope != string(expected[index].Scope) {
			return nil, ErrBootstrapConflict
		}
	}
	return records, nil
}

var _ BootstrapStore = (*GormBootstrapStore)(nil)
