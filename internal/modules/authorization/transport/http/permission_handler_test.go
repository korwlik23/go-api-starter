package http

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	accountsports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	authorizationdomain "github.com/korwlik23/go-api-starter/internal/modules/authorization/domain"
	authorizationports "github.com/korwlik23/go-api-starter/internal/modules/authorization/ports"
	"github.com/stretchr/testify/require"
)

func TestPermissionHandlerReturnsSortedUniqueEffectiveKeys(t *testing.T) {
	now := time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	first, err := authorizationdomain.ParsePermissionKey(
		"content.articles.update.own",
	)
	require.NoError(t, err)
	second, err := authorizationdomain.ParsePermissionKey(
		"content.articles.read.any",
	)
	require.NoError(t, err)
	handler, err := NewPermissionHandler(
		accountResolverStub{resolved: accountsports.ResolvedAccount{
			Account: accountsdomain.Account{ID: "account-id"},
		}},
		effectiveGrantReaderStub{grants: []authorizationports.EffectiveGrant{
			{Key: first}, {Key: second}, {Key: first},
		}},
	)
	require.NoError(t, err)
	router := gin.New()
	router.Use(permissionTestAuthentication(t, now))
	router.GET("/api/v1/authorization/effective-permissions", handler.GetEffective)
	request := httptest.NewRequest(
		nethttp.MethodGet,
		"/api/v1/authorization/effective-permissions",
		nil,
	)
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "session-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.JSONEq(t, `{
		"account_id":"account-id",
		"permissions":[
			"content.articles.read.any",
			"content.articles.update.own"
		]
	}`, response.Body.String())
}

type effectiveGrantReaderStub struct {
	grants []authorizationports.EffectiveGrant
	err    error
}

func (stub effectiveGrantReaderStub) EffectiveGrants(
	context.Context,
	string,
	string,
) ([]authorizationports.EffectiveGrant, error) {
	return stub.grants, stub.err
}
