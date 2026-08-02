package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	settingsapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/settings"
	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/stretchr/testify/require"
)

func TestM3B5SettingsHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	handler, err := NewSettingsHandler(nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidSettingsHandler)
}

func TestM3B5SettingsHandlerRejectsUnauthenticatedGet(t *testing.T) {
	t.Parallel()
	handler, err := NewSettingsHandler(&settingsManagerFake{}, &discoverabilityAccountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/discoverability/seo", handler.Get)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/discoverability/seo?locale_id=locale", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

type settingsManagerFake struct{}

func (*settingsManagerFake) Get(context.Context, string, string) (discoverabilitydomain.SEODefaults, error) {
	return discoverabilitydomain.SEODefaults{}, nil
}
func (*settingsManagerFake) Update(context.Context, settingsapp.UpdateInput) (discoverabilitydomain.SEODefaults, error) {
	return discoverabilitydomain.SEODefaults{}, nil
}
