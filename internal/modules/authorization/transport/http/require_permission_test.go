package http

import (
	"bytes"
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	accountsports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	authorizationapp "github.com/korwlik23/go-api-starter/internal/modules/authorization/application"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/stretchr/testify/require"
)

func TestRequirePermissionDefaultsToForbiddenWithoutGrant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	authentication := permissionTestAuthentication(t, now)
	key, err := authorizationdomain.ParsePermissionKey(
		"content.articles.read.any",
	)
	require.NoError(t, err)
	guard, err := RequirePermission(PermissionGuardDependencies{
		Accounts: accountResolverStub{resolved: accountsports.ResolvedAccount{
			Account: accountsdomain.Account{ID: "account-id"},
		}},
		Permissions: &permissionEvaluatorStub{allowed: false},
		Key:         key,
	})
	require.NoError(t, err)
	router := gin.New()
	router.Use(authentication)
	router.GET("/protected", guard, func(ctx *gin.Context) {
		ctx.Status(nethttp.StatusNoContent)
	})
	request := httptest.NewRequest(nethttp.MethodGet, "/protected", nil)
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "session-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusForbidden, response.Code)
	require.NotContains(t, response.Body.String(), "role")
}

func TestRequirePermissionAllowsOnlyEvaluatorGrantAndInjectsAccount(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	key, err := authorizationdomain.ParsePermissionKey(
		"content.articles.read.any",
	)
	require.NoError(t, err)
	evaluator := &permissionEvaluatorStub{allowed: true}
	guard, err := RequirePermission(PermissionGuardDependencies{
		Accounts: accountResolverStub{resolved: accountsports.ResolvedAccount{
			Account: accountsdomain.Account{ID: "account-id"},
		}},
		Permissions: evaluator,
		Key:         key,
	})
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, now))
	router.GET("/protected", guard, func(ctx *gin.Context) {
		resolved, ok := ResolvedAccountFromContext(ctx.Request.Context())
		require.True(t, ok)
		require.Equal(t, "account-id", resolved.Account.ID)
		ctx.Status(nethttp.StatusNoContent)
	})
	request := httptest.NewRequest(nethttp.MethodGet, "/protected", nil)
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "session-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusNoContent, response.Code)
	require.Equal(t, "account-id", evaluator.request.AccountID)
}

func permissionTestAuthentication(
	t *testing.T,
	now time.Time,
) gin.HandlerFunc {
	t.Helper()
	codec, err := platformcrypto.NewTokenCodec(
		bytes.Repeat([]byte{0x42}, 32),
		"identity.session.v1",
	)
	require.NoError(t, err)
	userID := "user-id"
	version := int64(7)
	middleware, err := platformmiddleware.NewSessionAuthentication(
		platformmiddleware.SessionAuthenticationConfig{
			CookieName: "session",
			Sessions: permissionSessionReaderStub{session: identitydomain.Session{
				TokenHash:            identitydomain.TokenHash(codec.Hash("session-token")),
				State:                identitydomain.SessionStateAuthenticated,
				UserID:               &userID,
				AuthorizationVersion: &version,
				IdleExpiresAt:        now.Add(time.Hour),
				AbsoluteExpiresAt:    now.Add(2 * time.Hour),
			}},
			AuthorizationVersions: permissionVersionReaderStub{version: version},
			SessionTokens:         codec,
			Now:                   func() time.Time { return now },
		},
	)
	require.NoError(t, err)
	return middleware
}

type accountResolverStub struct {
	resolved accountsports.ResolvedAccount
	err      error
}

func (stub accountResolverStub) Execute(
	context.Context,
	string,
) (accountsports.ResolvedAccount, error) {
	return stub.resolved, stub.err
}

type permissionEvaluatorStub struct {
	allowed bool
	request authorizationapp.PermissionRequest
	err     error
}

func (stub *permissionEvaluatorStub) Execute(
	_ context.Context,
	request authorizationapp.PermissionRequest,
) (bool, error) {
	stub.request = request
	return stub.allowed, stub.err
}

type permissionSessionReaderStub struct {
	session identitydomain.Session
}

func (stub permissionSessionReaderStub) FindByTokenHash(
	context.Context,
	identitydomain.TokenHash,
) (identitydomain.Session, error) {
	return stub.session, nil
}

type permissionVersionReaderStub struct {
	version int64
}

func (stub permissionVersionReaderStub) CurrentAuthorizationVersion(
	context.Context,
	string,
) (int64, error) {
	return stub.version, nil
}
