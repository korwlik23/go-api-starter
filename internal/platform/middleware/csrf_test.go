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
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/stretchr/testify/require"
)

func TestCSRFRejectsMissingOrIncorrectMutationTokens(t *testing.T) {
	sessionCodec, csrfCodec := csrfTestCodecs(t)
	sessionToken := "session-token"
	sessionHash := identitydomain.TokenHash(sessionCodec.Hash(sessionToken))
	repository := &csrfSessionReader{
		session: identitydomain.Session{
			TokenHash:     sessionHash,
			CSRFTokenHash: identitydomain.TokenHash(csrfCodec.Hash("correct-csrf")),
		},
	}
	router := csrfTestRouter(t, repository, sessionCodec, csrfCodec)

	testCases := []struct {
		name       string
		withCookie bool
		csrfToken  string
	}{
		{name: "missing cookie", csrfToken: "correct-csrf"},
		{name: "missing header", withCookie: true},
		{name: "incorrect header", withCookie: true, csrfToken: "wrong-csrf"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(nethttp.MethodPost, "/mutation", nil)
			request.Header.Set("Origin", "https://admin.example.com")
			if testCase.withCookie {
				request.AddCookie(&nethttp.Cookie{Name: "session", Value: sessionToken})
			}
			if testCase.csrfToken != "" {
				request.Header.Set(CSRFTokenHeader, testCase.csrfToken)
			}
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, nethttp.StatusForbidden, response.Code)
			require.Contains(t, response.Body.String(), `"code":"csrf_invalid"`)
			require.NotContains(t, response.Body.String(), sessionToken)
			if testCase.csrfToken != "" {
				require.NotContains(t, response.Body.String(), testCase.csrfToken)
			}
		})
	}
}

func TestCSRFAcceptsSessionBoundTokenAndHashesCookieBeforeLookup(t *testing.T) {
	sessionCodec, csrfCodec := csrfTestCodecs(t)
	sessionToken := "session-token"
	expectedHash := identitydomain.TokenHash(sessionCodec.Hash(sessionToken))
	repository := &csrfSessionReader{
		session: identitydomain.Session{
			TokenHash:     expectedHash,
			CSRFTokenHash: identitydomain.TokenHash(csrfCodec.Hash("correct-csrf")),
		},
	}
	router := csrfTestRouter(t, repository, sessionCodec, csrfCodec)

	request := httptest.NewRequest(nethttp.MethodPost, "/mutation", nil)
	request.Header.Set("Origin", "https://admin.example.com")
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: sessionToken})
	request.Header.Set(CSRFTokenHeader, "correct-csrf")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusNoContent, response.Code)
	require.Equal(t, expectedHash, repository.requestedHash)
}

func TestCSRFFailsClosedWhenSessionLookupFails(t *testing.T) {
	sessionCodec, csrfCodec := csrfTestCodecs(t)
	repository := &csrfSessionReader{err: errors.New("private database error")}
	router := csrfTestRouter(t, repository, sessionCodec, csrfCodec)

	request := httptest.NewRequest(nethttp.MethodPost, "/mutation", nil)
	request.Header.Set("Origin", "https://admin.example.com")
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "session-token"})
	request.Header.Set(CSRFTokenHeader, "csrf-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusForbidden, response.Code)
	require.NotContains(t, response.Body.String(), "private database error")
}

func TestCSRFRejectsRevokedSession(t *testing.T) {
	sessionCodec, csrfCodec := csrfTestCodecs(t)
	revokedAt := time.Date(2026, time.July, 30, 3, 0, 0, 0, time.UTC)
	repository := &csrfSessionReader{
		session: identitydomain.Session{
			CSRFTokenHash: identitydomain.TokenHash(csrfCodec.Hash("correct-csrf")),
			RevokedAt:     &revokedAt,
		},
	}
	router := csrfTestRouter(t, repository, sessionCodec, csrfCodec)

	request := httptest.NewRequest(nethttp.MethodPost, "/mutation", nil)
	request.Header.Set("Origin", "https://admin.example.com")
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "session-token"})
	request.Header.Set(CSRFTokenHeader, "correct-csrf")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusForbidden, response.Code)
}

