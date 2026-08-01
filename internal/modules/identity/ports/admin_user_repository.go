package ports

import (
	"context"
	"errors"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
)

var (
	ErrInvalidAdminUserQuery = errors.New("invalid admin user query")
	ErrAdminUserConflict     = errors.New("admin user version conflict")
)

type AdminUserListFilter struct {
	Status  domain.UserStatus
	Email   string
	Page    int
	PerPage int
}

type AdminUserSummary struct {
	User                  domain.User
	MembershipCount       int
	ActiveMembershipCount int
}

type AdminUserPage struct {
	Items      []AdminUserSummary
	Page       int
	PerPage    int
	TotalItems int
	TotalPages int
}

type AdminUserMembership struct {
	ID                   string
	AccountID            string
	AccountSlug          string
	Status               string
	AuthorizationVersion int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type AdminUserDetail struct {
	User        domain.User
	Memberships []AdminUserMembership
}

type AdminUserQueryRepository interface {
	ListAdminUsers(context.Context, AdminUserListFilter) (AdminUserPage, error)
	FindAdminUser(context.Context, string) (AdminUserDetail, error)
}

type AdminUserCommandRepository interface {
	UpdateUserStatus(context.Context, string, int64, domain.UserStatus, time.Time) (domain.User, error)
}
