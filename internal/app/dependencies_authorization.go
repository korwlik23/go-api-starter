package app

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	accountsmodule "github.com/korwlik23/go-api-starter/internal/modules/accounts"
	accountsgorm "github.com/korwlik23/go-api-starter/internal/modules/accounts/adapters/gorm"
	accountsapp "github.com/korwlik23/go-api-starter/internal/modules/accounts/application"
	accountsports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	accountshttp "github.com/korwlik23/go-api-starter/internal/modules/accounts/transport/http"
	authorizationmodule "github.com/korwlik23/go-api-starter/internal/modules/authorization"
	authorizationgorm "github.com/korwlik23/go-api-starter/internal/modules/authorization/adapters/gorm"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	authorizationhttp "github.com/korwlik23/go-api-starter/internal/modules/authorization/transport/http"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	operationsmodule "github.com/korwlik23/go-api-starter/internal/modules/operations"
	operationshttp "github.com/korwlik23/go-api-starter/internal/modules/operations/transport/http"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	gormdb "gorm.io/gorm"
)

var errAuthorizationComposition = errors.New(
	"app: authorization composition failed",
)

type authorizationRuntime struct {
	currentAccount       *accountshttp.CurrentAccountHandler
	effectivePermissions *authorizationhttp.PermissionHandler
	modules              *operationshttp.ModuleHandler
	currentAccountGuard  gin.HandlerFunc
	effectiveGuard       gin.HandlerFunc
	modulesGuard         gin.HandlerFunc
	grantGuard           gin.HandlerFunc
	grantRolePermission  *authorizationhttp.GrantRolePermissionHandler
	systemAuthorizer     systemPermissionAuthorizer
}

func buildAuthorizationRuntime(
	db *gormdb.DB,
	modules moduleProjectionReader,
) (authorizationRuntime, error) {
	accounts, err := accountsgorm.NewAccountRepository(db)
	if err != nil {
		return authorizationRuntime{}, err
	}
	transactions, err := database.NewTransactionManager(db)
	if err != nil {
		return authorizationRuntime{}, err
	}
	grantRepository, err := authorizationgorm.NewTransactionalGrantRolePermissionRepository(
		transactions,
		db.Dialector.Name(),
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	grantService, err := authorizationapp.NewGrantRolePermissionService(
		transactions,
		grantRepository,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	grantHandler, err := authorizationhttp.NewGrantRolePermissionHandler(grantService)
	if err != nil {
		return authorizationRuntime{}, err
	}
	resolveAccount, err := accountsapp.NewResolveAccount(accounts)
	if err != nil {
		return authorizationRuntime{}, err
	}
	permissions, err := authorizationgorm.NewPermissionRepository(db)
	if err != nil {
		return authorizationRuntime{}, err
	}
	evaluator, err := authorizationapp.NewEvaluatePermission(
		permissions,
		principalOwnership{accounts: resolveAccount},
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	currentAccount, err := accountshttp.NewCurrentAccountHandler(resolveAccount)
	if err != nil {
		return authorizationRuntime{}, err
	}
	effective, err := authorizationhttp.NewPermissionHandler(
		resolveAccount,
		permissions,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	moduleHandler, err := operationshttp.NewModuleHandler(modules)
	if err != nil {
		return authorizationRuntime{}, err
	}
	currentAccountGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		accountsmodule.PermissionCurrentReadOwn,
		currentPrincipalResourceID,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	effectiveGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		authorizationmodule.PermissionEffectiveReadOwn,
		currentPrincipalResourceID,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	modulesGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		operationsmodule.PermissionFoundationReadSystem,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	grantGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		authorizationmodule.PermissionRolePermissionGrant,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	return authorizationRuntime{
		currentAccount: currentAccount, effectivePermissions: effective,
		modules: moduleHandler, currentAccountGuard: currentAccountGuard,
		effectiveGuard: effectiveGuard, modulesGuard: modulesGuard,
		grantGuard: grantGuard, grantRolePermission: grantHandler,
		systemAuthorizer: systemPermissionAuthorizer{evaluator: evaluator},
	}, nil
}

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
	Execute(
		context.Context,
		string,
	) (accountsports.ResolvedAccount, error)
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