func TestCSRFAllowsSafeMethodsWithoutSessionLookup(t *testing.T) {
	sessionCodec, csrfCodec := csrfTestCodecs(t)
	repository := &csrfSessionReader{}
	router := csrfTestRouter(t, repository, sessionCodec, csrfCodec)

	request := httptest.NewRequest(nethttp.MethodGet, "/mutation", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusNoContent, response.Code)
	require.Equal(t, 0, repository.calls)
}

func TestCSRFRequiresExactTrustedOriginOrReferer(t *testing.T) {
	sessionCodec, csrfCodec := csrfTestCodecs(t)
	repository := &csrfSessionReader{
		session: identitydomain.Session{
			CSRFTokenHash: identitydomain.TokenHash(csrfCodec.Hash("correct-csrf")),
		},
	}
	router := csrfTestRouter(t, repository, sessionCodec, csrfCodec)

	testCases := []struct {
		name     string
		origin   string
		referer  string
		expected int
	}{
		{
			name:     "exact origin",
			origin:   "https://admin.example.com",
			expected: nethttp.StatusNoContent,
		},
		{
			name:     "trusted referer fallback",
			referer:  "https://admin.example.com/settings/profile?tab=security",
			expected: nethttp.StatusNoContent,
		},
		{
			name:     "foreign origin",
			origin:   "https://evil.example.com",
			expected: nethttp.StatusForbidden,
		},
		{
			name:     "foreign referer",
			referer:  "https://evil.example.com/settings",
			expected: nethttp.StatusForbidden,
		},
		{
			name:     "malformed origin",
			origin:   "://broken",
			expected: nethttp.StatusForbidden,
		},
		{
			name:     "opaque origin",
			origin:   "null",
			expected: nethttp.StatusForbidden,
		},
		{
			name:     "missing browser source",
			expected: nethttp.StatusForbidden,
		},
		{
			name:     "foreign origin wins over trusted referer",
			origin:   "https://evil.example.com",
			referer:  "https://admin.example.com/settings",
			expected: nethttp.StatusForbidden,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(nethttp.MethodPost, "/mutation", nil)
			request.AddCookie(&nethttp.Cookie{Name: "session", Value: "session-token"})
			request.Header.Set(CSRFTokenHeader, "correct-csrf")
			if testCase.origin != "" {
				request.Header.Set("Origin", testCase.origin)
			}
			if testCase.referer != "" {
				request.Header.Set("Referer", testCase.referer)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, testCase.expected, response.Code)
		})
	}
}

func csrfTestRouter(
	t *testing.T,
	repository *csrfSessionReader,
	sessionCodec *platformcrypto.TokenCodec,
	csrfCodec *platformcrypto.TokenCodec,
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	guard, err := NewCSRF(CSRFConfig{
		TrustedOrigin: "https://admin.example.com",
		CookieName:    "session",
		Sessions:      repository,
		SessionTokens: sessionCodec,
		CSRFTokens:    csrfCodec,
	})
	require.NoError(t, err)

	router := gin.New()
	router.Use(RequestID(func() string { return "request-id" }))
	router.Use(guard)
	router.Any("/mutation", func(ctx *gin.Context) {
		ctx.Status(nethttp.StatusNoContent)
	})
	return router
}

func csrfTestCodecs(t *testing.T) (*platformcrypto.TokenCodec, *platformcrypto.TokenCodec) {
	t.Helper()
	sessionCodec, err := platformcrypto.NewTokenCodec(
		bytes.Repeat([]byte{0x42}, 32),
		"identity.session.v1",
	)
	require.NoError(t, err)
	csrfCodec, err := platformcrypto.NewTokenCodec(
		bytes.Repeat([]byte{0x24}, 32),
		"identity.csrf.v1",
	)
	require.NoError(t, err)
	return sessionCodec, csrfCodec
}

type csrfSessionReader struct {
	session       identitydomain.Session
	err           error
	requestedHash identitydomain.TokenHash
	calls         int
}

func (reader *csrfSessionReader) FindByTokenHash(
	_ context.Context,
	hash identitydomain.TokenHash,
) (identitydomain.Session, error) {
	reader.calls++
	reader.requestedHash = hash
	return reader.session, reader.err
}
