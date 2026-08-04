package app

import (
	"context"

	"github.com/gin-gonic/gin"
	accountsapp "github.com/korwlik23/go-api-starter/internal/modules/accounts/application"
	accountsports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	authorizationhttp "github.com/korwlik23/go-api-starter/internal/modules/authorization/transport/http"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

func newPermissionGuard(
	accounts *accountsapp.ResolveAccount,
	evaluator *authorizationapp.EvaluatePermission,
	rawKey string,
	resourceID authorizationhttp.ResourceIDResolver,
) (gin.HandlerFunc, error) {
	key, err := authorizationdomain.ParsePermissionKey(rawKey)
	if err != nil {
		return nil, errAuthorizationComposition
	}
	return authorizationhttp.RequirePermission(
		authorizationhttp.PermissionGuardDependencies{
			Accounts: accounts, Permissions: evaluator,
			Key: key, ResourceID: resourceID,
		},
	)
}

func currentPrincipalResourceID(ctx *gin.Context) string {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok ||
		session.State != identitydomain.SessionStateAuthenticated ||
		session.UserID == nil {
		return ""
	}
	return *session.UserID
}

type accountResolver interface {
	Execute(context.Context, string) (accountsports.ResolvedAccount, error)
}

type principalOwnership struct {
	accounts accountResolver
}

type systemPermissionAuthorizer struct {
	evaluator *authorizationapp.EvaluatePermission
}

func (authorizer systemPermissionAuthorizer) Allowed(
	ctx context.Context,
	userID string,
	permission authorizationdomain.PermissionKey,
) (bool, error) {
	if authorizer.evaluator == nil {
		return false, errAuthorizationComposition
	}
	return authorizer.evaluator.Execute(
		ctx,
		authorizationapp.PermissionRequest{
			UserID: userID, Permission: permission,
		},
	)
}

func (ownership principalOwnership) OwnerWithinAccount(
	ctx context.Context,
	accountID string,
	resourceID string,
) (string, bool, error) {
	resolved, err := ownership.accounts.Execute(ctx, resourceID)
	if err != nil {
		return "", false, err
	}
	if resolved.Account.ID != accountID ||
		resolved.Membership.UserID != resourceID {
		return "", false, nil
	}
	return resourceID, true, nil
}
