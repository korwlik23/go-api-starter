package app

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	accountsmodule "github.com/korwlik23/go-api-starter/internal/modules/accounts"
	accountsgorm "github.com/korwlik23/go-api-starter/internal/modules/accounts/adapters/gorm"
	accountsapp "github.com/korwlik23/go-api-starter/internal/modules/accounts/application"
	accountsports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	accountshttp "github.com/korwlik23/go-api-starter/internal/modules/accounts/transport/http"
	auditmodule "github.com/korwlik23/go-api-starter/internal/modules/audit"
	auditgorm "github.com/korwlik23/go-api-starter/internal/modules/audit/adapters/gorm"
	auditapp "github.com/korwlik23/go-api-starter/internal/modules/audit/application"
	audithttp "github.com/korwlik23/go-api-starter/internal/modules/audit/transport/http"
	authorizationmodule "github.com/korwlik23/go-api-starter/internal/modules/authorization"
	authorizationgorm "github.com/korwlik23/go-api-starter/internal/modules/authorization/adapters/gorm"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	authorizationhttp "github.com/korwlik23/go-api-starter/internal/modules/authorization/transport/http"
	identitymodule "github.com/korwlik23/go-api-starter/internal/modules/identity"
	identitygorm "github.com/korwlik23/go-api-starter/internal/modules/identity/adapters/gorm"
	identityapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	identityhttp "github.com/korwlik23/go-api-starter/internal/modules/identity/transport/http"
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
	transactions             *database.TransactionManager
	auditor                  authorizationAuditAppender
	currentAccount           *accountshttp.CurrentAccountHandler
	effectivePermissions     *authorizationhttp.PermissionHandler
	modules                  *operationshttp.ModuleHandler
	currentAccountGuard      gin.HandlerFunc
	effectiveGuard           gin.HandlerFunc
	modulesGuard             gin.HandlerFunc
	grantGuard               gin.HandlerFunc
	grantRolePermission      *authorizationhttp.GrantRolePermissionHandler
	assignmentGuard          gin.HandlerFunc
	revokeGuard              gin.HandlerFunc
	roleAssignment           *authorizationhttp.RoleAssignmentHandler
	rolesReadGuard           gin.HandlerFunc
	rolesManageGuard         gin.HandlerFunc
	rolePermissionsReadGuard gin.HandlerFunc
	roleAssignmentsReadGuard gin.HandlerFunc
	roleHandler              *authorizationhttp.RoleHandler
	adminUsers               *identityhttp.AdminUserHandler
	adminUserReadGuard       gin.HandlerFunc
	adminUserManageGuard     gin.HandlerFunc
	memberships              *accountshttp.MembershipHandler
	membershipReadGuard      gin.HandlerFunc
	membershipManageGuard    gin.HandlerFunc
	auditEvents              *audithttp.EventHandler
	auditEventsReadGuard     gin.HandlerFunc
	systemAuthorizer         systemPermissionAuthorizer
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
	auditRepository, err := auditgorm.NewTransactionalEventRepository(
		transactions,
		db.Dialector.Name(),
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	auditor, err := auditapp.NewAppendEvent(auditapp.AppendDependencies{
		Repository: auditRepository,
		NewID:      newRuntimeID,
		Now:        time.Now,
	})
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
		authorizationAuditAdapter{appender: auditor},
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	grantHandler, err := authorizationhttp.NewGrantRolePermissionHandler(grantService)
	if err != nil {
		return authorizationRuntime{}, err
	}
	assignmentRepository, err := authorizationgorm.NewTransactionalRoleAssignmentRepository(
		transactions,
		db.Dialector.Name(),
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	assignmentService, err := authorizationapp.NewRoleAssignmentService(
		transactions,
		assignmentRepository,
		authorizationAuditAdapter{appender: auditor},
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	assignmentHandler, err := authorizationhttp.NewRoleAssignmentHandler(assignmentService)
	if err != nil {
		return authorizationRuntime{}, err
	}
	roleQueryRepository, err := authorizationgorm.NewRoleQueryRepository(db)
	if err != nil {
		return authorizationRuntime{}, err
	}
	roleCommandRepository, err := authorizationgorm.NewTransactionalRoleCommandRepository(
		transactions,
		db.Dialector.Name(),
	)
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
	systemAuthorizer := systemPermissionAuthorizer{evaluator: evaluator}
	roleManagement, err := authorizationapp.NewRoleManagementService(
		transactions,
		systemAuthorizer,
		roleCommandRepository,
		authorizationAuditAdapter{appender: auditor},
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	roleHandler, err := authorizationhttp.NewRoleHandler(roleQueryRepository, roleManagement)
	if err != nil {
		return authorizationRuntime{}, err
	}
	auditQuery, err := auditapp.NewListSystemEvents(systemAuthorizer, auditRepository)
	if err != nil {
		return authorizationRuntime{}, err
	}
	auditEvents, err := audithttp.NewEventHandler(auditQuery)
	if err != nil {
		return authorizationRuntime{}, err
	}
	adminUserQuery, err := identitygorm.NewAdminUserQueryRepository(db)
	if err != nil {
		return authorizationRuntime{}, err
	}
	adminUserCommands, err := identitygorm.NewTransactionalAdminUserCommandRepository(
		transactions,
		db.Dialector.Name(),
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	adminUsersService, err := identityapp.NewAdminUserService(
		transactions,
		systemAuthorizer,
		adminUserQuery,
		adminUserCommands,
		authorizationAuditAdapter{appender: auditor},
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	adminUsers, err := identityhttp.NewAdminUserHandler(adminUsersService)
	if err != nil {
		return authorizationRuntime{}, err
	}
	membershipQuery, err := accountsgorm.NewMembershipQueryRepository(db)
	if err != nil {
		return authorizationRuntime{}, err
	}
	membershipCommands, err := accountsgorm.NewTransactionalMembershipCommandRepository(
		transactions,
		db.Dialector.Name(),
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	membershipService, err := accountsapp.NewMembershipService(
		transactions,
		systemAuthorizer,
		membershipQuery,
		membershipCommands,
		authorizationAuditAdapter{appender: auditor},
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	memberships, err := accountshttp.NewMembershipHandler(membershipService)
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
	assignmentGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		authorizationmodule.PermissionRoleAssignmentAssign,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	revokeGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		authorizationmodule.PermissionRoleAssignmentRevoke,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	rolesReadGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		authorizationmodule.PermissionRolesReadSystem,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	rolesManageGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		authorizationmodule.PermissionRolesManageSystem,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	rolePermissionsReadGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		authorizationmodule.PermissionRolePermissionsReadSystem,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	roleAssignmentsReadGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		authorizationmodule.PermissionRoleAssignmentsReadSystem,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	adminUserReadGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		identitymodule.PermissionUsersReadSystem,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	adminUserManageGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		identitymodule.PermissionUsersManageSystem,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	membershipReadGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		accountsmodule.PermissionMembershipsReadSystem,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	membershipManageGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		accountsmodule.PermissionMembershipsManageSystem,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	auditEventsReadGuard, err := newPermissionGuard(
		resolveAccount,
		evaluator,
		auditmodule.PermissionEventsReadSystem,
		nil,
	)
	if err != nil {
		return authorizationRuntime{}, err
	}
	return authorizationRuntime{
		transactions:   transactions,
		auditor:        auditor,
		currentAccount: currentAccount, effectivePermissions: effective,
		modules: moduleHandler, currentAccountGuard: currentAccountGuard,
		effectiveGuard: effectiveGuard, modulesGuard: modulesGuard,
		grantGuard: grantGuard, grantRolePermission: grantHandler,
		assignmentGuard: assignmentGuard, revokeGuard: revokeGuard,
		roleAssignment: assignmentHandler,
		rolesReadGuard: rolesReadGuard, rolesManageGuard: rolesManageGuard,
		rolePermissionsReadGuard: rolePermissionsReadGuard,
		roleAssignmentsReadGuard: roleAssignmentsReadGuard,
		roleHandler:              roleHandler,
		adminUsers:               adminUsers,
		adminUserReadGuard:       adminUserReadGuard,
		adminUserManageGuard:     adminUserManageGuard,
		memberships:              memberships,
		membershipReadGuard:      membershipReadGuard,
		membershipManageGuard:    membershipManageGuard,
		auditEvents:              auditEvents,
		auditEventsReadGuard:     auditEventsReadGuard,
		systemAuthorizer:         systemAuthorizer,
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
