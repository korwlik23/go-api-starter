package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	accountsdomain "github.com/korwlik23/go-api-starter/internal/modules/accounts/domain"
	accountports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	contentapp "github.com/korwlik23/go-api-starter/internal/modules/publishing/application/content"
	"github.com/korwlik23/go-api-starter/internal/modules/publishing/ports"
	"github.com/stretchr/testify/require"
)

func TestM3A5ContentHandlerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()

	handler, err := NewContentHandler(nil, nil)
	require.Nil(t, handler)
	require.ErrorIs(t, err, ErrInvalidContentHandler)
}

func TestM3A5ContentHandlerRejectsUnauthenticatedMutation(t *testing.T) {
	t.Parallel()

	handler, err := NewContentHandler(&contentManagerFake{}, &accountResolverFake{})
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/content", handler.Create)
	response := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/content", nil)
	router.ServeHTTP(response, request)
	require.Equal(t, 401, response.Code)
	require.Contains(t, response.Body.String(), "authentication_required")
}

func TestM3A5StrictDecoderRejectsDuplicateKeys(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/content", bytes.NewBufferString(`{"title":"one","title":"two"}`))
	context, engine := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	_ = engine
	var destination struct {
		Title string `json:"title"`
	}
	require.Error(t, decodeStrictJSON(context, &destination))
}

func TestM3A5ContentMutationRequiresJSONMediaType(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/content", nil)
	require.Error(t, requireJSONContentType(context))
	context.Request.Header.Set("Content-Type", "application/json; charset=utf-8")
	require.NoError(t, requireJSONContentType(context))
}

type contentManagerFake struct{}

type accountResolverFake struct{}

func (*contentManagerFake) Create(
	context.Context,
	contentapp.CreateInput,
) (ports.ContentDetail, error) {
	return ports.ContentDetail{}, nil
}

func (*contentManagerFake) Update(
	context.Context,
	contentapp.UpdateInput,
) (ports.ContentDetail, error) {
	return ports.ContentDetail{}, nil
}

func (*contentManagerFake) Delete(context.Context, contentapp.DeleteInput) error {
	return nil
}

func (*accountResolverFake) Execute(
	context.Context,
	string,
) (accountports.ResolvedAccount, error) {
	return accountports.ResolvedAccount{
		Account: accountsdomain.Account{ID: "account-id"},
	}, nil
}
