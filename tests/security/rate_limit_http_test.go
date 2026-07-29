package security_test

import (
	"bytes"
	"io"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	identityhttp "github.com/korwlik23/go-api-starter/internal/modules/identity/transport/http"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/ratelimit"
	"github.com/stretchr/testify/require"
)

func TestIdentityEndpointsUseBaselineAndStricterNamedPolicies(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name           string
		path           string
		stricterPolicy string
	}{
		{name: "login", path: "/api/v1/auth/login", stricterPolicy: "login"},
		{
			name:           "password recovery",
			path:           "/api/v1/auth/password/recovery",
			stricterPolicy: "recovery",
		},
		{
			name:           "MFA verification",
			path:           "/api/v1/auth/mfa/verify",
			stricterPolicy: "mfa",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			limiter := &policyDecider{
				decisions: map[string]ratelimit.Decision{
					"baseline": {Allowed: true},
					testCase.stricterPolicy: {
						Allowed:    false,
						Reason:     ratelimit.DenialRateExceeded,
						RetryAfter: time.Second,
					},
				},
			}
			limits := newIdentityRateLimits(t, limiter)
			handlerCalls := 0
			router := gin.New()
			router.Use(platformmiddleware.RequestID(func() string { return "request-id" }))
			api := router.Group("/api/v1", limits.Baseline)
			switch testCase.stricterPolicy {
			case "login":
				api.POST("/auth/login", limits.Login, countHandler(&handlerCalls))
			case "recovery":
				api.POST(
					"/auth/password/recovery",
					limits.Recovery,
					countHandler(&handlerCalls),
				)
			case "mfa":
				api.POST("/auth/mfa/verify", limits.MFA, countHandler(&handlerCalls))
			}
			request := httptest.NewRequest(nethttp.MethodPost, testCase.path, nil)
			request.RemoteAddr = "198.51.100.10:54321"
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, nethttp.StatusTooManyRequests, response.Code)
			require.Zero(t, handlerCalls)
			require.Equal(t, []string{"baseline", testCase.stricterPolicy}, limiter.calls)
			require.Contains(t, response.Body.String(), `"code":"rate_limit.exceeded"`)
		})
	}
}

func TestOrdinaryAPIEndpointUsesBaselinePolicy(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	limiter := &policyDecider{decisions: map[string]ratelimit.Decision{
		"baseline": {
			Allowed:    false,
			Reason:     ratelimit.DenialRateExceeded,
			RetryAfter: time.Second,
		},
	}}
	limits := newIdentityRateLimits(t, limiter)
	handlerCalls := 0
	router := gin.New()
	api := router.Group("/api/v1", limits.Baseline)
	api.GET("/account", countHandler(&handlerCalls))
	request := httptest.NewRequest(nethttp.MethodGet, "/api/v1/account", nil)
	request.RemoteAddr = "198.51.100.10:54321"
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusTooManyRequests, response.Code)
	require.Zero(t, handlerCalls)
	require.Equal(t, []string{"baseline"}, limiter.calls)
}

func newIdentityRateLimits(
	t *testing.T,
	limiter *policyDecider,
) identityhttp.RateLimits {
	t.Helper()
	keyer, err := platformmiddleware.NewHMACClientKeyer(bytes.Repeat([]byte{0x42}, 32))
	require.NoError(t, err)
	limits, err := identityhttp.NewRateLimits(identityhttp.RateLimitDependencies{
		Limiter:        limiter,
		TrustedProxies: config.TrustedProxySet{},
		ClientKey:      keyer,
		Logger: slog.New(slog.NewJSONHandler(
			io.Discard,
			nil,
		)),
	})
	require.NoError(t, err)
	return limits
}

func countHandler(calls *int) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		*calls++
		ctx.Status(nethttp.StatusNoContent)
	}
}

type policyDecider struct {
	decisions map[string]ratelimit.Decision
	calls     []string
}

func (decider *policyDecider) Allow(
	policyName string,
	_ string,
) (ratelimit.Decision, error) {
	decider.calls = append(decider.calls, policyName)
	return decider.decisions[policyName], nil
}
