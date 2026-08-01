package application

import (
	"context"
	"errors"
	"strings"

	authorizationmodule "github.com/korwlik23/go-api-starter/internal/modules/authorization"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

const PermissionRolesManageSystem = authorizationmodule.PermissionRolesManageSystem

var (
	ErrInvalidRoleManagementService = errors.New("invalid role management service")
	ErrInvalidRoleManagementCommand = errors.New("invalid role management command")
	ErrRoleManagementDenied         = errors.New("role management denied")
)

type CreateRoleCommand struct {
	ActorUserID string
	AccountID   string
	Name        string
	RequestID   string
}

type UpdateRoleCommand struct {
	ActorUserID     string
	AccountID       string
	RoleID          string
	Name            string
	RequestID       string
	ExpectedVersion int64
}

type RoleManagementAuthorizer interface {
	Allowed(context.Context, string, authorizationdomain.PermissionKey) (bool, error)
}

type RoleManagementAuditor interface {
	RecordRoleChange(context.Context, RoleManagementAuditEvent) error
}

type RoleManagementAuditEvent struct {
	Scope       string
	AccountID   string
	ActorUserID string
	RequestID   string
	Action      string
	RoleID      string
}

type RoleManagementService struct {
	transactions database.Transactor
	authorizer   RoleManagementAuthorizer
	repository   ports.RoleCommandRepository
	auditor      RoleManagementAuditor
}

func NewRoleManagementService(
	transactions database.Transactor,
	authorizer RoleManagementAuthorizer,
	repository ports.RoleCommandRepository,
	auditors ...RoleManagementAuditor,
) (*RoleManagementService, error) {
	if transactions == nil || authorizer == nil || repository == nil || len(auditors) > 1 {
		return nil, ErrInvalidRoleManagementService
	}
	var auditor RoleManagementAuditor
	if len(auditors) == 1 {
		auditor = auditors[0]
	}
	return &RoleManagementService{
		transactions: transactions,
		authorizer:   authorizer,
		repository:   repository,
		auditor:      auditor,
	}, nil
}

func (service *RoleManagementService) Create(
	ctx context.Context,
	command CreateRoleCommand,
) (role authorizationdomain.Role, err error) {
	if service == nil || ctx == nil || !validCreateRoleCommand(command) {
		return authorizationdomain.Role{}, ErrInvalidRoleManagementCommand
	}
	if !service.allowed(ctx, command.ActorUserID) {
		return authorizationdomain.Role{}, ErrRoleManagementDenied
	}
	err = service.transactions.WithinTransaction(ctx, func(transactionContext context.Context) error {
		role, err = service.repository.CreateAccountRole(
			transactionContext,
			command.AccountID,
			command.Name,
		)
		if err != nil || service.auditor == nil {
			return err
		}
		return service.auditor.RecordRoleChange(transactionContext, RoleManagementAuditEvent{
			Scope:       string(authorizationdomain.RoleOwnerAccount),
			AccountID:   role.AccountID,
			ActorUserID: command.ActorUserID,
			RequestID:   command.RequestID,
			Action:      "authorization.role.created",
			RoleID:      role.ID,
		})
	})
	if err != nil {
		return authorizationdomain.Role{}, err
	}
	return role, nil
}

func (service *RoleManagementService) Update(
	ctx context.Context,
	command UpdateRoleCommand,
) (role authorizationdomain.Role, err error) {
	if service == nil || ctx == nil || !validUpdateRoleCommand(command) {
		return authorizationdomain.Role{}, ErrInvalidRoleManagementCommand
	}
	if !service.allowed(ctx, command.ActorUserID) {
		return authorizationdomain.Role{}, ErrRoleManagementDenied
	}
	err = service.transactions.WithinTransaction(ctx, func(transactionContext context.Context) error {
		role, err = service.repository.UpdateAccountRoleName(
			transactionContext,
			command.AccountID,
			command.RoleID,
			command.ExpectedVersion,
			command.Name,
		)
		if err != nil || service.auditor == nil {
			return err
		}
		return service.auditor.RecordRoleChange(transactionContext, RoleManagementAuditEvent{
			Scope:       string(authorizationdomain.RoleOwnerAccount),
			AccountID:   role.AccountID,
			ActorUserID: command.ActorUserID,
			RequestID:   command.RequestID,
			Action:      "authorization.role.updated",
			RoleID:      role.ID,
		})
	})
	if err != nil {
		return authorizationdomain.Role{}, err
	}
	return role, nil
}

func (service *RoleManagementService) allowed(ctx context.Context, actorUserID string) bool {
	key, err := authorizationdomain.ParsePermissionKey(PermissionRolesManageSystem)
	if err != nil {
		return false
	}
	allowed, err := service.authorizer.Allowed(ctx, actorUserID, key)
	return err == nil && allowed
}

func validCreateRoleCommand(command CreateRoleCommand) bool {
	return strings.TrimSpace(command.ActorUserID) != "" &&
		strings.TrimSpace(command.AccountID) != "" && validRoleName(command.Name)
}

func validUpdateRoleCommand(command UpdateRoleCommand) bool {
	return strings.TrimSpace(command.ActorUserID) != "" &&
		strings.TrimSpace(command.AccountID) != "" &&
		strings.TrimSpace(command.RoleID) != "" &&
		command.ExpectedVersion >= 1 && validRoleName(command.Name)
}

func validRoleName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" && trimmed == name && len(trimmed) <= 128
}
