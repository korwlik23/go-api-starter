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

func TestLoginHTTPRotatesAnonymousSessionAndReturnsGeneratedResponse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 9, 0, 0, 0, time.UTC)
	codec, err := platformcrypto.NewTokenCodec(
		bytes.Repeat([]byte{0x42}, 32),
		"identity.session.v1",
	)
	require.NoError(t, err)
	anonymousHash := identitydomain.TokenHash(codec.Hash("anonymous-token"))
	executor := &loginExecutorStub{result: authapp.LoginResult{
		Status:       authapp.LoginStatusAuthenticated,
		SessionToken: "authenticated-token",
		CSRFToken:    "new-csrf-token",
	}}
	handler, err := identityhttp.NewLoginHandler(
		executor,
		func(ctx *gin.Context, token string) {
			ctx.Header("X-Test-Session", token)
		},
		func(*nethttp.Request) (string, error) { return "198.51.100.20", nil },
	)
	require.NoError(t, err)
	router := loginTestRouter(t, now, codec, anonymousHash, handler)
	request := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/auth/login",
		bytes.NewBufferString(`{"email":"person@example.com","password":"secret"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "anonymous-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, "authenticated-token", response.Header().Get("X-Test-Session"))
	require.Contains(t, response.Body.String(), `"status":"authenticated"`)
	require.Contains(t, response.Body.String(), `"csrf_token":"new-csrf-token"`)
	require.Equal(t, anonymousHash, executor.command.CurrentSessionHash)
	require.Equal(t, "198.51.100.20", executor.command.ClientIP)
}

func TestLoginHTTPRejectsMissingAnonymousSessionBeforeUseCase(t *testing.T) {
	t.Parallel()

	executor := &loginExecutorStub{}
	handler, err := identityhttp.NewLoginHandler(
		executor,
		func(*gin.Context, string) {},
		func(*nethttp.Request) (string, error) { return "198.51.100.20", nil },
	)
	require.NoError(t, err)
	router := gin.New()
	router.POST("/api/v1/auth/login", handler.Login)
	request := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/auth/login",
		bytes.NewBufferString(`{"email":"person@example.com","password":"secret"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusUnauthorized, response.Code)
	require.Zero(t, executor.calls)
}

func loginTestRouter(
	t *testing.T,
	now time.Time,
	codec *platformcrypto.TokenCodec,
	sessionHash identitydomain.TokenHash,
	handler *identityhttp.LoginHandler,
) *gin.Engine {
	t.Helper()
	authentication, err := platformmiddleware.NewSessionAuthentication(
		platformmiddleware.SessionAuthenticationConfig{
			CookieName: "session",
			Sessions: loginSessionReaderStub{session: identitydomain.Session{
				ID:                "anonymous-session",
				TokenHash:         sessionHash,
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
	router.POST("/api/v1/auth/login", handler.Login)
	return router
}

type loginExecutorStub struct {
	result  authapp.LoginResult
	err     error
	command authapp.LoginCommand
	calls   int
}

func (stub *loginExecutorStub) Execute(
	_ context.Context,
	command authapp.LoginCommand,
) (authapp.LoginResult, error) {
	stub.calls++
	stub.command = command
	return stub.result, stub.err
}

type loginSessionReaderStub struct {
	session identitydomain.Session
}

func (stub loginSessionReaderStub) FindByTokenHash(
	context.Context,
	identitydomain.TokenHash,
) (identitydomain.Session, error) {
	return stub.session, nil
}

type authorizationVersionReaderForHTTPTest struct{}

func (authorizationVersionReaderForHTTPTest) CurrentAuthorizationVersion(
	context.Context,
	string,
) (int64, error) {
	return 1, nil
}
