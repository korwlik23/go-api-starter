package application

import (
	"context"
	"errors"
	"strings"

	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
)

var ErrInvalidPermissionEvaluation = errors.New(
	"invalid permission evaluation",
)

type PermissionRequest struct {
	UserID     string
	AccountID  string
	Permission authorizationdomain.PermissionKey
	ResourceID string
}

type EvaluatePermission struct {
	grants    ports.PermissionRepository
	ownership ports.ResourceOwnershipRepository
}

func NewEvaluatePermission(
	grants ports.PermissionRepository,
	ownership ports.ResourceOwnershipRepository,
) (*EvaluatePermission, error) {
	if grants == nil || ownership == nil {
		return nil, ErrInvalidPermissionEvaluation
	}
	return &EvaluatePermission{grants: grants, ownership: ownership}, nil
}

func (evaluator *EvaluatePermission) Execute(
	ctx context.Context,
	request PermissionRequest,
) (bool, error) {
	if strings.TrimSpace(request.UserID) == "" ||
		request.Permission.String() == "..." {
		return false, ErrInvalidPermissionEvaluation
	}
	canonical, err := authorizationdomain.ParsePermissionKey(
		request.Permission.String(),
	)
	if err != nil || canonical != request.Permission {
		return false, ErrInvalidPermissionEvaluation
	}
	grants, err := evaluator.grants.EffectiveGrants(
		ctx,
		request.UserID,
		request.AccountID,
	)
	if err != nil {
		return false, err
	}
	for _, grant := range grants {
		if grant.Key != request.Permission {
			continue
		}
		allowed, evaluateErr := evaluator.evaluateGrant(ctx, request, grant)
		if evaluateErr != nil {
			return false, evaluateErr
		}
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

func (evaluator *EvaluatePermission) evaluateGrant(
	ctx context.Context,
	request PermissionRequest,
	grant ports.EffectiveGrant,
) (bool, error) {
	switch request.Permission.Scope {
	case authorizationdomain.PermissionScopeSystem:
		return grant.Source == ports.GrantSourceSystemRole &&
			grant.AccountID == "", nil
	case authorizationdomain.PermissionScopeAny:
		return grant.Source == ports.GrantSourceAccountRole &&
			request.AccountID != "" &&
			grant.AccountID == request.AccountID, nil
	case authorizationdomain.PermissionScopeOwn:
		if grant.Source != ports.GrantSourceAccountRole ||
			request.AccountID == "" ||
			grant.AccountID != request.AccountID ||
			request.ResourceID == "" {
			return false, nil
		}
		ownerID, found, err := evaluator.ownership.OwnerWithinAccount(
			ctx,
			request.AccountID,
			request.ResourceID,
		)
		if err != nil {
			return false, err
		}
		return found && ownerID == request.UserID, nil
	default:
		return false, ErrInvalidPermissionEvaluation
	}
}
