package gorm

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/operations/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

type ModuleStateRepository struct {
	db          *gormdb.DB
	sqlDB       *sql.DB
	dialect     string
	lockTimeout time.Duration
}

type moduleStateRecord struct {
	ID                string    `gorm:"column:module_id"`
	Enabled           bool      `gorm:"column:enabled"`
	ModuleChecksum    []byte    `gorm:"column:module_checksum"`
	ConfigChecksum    []byte    `gorm:"column:config_checksum"`
	CatalogChecksum   []byte    `gorm:"column:catalog_checksum"`
	CatalogEpoch      int64     `gorm:"column:catalog_epoch"`
	ReconcileRevision int64     `gorm:"column:reconcile_revision"`
	ReconciledAt      time.Time `gorm:"column:reconciled_at"`
}

type permissionRecord struct {
	ID        databaseid.ID `gorm:"column:id"`
	Key       string        `gorm:"column:permission_key"`
	ModuleID  string        `gorm:"column:module_id"`
	Scope     string        `gorm:"column:scope"`
	Tier      string        `gorm:"column:tier"`
	Delegable bool          `gorm:"column:delegable"`
	Active    bool          `gorm:"column:active"`
}

type databaseIDParameter struct {
	value driver.Value
}

func (parameter databaseIDParameter) Value() (driver.Value, error) {
	return parameter.value, nil
}

func NewModuleStateRepository(
	db *gormdb.DB,
	lockTimeout time.Duration,
) (*ModuleStateRepository, error) {
	if db == nil || lockTimeout <= 0 {
		return nil, ErrInvalidModuleStateRepository
	}
	dialect := db.Dialector.Name()
	if dialect != "postgres" && dialect != "mysql" {
		return nil, ErrInvalidModuleStateRepository
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, ErrInvalidModuleStateRepository
	}
	return &ModuleStateRepository{
		db:          db,
		sqlDB:       sqlDB,
		dialect:     dialect,
		lockTimeout: lockTimeout,
	}, nil
}

