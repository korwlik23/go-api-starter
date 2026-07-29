package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	accountsports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidPermissionGuard = errors.New(
	"invalid permission guard dependencies",
)

type accountResolver interface {
	Execute(
		ctx context.Context,
		userID string,
	) (accountsports.ResolvedAccount, error)
}

type permissionEvaluator interface {
	Execute(
		ctx context.Context,
		request authorizationapp.PermissionRequest,
	) (bool, error)
}

type ResourceIDResolver func(*gin.Context) string

type PermissionGuardDependencies struct {
	Accounts    accountResolver
	Permissions permissionEvaluator
	Key         authorizationdomain.PermissionKey
	ResourceID  ResourceIDResolver
}

type resolvedAccountContextKey struct{}

func RequirePermission(
	dependencies PermissionGuardDependencies,
) (gin.HandlerFunc, error) {
	if dependencies.Accounts == nil ||
		dependencies.Permissions == nil {
		return nil, ErrInvalidPermissionGuard
	}
	if _, err := authorizationdomain.ParsePermissionKey(
		dependencies.Key.String(),
	); err != nil {
		return nil, ErrInvalidPermissionGuard
	}
	if dependencies.Key.Scope == authorizationdomain.PermissionScopeOwn &&
		dependencies.ResourceID == nil {
		return nil, ErrInvalidPermissionGuard
	}

	return func(ctx *gin.Context) {
		session, ok := platformmiddleware.SessionFromContext(
			ctx.Request.Context(),
		)
		if !ok ||
			session.State != identitydomain.SessionStateAuthenticated ||
			session.UserID == nil {
			writePermissionProblem(
				ctx,
				nethttp.StatusUnauthorized,
				"authentication_required",
				"Authentication is required.",
				nil,
			)
			return
		}
		resolved, err := dependencies.Accounts.Execute(
			ctx.Request.Context(),
			*session.UserID,
		)
		if err != nil {
			writePermissionProblem(
				ctx,
				nethttp.StatusForbidden,
				"permission_denied",
				"Permission is required.",
				err,
			)
			return
		}
		resourceID := ""
		if dependencies.ResourceID != nil {
			resourceID = dependencies.ResourceID(ctx)
		}
		allowed, err := dependencies.Permissions.Execute(
			ctx.Request.Context(),
			authorizationapp.PermissionRequest{
				UserID:     *session.UserID,
				AccountID:  resolved.Account.ID,
				Permission: dependencies.Key,
				ResourceID: resourceID,
			},
		)
		if err != nil || !allowed {
			writePermissionProblem(
				ctx,
				nethttp.StatusForbidden,
				"permission_denied",
				"Permission is required.",
				err,
			)
			return
		}
		requestContext := context.WithValue(
			ctx.Request.Context(),
			resolvedAccountContextKey{},
			resolved,
		)
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Next()
	}, nil
}

func ResolvedAccountFromContext(
	ctx context.Context,
) (accountsports.ResolvedAccount, bool) {
	resolved, ok := ctx.Value(resolvedAccountContextKey{}).(accountsports.ResolvedAccount)
	return resolved, ok
}

func writePermissionProblem(
	ctx *gin.Context,
	status int,
	code string,
	message string,
	cause error,
) {
	response.WriteProblem(
		ctx,
		status,
		platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(code, message, nil, cause),
	)
}
