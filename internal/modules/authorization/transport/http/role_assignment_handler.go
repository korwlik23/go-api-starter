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

var ErrInvalidRoleAssignmentHandler = errors.New("invalid role assignment handler")

type roleAssignmentService interface {
	Assign(context.Context, authorizationapp.AssignRoleCommand) (authorizationapp.RoleAssignmentResult, error)
	Revoke(context.Context, authorizationapp.RevokeRoleCommand) (authorizationapp.RoleRevokeResult, error)
}

type RoleAssignmentHandler struct {
	service roleAssignmentService
}

func NewRoleAssignmentHandler(service roleAssignmentService) (*RoleAssignmentHandler, error) {
	if service == nil {
		return nil, ErrInvalidRoleAssignmentHandler
	}
	return &RoleAssignmentHandler{service: service}, nil
}

func (handler *RoleAssignmentHandler) Assign(ctx *gin.Context) {
	session, ok := roleAssignmentSession(ctx)
	if !ok {
		writeRoleAssignmentProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return
	}
	roleID := strings.TrimSpace(ctx.Param("role_id"))
	if roleID == "" {
		writeRoleAssignmentProblem(ctx, http.StatusBadRequest, "invalid_request", "The role_id path parameter is required.", nil)
		return
	}
	request, err := decodeRoleAssignmentJSON(ctx)
	if err != nil || request.ExpectedVersion < 1 || strings.TrimSpace(request.TargetUserID) == "" {
		writeRoleAssignmentProblem(ctx, http.StatusBadRequest, "invalid_request", "The role assignment request is invalid.", err)
		return
	}
	accountID := request.AccountID
	if accountID == "" {
		if resolved, resolvedOK := ResolvedAccountFromContext(ctx.Request.Context()); resolvedOK {
			accountID = resolved.Account.ID
		}
	}
	result, err := handler.service.Assign(ctx.Request.Context(), authorizationapp.AssignRoleCommand{
		GrantorUserID:   *session.UserID,
		RoleID:          roleID,
		Scope:           request.Scope,
		AccountID:       accountID,
		TargetUserID:    strings.TrimSpace(request.TargetUserID),
		ExpectedVersion: request.ExpectedVersion,
	})
	if err != nil {
		writeRoleAssignmentServiceError(ctx, err, "The role assignment could not be completed.")
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adminapi.RoleAssignmentResponse{
		AffectedPrincipals: result.AffectedPrincipals,
		AssignmentId:       result.AssignmentID,
		RoleId:             result.RoleID,
		Version:            int(result.Version),
		SessionOutcome:     roleAssignmentSessionOutcome(result.AffectedPrincipals, strings.TrimSpace(request.TargetUserID) == *session.UserID),
	})
}

func (handler *RoleAssignmentHandler) Revoke(ctx *gin.Context) {
	session, ok := roleAssignmentSession(ctx)
	if !ok {
		writeRoleAssignmentProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return
	}
	assignmentID := strings.TrimSpace(ctx.Param("assignment_id"))
	if assignmentID == "" {
		writeRoleAssignmentProblem(ctx, http.StatusBadRequest, "invalid_request", "The assignment_id path parameter is required.", nil)
		return
	}
	var request adminapi.RoleRevokeRequest
	if err := decodeGrantStrictJSON(ctx, &request); err != nil || request.ExpectedVersion < 1 {
		writeRoleAssignmentProblem(ctx, http.StatusBadRequest, "invalid_request", "The role revoke request is invalid.", err)
		return
	}
	result, err := handler.service.Revoke(ctx.Request.Context(), authorizationapp.RevokeRoleCommand{
		GrantorUserID:   *session.UserID,
		AssignmentID:    assignmentID,
		ExpectedVersion: int64(request.ExpectedVersion),
	})
	if err != nil {
		writeRoleAssignmentServiceError(ctx, err, "The role assignment could not be revoked.")
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adminapi.RoleRevokeResponse{
		AffectedPrincipals: result.AffectedPrincipals,
		AssignmentId:       result.AssignmentID,
		Revoked:            adminapi.RoleRevokeResponseRevoked(result.Revoked),
		Version:            int(result.Version),
		SessionOutcome:     roleAssignmentSessionOutcome(result.AffectedPrincipals, result.AffectedUserID == *session.UserID),
	})
}

