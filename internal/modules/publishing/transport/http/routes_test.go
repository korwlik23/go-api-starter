package http

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestM3A5RegisterRoutesExposesDraftMutationContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/api/v1"), RouteHandlers{Content: &ContentHandler{}})

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		"POST /api/v1/publishing/pages",
		"PATCH /api/v1/publishing/pages/:page_id",
		"DELETE /api/v1/publishing/pages/:page_id",
		"POST /api/v1/publishing/posts",
		"PATCH /api/v1/publishing/posts/:post_id",
		"DELETE /api/v1/publishing/posts/:post_id",
	} {
		_, exists := routes[expected]
		require.True(t, exists, expected)
	}
}
