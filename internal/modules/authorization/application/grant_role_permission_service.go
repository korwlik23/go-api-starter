package application

import (
	"context"
	"errors"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

var (
	ErrInvalidGrantService = errors.New("invalid role permission grant service")
	ErrGrantConflict       = errors.New("role permission grant conflict")
)

type GrantRolePermissionCommand struct {
	GrantorUserID   string
	AccountID       string
	RoleID          string
	PermissionKey   string
	RequestID       string
	ExpectedVersion int64
}

type GrantRolePermissionResult struct {
	RoleID             string
	PermissionKey      string
	Scope              string
	AccountID          string
	Version            int64
	AffectedPrincipals int
}

type GrantRolePermissionRepository interface {
	GrantRolePermission(
		context.Context,
		GrantRolePermissionCommand,
	) (GrantRolePermissionResult, error)
}

type GrantRolePermissionAuditEvent struct {
	Scope         string
	AccountID     string
	ActorUserID   string
	RequestID     string
	Action        string
	RoleID        string
	PermissionKey string
}

type GrantRolePermissionAuditor interface {
	RecordRolePermissionGrant(context.Context, GrantRolePermissionAuditEvent) error
}

type GrantRolePermissionService struct {
	transactions database.Transactor
	repository   GrantRolePermissionRepository
	auditor      GrantRolePermissionAuditor
}

func NewGrantRolePermissionService(
	transactions database.Transactor,
	repository GrantRolePermissionRepository,
	auditors ...GrantRolePermissionAuditor,
) (*GrantRolePermissionService, error) {
	if transactions == nil || repository == nil {
		return nil, ErrInvalidGrantService
	}
	if len(auditors) > 1 {
		return nil, ErrInvalidGrantService
	}
	var auditor GrantRolePermissionAuditor
	if len(auditors) == 1 {
		auditor = auditors[0]
	}
	return &GrantRolePermissionService{
		transactions: transactions, repository: repository, auditor: auditor,
	}, nil
}

func (service *GrantRolePermissionService) Execute(
	ctx context.Context,
	command GrantRolePermissionCommand,
) (result GrantRolePermissionResult, err error) {
	if service == nil || ctx == nil || !validGrantCommand(command) {
		return GrantRolePermissionResult{}, ErrInvalidGrantService
	}
	err = service.transactions.WithinTransaction(ctx, func(transactionContext context.Context) error {
		result, err = service.repository.GrantRolePermission(transactionContext, command)
		if err != nil || service.auditor == nil {
			return err
		}
		return service.auditor.RecordRolePermissionGrant(transactionContext, GrantRolePermissionAuditEvent{
			Scope: result.Scope, AccountID: result.AccountID, ActorUserID: command.GrantorUserID,
			RequestID: command.RequestID, Action: "authorization.role.permission.granted",
			RoleID: result.RoleID, PermissionKey: result.PermissionKey,
		})
	})
	if err != nil {
		return GrantRolePermissionResult{}, err
	}
	return result, nil
}

func validGrantCommand(command GrantRolePermissionCommand) bool {
	return strings.TrimSpace(command.GrantorUserID) != "" &&
		strings.TrimSpace(command.RoleID) != "" &&
		strings.TrimSpace(command.PermissionKey) != "" &&
		command.ExpectedVersion >= 1
}
