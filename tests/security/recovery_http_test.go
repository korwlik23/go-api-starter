package security_test

import (
	"bytes"
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	authapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/auth"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	identityhttp "github.com/korwlik23/go-api-starter/internal/modules/identity/transport/http"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/stretchr/testify/require"
)

func TestRecoveryHTTPReturnsGenericAcceptedResponse(t *testing.T) {
	t.Parallel()

	requester := &recoveryRequesterStub{}
	handler, err := identityhttp.NewRecoveryHandler(
		requester,
		&passwordResetterStub{},
		func(*gin.Context, string) {},
	)
	require.NoError(t, err)
	router := gin.New()
	router.POST("/api/v1/auth/password/recovery", handler.Request)
	request := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/auth/password/recovery",
		bytes.NewBufferString(`{"email":"missing@example.com"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusAccepted, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Contains(t, response.Body.String(), `"status":"accepted"`)
	require.NotContains(t, response.Body.String(), "missing@example.com")
	require.Equal(t, "missing@example.com", requester.email)
}

func TestResetPasswordHTTPRotatesAnonymousSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 10, 0, 0, 0, time.UTC)
	codec, err := platformcrypto.NewTokenCodec(
		bytes.Repeat([]byte{0x42}, 32),
		"identity.session.v1",
	)
	require.NoError(t, err)
	anonymousHash := identitydomain.TokenHash(codec.Hash("anonymous-token"))
	resetter := &passwordResetterStub{result: authapp.ResetPasswordResult{
		SessionToken: "rotated-session-token",
		CSRFToken:    "rotated-csrf-token",
	}}
	handler, err := identityhttp.NewRecoveryHandler(
		&recoveryRequesterStub{},
		resetter,
		func(ctx *gin.Context, token string) {
			ctx.Header("X-Test-Session", token)
		},
	)
	require.NoError(t, err)
	authentication, err := platformmiddleware.NewSessionAuthentication(
		platformmiddleware.SessionAuthenticationConfig{
			CookieName: "session",
			Sessions: loginSessionReaderStub{session: identitydomain.Session{
				ID:                "anonymous-session",
				TokenHash:         anonymousHash,
				State:             identitydomain.SessionStateAnonymous,
				IdleExpiresAt:     now.Add(time.Minute),
				AbsoluteExpiresAt: now.Add(time.Hour),
			}},
			AuthorizationVersions: authorizationVersionReaderForHTTPTest{},
			SessionTokens:         codec,
			Now:                   func() time.Time { return now },
		},
	)
	require.NoError(t, err)
	router := gin.New()
	router.Use(authentication)
	router.POST("/api/v1/auth/password/reset", handler.Reset)
	request := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/auth/password/reset",
		bytes.NewBufferString(
			`{"token":"token-id.raw-secret","new_password":"replacement password"}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "anonymous-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, "no-referrer", response.Header().Get("Referrer-Policy"))
	require.Equal(t, "rotated-session-token", response.Header().Get("X-Test-Session"))
	require.Contains(t, response.Body.String(), `"status":"password_reset"`)
	require.Contains(t, response.Body.String(), `"csrf_token":"rotated-csrf-token"`)
	require.Equal(t, anonymousHash, resetter.command.CurrentSessionHash)
}

type recoveryRequesterStub struct {
	email string
	err   error
}

func (stub *recoveryRequesterStub) Execute(_ context.Context, email string) error {
	stub.email = email
	return stub.err
}

type passwordResetterStub struct {
	result  authapp.ResetPasswordResult
	command authapp.ResetPasswordCommand
	err     error
}

func (stub *passwordResetterStub) Execute(
	_ context.Context,
	command authapp.ResetPasswordCommand,
) (authapp.ResetPasswordResult, error) {
	stub.command = command
	return stub.result, stub.err
}
