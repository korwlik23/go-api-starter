package gorm

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/databaseid"
	gormdb "gorm.io/gorm"
)

var (
	ErrInvalidRoleQueryRepository = errors.New("invalid role query repository")
	ErrInvalidRoleQuery           = ports.ErrInvalidRoleQuery
)

const (
	defaultRolePage    = 25
	maxRolePage        = 100
	maxRolePermissions = 256
)

type RoleQueryRepository struct {
	db *gormdb.DB
}

type roleQueryRecord struct {
	ID        databaseid.ID  `gorm:"column:id"`
	OwnerType string         `gorm:"column:owner_type"`
	AccountID *databaseid.ID `gorm:"column:account_id"`
	Name      string         `gorm:"column:name"`
	Version   int64          `gorm:"column:version"`
	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
}

type rolePermissionRecord struct {
	PermissionKey string `gorm:"column:permission_key"`
	Active        bool   `gorm:"column:active"`
	Delegable     bool   `gorm:"column:delegable"`
	Tier          string `gorm:"column:tier"`
}

type roleVersionRecord struct {
	ID      databaseid.ID `gorm:"column:id"`
	Version int64         `gorm:"column:version"`
}

type roleAssignmentRecord struct {
	AssignmentID databaseid.ID  `gorm:"column:assignment_id"`
	RoleID       databaseid.ID  `gorm:"column:role_id"`
	Scope        string         `gorm:"column:scope"`
	AccountID    *databaseid.ID `gorm:"column:account_id"`
	UserID       databaseid.ID  `gorm:"column:user_id"`
	CreatedAt    time.Time      `gorm:"column:created_at"`
}

func NewRoleQueryRepository(db *gormdb.DB) (*RoleQueryRepository, error) {
	if db == nil {
		return nil, ErrInvalidRoleQueryRepository
	}
	return &RoleQueryRepository{db: db}, nil
}

