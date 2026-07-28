package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestA7(t *testing.T) {
	t.Parallel()

	internalError := errors.New("sql: private connection detail")
	router := gin.New()
	router.GET("/problem", func(ctx *gin.Context) {
		problem := NewProblem(
			"authorization.permission_denied",
			"Request could not be completed.",
			FieldErrors{
				"account_id": {"validation.not_allowed"},
			},
			internalError,
		)
		WriteProblem(ctx, http.StatusForbidden, "request-123", problem)
	})

	request := httptest.NewRequest(http.MethodGet, "/problem", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusForbidden, response.Code)
	require.Equal(t, "application/json; charset=utf-8", response.Header().Get("Content-Type"))
	require.NotContains(t, response.Body.String(), internalError.Error())

	var envelope struct {
		Error struct {
			Code      string      `json:"code"`
			Message   string      `json:"message"`
			Fields    FieldErrors `json:"fields"`
			RequestID string      `json:"request_id"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.Equal(t, "authorization.permission_denied", envelope.Error.Code)
	require.Equal(t, "Request could not be completed.", envelope.Error.Message)
	require.Equal(t, FieldErrors{
		"account_id": {"validation.not_allowed"},
	}, envelope.Error.Fields)
	require.Equal(t, "request-123", envelope.Error.RequestID)
}
