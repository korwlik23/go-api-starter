package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	accountports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	redirectsapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/redirects"
	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"github.com/stretchr/testify/require"
)

func TestM3B5RedirectHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	handler, err := NewRedirectHandler(nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidRedirectHandler)
}

func TestM3B5RedirectHandlerRejectsUnauthenticatedList(t *testing.T) {
	t.Parallel()
	handler, err := NewRedirectHandler(&redirectManagerFake{}, &discoverabilityAccountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/discoverability/redirects", handler.List)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/discoverability/redirects", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

type redirectManagerFake struct{}

func (*redirectManagerFake) List(context.Context, redirectsapp.ListInput) (ports.RedirectListPage, error) {
	return ports.RedirectListPage{}, nil
}
func (*redirectManagerFake) Get(context.Context, string, string) (discoverabilitydomain.Redirect, error) {
	return discoverabilitydomain.Redirect{}, nil
}
func (*redirectManagerFake) Create(context.Context, redirectsapp.CreateInput) (discoverabilitydomain.Redirect, error) {
	return discoverabilitydomain.Redirect{}, nil
}
func (*redirectManagerFake) Update(context.Context, redirectsapp.UpdateInput) (discoverabilitydomain.Redirect, error) {
	return discoverabilitydomain.Redirect{}, nil
}
func (*redirectManagerFake) Delete(context.Context, redirectsapp.DeleteInput) error { return nil }

type discoverabilityAccountResolverFake struct{}

func (*discoverabilityAccountResolverFake) Execute(context.Context, string) (accountports.ResolvedAccount, error) {
	return accountports.ResolvedAccount{}, nil
}