func (repository *RoleQueryRepository) ListRoles(
	ctx context.Context,
	filter ports.RoleListFilter,
) (ports.RolePage, error) {
	request, err := normalizeRolePageRequest(filter.RolePageRequest)
	if err != nil {
		return ports.RolePage{}, err
	}
	query := repository.db.WithContext(ctx).Table("roles")
	if filter.RoleOwnerType != "" {
		switch filter.RoleOwnerType {
		case authorizationdomain.RoleOwnerSystem, authorizationdomain.RoleOwnerAccount:
			query = query.Where("owner_type = ?", string(filter.RoleOwnerType))
		default:
			return ports.RolePage{}, ErrInvalidRoleQuery
		}
	}
	if filter.AccountID != "" {
		encodedAccountID, encodeErr := databaseid.Encode(
			repository.db.Dialector.Name(),
			filter.AccountID,
		)
		if encodeErr != nil {
			return ports.RolePage{}, ErrInvalidRoleQuery
		}
		query = query.Where("account_id = ?", encodedAccountID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ports.RolePage{}, fmt.Errorf("count authorization roles: %w", err)
	}
	var records []roleQueryRecord
	if err := query.
		Order("created_at DESC").
		Order("id DESC").
		Limit(request.PerPage).
		Offset((request.Page - 1) * request.PerPage).
		Find(&records).Error; err != nil {
		return ports.RolePage{}, fmt.Errorf("list authorization roles: %w", err)
	}
	items := make([]authorizationdomain.Role, 0, len(records))
	for _, record := range records {
		role, mapErr := mapRoleQueryRecord(record)
		if mapErr != nil {
			return ports.RolePage{}, mapErr
		}
		items = append(items, role)
	}
	return ports.RolePage{
		Items: items,
		Page:  rolePageInfo(request, total),
	}, nil
}

func (repository *RoleQueryRepository) ListRolePermissions(
	ctx context.Context,
	roleID string,
) (ports.RolePermissionList, error) {
	encodedRoleID, err := databaseid.Encode(repository.db.Dialector.Name(), roleID)
	if err != nil {
		return ports.RolePermissionList{}, ErrInvalidRoleQuery
	}
	var role roleVersionRecord
	roleResult := repository.db.WithContext(ctx).Raw(
		`SELECT id, version FROM roles WHERE id = ?`,
		encodedRoleID,
	).Scan(&role)
	if roleResult.Error != nil {
		return ports.RolePermissionList{}, fmt.Errorf("find role for permissions: %w", roleResult.Error)
	}
	if roleResult.RowsAffected != 1 || role.ID.String() != roleID || role.Version < 1 {
		return ports.RolePermissionList{}, ports.ErrRoleNotFound
	}
	var records []rolePermissionRecord
	if err := repository.db.WithContext(ctx).Raw(
		`SELECT permissions.permission_key, permissions.active, permissions.delegable, permissions.tier
FROM role_permissions
JOIN permissions ON permissions.id = role_permissions.permission_id
WHERE role_permissions.role_id = ?
ORDER BY permissions.permission_key ASC
LIMIT ?`,
		encodedRoleID,
		maxRolePermissions,
	).Scan(&records).Error; err != nil {
		return ports.RolePermissionList{}, fmt.Errorf("list role permissions: %w", err)
	}
	items := make([]ports.RolePermissionView, 0, len(records))
	for _, record := range records {
		if _, parseErr := authorizationdomain.ParsePermissionKey(record.PermissionKey); parseErr != nil ||
			!validPermissionTier(record.Tier) {
			return ports.RolePermissionList{}, ErrInvalidRoleQuery
		}
		items = append(items, ports.RolePermissionView{
			PermissionKey: record.PermissionKey,
			Active:        record.Active,
			Delegable:     record.Delegable,
			Tier:          record.Tier,
		})
	}
	return ports.RolePermissionList{RoleID: roleID, RoleVersion: role.Version, Items: items}, nil
}

func (repository *RoleQueryRepository) ListRoleAssignments(
	ctx context.Context,
	roleID string,
	request ports.RolePageRequest,
) (ports.RoleAssignmentPage, error) {
	request, err := normalizeRolePageRequest(request)
	if err != nil {
		return ports.RoleAssignmentPage{}, err
	}
	encodedRoleID, err := databaseid.Encode(repository.db.Dialector.Name(), roleID)
	if err != nil {
		return ports.RoleAssignmentPage{}, ErrInvalidRoleQuery
	}
	union := `
SELECT assignment_id, role_id, 'system' AS scope, NULL AS account_id, user_id, created_at
FROM system_role_assignments
WHERE role_id = ?
UNION ALL
SELECT membership_roles.assignment_id, membership_roles.role_id, 'account' AS scope,
       membership_roles.account_id, memberships.user_id, membership_roles.created_at
FROM membership_roles
JOIN memberships
  ON memberships.account_id = membership_roles.account_id
 AND memberships.id = membership_roles.membership_id
WHERE membership_roles.role_id = ?`
	var total int64
	if err := repository.db.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM ("+union+") AS role_assignments",
		encodedRoleID,
		encodedRoleID,
	).Scan(&total).Error; err != nil {
		return ports.RoleAssignmentPage{}, fmt.Errorf("count role assignments: %w", err)
	}
	var records []roleAssignmentRecord
	if err := repository.db.WithContext(ctx).Raw(
		"SELECT assignment_id, role_id, scope, account_id, user_id, created_at FROM ("+
			union+
			`) AS role_assignments
ORDER BY created_at DESC, assignment_id DESC
LIMIT ? OFFSET ?`,
		encodedRoleID,
		encodedRoleID,
		request.PerPage,
		(request.Page-1)*request.PerPage,
	).Scan(&records).Error; err != nil {
		return ports.RoleAssignmentPage{}, fmt.Errorf("list role assignments: %w", err)
	}
	items := make([]ports.RoleAssignmentView, 0, len(records))
	for _, record := range records {
		item, mapErr := mapRoleAssignmentRecord(record, roleID)
		if mapErr != nil {
			return ports.RoleAssignmentPage{}, mapErr
		}
		items = append(items, item)
	}
	return ports.RoleAssignmentPage{
		Items: items,
		Page:  rolePageInfo(request, total),
	}, nil
}

