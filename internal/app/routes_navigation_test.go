package app

import (
	"testing"

	"github.com/gin-gonic/gin"
	navigationhttp "github.com/korwlik23/go-api-starter/internal/modules/navigation/transport/http"
	"github.com/stretchr/testify/require"
)

func TestM3B4NavigationRoutesApplyIdentityBoundaryAndExposeContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noOp := func(ctx *gin.Context) { ctx.Next() }
	RegisterNavigationRoutes(router.Group("/api/v1"), &navigationhttp.MenuHandler{}, identityRuntime{authenticate: noOp, csrf: noOp})
	require.ElementsMatch(t, []string{
		"GET /api/v1/navigation/menus",
		"POST /api/v1/navigation/menus",
		"GET /api/v1/navigation/menus/:menu_id",
		"PATCH /api/v1/navigation/menus/:menu_id",
		"DELETE /api/v1/navigation/menus/:menu_id",
		"POST /api/v1/navigation/menus/:menu_id/reorder",
	}, routeKeys(router))
}

func TestM3B4DisabledNavigationRoutesAreNotRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterNavigationRoutes(router.Group("/api/v1"), nil, identityRuntime{})
	require.Empty(t, router.Routes())
}
