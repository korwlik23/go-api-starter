package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	accountsapp "github.com/korwlik23/go-api-starter/internal/modules/accounts/application"
	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var ErrInvalidMembershipHandler = errors.New("invalid membership handler")

type MembershipHandler struct {
	service *accountsapp.MembershipService
}

func NewMembershipHandler(service *accountsapp.MembershipService) (*MembershipHandler, error) {
	if service == nil {
		return nil, ErrInvalidMembershipHandler
	}
	return &MembershipHandler{service: service}, nil
}

func (handler *MembershipHandler) List(ctx *gin.Context) {
	actor, ok := authenticatedMembershipUser(ctx)
	if !ok {
		return
	}
	accountID := strings.TrimSpace(ctx.Param("account_id"))
	if accountID == "" {
		writeMembershipProblem(ctx, http.StatusBadRequest, "invalid_request", "The account_id path parameter is required.", nil)
		return
	}
	filter, err := parseMembershipFilter(ctx, accountID)
	if err != nil {
		writeMembershipProblem(ctx, http.StatusBadRequest, "invalid_request", "The membership list request is invalid.", err)
		return
	}
	page, err := handler.service.List(ctx.Request.Context(), actor, filter)
	if err != nil {
		writeMembershipError(ctx, err, "The memberships could not be listed.")
		return
	}
	items := make([]adminapi.MembershipResponse, 0, len(page.Items))
	for _, item := range page.Items {
		mapped, mapErr := membershipResponse(item)
		if mapErr != nil {
			writeMembershipProblem(ctx, http.StatusInternalServerError, "internal_error", "The memberships could not be listed.", mapErr)
			return
		}
		items = append(items, mapped)
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adminapi.MembershipListResponse{Items: items, Page: adminapi.PageInfo{Page: page.Page, PerPage: page.PerPage, TotalItems: page.TotalItems, TotalPages: page.TotalPages}})
}

func (handler *MembershipHandler) UpdateStatus(ctx *gin.Context) {
	actor, ok := authenticatedMembershipUser(ctx)
	if !ok {
		return
	}
	accountID := strings.TrimSpace(ctx.Param("account_id"))
	membershipID := strings.TrimSpace(ctx.Param("membership_id"))
	if accountID == "" || membershipID == "" {
		writeMembershipProblem(ctx, http.StatusBadRequest, "invalid_request", "The membership path parameters are required.", nil)
		return
	}
	var request adminapi.MembershipStatusRequest
	if err := decodeMembershipJSON(ctx, &request); err != nil || request.ExpectedVersion < 1 || !request.Status.Valid() {
		writeMembershipProblem(ctx, http.StatusBadRequest, "invalid_request", "The membership status request is invalid.", err)
		return
	}
	view, err := handler.service.UpdateStatus(ctx.Request.Context(), accountsapp.UpdateMembershipStatusCommand{ActorUserID: actor, AccountID: accountID, MembershipID: membershipID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), ExpectedVersion: int64(request.ExpectedVersion), Status: accountsdomain.MembershipStatus(request.Status)})
	if err != nil {
		writeMembershipError(ctx, err, "The membership status could not be changed.")
		return
	}
	body, mapErr := membershipResponse(view)
	if mapErr != nil {
		writeMembershipProblem(ctx, http.StatusInternalServerError, "internal_error", "The membership status could not be changed.", mapErr)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, body)
}

func authenticatedMembershipUser(ctx *gin.Context) (string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil {
		writeMembershipProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return "", false
	}
	return *session.UserID, true
}
func parseMembershipFilter(ctx *gin.Context, accountID string) (ports.MembershipListFilter, error) {
	allowed := map[string]bool{"status": true, "page": true, "per_page": true}
	for key, values := range ctx.Request.URL.Query() {
		if !allowed[key] || len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			return ports.MembershipListFilter{}, errors.New("invalid query parameter: " + key)
		}
	}
	filter := ports.MembershipListFilter{AccountID: accountID}
	if value, present := ctx.GetQuery("status"); present {
		if value != "active" && value != "disabled" {
			return filter, errors.New("invalid status")
		}
		filter.Status = accountsdomain.MembershipStatus(value)
	}
	page, perPage, err := parseMembershipPage(ctx)
	if err != nil {
		return filter, err
	}
	filter.Page = page
	filter.PerPage = perPage
	return filter, nil
}
func parseMembershipPage(ctx *gin.Context) (int, int, error) {
	page, err := positiveMembershipQuery(ctx, "page", 1)
	if err != nil {
		return 0, 0, err
	}
	perPage, err := positiveMembershipQuery(ctx, "per_page", 25)
	if err != nil {
		return 0, 0, err
	}
	if perPage > 100 {
		return 0, 0, errors.New("per_page exceeds the maximum")
	}
	return page, perPage, nil
}
func positiveMembershipQuery(ctx *gin.Context, key string, fallback int) (int, error) {
	value, present := ctx.GetQuery(key)
	if !present {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errors.New(key + " must be a positive integer")
	}
	return parsed, nil
}
func membershipResponse(view ports.MembershipView) (adminapi.MembershipResponse, error) {
	if view.ID == "" || view.AccountID == "" || view.AccountSlug == "" || view.UserID == "" || view.UserEmail == "" || !validMembershipStatus(view.Status) || view.AuthorizationVersion < 1 || view.CreatedAt.IsZero() || view.UpdatedAt.IsZero() || view.UpdatedAt.Before(view.CreatedAt) {
		return adminapi.MembershipResponse{}, errors.New("invalid membership projection")
	}
	return adminapi.MembershipResponse{Id: view.ID, AccountId: view.AccountID, AccountSlug: view.AccountSlug, UserId: view.UserID, UserEmail: openapi_types.Email(view.UserEmail), Status: adminapi.MembershipResponseStatus(view.Status), AuthorizationVersion: int(view.AuthorizationVersion), CreatedAt: view.CreatedAt, UpdatedAt: view.UpdatedAt}, nil
}
func validMembershipStatus(status accountsdomain.MembershipStatus) bool {
	return status == accountsdomain.MembershipStatusActive || status == accountsdomain.MembershipStatusDisabled
}
func decodeMembershipJSON(ctx *gin.Context, destination any) error {
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
func writeMembershipError(ctx *gin.Context, err error, message string) {
	switch {
	case errors.Is(err, accountsapp.ErrMembershipDenied):
		writeMembershipProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", nil)
	case errors.Is(err, accountsapp.ErrInvalidMembershipCommand), errors.Is(err, ports.ErrInvalidMembershipQuery):
		writeMembershipProblem(ctx, http.StatusBadRequest, "invalid_request", "The membership request is invalid.", nil)
	case errors.Is(err, ports.ErrMembershipNotFound):
		writeMembershipProblem(ctx, http.StatusNotFound, "not_found", "The membership was not found.", nil)
	case errors.Is(err, ports.ErrMembershipConflict):
		writeMembershipProblem(ctx, http.StatusConflict, "conflict", "The membership changed before the request completed.", nil)
	default:
		writeMembershipProblem(ctx, http.StatusInternalServerError, "internal_error", message, err)
	}
}
func writeMembershipProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem(code, message, nil, cause))
}