func normalizeRolePageRequest(request ports.RolePageRequest) (ports.RolePageRequest, error) {
	if request.Page == 0 {
		request.Page = 1
	}
	if request.PerPage == 0 {
		request.PerPage = defaultRolePage
	}
	if request.Page < 1 || request.PerPage < 1 || request.PerPage > maxRolePage {
		return ports.RolePageRequest{}, ErrInvalidRoleQuery
	}
	return request, nil
}

func rolePageInfo(request ports.RolePageRequest, total int64) ports.PageInfo {
	if total < 0 {
		total = 0
	}
	totalPages := int64(0)
	if total > 0 {
		totalPages = (total + int64(request.PerPage) - 1) / int64(request.PerPage)
	}
	if totalPages > math.MaxInt {
		totalPages = math.MaxInt
	}
	return ports.PageInfo{
		Page:       request.Page,
		PerPage:    request.PerPage,
		TotalItems: int(total),
		TotalPages: int(totalPages),
	}
}

func mapRoleQueryRecord(record roleQueryRecord) (authorizationdomain.Role, error) {
	if record.ID.String() == "" || record.Version < 1 || strings.TrimSpace(record.Name) == "" ||
		record.CreatedAt.IsZero() || record.UpdatedAt.IsZero() || record.UpdatedAt.Before(record.CreatedAt) {
		return authorizationdomain.Role{}, ErrInvalidRoleQuery
	}
	var ownerType authorizationdomain.RoleOwnerType
	switch record.OwnerType {
	case string(authorizationdomain.RoleOwnerSystem):
		ownerType = authorizationdomain.RoleOwnerSystem
		if record.AccountID != nil {
			return authorizationdomain.Role{}, ErrInvalidRoleQuery
		}
	case string(authorizationdomain.RoleOwnerAccount):
		ownerType = authorizationdomain.RoleOwnerAccount
		if record.AccountID == nil || record.AccountID.String() == "" {
			return authorizationdomain.Role{}, ErrInvalidRoleQuery
		}
	default:
		return authorizationdomain.Role{}, ErrInvalidRoleQuery
	}
	return authorizationdomain.Role{
		ID:        record.ID.String(),
		OwnerType: ownerType,
		AccountID: idString(record.AccountID),
		Name:      record.Name,
		Version:   record.Version,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}, nil
}

func mapRoleAssignmentRecord(
	record roleAssignmentRecord,
	roleID string,
) (ports.RoleAssignmentView, error) {
	if record.AssignmentID.String() == "" ||
		record.RoleID.String() != roleID ||
		record.UserID.String() == "" ||
		record.CreatedAt.IsZero() {
		return ports.RoleAssignmentView{}, ErrInvalidRoleQuery
	}
	if record.Scope != string(authorizationdomain.RoleOwnerSystem) &&
		record.Scope != string(authorizationdomain.RoleOwnerAccount) {
		return ports.RoleAssignmentView{}, ErrInvalidRoleQuery
	}
	accountID := idString(record.AccountID)
	if record.Scope == string(authorizationdomain.RoleOwnerSystem) && accountID != "" ||
		record.Scope == string(authorizationdomain.RoleOwnerAccount) && accountID == "" {
		return ports.RoleAssignmentView{}, ErrInvalidRoleQuery
	}
	return ports.RoleAssignmentView{
		AssignmentID: record.AssignmentID.String(),
		RoleID:       record.RoleID.String(),
		Scope:        record.Scope,
		AccountID:    accountID,
		UserID:       record.UserID.String(),
		CreatedAt:    record.CreatedAt,
	}, nil
}

func idString(id *databaseid.ID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func validPermissionTier(value string) bool {
	switch value {
	case "core", "default", "optional":
		return true
	default:
		return false
	}
}

var _ ports.RoleQueryRepository = (*RoleQueryRepository)(nil)
