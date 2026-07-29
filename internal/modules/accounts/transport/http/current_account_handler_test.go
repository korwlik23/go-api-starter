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
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/stretchr/testify/require"
)

func TestCurrentAccountHandlerReturnsHiddenResolvedAccount(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	handler, err := NewCurrentAccountHandler(
		currentAccountResolverStub{resolved: accountsports.ResolvedAccount{
			Account: accountsdomain.Account{
				ID:   "account-id",
				Slug: "personal",
			},
		}},
	)
	require.NoError(t, err)
	router := gin.New()
	router.Use(currentAccountAuthentication(t, now))
	router.GET("/api/v1/accounts/current", handler.Get)
	request := httptest.NewRequest(
		nethttp.MethodGet,
		"/api/v1/accounts/current",
		nil,
	)
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "session-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.JSONEq(t, `{
		"id":"account-id",
		"name":"personal",
		"slug":"personal"
	}`, response.Body.String())
}

func currentAccountAuthentication(
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
			Sessions: currentAccountSessionReaderStub{
				session: identitydomain.Session{
					TokenHash: identitydomain.TokenHash(
						codec.Hash("session-token"),
					),
					State:                identitydomain.SessionStateAuthenticated,
					UserID:               &userID,
					AuthorizationVersion: &version,
					IdleExpiresAt:        now.Add(time.Hour),
					AbsoluteExpiresAt:    now.Add(2 * time.Hour),
				},
			},
			AuthorizationVersions: currentAccountVersionReaderStub{
				version: version,
			},
			SessionTokens: codec,
			Now:           func() time.Time { return now },
		},
	)
	require.NoError(t, err)
	return middleware
}

type currentAccountResolverStub struct {
	resolved accountsports.ResolvedAccount
	err      error
}

func (stub currentAccountResolverStub) Execute(
	context.Context,
	string,
) (accountsports.ResolvedAccount, error) {
	return stub.resolved, stub.err
}

type currentAccountSessionReaderStub struct {
	session identitydomain.Session
}

func (stub currentAccountSessionReaderStub) FindByTokenHash(
	context.Context,
	identitydomain.TokenHash,
) (identitydomain.Session, error) {
	return stub.session, nil
}

type currentAccountVersionReaderStub struct {
	version int64
}

func (stub currentAccountVersionReaderStub) CurrentAuthorizationVersion(
	context.Context,
	string,
) (int64, error) {
	return stub.version, nil
}
