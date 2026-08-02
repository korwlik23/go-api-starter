package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	schedulesapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/schedules"
	publishingdomain "github.com/korwlik23/go-api-starter/internal/modules/publishing/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3A8ScheduleHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	handler, err := NewScheduleHandler(nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidScheduleHandler)
}

func TestM3A8ScheduleHandlerRejectsUnauthenticatedCreate(t *testing.T) {
	t.Parallel()

	handler, err := NewScheduleHandler(&scheduleManagerFake{}, &accountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/schedules", handler.Create)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/schedules", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "authentication_required")
}

type scheduleManagerFake struct{}

func (*scheduleManagerFake) Create(context.Context, schedulesapp.CreateInput) (publishingdomain.PublicationSchedule, error) {
	return publishingdomain.PublicationSchedule{}, nil
}
func (*scheduleManagerFake) List(context.Context, schedulesapp.ListInput) (ports.SchedulePage, error) {
	return ports.SchedulePage{}, nil
}
func (*scheduleManagerFake) Cancel(context.Context, schedulesapp.CancelInput) error { return nil }
