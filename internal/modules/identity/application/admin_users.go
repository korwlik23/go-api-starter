package application

import (
	"context"
	"errors"
	"strings"
	"time"

	authdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	identitymodule "github.com/korwlik23/go-api-starter/internal/modules/identity"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var (
	ErrInvalidAdminUserService = errors.New("invalid admin user service")
	ErrAdminUserDenied         = errors.New("admin user management denied")
	ErrInvalidAdminUserCommand = errors.New("invalid admin user command")
)

type AdminUserPermissionAuthorizer interface {
	Allowed(context.Context, string, authdomain.PermissionKey) (bool, error)
}

type AdminUserAuditEvent struct {
	ActorUserID string
	RequestID   string
	UserID      string
	Status      identitydomain.UserStatus
}

type AdminUserAuditor interface {
	RecordUserStatusChange(context.Context, AdminUserAuditEvent) error
}

type AdminUserService struct {
	transactions database.Transactor
	authorizer   AdminUserPermissionAuthorizer
	query        ports.AdminUserQueryRepository
	commands     ports.AdminUserCommandRepository
	auditor      AdminUserAuditor
	now          func() time.Time
}

func NewAdminUserService(transactions database.Transactor, authorizer AdminUserPermissionAuthorizer, query ports.AdminUserQueryRepository, commands ports.AdminUserCommandRepository, auditors ...AdminUserAuditor) (*AdminUserService, error) {
	if transactions == nil || authorizer == nil || query == nil || commands == nil || len(auditors) > 1 {
		return nil, ErrInvalidAdminUserService
	}
	var auditor AdminUserAuditor
	if len(auditors) == 1 {
		auditor = auditors[0]
	}
	return &AdminUserService{transactions: transactions, authorizer: authorizer, query: query, commands: commands, auditor: auditor, now: time.Now}, nil
}

func (service *AdminUserService) List(ctx context.Context, actorUserID string, filter ports.AdminUserListFilter) (ports.AdminUserPage, error) {
	if service == nil || ctx == nil || strings.TrimSpace(actorUserID) == "" {
		return ports.AdminUserPage{}, ErrInvalidAdminUserCommand
	}
	if !service.allowed(ctx, actorUserID, identitymodule.PermissionUsersReadSystem) {
		return ports.AdminUserPage{}, ErrAdminUserDenied
	}
	return service.query.ListAdminUsers(ctx, filter)
}

func (service *AdminUserService) Detail(ctx context.Context, actorUserID, userID string) (ports.AdminUserDetail, error) {
	if service == nil || ctx == nil || strings.TrimSpace(actorUserID) == "" || strings.TrimSpace(userID) == "" {
		return ports.AdminUserDetail{}, ErrInvalidAdminUserCommand
	}
	if !service.allowed(ctx, actorUserID, identitymodule.PermissionUsersReadSystem) {
		return ports.AdminUserDetail{}, ErrAdminUserDenied
	}
	return service.query.FindAdminUser(ctx, userID)
}

type UpdateAdminUserStatusCommand struct {
	ActorUserID, UserID, RequestID string
	ExpectedVersion                int64
	Status                         identitydomain.UserStatus
}

func (service *AdminUserService) UpdateStatus(ctx context.Context, command UpdateAdminUserStatusCommand) (user identitydomain.User, err error) {
	if service == nil || ctx == nil || strings.TrimSpace(command.ActorUserID) == "" || strings.TrimSpace(command.UserID) == "" || command.ExpectedVersion < 1 || !validAdminUserStatus(command.Status) {
		return identitydomain.User{}, ErrInvalidAdminUserCommand
	}
	if !service.allowed(ctx, command.ActorUserID, identitymodule.PermissionUsersManageSystem) {
		return identitydomain.User{}, ErrAdminUserDenied
	}
	err = service.transactions.WithinTransaction(ctx, func(tx context.Context) error {
		user, err = service.commands.UpdateUserStatus(tx, command.UserID, command.ExpectedVersion, command.Status, service.now().UTC())
		if err != nil || service.auditor == nil {
			return err
		}
		return service.auditor.RecordUserStatusChange(tx, AdminUserAuditEvent{ActorUserID: command.ActorUserID, RequestID: command.RequestID, UserID: user.ID, Status: user.Status})
	})
	return user, err
}

func (service *AdminUserService) allowed(ctx context.Context, actor string, raw string) bool {
	key, err := authdomain.ParsePermissionKey(raw)
	if err != nil {
		return false
	}
	allowed, err := service.authorizer.Allowed(ctx, actor, key)
	return err == nil && allowed
}
func validAdminUserStatus(status identitydomain.UserStatus) bool {
	return status == identitydomain.UserStatusActive || status == identitydomain.UserStatusDisabled
}

var _ AdminUserPermissionAuthorizer = (interface {
	Allowed(context.Context, string, authdomain.PermissionKey) (bool, error)
})(nil)
