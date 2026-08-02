package app

import (
	"testing"

	"github.com/gin-gonic/gin"
	mediahttp "github.com/korwlik23/go-api-starter/internal/modules/media/transport/http"
	"github.com/stretchr/testify/require"
)

func TestM3B3MediaRoutesApplyIdentityBoundaryAndExposeContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noOp := func(ctx *gin.Context) { ctx.Next() }
	RegisterMediaRoutes(router.Group("/api/v1"), &mediahttp.UploadHandler{}, &mediahttp.AssetHandler{}, identityRuntime{authenticate: noOp, csrf: noOp})
	require.ElementsMatch(t, []string{
		"POST /api/v1/media/uploads",
		"POST /api/v1/media/uploads/:upload_id/finalize",
		"GET /api/v1/media/assets",
		"DELETE /api/v1/media/assets/:asset_id",
	}, routeKeys(router))
}

func TestM3B3DisabledMediaRoutesAreNotRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterMediaRoutes(router.Group("/api/v1"), nil, nil, identityRuntime{})
	require.Empty(t, router.Routes())
}
