package app

import (
	"errors"
	"sort"

	gormdb "gorm.io/gorm"
)

func bootstrapIdentity(
	db *gormdb.DB,
	table string,
	predicate string,
	value any,
) (string, bool, error) {
	var record bootstrapIdentityRecord
	err := db.Table(table).Select("id").Where(predicate, value).Take(&record).Error
	if errors.Is(err, gormdb.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil || record.ID.String() == "" {
		return "", false, ErrBootstrapFailed
	}
	return record.ID.String(), true, nil
}

func bootstrapRoleExists(
	db *gormdb.DB,
	roleID any,
	ownerType string,
	accountID any,
) bool {
	var count int64
	query := db.Table("roles").Where(
		"id = ? AND owner_type = ?",
		roleID,
		ownerType,
	)
	if accountID == nil {
		query = query.Where("account_id IS NULL")
	} else {
		query = query.Where("account_id = ?", accountID)
	}
	return query.Count(&count).Error == nil && count == 1
}

func bootstrapAssignmentExists(
	db *gormdb.DB,
	table string,
	predicate string,
	arguments ...any,
) bool {
	var count int64
	return db.Table(table).
		Where(predicate, arguments...).
		Count(&count).Error == nil &&
		count == 1
}

func bootstrapRolePermissionKeys(
	db *gormdb.DB,
	roleID any,
) ([]string, error) {
	var keys []string
	err := db.Table("role_permissions").
		Select("permissions.permission_key").
		Joins("JOIN permissions ON permissions.id = role_permissions.permission_id").
		Where("role_permissions.role_id = ?", roleID).
		Order("permissions.permission_key").
		Pluck("permissions.permission_key", &keys).
		Error
	return keys, err
}

const stringScopeSystem = "system"

func partitionPermissionKeys(
	permissions []bootstrapPermissionRecord,
) ([]string, []string) {
	account := make([]string, 0)
	system := make([]string, 0)
	for _, permission := range permissions {
		if permission.Scope == stringScopeSystem {
			system = append(system, permission.Key)
		} else {
			account = append(account, permission.Key)
		}
	}
	sort.Strings(account)
	sort.Strings(system)
	return account, system
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
