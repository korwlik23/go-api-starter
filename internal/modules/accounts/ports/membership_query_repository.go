package ports

import (
	"context"
	"errors"
	"time"

	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
)

var (
	ErrInvalidMembershipQuery = errors.New("invalid membership query")
	ErrMembershipConflict     = errors.New("membership version conflict")
	ErrMembershipNotFound     = errors.New("membership not found")
)

type MembershipListFilter struct {
	AccountID string
	Status    accountsdomain.MembershipStatus
	Page      int
	PerPage   int
}

type MembershipView struct {
	ID                   string
	AccountID            string
	AccountSlug          string
	UserID               string
	UserEmail            string
	Status               accountsdomain.MembershipStatus
	AuthorizationVersion int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type MembershipPage struct {
	Items      []MembershipView
	Page       int
	PerPage    int
	TotalItems int
	TotalPages int
}

type MembershipQueryRepository interface {
	ListMemberships(context.Context, MembershipListFilter) (MembershipPage, error)
}

type MembershipCommandRepository interface {
	UpdateMembershipStatus(context.Context, string, string, int64, accountsdomain.MembershipStatus, time.Time) (MembershipView, error)
}
