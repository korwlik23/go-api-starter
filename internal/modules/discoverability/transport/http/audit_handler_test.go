package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	auditapp "github.com/korwlik23/go-api-starter/internal/modules/discoverability/application/audit"
	discoverabilitydomain "github.com/korwlik23/go-api-starter/internal/modules/discoverability/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/discoverability/ports"
	"github.com/stretchr/testify/require"
)

func TestM3B6AuditHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	handler, err := NewAuditHandler(nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidAuditHandler)
}

func TestM3B6AuditHandlerRejectsUnauthenticatedRun(t *testing.T) {
	t.Parallel()
	handler, err := NewAuditHandler(&auditManagerFake{}, &discoverabilityAccountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/discoverability/audits", handler.Run)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/discoverability/audits", nil))
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

type auditManagerFake struct{}

func (*auditManagerFake) Run(context.Context, auditapp.RunInput) (discoverabilitydomain.ContentAudit, error) {
	return discoverabilitydomain.ContentAudit{}, nil
}
func (*auditManagerFake) Get(context.Context, string, string) (discoverabilitydomain.ContentAudit, error) {
	return discoverabilitydomain.ContentAudit{}, nil
}
func (*auditManagerFake) List(context.Context, auditapp.ListInput) (ports.ContentAuditListPage, error) {
	return ports.ContentAuditListPage{}, nil
}
