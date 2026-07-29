package middleware

import (
	"bytes"
	"context"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/stretchr/testify/require"
)

func TestSessionAuthenticationAddsActiveSessionToRequestContext(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 3, 0, 0, 0, time.UTC)
	userID := "018f1f64-7b2a-7c10-8bf2-3c40d5f67586"
	repository := &sessionAuthenticationReader{
		session: identitydomain.Session{
			ID:                "018f1f64-7b2a-7c10-8bf2-3c40d5f67585",
			State:             identitydomain.SessionStateAuthenticated,
			UserID:            &userID,
			IdleExpiresAt:     now.Add(time.Minute),
			AbsoluteExpiresAt: now.Add(time.Hour),
		},
	}
	router, codec := sessionAuthenticationTestRouter(t, now, repository)

	request := httptest.NewRequest(nethttp.MethodGet, "/session", nil)
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "raw-session-token"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusNoContent, response.Code)
	require.Equal(
		t,
		identitydomain.TokenHash(codec.Hash("raw-session-token")),
		repository.requestedHash,
	)
}

func TestSessionAuthenticationDoesNotReuseRevokedOrExpiredSession(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 3, 0, 0, 0, time.UTC)
	revokedAt := now.Add(-time.Second)
	testCases := []struct {
		name    string
		session identitydomain.Session
	}{
		{
			name: "revoked",
			session: identitydomain.Session{
				RevokedAt:         &revokedAt,
				IdleExpiresAt:     now.Add(time.Minute),
				AbsoluteExpiresAt: now.Add(time.Hour),
			},
		},
		{
			name: "idle expired",
			session: identitydomain.Session{
				IdleExpiresAt:     now,
				AbsoluteExpiresAt: now.Add(time.Hour),
			},
		},
		{
			name: "absolute expired",
			session: identitydomain.Session{
				IdleExpiresAt:     now.Add(time.Minute),
				AbsoluteExpiresAt: now,
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			router, _ := sessionAuthenticationTestRouter(
				t,
				now,
				&sessionAuthenticationReader{session: testCase.session},
			)
			request := httptest.NewRequest(nethttp.MethodGet, "/session", nil)
			request.AddCookie(&nethttp.Cookie{Name: "session", Value: "old-token"})
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, nethttp.StatusUnauthorized, response.Code)
			require.Contains(t, response.Body.String(), `"code":"authentication_required"`)
		})
	}
}

func TestSessionAuthenticationTreatsMissingOrUnknownCookieAsAnonymous(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 3, 0, 0, 0, time.UTC)
	testCases := []struct {
		name       string
		withCookie bool
	}{
		{name: "missing cookie"},
		{name: "unknown cookie", withCookie: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			repository := &sessionAuthenticationReader{err: ports.ErrSessionNotFound}
			router, _ := sessionAuthenticationTestRouter(t, now, repository)
			request := httptest.NewRequest(nethttp.MethodGet, "/anonymous", nil)
			if testCase.withCookie {
				request.AddCookie(&nethttp.Cookie{Name: "session", Value: "unknown"})
			}
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, nethttp.StatusNoContent, response.Code)
			if !testCase.withCookie {
				require.Zero(t, repository.calls)
			}
		})
	}
}

func TestSessionAuthenticationFailsClosedOnRepositoryFailure(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 30, 3, 0, 0, 0, time.UTC)
	router, _ := sessionAuthenticationTestRouter(t, now, &sessionAuthenticationReader{
		err: errors.New("private database failure"),
	})
	request := httptest.NewRequest(nethttp.MethodGet, "/anonymous", nil)
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "raw-session-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusInternalServerError, response.Code)
	require.Contains(t, response.Body.String(), `"code":"session_lookup_failed"`)
	require.NotContains(t, response.Body.String(), "private database failure")
	require.NotContains(t, response.Body.String(), "raw-session-token")
}

func sessionAuthenticationTestRouter(
	t *testing.T,
	now time.Time,
	repository *sessionAuthenticationReader,
) (*gin.Engine, *platformcrypto.TokenCodec) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	codec, err := platformcrypto.NewTokenCodec(
		bytes.Repeat([]byte{0x42}, 32),
		"identity.session.v1",
	)
	require.NoError(t, err)
	authentication, err := NewSessionAuthentication(SessionAuthenticationConfig{
		CookieName:    "session",
		Sessions:      repository,
		SessionTokens: codec,
		Now:           func() time.Time { return now },
	})
	require.NoError(t, err)

	router := gin.New()
	router.Use(RequestID(func() string { return "request-id" }))
	router.Use(authentication)
	router.GET("/anonymous", func(ctx *gin.Context) {
		ctx.Status(nethttp.StatusNoContent)
	})
	router.GET("/session", func(ctx *gin.Context) {
		if _, ok := SessionFromContext(ctx.Request.Context()); !ok {
			writeAuthenticationRequired(ctx)
			return
		}
		ctx.Status(nethttp.StatusNoContent)
	})
	return router, codec
}

type sessionAuthenticationReader struct {
	session       identitydomain.Session
	err           error
	requestedHash identitydomain.TokenHash
	calls         int
}

func (reader *sessionAuthenticationReader) FindByTokenHash(
	_ context.Context,
	hash identitydomain.TokenHash,
) (identitydomain.Session, error) {
	reader.calls++
	reader.requestedHash = hash
	return reader.session, reader.err
}