type decodedRoleAssignment struct {
	Scope           string
	TargetUserID    string
	AccountID       string
	ExpectedVersion int64
}

func decodeRoleAssignmentJSON(ctx *gin.Context) (decodedRoleAssignment, error) {
	var raw map[string]json.RawMessage
	decoder := json.NewDecoder(ctx.Request.Body)
	if err := decoder.Decode(&raw); err != nil {
		return decodedRoleAssignment{}, err
	}
	if raw == nil {
		return decodedRoleAssignment{}, errors.New("role assignment body must be an object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return decodedRoleAssignment{}, errors.New("multiple JSON values")
		}
		return decodedRoleAssignment{}, err
	}
	var scope string
	if err := json.Unmarshal(raw["scope"], &scope); err != nil {
		return decodedRoleAssignment{}, err
	}
	allowed := map[string]bool{
		"scope": true, "user_id": true, "expected_version": true,
	}
	if scope == authorizationapp.AssignmentScopeAccount {
		allowed["account_id"] = true
	} else if scope != authorizationapp.AssignmentScopeSystem {
		return decodedRoleAssignment{}, errors.New("unknown role assignment scope")
	}
	for key := range raw {
		if !allowed[key] {
			return decodedRoleAssignment{}, errors.New("unknown role assignment field: " + key)
		}
	}
	var targetUserID string
	if err := json.Unmarshal(raw["user_id"], &targetUserID); err != nil {
		return decodedRoleAssignment{}, err
	}
	var expectedVersion int
	if err := json.Unmarshal(raw["expected_version"], &expectedVersion); err != nil {
		return decodedRoleAssignment{}, err
	}
	accountID := ""
	if scope == authorizationapp.AssignmentScopeAccount {
		if err := json.Unmarshal(raw["account_id"], &accountID); err != nil {
			return decodedRoleAssignment{}, err
		}
	}
	return decodedRoleAssignment{
		Scope: scope, TargetUserID: targetUserID, AccountID: accountID,
		ExpectedVersion: int64(expectedVersion),
	}, nil
}

func roleAssignmentSession(ctx *gin.Context) (identitydomain.Session, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil {
		return identitydomain.Session{}, false
	}
	return session, true
}

func roleAssignmentSessionOutcome(affected int, currentAffected bool) adminapi.SessionInvalidationOutcome {
	current := adminapi.SessionInvalidationOutcomeCurrentPrincipalUnchanged
	if currentAffected {
		current = adminapi.SessionInvalidationOutcomeCurrentPrincipalRotated
	}
	other := adminapi.SessionInvalidationOutcomeOtherPrincipalsUnchanged
	if affected > 0 {
		other = adminapi.SessionInvalidationOutcomeOtherPrincipalsInvalidated
	}
	return adminapi.SessionInvalidationOutcome{
		CurrentPrincipal: current,
		OtherPrincipals:  other,
	}
}

func writeRoleAssignmentServiceError(ctx *gin.Context, cause error, message string) {
	switch {
	case errors.Is(cause, authorizationapp.ErrRoleAssignmentDenied):
		writeRoleAssignmentProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", nil)
	case errors.Is(cause, authorizationapp.ErrLastAuthorizationManager):
		writeRoleAssignmentProblem(ctx, http.StatusConflict, "last_authorization_manager", "The last authorization manager cannot be revoked.", nil)
	case errors.Is(cause, authorizationapp.ErrRoleAssignmentConflict):
		writeRoleAssignmentProblem(ctx, http.StatusConflict, "conflict", "The role changed before the request completed.", nil)
	case errors.Is(cause, authorizationapp.ErrInvalidRoleAssignmentCommand):
		writeRoleAssignmentProblem(ctx, http.StatusBadRequest, "invalid_request", "The role assignment request is invalid.", nil)
	default:
		writeRoleAssignmentProblem(ctx, http.StatusInternalServerError, "internal_error", message, cause)
	}
}

func writeRoleAssignmentProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(
		ctx,
		status,
		platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(code, message, nil, cause),
	)
}

var _ roleAssignmentService = (*authorizationapp.RoleAssignmentService)(nil)
