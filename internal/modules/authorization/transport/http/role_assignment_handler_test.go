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

func TestRoleAssignmentHandlerAssignsSystemRoleAndMapsResult(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	service := &roleAssignmentServiceStub{assignResult: authorizationapp.RoleAssignmentResult{
		AssignmentID: "assignment-1", RoleID: "role-1", Version: 2, AffectedPrincipals: 1,
	}}
	handler, err := NewRoleAssignmentHandler(service)
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, now))
	router.POST("/roles/:role_id/assignments", handler.Assign)
	req := httptest.NewRequest(http.MethodPost, "/roles/role-1/assignments", strings.NewReader(
		`{"scope":"system","user_id":"target-user","expected_version":1}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code)
	var body adminapi.RoleAssignmentResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, "assignment-1", body.AssignmentId)
	require.Equal(t, "role-1", body.RoleId)
	require.Equal(t, 2, body.Version)
	require.Equal(t, 1, body.AffectedPrincipals)
	require.Equal(t, "user-id", service.assignCommand.GrantorUserID)
	require.Equal(t, authorizationapp.AssignmentScopeSystem, service.assignCommand.Scope)
	require.Equal(t, "role-1", service.assignCommand.RoleID)
	require.Equal(t, "target-user", service.assignCommand.TargetUserID)
}

func TestRoleAssignmentHandlerRevokesAndMapsResult(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	service := &roleAssignmentServiceStub{revokeResult: authorizationapp.RoleRevokeResult{
		AssignmentID: "assignment-1", RoleID: "role-1", Version: 3, Revoked: true, AffectedPrincipals: 1,
	}}
	handler, err := NewRoleAssignmentHandler(service)
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, now))
	router.DELETE("/assignments/:assignment_id", handler.Revoke)
	req := httptest.NewRequest(http.MethodDelete, "/assignments/assignment-1", strings.NewReader(
		`{"expected_version":2}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code)
	var body adminapi.RoleRevokeResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
	require.Equal(t, "assignment-1", body.AssignmentId)
	require.Equal(t, adminapi.RoleRevokeResponseRevokedTrue, body.Revoked)
	require.Equal(t, 3, body.Version)
	require.Equal(t, "assignment-1", service.revokeCommand.AssignmentID)
	require.Equal(t, int64(2), service.revokeCommand.ExpectedVersion)
}

func TestRoleAssignmentHandlerRejectsUnknownFields(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	service := &roleAssignmentServiceStub{}
	handler, err := NewRoleAssignmentHandler(service)
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, now))
	router.POST("/roles/:role_id/assignments", handler.Assign)
	req := httptest.NewRequest(http.MethodPost, "/roles/role-1/assignments", strings.NewReader(
		`{"scope":"system","user_id":"target-user","expected_version":1,"unexpected":true}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	require.Equal(t, http.StatusBadRequest, res.Code)
	require.False(t, service.assignCalled)
}

type roleAssignmentServiceStub struct {
	assignCalled  bool
	assignCommand authorizationapp.AssignRoleCommand
	assignResult  authorizationapp.RoleAssignmentResult
	assignErr     error
	revokeCalled  bool
	revokeCommand authorizationapp.RevokeRoleCommand
	revokeResult  authorizationapp.RoleRevokeResult
	revokeErr     error
}

func (stub *roleAssignmentServiceStub) Assign(
	_ context.Context,
	command authorizationapp.AssignRoleCommand,
) (authorizationapp.RoleAssignmentResult, error) {
	stub.assignCalled = true
	stub.assignCommand = command
	return stub.assignResult, stub.assignErr
}

func (stub *roleAssignmentServiceStub) Revoke(
	_ context.Context,
	command authorizationapp.RevokeRoleCommand,
) (authorizationapp.RoleRevokeResult, error) {
	stub.revokeCalled = true
	stub.revokeCommand = command
	return stub.revokeResult, stub.revokeErr
}
