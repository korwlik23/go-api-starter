package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	"github.com/stretchr/testify/require"
)

func TestGrantRolePermissionHandlerStrictlyDecodesAndMapsResult(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	service := &grantServiceStub{result: authorizationapp.GrantRolePermissionResult{
		RoleID: "role-1", PermissionKey: "content.items.read.system", Version: 2,
		AffectedPrincipals: 3,
	}}
	handler, err := NewGrantRolePermissionHandler(service)
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, now))
	router.POST("/api/v1/authorization/roles/:role_id/permissions", handler.Grant)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/authorization/roles/role-1/permissions", strings.NewReader(`{"expected_version":1,"permission_key":"content.items.read.system"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code)
	var body adminapi.PermissionGrantResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, "role-1", body.RoleId)
	require.Equal(t, 2, body.Version)
	require.Equal(t, 3, body.AffectedPrincipals)
	require.Equal(t, "user-id", service.command.GrantorUserID)
	require.Equal(t, int64(1), service.command.ExpectedVersion)
}

func TestGrantRolePermissionHandlerRejectsUnknownJSONFields(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	service := &grantServiceStub{}
	handler, err := NewGrantRolePermissionHandler(service)
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, now))
	router.POST("/roles/:role_id/permissions", handler.Grant)
	req := httptest.NewRequest(http.MethodPost, "/roles/role-1/permissions", strings.NewReader(`{"expected_version":1,"permission_key":"x.y.z.system","unexpected":true}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusBadRequest, res.Code)
	require.False(t, service.called)
}

type grantServiceStub struct {
	called  bool
	command authorizationapp.GrantRolePermissionCommand
	result  authorizationapp.GrantRolePermissionResult
	err     error
}

func (stub *grantServiceStub) Execute(_ context.Context, command authorizationapp.GrantRolePermissionCommand) (authorizationapp.GrantRolePermissionResult, error) {
	stub.called = true
	stub.command = command
	return stub.result, stub.err
}
