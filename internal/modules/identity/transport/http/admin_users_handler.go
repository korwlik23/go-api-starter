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
	identityapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var ErrInvalidAdminUserHandler = errors.New("invalid admin user handler")

type AdminUserHandler struct{ service *identityapp.AdminUserService }

func NewAdminUserHandler(service *identityapp.AdminUserService) (*AdminUserHandler, error) {
	if service == nil {
		return nil, ErrInvalidAdminUserHandler
	}
	return &AdminUserHandler{service: service}, nil
}

func (handler *AdminUserHandler) List(ctx *gin.Context) {
	actor, ok := authenticatedAdminUser(ctx)
	if !ok {
		return
	}
	filter, err := parseAdminUserFilter(ctx)
	if err != nil {
		writeAdminUserProblem(ctx, http.StatusBadRequest, "invalid_request", "The user list request is invalid.", err)
		return
	}
	page, err := handler.service.List(ctx.Request.Context(), actor, filter)
	if err != nil {
		writeAdminUserError(ctx, err, "The users could not be listed.")
		return
	}
	items := make([]adminapi.AdminUserListItem, 0, len(page.Items))
	for _, item := range page.Items {
		mapped, mapErr := adminUserListItem(item)
		if mapErr != nil {
			writeAdminUserProblem(ctx, http.StatusInternalServerError, "internal_error", "The users could not be listed.", mapErr)
			return
		}
		items = append(items, mapped)
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adminapi.AdminUserListResponse{Items: items, Page: adminapi.PageInfo{Page: page.Page, PerPage: page.PerPage, TotalItems: page.TotalItems, TotalPages: page.TotalPages}})
}

func (handler *AdminUserHandler) Get(ctx *gin.Context) {
	actor, ok := authenticatedAdminUser(ctx)
	if !ok {
		return
	}
	userID := strings.TrimSpace(ctx.Param("user_id"))
	if userID == "" {
		writeAdminUserProblem(ctx, http.StatusBadRequest, "invalid_request", "The user_id path parameter is required.", nil)
		return
	}
	detail, err := handler.service.Detail(ctx.Request.Context(), actor, userID)
	if err != nil {
		writeAdminUserError(ctx, err, "The user could not be loaded.")
		return
	}
	user, mapErr := adminUserResponse(detail.User)
	if mapErr != nil {
		writeAdminUserProblem(ctx, http.StatusInternalServerError, "internal_error", "The user could not be loaded.", mapErr)
		return
	}
	memberships := make([]adminapi.AdminUserMembershipSummary, 0, len(detail.Memberships))
	for _, membership := range detail.Memberships {
		if membership.ID == "" || membership.AccountID == "" || membership.AccountSlug == "" {
			writeAdminUserProblem(ctx, http.StatusInternalServerError, "internal_error", "The user could not be loaded.", errors.New("invalid membership projection"))
			return
		}
		memberships = append(memberships, adminapi.AdminUserMembershipSummary{Id: membership.ID, AccountId: membership.AccountID, AccountSlug: membership.AccountSlug, Status: adminapi.AdminUserMembershipSummaryStatus(membership.Status), AuthorizationVersion: int(membership.AuthorizationVersion), CreatedAt: membership.CreatedAt, UpdatedAt: membership.UpdatedAt})
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adminapi.AdminUserDetailResponse{User: user, Memberships: memberships})
}

func (handler *AdminUserHandler) UpdateStatus(ctx *gin.Context) {
	actor, ok := authenticatedAdminUser(ctx)
	if !ok {
		return
	}
	userID := strings.TrimSpace(ctx.Param("user_id"))
	if userID == "" {
		writeAdminUserProblem(ctx, http.StatusBadRequest, "invalid_request", "The user_id path parameter is required.", nil)
		return
	}
	var request adminapi.AdminUserStatusRequest
	if err := decodeAdminUserJSON(ctx, &request); err != nil || request.ExpectedVersion < 1 || !request.Status.Valid() {
		writeAdminUserProblem(ctx, http.StatusBadRequest, "invalid_request", "The user status request is invalid.", err)
		return
	}
	user, err := handler.service.UpdateStatus(ctx.Request.Context(), identityapp.UpdateAdminUserStatusCommand{ActorUserID: actor, UserID: userID, RequestID: platformmiddleware.RequestIDFromContext(ctx.Request.Context()), ExpectedVersion: int64(request.ExpectedVersion), Status: identitydomain.UserStatus(request.Status)})
	if err != nil {
		writeAdminUserError(ctx, err, "The user status could not be changed.")
		return
	}
	body, mapErr := adminUserResponse(user)
	if mapErr != nil {
		writeAdminUserProblem(ctx, http.StatusInternalServerError, "internal_error", "The user status could not be changed.", mapErr)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, body)
}

