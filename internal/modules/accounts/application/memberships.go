package application

import (
	"context"
	"errors"
	"strings"
	"time"

	accountsmodule "github.com/korwlik23/go-api-starter/internal/modules/accounts"
	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var (
	ErrInvalidMembershipService = errors.New("invalid membership service")
	ErrMembershipDenied         = errors.New("membership management denied")
	ErrInvalidMembershipCommand = errors.New("invalid membership command")
)

type MembershipAuthorizer interface {
	Allowed(context.Context, string, authdomain.PermissionKey) (bool, error)
}
type MembershipAuditEvent struct {
	ActorUserID, RequestID, AccountID, MembershipID string
	Status                                          accountsdomain.MembershipStatus
}
type MembershipAuditor interface {
	RecordMembershipStatusChange(context.Context, MembershipAuditEvent) error
}

type MembershipService struct {
	transactions database.Transactor
	authorizer   MembershipAuthorizer
	query        ports.MembershipQueryRepository
	commands     ports.MembershipCommandRepository
	auditor      MembershipAuditor
	now          func() time.Time
}

func NewMembershipService(transactions database.Transactor, authorizer MembershipAuthorizer, query ports.MembershipQueryRepository, commands ports.MembershipCommandRepository, auditors ...MembershipAuditor) (*MembershipService, error) {
	if transactions == nil || authorizer == nil || query == nil || commands == nil || len(auditors) > 1 {
		return nil, ErrInvalidMembershipService
	}
	var auditor MembershipAuditor
	if len(auditors) == 1 {
		auditor = auditors[0]
	}
	return &MembershipService{transactions: transactions, authorizer: authorizer, query: query, commands: commands, auditor: auditor, now: time.Now}, nil
}

func (service *MembershipService) List(ctx context.Context, actorUserID string, filter ports.MembershipListFilter) (ports.MembershipPage, error) {
	if service == nil || ctx == nil || strings.TrimSpace(actorUserID) == "" || strings.TrimSpace(filter.AccountID) == "" {
		return ports.MembershipPage{}, ErrInvalidMembershipCommand
	}
	if !service.allowed(ctx, actorUserID, accountsmodule.PermissionMembershipsReadSystem) {
		return ports.MembershipPage{}, ErrMembershipDenied
	}
	return service.query.ListMemberships(ctx, filter)
}

type UpdateMembershipStatusCommand struct {
	ActorUserID, AccountID, MembershipID, RequestID string
	ExpectedVersion                                 int64
	Status                                          accountsdomain.MembershipStatus
}

func (service *MembershipService) UpdateStatus(ctx context.Context, command UpdateMembershipStatusCommand) (view ports.MembershipView, err error) {
	if service == nil || ctx == nil || strings.TrimSpace(command.ActorUserID) == "" || strings.TrimSpace(command.AccountID) == "" || strings.TrimSpace(command.MembershipID) == "" || command.ExpectedVersion < 1 || !validMembershipStatus(command.Status) {
		return ports.MembershipView{}, ErrInvalidMembershipCommand
	}
	if !service.allowed(ctx, command.ActorUserID, accountsmodule.PermissionMembershipsManageSystem) {
		return ports.MembershipView{}, ErrMembershipDenied
	}
	err = service.transactions.WithinTransaction(ctx, func(tx context.Context) error {
		view, err = service.commands.UpdateMembershipStatus(tx, command.AccountID, command.MembershipID, command.ExpectedVersion, command.Status, service.now().UTC())
		if err != nil || service.auditor == nil {
			return err
		}
		return service.auditor.RecordMembershipStatusChange(tx, MembershipAuditEvent{ActorUserID: command.ActorUserID, RequestID: command.RequestID, AccountID: view.AccountID, MembershipID: view.ID, Status: view.Status})
	})
	return view, err
}

func (service *MembershipService) allowed(ctx context.Context, actor, raw string) bool {
	key, err := authdomain.ParsePermissionKey(raw)
	if err != nil {
		return false
	}
	allowed, err := service.authorizer.Allowed(ctx, actor, key)
	return err == nil && allowed
}
func validMembershipStatus(status accountsdomain.MembershipStatus) bool {
	return status == accountsdomain.MembershipStatusActive || status == accountsdomain.MembershipStatusDisabled
}

var _ *MembershipService = (*MembershipService)(nil)
