package application

import (
	"context"
	"errors"
	"strings"

	"github.com/korwlik23/go-api-starter/internal/platform/database"
)

const (
	AssignmentScopeAccount = "account"
	AssignmentScopeSystem  = "system"
)

var (
	ErrInvalidRoleAssignmentService = errors.New("invalid role assignment service")
	ErrInvalidRoleAssignmentCommand = errors.New("invalid role assignment command")
	ErrRoleAssignmentConflict       = errors.New("role assignment conflict")
	ErrRoleAssignmentDenied         = errors.New("role assignment denied")
	ErrLastAuthorizationManager     = errors.New("last authorization manager")
)

type AssignRoleCommand struct {
	GrantorUserID   string
	RoleID          string
	Scope           string
	AccountID       string
	TargetUserID    string
	RequestID       string
	ExpectedVersion int64
}

type RevokeRoleCommand struct {
	GrantorUserID   string
	AssignmentID    string
	RequestID       string
	ExpectedVersion int64
}

type RoleAssignmentResult struct {
	AssignmentID       string
	RoleID             string
	Scope              string
	AccountID          string
	AffectedUserID     string
	Version            int64
	AffectedPrincipals int
}

type RoleRevokeResult struct {
	AssignmentID       string
	RoleID             string
	Scope              string
	AccountID          string
	AffectedUserID     string
	Version            int64
	Revoked            bool
	AffectedPrincipals int
}

type RoleAssignmentRepository interface {
	AssignRole(context.Context, AssignRoleCommand) (RoleAssignmentResult, error)
	RevokeRole(context.Context, RevokeRoleCommand) (RoleRevokeResult, error)
}

type RoleAssignmentAuditEvent struct {
	Scope        string
	AccountID    string
	ActorUserID  string
	RequestID    string
	Action       string
	AssignmentID string
	RoleID       string
	TargetUserID string
}

type RoleAssignmentAuditor interface {
	RecordRoleAssignment(context.Context, RoleAssignmentAuditEvent) error
}

type RoleAssignmentService struct {
	transactions database.Transactor
	repository   RoleAssignmentRepository
	auditor      RoleAssignmentAuditor
}

func NewRoleAssignmentService(
	transactions database.Transactor,
	repository RoleAssignmentRepository,
	auditors ...RoleAssignmentAuditor,
) (*RoleAssignmentService, error) {
	if transactions == nil || repository == nil {
		return nil, ErrInvalidRoleAssignmentService
	}
	if len(auditors) > 1 {
		return nil, ErrInvalidRoleAssignmentService
	}
	var auditor RoleAssignmentAuditor
	if len(auditors) == 1 {
		auditor = auditors[0]
	}
	return &RoleAssignmentService{
		transactions: transactions, repository: repository, auditor: auditor,
	}, nil
}

func (service *RoleAssignmentService) Assign(
	ctx context.Context,
	command AssignRoleCommand,
) (result RoleAssignmentResult, err error) {
	if service == nil || ctx == nil || !validAssignRoleCommand(command) {
		return RoleAssignmentResult{}, ErrInvalidRoleAssignmentCommand
	}
	err = service.transactions.WithinTransaction(ctx, func(transactionContext context.Context) error {
		result, err = service.repository.AssignRole(transactionContext, command)
		if err != nil {
			return err
		}
		if service.auditor == nil {
			return nil
		}
		return service.auditor.RecordRoleAssignment(transactionContext, RoleAssignmentAuditEvent{
			Scope:        result.Scope,
			AccountID:    result.AccountID,
			ActorUserID:  command.GrantorUserID,
			RequestID:    command.RequestID,
			Action:       "authorization.role.assignment.assigned",
			AssignmentID: result.AssignmentID,
			RoleID:       result.RoleID,
			TargetUserID: result.AffectedUserID,
		})
	})
	if err != nil {
		return RoleAssignmentResult{}, err
	}
	return result, nil
}

func (service *RoleAssignmentService) Revoke(
	ctx context.Context,
	command RevokeRoleCommand,
) (result RoleRevokeResult, err error) {
	if service == nil || ctx == nil || !validRevokeRoleCommand(command) {
		return RoleRevokeResult{}, ErrInvalidRoleAssignmentCommand
	}
	err = service.transactions.WithinTransaction(ctx, func(transactionContext context.Context) error {
		result, err = service.repository.RevokeRole(transactionContext, command)
		if err != nil {
			return err
		}
		if service.auditor == nil {
			return nil
		}
		return service.auditor.RecordRoleAssignment(transactionContext, RoleAssignmentAuditEvent{
			Scope:        result.Scope,
			AccountID:    result.AccountID,
			ActorUserID:  command.GrantorUserID,
			RequestID:    command.RequestID,
			Action:       "authorization.role.assignment.revoked",
			AssignmentID: result.AssignmentID,
			RoleID:       result.RoleID,
			TargetUserID: result.AffectedUserID,
		})
	})
	if err != nil {
		return RoleRevokeResult{}, err
	}
	return result, nil
}

func validAssignRoleCommand(command AssignRoleCommand) bool {
	return strings.TrimSpace(command.GrantorUserID) != "" &&
		strings.TrimSpace(command.RoleID) != "" &&
		strings.TrimSpace(command.TargetUserID) != "" &&
		(command.Scope == AssignmentScopeSystem || command.Scope == AssignmentScopeAccount) &&
		(command.Scope != AssignmentScopeAccount || strings.TrimSpace(command.AccountID) != "") &&
		command.ExpectedVersion >= 1
}

func validRevokeRoleCommand(command RevokeRoleCommand) bool {
	return strings.TrimSpace(command.GrantorUserID) != "" &&
		strings.TrimSpace(command.AssignmentID) != "" &&
		command.ExpectedVersion >= 1
}
