package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestA6(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		incomingID string
		expectedID string
	}{
		{
			name:       "uses valid incoming ID",
			incomingID: "request.client-123",
			expectedID: "request.client-123",
		},
		{
			name:       "generates ID when header is missing",
			expectedID: "request.generated-456",
		},
		{
			name:       "replaces invalid incoming ID",
			incomingID: "invalid request id",
			expectedID: "request.generated-456",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var observedID string
			router := gin.New()
			router.Use(RequestID(func() string {
				return "request.generated-456"
			}))
			router.GET("/probe", func(ctx *gin.Context) {
				observedID = RequestIDFromContext(ctx.Request.Context())
				ctx.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/probe", nil)
			if test.incomingID != "" {
				request.Header.Set(RequestIDHeader, test.incomingID)
			}
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusNoContent, response.Code)
			require.Equal(t, test.expectedID, response.Header().Get(RequestIDHeader))
			require.Equal(t, test.expectedID, observedID)
		})
	}
}
