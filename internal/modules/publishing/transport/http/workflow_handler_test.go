package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	workflowapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/workflow"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3A7WorkflowHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	handler, err := NewWorkflowHandler(nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidWorkflowHandler)
}

func TestM3A7WorkflowHandlerRejectsUnauthenticatedTransition(t *testing.T) {
	t.Parallel()

	handler, err := NewWorkflowHandler(&workflowManagerFake{}, &accountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/workflow", handler.Transition)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/workflow", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "authentication_required")
}

type workflowManagerFake struct{}

func (*workflowManagerFake) Transition(context.Context, workflowapp.TransitionInput) (ports.ContentDetail, error) {
	return ports.ContentDetail{}, nil
}
