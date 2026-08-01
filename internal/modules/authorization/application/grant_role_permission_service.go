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
	ExpectedVersion int64
}

type GrantRolePermissionResult struct {
	RoleID             string
	PermissionKey      string
	Version            int64
	AffectedPrincipals int
}

type GrantRolePermissionRepository interface {
	GrantRolePermission(
		context.Context,
		GrantRolePermissionCommand,
	) (GrantRolePermissionResult, error)
}

type GrantRolePermissionService struct {
	transactions database.Transactor
	repository   GrantRolePermissionRepository
}

func NewGrantRolePermissionService(
	transactions database.Transactor,
	repository GrantRolePermissionRepository,
) (*GrantRolePermissionService, error) {
	if transactions == nil || repository == nil {
		return nil, ErrInvalidGrantService
	}
	return &GrantRolePermissionService{
		transactions: transactions,
		repository:   repository,
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
		return err
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
