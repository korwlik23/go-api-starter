package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/stretchr/testify/require"
)

func TestRoleHandlerListsRolesWithBoundedFilters(t *testing.T) {
	query := &roleQueryFake{roles: ports.RolePage{
		Items: []authorizationdomain.Role{{
			ID: "role-1", OwnerType: authorizationdomain.RoleOwnerAccount,
			AccountID: "account-1", Name: "Editors", Version: 2,
			CreatedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		}},
		Page: ports.PageInfo{Page: 2, PerPage: 10, TotalItems: 11, TotalPages: 2},
	}}
	handler, err := NewRoleHandler(query, &roleManagementFake{})
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)))
	router.GET("/roles", handler.ListRoles)

	request := httptest.NewRequest(
		http.MethodGet,
		"/roles?scope=account&account_id=account-1&page=2&per_page=10",
		nil,
	)
	request.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, authorizationdomain.RoleOwnerAccount, query.filter.RoleOwnerType)
	require.Equal(t, "account-1", query.filter.AccountID)
	require.JSONEq(t, `{
      "items": [{
        "id": "role-1", "owner_type": "account", "account_id": "account-1",
        "name": "Editors", "version": 2,
        "created_at": "2026-08-01T00:00:00Z", "updated_at": "2026-08-01T00:00:00Z"
      }],
      "page": {"page": 2, "per_page": 10, "total_items": 11, "total_pages": 2}
    }`, response.Body.String())
}

func TestRoleHandlerRejectsUnknownCreateFields(t *testing.T) {
	handler, err := NewRoleHandler(&roleQueryFake{}, &roleManagementFake{})
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)))
	router.POST("/roles", handler.CreateRole)

	request := httptest.NewRequest(
		http.MethodPost,
		"/roles",
		strings.NewReader(`{"scope":"account","account_id":"account-1","name":"Editors","unexpected":true}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "invalid_request")
}

func TestRoleHandlerMapsRoleVersionConflict(t *testing.T) {
	management := &roleManagementFake{updateErr: ports.ErrRoleConflict}
	handler, err := NewRoleHandler(&roleQueryFake{}, management)
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)))
	router.PATCH("/roles/:role_id", handler.UpdateRole)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/roles/role-1",
		strings.NewReader(`{"name":"Reviewers","expected_version":2}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusConflict, response.Code)
	require.Contains(t, response.Body.String(), "conflict")
}

func TestRoleHandlerListsRolePermissionsWithVersion(t *testing.T) {
	query := &roleQueryFake{permissions: ports.RolePermissionList{
		RoleID: "role-1", RoleVersion: 4,
		Items: []ports.RolePermissionView{{
			PermissionKey: "authorization.roles.read.system", Active: true,
			Delegable: false, Tier: "core",
		}},
	}}
	handler, err := NewRoleHandler(query, &roleManagementFake{})
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)))
	router.GET("/roles/:role_id/permissions", handler.ListRolePermissions)

	request := httptest.NewRequest(http.MethodGet, "/roles/role-1/permissions", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{
      "role_id":"role-1", "role_version":4,
      "items":[{"permission_key":"authorization.roles.read.system","active":true,"delegable":false,"tier":"core"}]
    }`, response.Body.String())
}

func TestRoleHandlerListsRoleAssignmentsWithPagination(t *testing.T) {
	query := &roleQueryFake{assignments: ports.RoleAssignmentPage{
		Items: []ports.RoleAssignmentView{{
			AssignmentID: "assignment-1", RoleID: "role-1", Scope: "account",
			AccountID: "account-1", UserID: "user-1",
			CreatedAt: time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		}},
		Page: ports.PageInfo{Page: 2, PerPage: 10, TotalItems: 11, TotalPages: 2},
	}}
	handler, err := NewRoleHandler(query, &roleManagementFake{})
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)))
	router.GET("/roles/:role_id/assignments", handler.ListRoleAssignments)

	request := httptest.NewRequest(http.MethodGet, "/roles/role-1/assignments?page=2&per_page=10", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{
      "items":[{"assignment_id":"assignment-1","role_id":"role-1","scope":"account","account_id":"account-1","user_id":"user-1","created_at":"2026-08-01T00:00:00Z"}],
      "page":{"page":2,"per_page":10,"total_items":11,"total_pages":2}
    }`, response.Body.String())
}

type roleQueryFake struct {
	roles       ports.RolePage
	permissions ports.RolePermissionList
	assignments ports.RoleAssignmentPage
	filter      ports.RoleListFilter
	err         error
}

func (fake *roleQueryFake) ListRoles(_ context.Context, filter ports.RoleListFilter) (ports.RolePage, error) {
	fake.filter = filter
	return fake.roles, fake.err
}

func (fake *roleQueryFake) ListRolePermissions(context.Context, string) (ports.RolePermissionList, error) {
	return fake.permissions, fake.err
}

func (fake *roleQueryFake) ListRoleAssignments(context.Context, string, ports.RolePageRequest) (ports.RoleAssignmentPage, error) {
	return fake.assignments, fake.err
}

type roleManagementFake struct {
	createdRole authorizationdomain.Role
	updatedRole authorizationdomain.Role
	createErr   error
	updateErr   error
}

func (fake *roleManagementFake) Create(context.Context, authorizationapp.CreateRoleCommand) (authorizationdomain.Role, error) {
	return fake.createdRole, fake.createErr
}

func (fake *roleManagementFake) Update(context.Context, authorizationapp.UpdateRoleCommand) (authorizationdomain.Role, error) {
	return fake.updatedRole, fake.updateErr
}