func authenticatedAdminUser(ctx *gin.Context) (string, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAuthenticated || session.UserID == nil {
		writeAdminUserProblem(ctx, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
		return "", false
	}
	return *session.UserID, true
}

func parseAdminUserFilter(ctx *gin.Context) (ports.AdminUserListFilter, error) {
	allowed := map[string]bool{"status": true, "email": true, "page": true, "per_page": true}
	for key, values := range ctx.Request.URL.Query() {
		if !allowed[key] || len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			return ports.AdminUserListFilter{}, errors.New("invalid query parameter: " + key)
		}
	}
	filter := ports.AdminUserListFilter{Email: ctx.Query("email")}
	if value, present := ctx.GetQuery("status"); present {
		if value != "active" && value != "disabled" {
			return filter, errors.New("invalid status")
		}
		filter.Status = identitydomain.UserStatus(value)
	}
	page, perPage, err := parseAdminPage(ctx)
	if err != nil {
		return filter, err
	}
	filter.Page = page
	filter.PerPage = perPage
	return filter, nil
}

func parseAdminPage(ctx *gin.Context) (int, int, error) {
	page, err := positiveQuery(ctx, "page", 1)
	if err != nil {
		return 0, 0, err
	}
	perPage, err := positiveQuery(ctx, "per_page", 25)
	if err != nil {
		return 0, 0, err
	}
	if perPage > 100 {
		return 0, 0, errors.New("per_page exceeds the maximum")
	}
	return page, perPage, nil
}
func positiveQuery(ctx *gin.Context, key string, fallback int) (int, error) {
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

func adminUserResponse(user identitydomain.User) (adminapi.AdminUserResponse, error) {
	if user.ID == "" || user.Email == "" || !validAdminUserStatus(user.Status) || user.AuthorizationVersion < 1 || user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() || user.UpdatedAt.Before(user.CreatedAt) {
		return adminapi.AdminUserResponse{}, errors.New("invalid user projection")
	}
	return adminapi.AdminUserResponse{Id: user.ID, Email: openapi_types.Email(user.Email), Status: adminapi.AdminUserResponseStatus(user.Status), AuthorizationVersion: int(user.AuthorizationVersion), CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}, nil
}
func adminUserListItem(item ports.AdminUserSummary) (adminapi.AdminUserListItem, error) {
	user, err := adminUserResponse(item.User)
	if err != nil {
		return adminapi.AdminUserListItem{}, err
	}
	return adminapi.AdminUserListItem{Id: user.Id, Email: user.Email, Status: adminapi.AdminUserListItemStatus(item.User.Status), AuthorizationVersion: user.AuthorizationVersion, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt, MembershipCount: item.MembershipCount, ActiveMembershipCount: item.ActiveMembershipCount}, nil
}
func validAdminUserStatus(status identitydomain.UserStatus) bool {
	return status == identitydomain.UserStatusActive || status == identitydomain.UserStatusDisabled
}
func decodeAdminUserJSON(ctx *gin.Context, destination any) error {
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
func writeAdminUserError(ctx *gin.Context, err error, message string) {
	switch {
	case errors.Is(err, identityapp.ErrAdminUserDenied):
		writeAdminUserProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", nil)
	case errors.Is(err, identityapp.ErrInvalidAdminUserCommand), errors.Is(err, ports.ErrInvalidAdminUserQuery):
		writeAdminUserProblem(ctx, http.StatusBadRequest, "invalid_request", "The user request is invalid.", nil)
	case errors.Is(err, ports.ErrUserNotFound):
		writeAdminUserProblem(ctx, http.StatusNotFound, "not_found", "The user was not found.", nil)
	case errors.Is(err, ports.ErrAdminUserConflict):
		writeAdminUserProblem(ctx, http.StatusConflict, "conflict", "The user changed before the request completed.", nil)
	default:
		writeAdminUserProblem(ctx, http.StatusInternalServerError, "internal_error", message, err)
	}
}
func writeAdminUserProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(ctx, status, platformmiddleware.RequestIDFromContext(ctx.Request.Context()), response.NewProblem(code, message, nil, cause))
}
