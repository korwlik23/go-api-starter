package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/ratelimit"
	"github.com/stretchr/testify/require"
)

func TestA13(t *testing.T) {
	t.Parallel()

	t.Run("denials return a stable problem and rounded retry delay", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name               string
			decision           ratelimit.Decision
			expectedRetryAfter string
			expectedLogReason  string
		}{
			{
				name: "rate exceeded rounds up to a whole second",
				decision: ratelimit.Decision{
					Reason:     ratelimit.DenialRateExceeded,
					RetryAfter: 1500 * time.Millisecond,
				},
				expectedRetryAfter: "2",
				expectedLogReason:  "rate_exceeded",
			},
			{
				name: "capacity saturation has a minimum one second retry",
				decision: ratelimit.Decision{
					Reason: ratelimit.DenialCapacitySaturated,
				},
				expectedRetryAfter: "1",
				expectedLogReason:  "capacity_saturated",
			},
		}

		for _, test := range tests {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				decider := &stubRateLimitDecider{decision: test.decision}
				var logOutput bytes.Buffer
				handlerCalled := false
				router := gin.New()
				router.Use(RequestID(func() string {
					return "request.rate-limit-123"
				}))
				router.Use(RateLimit(newRateLimitTestOptions(
					t,
					decider,
					config.TrustedProxySet{},
					&logOutput,
				)))
				router.GET("/probe", func(ctx *gin.Context) {
					handlerCalled = true
					ctx.Status(http.StatusNoContent)
				})

				request := httptest.NewRequest(http.MethodGet, "/probe", nil)
				request.RemoteAddr = "198.51.100.10:49152"
				request.Header.Set("X-Forwarded-For", "203.0.113.77")
				request.Header.Set("Authorization", "Bearer private-auth-secret")
				request.Header.Set("Cookie", "session=private-cookie-secret")
				responseRecorder := httptest.NewRecorder()

				router.ServeHTTP(responseRecorder, request)

				require.Equal(t, http.StatusTooManyRequests, responseRecorder.Code)
				require.Equal(
					t,
					test.expectedRetryAfter,
					responseRecorder.Header().Get("Retry-After"),
				)
				require.Equal(
					t,
					"no-store",
					responseRecorder.Header().Get("Cache-Control"),
				)
				require.False(t, handlerCalled)
				require.Len(t, decider.calls, 1)
				require.Equal(t, "baseline", decider.calls[0].policy)
				require.NotEmpty(t, decider.calls[0].clientKey)
				require.NotContains(t, decider.calls[0].clientKey, "198.51.100.10")
				require.NotContains(t, decider.calls[0].clientKey, "203.0.113.77")
				require.NotContains(t, responseRecorder.Body.String(), "198.51.100.10")
				require.NotContains(t, responseRecorder.Body.String(), "203.0.113.77")
				require.NotContains(t, responseRecorder.Body.String(), "private-auth-secret")
				require.NotContains(t, responseRecorder.Body.String(), "private-cookie-secret")
				require.NotContains(t, logOutput.String(), "198.51.100.10")
				require.NotContains(t, logOutput.String(), "203.0.113.77")
				require.NotContains(t, logOutput.String(), "private-auth-secret")
				require.NotContains(t, logOutput.String(), "private-cookie-secret")
				require.NotContains(t, logOutput.String(), decider.calls[0].clientKey)
				require.Contains(t, logOutput.String(), `"policy":"baseline"`)
				require.Contains(
					t,
					logOutput.String(),
					`"request_id":"request.rate-limit-123"`,
				)
				require.Contains(
					t,
					logOutput.String(),
					`"reason":"`+test.expectedLogReason+`"`,
				)

				problem := decodeRateLimitProblem(t, responseRecorder)
				require.Equal(t, "rate_limit.exceeded", problem.Error.Code)
				require.Equal(t, "Request could not be completed.", problem.Error.Message)
				require.Empty(t, problem.Error.Fields)
				require.Equal(t, "request.rate-limit-123", problem.Error.RequestID)
			})
		}
	})

	t.Run("untrusted peers cannot spoof their client address", func(t *testing.T) {
		t.Parallel()

		trustedProxies, err := config.ParseTrustedProxyCIDRs([]string{"10.0.0.0/8"})
		require.NoError(t, err)
		decider := &stubRateLimitDecider{
			decision: ratelimit.Decision{Allowed: true},
		}
		router := newAllowedRateLimitRouter(t, decider, trustedProxies)

		spoofedRequest := httptest.NewRequest(http.MethodGet, "/probe", nil)
		spoofedRequest.RemoteAddr = "198.51.100.20:49152"
		spoofedRequest.Header.Set("X-Forwarded-For", "203.0.113.88")
		spoofedResponse := httptest.NewRecorder()
		router.ServeHTTP(spoofedResponse, spoofedRequest)

		directRequest := httptest.NewRequest(http.MethodGet, "/probe", nil)
		directRequest.RemoteAddr = "198.51.100.20:60000"
		directResponse := httptest.NewRecorder()
		router.ServeHTTP(directResponse, directRequest)

		require.Equal(t, http.StatusNoContent, spoofedResponse.Code)
		require.Equal(t, http.StatusNoContent, directResponse.Code)
		require.Len(t, decider.calls, 2)
		require.Equal(t, decider.calls[0].clientKey, decider.calls[1].clientKey)
	})

	t.Run("trusted proxy chains select the nearest untrusted address", func(t *testing.T) {
		t.Parallel()

		trustedProxies, err := config.ParseTrustedProxyCIDRs([]string{"10.0.0.0/8"})
		require.NoError(t, err)
		decider := &stubRateLimitDecider{
			decision: ratelimit.Decision{Allowed: true},
		}
		router := newAllowedRateLimitRouter(t, decider, trustedProxies)

		proxiedRequest := httptest.NewRequest(http.MethodGet, "/probe", nil)
		proxiedRequest.RemoteAddr = "10.0.0.2:49152"
		proxiedRequest.Header.Set(
			"X-Forwarded-For",
			"192.0.2.200, 198.51.100.40, 10.0.0.3",
		)
		proxiedResponse := httptest.NewRecorder()
		router.ServeHTTP(proxiedResponse, proxiedRequest)

		selectedAddressRequest := httptest.NewRequest(http.MethodGet, "/probe", nil)
		selectedAddressRequest.RemoteAddr = "198.51.100.40:60000"
		selectedAddressResponse := httptest.NewRecorder()
		router.ServeHTTP(selectedAddressResponse, selectedAddressRequest)

		spoofedAddressRequest := httptest.NewRequest(http.MethodGet, "/probe", nil)
		spoofedAddressRequest.RemoteAddr = "192.0.2.200:60000"
		spoofedAddressResponse := httptest.NewRecorder()
		router.ServeHTTP(spoofedAddressResponse, spoofedAddressRequest)

		require.Equal(t, http.StatusNoContent, proxiedResponse.Code)
		require.Equal(t, http.StatusNoContent, selectedAddressResponse.Code)
		require.Equal(t, http.StatusNoContent, spoofedAddressResponse.Code)
		require.Len(t, decider.calls, 3)
		require.Equal(t, decider.calls[0].clientKey, decider.calls[1].clientKey)
		require.NotEqual(t, decider.calls[0].clientKey, decider.calls[2].clientKey)
		for _, call := range decider.calls {
			require.NotContains(t, call.clientKey, "10.0.0.2")
			require.NotContains(t, call.clientKey, "198.51.100.40")
			require.NotContains(t, call.clientKey, "192.0.2.200")
		}
	})

	t.Run("malformed forwarding data falls back to the trusted peer", func(t *testing.T) {
		t.Parallel()

		trustedProxies, err := config.ParseTrustedProxyCIDRs([]string{"10.0.0.0/8"})
		require.NoError(t, err)
		decider := &stubRateLimitDecider{
			decision: ratelimit.Decision{Allowed: true},
		}
		router := newAllowedRateLimitRouter(t, decider, trustedProxies)

		malformedRequest := httptest.NewRequest(http.MethodGet, "/probe", nil)
		malformedRequest.RemoteAddr = "10.0.0.2:49152"
		malformedRequest.Header.Set(
			"X-Forwarded-For",
			"198.51.100.40, invalid-address",
		)
		malformedResponse := httptest.NewRecorder()
		router.ServeHTTP(malformedResponse, malformedRequest)

		peerRequest := httptest.NewRequest(http.MethodGet, "/probe", nil)
		peerRequest.RemoteAddr = "10.0.0.2:60000"
		peerResponse := httptest.NewRecorder()
		router.ServeHTTP(peerResponse, peerRequest)

		require.Equal(t, http.StatusNoContent, malformedResponse.Code)
		require.Equal(t, http.StatusNoContent, peerResponse.Code)
		require.Len(t, decider.calls, 2)
		require.Equal(t, decider.calls[0].clientKey, decider.calls[1].clientKey)
	})

	t.Run("internal failures return a generic problem without sensitive data", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			remoteAddr string
			decider    *stubRateLimitDecider
		}{
			{
				name:       "invalid server remote address",
				remoteAddr: "private-invalid-remote-address",
				decider: &stubRateLimitDecider{
					decision: ratelimit.Decision{Allowed: true},
				},
			},
			{
				name:       "limiter operational error",
				remoteAddr: "198.51.100.30:49152",
				decider: &stubRateLimitDecider{
					err: errors.New("private limiter secret"),
				},
			},
		}

		for _, test := range tests {
			test := test
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				var logOutput bytes.Buffer
				router := gin.New()
				router.Use(RequestID(func() string {
					return "request.internal-456"
				}))
				router.Use(RateLimit(newRateLimitTestOptions(
					t,
					test.decider,
					config.TrustedProxySet{},
					&logOutput,
				)))
				router.GET("/probe", func(ctx *gin.Context) {
					ctx.Status(http.StatusNoContent)
				})

				request := httptest.NewRequest(http.MethodGet, "/probe", nil)
				request.RemoteAddr = test.remoteAddr
				request.Header.Set("Authorization", "Bearer private-auth-secret")
				request.Header.Set("Cookie", "session=private-cookie-secret")
				responseRecorder := httptest.NewRecorder()
				router.ServeHTTP(responseRecorder, request)

				require.Equal(
					t,
					http.StatusInternalServerError,
					responseRecorder.Code,
				)
				require.NotContains(
					t,
					responseRecorder.Body.String(),
					"private",
				)
				require.NotContains(t, logOutput.String(), "private")
				require.NotContains(t, responseRecorder.Body.String(), "198.51.100.30")
				require.NotContains(t, logOutput.String(), "198.51.100.30")
				problem := decodeRateLimitProblem(t, responseRecorder)
				require.Equal(t, "system.internal_error", problem.Error.Code)
				require.Equal(t, "Request could not be completed.", problem.Error.Message)
				require.Empty(t, problem.Error.Fields)
				require.Equal(t, "request.internal-456", problem.Error.RequestID)
			})
		}
	})

	t.Run("client keyer rejects weak key material", func(t *testing.T) {
		t.Parallel()

		_, err := NewHMACClientKeyer(make([]byte, 31))
		require.ErrorIs(t, err, errClientKeySecretTooShort)
	})
}

