package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	accountports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	menusapp "github.com/korwlik23/go-api-starter/internal/modules/navigation/application/menus"
	navigationdomain "github.com/korwlik23/go-api-starter/internal/modules/navigation/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/navigation/ports"
	"github.com/stretchr/testify/require"
)

func TestM3B4MenuHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	handler, err := NewMenuHandler(nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidMenuHandler)
}

func TestM3B4MenuHandlerRejectsUnauthenticatedList(t *testing.T) {
	t.Parallel()

	handler, err := NewMenuHandler(&menuManagerFake{}, &navigationAccountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/navigation/menus", handler.List)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/navigation/menus", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "authentication_required")
}

func TestM3B4MenuHandlerRejectsDuplicateJSONKeys(t *testing.T) {
	t.Parallel()

	handler, err := NewMenuHandler(&menuManagerFake{}, &navigationAccountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/navigation/menus", handler.Create)
	request := httptest.NewRequest(http.MethodPost, "/navigation/menus", strings.NewReader(`{"locale_id":"018f1f64-7b2a-7c10-8bf2-3c40d5f67580","key":"main","key":"other","name":"Main"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

type menuManagerFake struct {
	createErr error
}

func (*menuManagerFake) List(context.Context, menusapp.ListInput) (ports.MenuListPage, error) {
	return ports.MenuListPage{}, nil
}
func (*menuManagerFake) Get(context.Context, string, string) (navigationdomain.Menu, error) {
	return navigationdomain.Menu{}, nil
}
func (fake *menuManagerFake) Create(context.Context, menusapp.CreateInput) (navigationdomain.Menu, error) {
	return navigationdomain.Menu{}, fake.createErr
}
func (*menuManagerFake) Update(context.Context, menusapp.UpdateInput) (navigationdomain.Menu, error) {
	return navigationdomain.Menu{}, nil
}
func (*menuManagerFake) Delete(context.Context, menusapp.DeleteInput) error { return nil }
func (*menuManagerFake) Reorder(context.Context, menusapp.ReorderInput) (navigationdomain.Menu, error) {
	return navigationdomain.Menu{}, nil
}

type navigationAccountResolverFake struct{}

func (*navigationAccountResolverFake) Execute(context.Context, string) (accountports.ResolvedAccount, error) {
	return accountports.ResolvedAccount{Account: accountsdomain.Account{ID: "018f1f64-7b2a-7c10-8bf2-3c40d5f67580"}}, nil
}
