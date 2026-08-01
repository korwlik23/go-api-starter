package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidGrantRolePermissionHandler = errors.New(
	"invalid grant role permission handler",
)

type grantRolePermissionService interface {
	Execute(
		context.Context,
		authorizationapp.GrantRolePermissionCommand,
	) (authorizationapp.GrantRolePermissionResult, error)
}

type GrantRolePermissionHandler struct {
	service grantRolePermissionService
}

func NewGrantRolePermissionHandler(
	service grantRolePermissionService,
) (*GrantRolePermissionHandler, error) {
	if service == nil {
		return nil, ErrInvalidGrantRolePermissionHandler
	}
	return &GrantRolePermissionHandler{service: service}, nil
}

func (handler *GrantRolePermissionHandler) Grant(ctx *gin.Context) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil {
		writeGrantRolePermissionProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return
	}
	roleID := strings.TrimSpace(ctx.Param("role_id"))
	if roleID == "" {
		writeGrantRolePermissionProblem(ctx, http.StatusBadRequest, "invalid_request", "The role_id path parameter is required.", nil)
		return
	}
	var request adminapi.PermissionGrantRequest
	if err := decodeGrantStrictJSON(ctx, &request); err != nil || request.ExpectedVersion < 1 || strings.TrimSpace(request.PermissionKey) == "" {
		writeGrantRolePermissionProblem(ctx, http.StatusBadRequest, "invalid_request", "The permission grant request is invalid.", err)
		return
	}
	accountID := ""
	if resolved, ok := ResolvedAccountFromContext(ctx.Request.Context()); ok {
		accountID = resolved.Account.ID
	}
	result, err := handler.service.Execute(ctx.Request.Context(), authorizationapp.GrantRolePermissionCommand{
		GrantorUserID:   *session.UserID,
		AccountID:       accountID,
		RoleID:          roleID,
		PermissionKey:   strings.TrimSpace(request.PermissionKey),
		RequestID:       platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		ExpectedVersion: int64(request.ExpectedVersion),
	})
	if err != nil {
		switch {
		case errors.Is(err, authorizationapp.ErrGrantDenied):
			writeGrantRolePermissionProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", nil)
		case errors.Is(err, authorizationapp.ErrGrantConflict):
			writeGrantRolePermissionProblem(ctx, http.StatusConflict, "conflict", "The role changed before the request completed.", nil)
		default:
			writeGrantRolePermissionProblem(ctx, http.StatusInternalServerError, "internal_error", "The permission grant could not be completed.", err)
		}
		return
	}
	other := adminapi.SessionInvalidationOutcomeOtherPrincipalsUnchanged
	if result.AffectedPrincipals > 0 {
		other = adminapi.SessionInvalidationOutcomeOtherPrincipalsInvalidated
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adminapi.PermissionGrantResponse{
		AffectedPrincipals: result.AffectedPrincipals,
		PermissionKey:      result.PermissionKey,
		RoleId:             result.RoleID,
		Version:            int(result.Version),
		SessionOutcome: adminapi.SessionInvalidationOutcome{
			CurrentPrincipal: adminapi.SessionInvalidationOutcomeCurrentPrincipalUnchanged,
			OtherPrincipals:  other,
		},
	})
}

func decodeGrantStrictJSON(ctx *gin.Context, destination any) error {
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func writeGrantRolePermissionProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(
		ctx,
		status,
		platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(code, message, nil, cause),
	)
}
