package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidRoleHandler = errors.New("invalid role handler dependencies")

const (
	roleDefaultPage    = 1
	roleDefaultPerPage = 25
	roleMaxPerPage     = 100
)

type roleQueryService interface {
	ListRoles(context.Context, ports.RoleListFilter) (ports.RolePage, error)
	ListRolePermissions(context.Context, string) (ports.RolePermissionList, error)
	ListRoleAssignments(context.Context, string, ports.RolePageRequest) (ports.RoleAssignmentPage, error)
}

type roleManagementService interface {
	Create(context.Context, authorizationapp.CreateRoleCommand) (authorizationdomain.Role, error)
	Update(context.Context, authorizationapp.UpdateRoleCommand) (authorizationdomain.Role, error)
}

type RoleHandler struct {
	query      roleQueryService
	management roleManagementService
}

func NewRoleHandler(query roleQueryService, management roleManagementService) (*RoleHandler, error) {
	if query == nil || management == nil {
		return nil, ErrInvalidRoleHandler
	}
	return &RoleHandler{query: query, management: management}, nil
}

func (handler *RoleHandler) ListRoles(ctx *gin.Context) {
	if _, ok := roleAssignmentSession(ctx); !ok {
		return
	}
	filter, err := parseRoleListFilter(ctx)
	if err != nil {
		writeRoleProblem(ctx, http.StatusBadRequest, "invalid_request", "The role list request is invalid.", err)
		return
	}
	page, err := handler.query.ListRoles(ctx.Request.Context(), filter)
	if err != nil {
		writeRoleQueryError(ctx, err, "The roles could not be listed.")
		return
	}
	items := make([]adminapi.RoleResponse, 0, len(page.Items))
	for _, role := range page.Items {
		item, mapErr := roleResponse(role)
		if mapErr != nil {
			writeRoleProblem(ctx, http.StatusInternalServerError, "internal_error", "The roles could not be listed.", mapErr)
			return
		}
		items = append(items, item)
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adminapi.RoleListResponse{Items: items, Page: pageInfoResponse(page.Page)})
}

