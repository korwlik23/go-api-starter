package http

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestM3A5RegisterRoutesExposesDraftMutationContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/api/v1"), RouteHandlers{Content: &ContentHandler{}, Revisions: &RevisionHandler{}, Workflow: &WorkflowHandler{}, Schedules: &ScheduleHandler{}, Preview: &PreviewHandler{}})

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		"GET /api/v1/publishing/pages",
		"POST /api/v1/publishing/pages",
		"GET /api/v1/publishing/pages/:page_id",
		"PATCH /api/v1/publishing/pages/:page_id",
		"DELETE /api/v1/publishing/pages/:page_id",
		"GET /api/v1/publishing/posts",
		"POST /api/v1/publishing/posts",
		"GET /api/v1/publishing/posts/:post_id",
		"PATCH /api/v1/publishing/posts/:post_id",
		"DELETE /api/v1/publishing/posts/:post_id",
		"GET /api/v1/publishing/content/:content_id/revisions",
		"POST /api/v1/publishing/content/:content_id/revisions/:revision_id/rollback",
		"POST /api/v1/publishing/content/:content_id/workflow",
		"GET /api/v1/publishing/schedules",
		"POST /api/v1/publishing/schedules",
		"DELETE /api/v1/publishing/schedules/:schedule_id",
		"POST /api/v1/publishing/content/:content_id/preview",
	} {
		_, exists := routes[expected]
		require.True(t, exists, expected)
	}
}

func TestM3C1RegisterPublicRoutesExposesReadOnlyPublishedContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterPublicRoutes(router.Group("/api/v1"), &PublicHandler{})
	RegisterPreviewExchangeRoute(router.Group("/api/v1"), &PreviewHandler{})

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		"GET /api/v1/public/content/:locale/*path",
		"GET /api/v1/public/posts/:locale",
		"GET /api/v1/public/posts/:locale/:slug",
		"GET /api/v1/public/menus/:locale/:location",
		"POST /api/v1/public/previews/exchange",
	} {
		_, exists := routes[expected]
		require.True(t, exists, expected)
	}
}
