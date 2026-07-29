package security_test

import (
	"bytes"
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	identityhttp "github.com/korwlik23/go-api-starter/internal/modules/identity/transport/http"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/stretchr/testify/require"
)

func TestLogoutHTTPRevokesServerSessionAndExpiresCookie(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	codec, err := platformcrypto.NewTokenCodec(
		bytes.Repeat([]byte{0x42}, 32),
		"identity.session.v1",
	)
	require.NoError(t, err)
	userID := "user-id"
	version := int64(7)
	authentication, err := platformmiddleware.NewSessionAuthentication(
		platformmiddleware.SessionAuthenticationConfig{
			CookieName: "session",
			Sessions: loginSessionReaderStub{session: identitydomain.Session{
				ID:                   "current-session",
				TokenHash:            identitydomain.TokenHash(codec.Hash("authenticated-token")),
				State:                identitydomain.SessionStateAuthenticated,
				UserID:               &userID,
				AuthorizationVersion: &version,
				IdleExpiresAt:        now.Add(time.Minute),
				AbsoluteExpiresAt:    now.Add(time.Hour),
			}},
			AuthorizationVersions: fixedAuthorizationVersionReader{version: version},
			SessionTokens:         codec,
			Now:                   func() time.Time { return now },
		},
	)
	require.NoError(t, err)
	revoker := &logoutRevokerStub{}
	handler, err := identityhttp.NewSessionHandler(
		revoker,
		func(ctx *gin.Context) (string, error) {
			return ctx.Cookie("session")
		},
		func(ctx *gin.Context) {
			ctx.Header("X-Test-Cookie-Cleared", "true")
		},
	)
	require.NoError(t, err)
	router := gin.New()
	router.Use(authentication)
	router.POST("/api/v1/auth/logout", handler.Logout)
	request := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "authenticated-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusNoContent, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, "true", response.Header().Get("X-Test-Cookie-Cleared"))
	require.Equal(t, "authenticated-token", revoker.rawToken)
	require.Equal(t, 1, revoker.calls)
}

type logoutRevokerStub struct {
	rawToken string
	calls    int
	err      error
}

func (stub *logoutRevokerStub) Execute(
	_ context.Context,
	rawToken string,
) error {
	stub.calls++
	stub.rawToken = rawToken
	return stub.err
}