func (repository *ModuleStateRepository) WithinReconcile(
	ctx context.Context,
	operation func(context.Context, ports.ReconcileSession) error,
) error {
	if repository == nil ||
		repository.db == nil ||
		repository.sqlDB == nil ||
		ctx == nil ||
		operation == nil {
		return ErrInvalidModuleStateRepository
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return repository.withinLockedTransaction(
		ctx,
		func(transactionContext context.Context, transaction *reconcileTransaction) error {
			return operation(
				transactionContext,
				&moduleStateSession{
					db:      transaction.db,
					dialect: repository.dialect,
				},
			)
		},
	)
}

func (repository *ModuleStateRepository) CurrentRevision(
	ctx context.Context,
) (int64, error) {
	if repository == nil || repository.db == nil || ctx == nil {
		return 0, ErrInvalidModuleStateRepository
	}
	var revisions []int64
	if err := repository.db.WithContext(ctx).
		Table("module_states").
		Order("module_id").
		Pluck("reconcile_revision", &revisions).Error; err != nil {
		return 0, ErrModuleStateRepositoryFailed
	}
	if len(revisions) == 0 {
		return 0, nil
	}
	revision := revisions[0]
	if revision < 1 {
		return 0, ErrModuleStateRepositoryFailed
	}
	for _, candidate := range revisions[1:] {
		if candidate != revision {
			return 0, ErrModuleStateRepositoryFailed
		}
	}
	return revision, nil
}

type moduleStateSession struct {
	db      *gormdb.DB
	dialect string
}

func (session *moduleStateSession) Snapshot(
	ctx context.Context,
) (ports.ReconcileSnapshot, error) {
	var modules []moduleStateRecord
	if err := session.db.WithContext(ctx).
		Table("module_states").
		Order("module_id").
		Find(&modules).Error; err != nil {
		return ports.ReconcileSnapshot{}, ErrModuleStateRepositoryFailed
	}
	var permissions []permissionRecord
	if err := session.db.WithContext(ctx).
		Table("permissions").
		Order("permission_key").
		Find(&permissions).Error; err != nil {
		return ports.ReconcileSnapshot{}, ErrModuleStateRepositoryFailed
	}
	snapshot, err := mapSnapshot(modules, permissions)
	if err != nil {
		return ports.ReconcileSnapshot{}, err
	}
	return snapshot, nil
}

func (session *moduleStateSession) Apply(
	ctx context.Context,
	projection ports.ReconcileProjection,
) error {
	if err := validateProjection(projection); err != nil {
		return err
	}
	for _, permission := range projection.Permissions {
		if err := session.upsertPermission(ctx, permission); err != nil {
			return err
		}
	}
	if err := session.db.WithContext(ctx).Exec("DELETE FROM module_states").Error; err != nil {
		return ErrModuleStateRepositoryFailed
	}
	for _, module := range projection.Modules {
		if err := session.insertModuleState(ctx, module); err != nil {
			return err
		}
	}
	return nil
}

func mapSnapshot(
	modules []moduleStateRecord,
	permissions []permissionRecord,
) (ports.ReconcileSnapshot, error) {
	snapshot := ports.ReconcileSnapshot{
		Modules:     make([]ports.ModuleRecord, 0, len(modules)),
		Permissions: make([]ports.PermissionRecord, 0, len(permissions)),
	}
	for index, module := range modules {
		if module.ReconcileRevision < 1 ||
			module.CatalogEpoch < 1 ||
			len(module.ModuleChecksum) != 32 ||
			len(module.ConfigChecksum) != 32 ||
			len(module.CatalogChecksum) != 32 {
			return ports.ReconcileSnapshot{}, ErrModuleStateRepositoryFailed
		}
		if index == 0 {
			snapshot.Revision = module.ReconcileRevision
			snapshot.CatalogEpoch = module.CatalogEpoch
		} else if snapshot.Revision != module.ReconcileRevision ||
			snapshot.CatalogEpoch != module.CatalogEpoch ||
			!bytes.Equal(snapshot.Modules[0].ConfigChecksum, module.ConfigChecksum) ||
			!bytes.Equal(snapshot.Modules[0].CatalogChecksum, module.CatalogChecksum) {
			return ports.ReconcileSnapshot{}, ErrModuleStateRepositoryFailed
		}
		snapshot.Modules = append(snapshot.Modules, ports.ModuleRecord{
			ID:                module.ID,
			Enabled:           module.Enabled,
			ModuleChecksum:    append([]byte(nil), module.ModuleChecksum...),
			ConfigChecksum:    append([]byte(nil), module.ConfigChecksum...),
			CatalogChecksum:   append([]byte(nil), module.CatalogChecksum...),
			CatalogEpoch:      module.CatalogEpoch,
			ReconcileRevision: module.ReconcileRevision,
		})
	}
	for _, permission := range permissions {
		if permission.ID.String() == "" {
			return ports.ReconcileSnapshot{}, ErrModuleStateRepositoryFailed
		}
		snapshot.Permissions = append(snapshot.Permissions, ports.PermissionRecord{
			ID:        permission.ID.String(),
			Key:       permission.Key,
			ModuleID:  permission.ModuleID,
			Scope:     permission.Scope,
			Tier:      permission.Tier,
			Delegable: permission.Delegable,
			Active:    permission.Active,
		})
	}
	return snapshot, nil
}

func validateProjection(projection ports.ReconcileProjection) error {
	if projection.Revision < 1 ||
		projection.CatalogEpoch < 1 ||
		len(projection.Modules) == 0 {
		return ErrModuleStateRepositoryFailed
	}
	for _, module := range projection.Modules {
		if module.ID == "" ||
			module.ReconcileRevision != projection.Revision ||
			module.CatalogEpoch != projection.CatalogEpoch ||
			len(module.ModuleChecksum) != 32 ||
			len(module.ConfigChecksum) != 32 ||
			len(module.CatalogChecksum) != 32 {
			return ErrModuleStateRepositoryFailed
		}
	}
	return nil
}

func (session *moduleStateSession) upsertPermission(
	ctx context.Context,
	permission ports.PermissionRecord,
) error {
	encodedID, err := databaseid.Encode(session.dialect, permission.ID)
	if err != nil {
		return ErrModuleStateRepositoryFailed
	}
	arguments := []any{
		databaseIDParameter{value: encodedID},
		permission.Key,
		permission.ModuleID,
		permission.Scope,
		permission.Tier,
		permission.Delegable,
		permission.Active,
	}
	query := postgresPermissionUpsert
	if session.dialect == "mysql" {
		query = mysqlPermissionUpsert
	}
	if err := session.db.WithContext(ctx).Exec(query, arguments...).Error; err != nil {
		return ErrModuleStateRepositoryFailed
	}
	return nil
}

func (session *moduleStateSession) insertModuleState(
	ctx context.Context,
	module ports.ModuleRecord,
) error {
	if err := session.db.WithContext(ctx).Exec(
		`INSERT INTO module_states (
			module_id, enabled, module_checksum, config_checksum, catalog_checksum,
			catalog_epoch, reconcile_revision
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		module.ID,
		module.Enabled,
		module.ModuleChecksum,
		module.ConfigChecksum,
		module.CatalogChecksum,
		module.CatalogEpoch,
		module.ReconcileRevision,
	).Error; err != nil {
		return ErrModuleStateRepositoryFailed
	}
	return nil
}

const postgresPermissionUpsert = `
INSERT INTO permissions (
	id, permission_key, module_id, scope, tier, delegable, active
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (permission_key) DO UPDATE SET
	scope = EXCLUDED.scope,
	tier = EXCLUDED.tier,
	delegable = EXCLUDED.delegable,
	active = EXCLUDED.active,
	updated_at = CURRENT_TIMESTAMP
`

const mysqlPermissionUpsert = `
INSERT INTO permissions (
	id, permission_key, module_id, scope, tier, delegable, active
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
	scope = VALUES(scope),
	tier = VALUES(tier),
	delegable = VALUES(delegable),
	active = VALUES(active),
	updated_at = CURRENT_TIMESTAMP(6)
`

var _ ports.ModuleStateRepository = (*ModuleStateRepository)(nil)