type rateLimitCall struct {
	policy    string
	clientKey string
}

type stubRateLimitDecider struct {
	decision ratelimit.Decision
	err      error
	calls    []rateLimitCall
}

func (decider *stubRateLimitDecider) Allow(
	policyName string,
	clientKey string,
) (ratelimit.Decision, error) {
	decider.calls = append(decider.calls, rateLimitCall{
		policy:    policyName,
		clientKey: clientKey,
	})
	return decider.decision, decider.err
}

func newAllowedRateLimitRouter(
	t *testing.T,
	decider RateLimitDecider,
	trustedProxies config.TrustedProxySet,
) *gin.Engine {
	t.Helper()

	router := gin.New()
	router.Use(RateLimit(newRateLimitTestOptions(
		t,
		decider,
		trustedProxies,
		io.Discard,
	)))
	router.GET("/probe", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})
	return router
}

func newRateLimitTestOptions(
	t *testing.T,
	decider RateLimitDecider,
	trustedProxies config.TrustedProxySet,
	logOutput io.Writer,
) RateLimitOptions {
	t.Helper()

	clientKeyer, err := NewHMACClientKeyer(bytes.Repeat([]byte{0x5a}, 32))
	require.NoError(t, err)

	return RateLimitOptions{
		Policy:         "baseline",
		Limiter:        decider,
		TrustedProxies: trustedProxies,
		ClientKey:      clientKeyer,
		Logger: slog.New(slog.NewJSONHandler(
			logOutput,
			nil,
		)),
	}
}

func decodeRateLimitProblem(
	t *testing.T,
	responseRecorder *httptest.ResponseRecorder,
) struct {
	Error struct {
		Code      string              `json:"code"`
		Message   string              `json:"message"`
		Fields    map[string][]string `json:"fields"`
		RequestID string              `json:"request_id"`
	} `json:"error"`
} {
	t.Helper()

	var envelope struct {
		Error struct {
			Code      string              `json:"code"`
			Message   string              `json:"message"`
			Fields    map[string][]string `json:"fields"`
			RequestID string              `json:"request_id"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(responseRecorder.Body.Bytes(), &envelope))
	return envelope
}
