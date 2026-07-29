package http

import (
	"context"
	"errors"
	nethttp "net/http"
	"slices"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	authorizationports "github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

var ErrInvalidPermissionHandler = errors.New(
	"invalid permission handler dependencies",
)

type effectiveGrantReader interface {
	EffectiveGrants(
		ctx context.Context,
		userID string,
		accountID string,
	) ([]authorizationports.EffectiveGrant, error)
}

type PermissionHandler struct {
	accounts accountResolver
	grants   effectiveGrantReader
}

func NewPermissionHandler(
	accounts accountResolver,
	grants effectiveGrantReader,
) (*PermissionHandler, error) {
	if accounts == nil || grants == nil {
		return nil, ErrInvalidPermissionHandler
	}
	return &PermissionHandler{accounts: accounts, grants: grants}, nil
}

func (handler *PermissionHandler) GetEffective(ctx *gin.Context) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
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
	resolved, err := handler.accounts.Execute(
		ctx.Request.Context(),
		*session.UserID,
	)
	if err != nil {
		writeEffectivePermissionFailure(ctx, err)
		return
	}
	grants, err := handler.grants.EffectiveGrants(
		ctx.Request.Context(),
		*session.UserID,
		resolved.Account.ID,
	)
	if err != nil {
		writeEffectivePermissionFailure(ctx, err)
		return
	}
	unique := make(map[string]struct{}, len(grants))
	for _, grant := range grants {
		unique[grant.Key.String()] = struct{}{}
	}
	permissions := make([]string, 0, len(unique))
	for key := range unique {
		permissions = append(permissions, key)
	}
	slices.Sort(permissions)
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusOK, adminapi.EffectivePermissionsResponse{
		AccountId:   resolved.Account.ID,
		Permissions: permissions,
	})
}

func writeEffectivePermissionFailure(ctx *gin.Context, cause error) {
	writePermissionProblem(
		ctx,
		nethttp.StatusForbidden,
		"permission_projection_failed",
		"Effective permissions could not be loaded.",
		cause,
	)
}