func (handler *RoleHandler) CreateRole(ctx *gin.Context) {
	session, ok := roleAssignmentSession(ctx)
	if !ok {
		return
	}
	var request adminapi.CreateRoleRequest
	if err := decodeGrantStrictJSON(ctx, &request); err != nil ||
		request.Scope != adminapi.CreateRoleRequestScopeAccount ||
		strings.TrimSpace(request.AccountId) == "" || strings.TrimSpace(request.Name) == "" {
		writeRoleProblem(ctx, http.StatusBadRequest, "invalid_request", "The role creation request is invalid.", err)
		return
	}
	role, err := handler.management.Create(ctx.Request.Context(), authorizationapp.CreateRoleCommand{
		ActorUserID: *session.UserID,
		AccountID:   strings.TrimSpace(request.AccountId),
		Name:        request.Name,
		RequestID:   platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
	})
	if err != nil {
		writeRoleManagementError(ctx, err, "The role could not be created.")
		return
	}
	body, err := roleResponse(role)
	if err != nil {
		writeRoleProblem(ctx, http.StatusInternalServerError, "internal_error", "The role could not be created.", err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusCreated, body)
}

func (handler *RoleHandler) UpdateRole(ctx *gin.Context) {
	session, ok := roleAssignmentSession(ctx)
	if !ok {
		return
	}
	roleID := strings.TrimSpace(ctx.Param("role_id"))
	if roleID == "" {
		writeRoleProblem(ctx, http.StatusBadRequest, "invalid_request", "The role_id path parameter is required.", nil)
		return
	}
	var request adminapi.UpdateRoleRequest
	if err := decodeGrantStrictJSON(ctx, &request); err != nil ||
		request.ExpectedVersion < 1 || strings.TrimSpace(request.Name) == "" {
		writeRoleProblem(ctx, http.StatusBadRequest, "invalid_request", "The role update request is invalid.", err)
		return
	}
	accountID := ""
	if resolved, resolvedOK := ResolvedAccountFromContext(ctx.Request.Context()); resolvedOK {
		accountID = resolved.Account.ID
	}
	role, err := handler.management.Update(ctx.Request.Context(), authorizationapp.UpdateRoleCommand{
		ActorUserID:     *session.UserID,
		AccountID:       accountID,
		RoleID:          roleID,
		Name:            request.Name,
		RequestID:       platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		ExpectedVersion: int64(request.ExpectedVersion),
	})
	if err != nil {
		writeRoleManagementError(ctx, err, "The role could not be updated.")
		return
	}
	body, err := roleResponse(role)
	if err != nil {
		writeRoleProblem(ctx, http.StatusInternalServerError, "internal_error", "The role could not be updated.", err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, body)
}

func (handler *RoleHandler) ListRolePermissions(ctx *gin.Context) {
	if _, ok := roleAssignmentSession(ctx); !ok {
		return
	}
	roleID := strings.TrimSpace(ctx.Param("role_id"))
	if roleID == "" {
		writeRoleProblem(ctx, http.StatusBadRequest, "invalid_request", "The role_id path parameter is required.", nil)
		return
	}
	permissions, err := handler.query.ListRolePermissions(ctx.Request.Context(), roleID)
	if err != nil {
		writeRoleQueryError(ctx, err, "The role permissions could not be listed.")
		return
	}
	roleVersion, ok := boundedInt(permissions.RoleVersion)
	if permissions.RoleID == "" || permissions.RoleID != roleID || !ok {
		writeRoleProblem(ctx, http.StatusInternalServerError, "internal_error", "The role permissions could not be listed.", errors.New("invalid role permission projection"))
		return
	}
	items := make([]adminapi.RolePermissionItem, 0, len(permissions.Items))
	for _, item := range permissions.Items {
		if _, err := authorizationdomain.ParsePermissionKey(item.PermissionKey); err != nil || !adminapi.RolePermissionItemTier(item.Tier).Valid() {
			writeRoleProblem(ctx, http.StatusInternalServerError, "internal_error", "The role permissions could not be listed.", errors.New("invalid role permission projection"))
			return
		}
		items = append(items, adminapi.RolePermissionItem{
			PermissionKey: item.PermissionKey,
			Active:        item.Active,
			Delegable:     item.Delegable,
			Tier:          adminapi.RolePermissionItemTier(item.Tier),
		})
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adminapi.RolePermissionListResponse{
		RoleId: roleID, RoleVersion: roleVersion, Items: items,
	})
}

func (handler *RoleHandler) ListRoleAssignments(ctx *gin.Context) {
	if _, ok := roleAssignmentSession(ctx); !ok {
		return
	}
	roleID := strings.TrimSpace(ctx.Param("role_id"))
	if roleID == "" {
		writeRoleProblem(ctx, http.StatusBadRequest, "invalid_request", "The role_id path parameter is required.", nil)
		return
	}
	if err := rejectUnknownRoleQueryKeys(ctx, map[string]bool{"page": true, "per_page": true}); err != nil {
		writeRoleProblem(ctx, http.StatusBadRequest, "invalid_request", "The role assignment list request is invalid.", err)
		return
	}
	request, err := parseRolePageRequest(ctx)
	if err != nil {
		writeRoleProblem(ctx, http.StatusBadRequest, "invalid_request", "The role assignment list request is invalid.", err)
		return
	}
	page, err := handler.query.ListRoleAssignments(ctx.Request.Context(), roleID, request)
	if err != nil {
		writeRoleQueryError(ctx, err, "The role assignments could not be listed.")
		return
	}
	items := make([]adminapi.RoleAssignmentItem, 0, len(page.Items))
	for _, assignment := range page.Items {
		item, mapErr := roleAssignmentResponse(assignment, roleID)
		if mapErr != nil {
			writeRoleProblem(ctx, http.StatusInternalServerError, "internal_error", "The role assignments could not be listed.", mapErr)
			return
		}
		items = append(items, item)
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, adminapi.RoleAssignmentListResponse{Items: items, Page: pageInfoResponse(page.Page)})
}

func parseRoleListFilter(ctx *gin.Context) (ports.RoleListFilter, error) {
	if err := rejectUnknownRoleQueryKeys(ctx, map[string]bool{
		"scope": true, "account_id": true, "page": true, "per_page": true,
	}); err != nil {
		return ports.RoleListFilter{}, err
	}
	request, err := parseRolePageRequest(ctx)
	if err != nil {
		return ports.RoleListFilter{}, err
	}
	filter := ports.RoleListFilter{RolePageRequest: request, AccountID: strings.TrimSpace(ctx.Query("account_id"))}
	if ctx.Query("account_id") != "" && filter.AccountID == "" {
		return ports.RoleListFilter{}, errors.New("account_id must not be blank")
	}
	scopeValue, scopePresent := ctx.GetQuery("scope")
	if scopePresent && strings.TrimSpace(scopeValue) == "" {
		return ports.RoleListFilter{}, errors.New("scope must not be blank")
	}
	scope := strings.TrimSpace(scopeValue)
	switch scope {
	case "":
	case string(authorizationdomain.RoleOwnerSystem):
		filter.RoleOwnerType = authorizationdomain.RoleOwnerSystem
	case string(authorizationdomain.RoleOwnerAccount):
		filter.RoleOwnerType = authorizationdomain.RoleOwnerAccount
	default:
		return ports.RoleListFilter{}, errors.New("unknown role scope")
	}
	return filter, nil
}

func parseRolePageRequest(ctx *gin.Context) (ports.RolePageRequest, error) {
	page, err := parsePositiveQueryInt(ctx, "page", roleDefaultPage)
	if err != nil {
		return ports.RolePageRequest{}, err
	}
	perPage, err := parsePositiveQueryInt(ctx, "per_page", roleDefaultPerPage)
	if err != nil {
		return ports.RolePageRequest{}, err
	}
	if perPage > roleMaxPerPage {
		return ports.RolePageRequest{}, errors.New("per_page exceeds the maximum")
	}
	return ports.RolePageRequest{Page: page, PerPage: perPage}, nil
}

func parsePositiveQueryInt(ctx *gin.Context, key string, fallback int) (int, error) {
	value, present := ctx.GetQuery(key)
	if !present {
		return fallback, nil
	}
	if strings.TrimSpace(value) == "" {
		return 0, errors.New(key + " must not be blank")
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errors.New(key + " must be a positive integer")
	}
	return parsed, nil
}

func rejectUnknownRoleQueryKeys(ctx *gin.Context, allowed map[string]bool) error {
	for key := range ctx.Request.URL.Query() {
		if !allowed[key] {
			return errors.New("unknown query parameter: " + key)
		}
		if len(ctx.Request.URL.Query()[key]) != 1 {
			return errors.New("query parameter must be provided once: " + key)
		}
	}
	return nil
}

func roleResponse(role authorizationdomain.Role) (adminapi.RoleResponse, error) {
	version, ok := boundedInt(role.Version)
	if role.ID == "" || !ok || strings.TrimSpace(role.Name) == "" || strings.TrimSpace(role.Name) != role.Name ||
		role.CreatedAt.IsZero() || role.UpdatedAt.IsZero() || role.UpdatedAt.Before(role.CreatedAt) {
		return adminapi.RoleResponse{}, errors.New("invalid role projection")
	}
	ownerType := adminapi.RoleResponseOwnerType(role.OwnerType)
	if !ownerType.Valid() {
		return adminapi.RoleResponse{}, errors.New("invalid role owner type")
	}
	var accountID *string
	if role.AccountID != "" {
		value := role.AccountID
		accountID = &value
	}
	if role.OwnerType == authorizationdomain.RoleOwnerAccount && accountID == nil {
		return adminapi.RoleResponse{}, errors.New("account role is missing account_id")
	}
	if role.OwnerType == authorizationdomain.RoleOwnerSystem && accountID != nil {
		return adminapi.RoleResponse{}, errors.New("system role must not have account_id")
	}
	return adminapi.RoleResponse{
		Id: role.ID, OwnerType: ownerType, AccountId: accountID, Name: role.Name,
		Version: version, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt,
	}, nil
}

func roleAssignmentResponse(assignment ports.RoleAssignmentView, roleID string) (adminapi.RoleAssignmentItem, error) {
	if assignment.AssignmentID == "" || assignment.RoleID == "" || assignment.RoleID != roleID ||
		assignment.UserID == "" || assignment.CreatedAt.IsZero() {
		return adminapi.RoleAssignmentItem{}, errors.New("invalid role assignment projection")
	}
	scope := adminapi.RoleAssignmentItemScope(assignment.Scope)
	if !scope.Valid() {
		return adminapi.RoleAssignmentItem{}, errors.New("invalid role assignment scope")
	}
	var accountID *string
	if assignment.AccountID != "" {
		value := assignment.AccountID
		accountID = &value
	}
	if scope == adminapi.RoleAssignmentItemScopeAccount && accountID == nil ||
		scope == adminapi.RoleAssignmentItemScopeSystem && accountID != nil {
		return adminapi.RoleAssignmentItem{}, errors.New("invalid role assignment account scope")
	}
	return adminapi.RoleAssignmentItem{
		AssignmentId: assignment.AssignmentID, RoleId: assignment.RoleID, Scope: scope,
		AccountId: accountID, UserId: assignment.UserID, CreatedAt: assignment.CreatedAt,
	}, nil
}

func pageInfoResponse(page ports.PageInfo) adminapi.PageInfo {
	return adminapi.PageInfo{Page: page.Page, PerPage: page.PerPage, TotalItems: page.TotalItems, TotalPages: page.TotalPages}
}

func boundedInt(value int64) (int, bool) {
	maxInt := int64(^uint(0) >> 1)
	return int(value), value >= 1 && value <= maxInt
}

func writeRoleManagementError(ctx *gin.Context, cause error, message string) {
	switch {
	case errors.Is(cause, authorizationapp.ErrRoleManagementDenied):
		writeRoleProblem(ctx, http.StatusForbidden, "permission_denied", "Permission is required.", nil)
	case errors.Is(cause, authorizationapp.ErrInvalidRoleManagementCommand):
		writeRoleProblem(ctx, http.StatusBadRequest, "invalid_request", "The role request is invalid.", nil)
	case errors.Is(cause, ports.ErrRoleNotFound):
		writeRoleProblem(ctx, http.StatusNotFound, "not_found", "The role was not found.", nil)
	case errors.Is(cause, ports.ErrRoleConflict):
		writeRoleProblem(ctx, http.StatusConflict, "conflict", "The role changed before the request completed.", nil)
	default:
		writeRoleProblem(ctx, http.StatusInternalServerError, "internal_error", message, cause)
	}
}

func writeRoleQueryError(ctx *gin.Context, cause error, message string) {
	switch {
	case errors.Is(cause, ports.ErrRoleNotFound):
		writeRoleProblem(ctx, http.StatusNotFound, "not_found", "The role was not found.", nil)
	case errors.Is(cause, ports.ErrInvalidRoleQuery):
		writeRoleProblem(ctx, http.StatusBadRequest, "invalid_request", "The role query is invalid.", nil)
	default:
		writeRoleProblem(ctx, http.StatusInternalServerError, "internal_error", message, cause)
	}
}

func writeRoleProblem(ctx *gin.Context, status int, code, message string, cause error) {
	response.WriteProblem(
		ctx,
		status,
		platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(code, message, nil, cause),
	)
}

var _ roleQueryService = (ports.RoleQueryRepository)(nil)
var _ roleManagementService = (*authorizationapp.RoleManagementService)(nil)
