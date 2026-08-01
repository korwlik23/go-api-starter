package ports

import (
	"context"
	"errors"
	"time"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
)

var ErrInvalidRoleQuery = errors.New("invalid role query")

type RolePageRequest struct {
	Page    int
	PerPage int
}

type PageInfo struct {
	Page       int
	PerPage    int
	TotalItems int
	TotalPages int
}

type RoleListFilter struct {
	RoleOwnerType authorizationdomain.RoleOwnerType
	AccountID     string
	RolePageRequest
}

type RolePage struct {
	Items []authorizationdomain.Role
	Page  PageInfo
}

type RolePermissionView struct {
	PermissionKey string
	Active        bool
	Delegable     bool
	Tier          string
}

type RolePermissionList struct {
	RoleID      string
	RoleVersion int64
	Items       []RolePermissionView
}

type RoleAssignmentView struct {
	AssignmentID string
	RoleID       string
	Scope        string
	AccountID    string
	UserID       string
	CreatedAt    time.Time
}

type RoleAssignmentPage struct {
	Items []RoleAssignmentView
	Page  PageInfo
}

type RoleQueryRepository interface {
	ListRoles(context.Context, RoleListFilter) (RolePage, error)
	ListRolePermissions(context.Context, string) (RolePermissionList, error)
	ListRoleAssignments(context.Context, string, RolePageRequest) (RoleAssignmentPage, error)
}
