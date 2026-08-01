package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/ratelimit"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

const (
	minClientKeySecretBytes = 32
	maxForwardedForBytes    = 4096
	maxForwardedForHops     = 32
)

var (
	errClientKeySecretTooShort = errors.New(
		"middleware: rate limit client key secret must contain at least 32 bytes",
	)
	errRemoteAddressInvalid = errors.New(
		"middleware: request remote address is invalid",
	)
)

// RateLimitDecider is the behavior required from a request rate limiter.
type RateLimitDecider interface {
	Allow(policyName, clientKey string) (ratelimit.Decision, error)
}

// ClientKeyer converts a canonical client address into an opaque process-local
// key. Implementations must not return the raw address.
type ClientKeyer func(netip.Addr) string

// RateLimitOptions contains the explicit dependencies for RateLimit.
type RateLimitOptions struct {
	Policy         string
	Limiter        RateLimitDecider
	TrustedProxies config.TrustedProxySet
	ClientKey      ClientKeyer
	Logger         *slog.Logger
}

// NewHMACClientKeyer creates a deterministic opaque keyer from process-scoped
// secret material. Callers should generate a fresh secret for each process.
func NewHMACClientKeyer(secret []byte) (ClientKeyer, error) {
	if len(secret) < minClientKeySecretBytes {
		return nil, errClientKeySecretTooShort
	}

	secretCopy := append([]byte(nil), secret...)
	return func(address netip.Addr) string {
		mac := hmac.New(sha256.New, secretCopy)
		_, _ = mac.Write([]byte("rate-limit-client\x00"))
		_, _ = mac.Write([]byte(canonicalAddress(address).String()))
		return hex.EncodeToString(mac.Sum(nil))
	}, nil
}

// RateLimit applies one named policy using a proxy-aware opaque client key.
func RateLimit(options RateLimitOptions) gin.HandlerFunc {
	if strings.TrimSpace(options.Policy) == "" {
		panic("rate limit policy is required")
	}
	if options.Limiter == nil {
		panic("rate limit decider is required")
	}
	if options.ClientKey == nil {
		panic("rate limit client keyer is required")
	}
	if options.Logger == nil {
		panic("rate limit logger is required")
	}

	return func(ctx *gin.Context) {
		clientAddress, err := resolveClientAddress(
			ctx.Request,
			options.TrustedProxies,
		)
		if err != nil {
			writeRateLimitInternalError(ctx, options, "client_address_invalid")
			return
		}

		clientKey := options.ClientKey(clientAddress)
		if clientKey == "" {
			writeRateLimitInternalError(ctx, options, "client_key_invalid")
			return
		}

		decision, err := options.Limiter.Allow(options.Policy, clientKey)
		if err != nil {
			writeRateLimitInternalError(ctx, options, "limiter_error")
			return
		}
		if decision.Allowed {
			ctx.Next()
			return
		}

		retryAfterSeconds := retryAfterSeconds(decision.RetryAfter)
		ctx.Header("Cache-Control", "no-store")
		ctx.Header("Retry-After", strconv.FormatInt(retryAfterSeconds, 10))
		options.Logger.WarnContext(
			ctx.Request.Context(),
			"rate limit denied",
			slog.String("policy", options.Policy),
			slog.String(
				"request_id",
				RequestIDFromContext(ctx.Request.Context()),
			),
			slog.String("reason", denialReasonName(decision.Reason)),
			slog.Int64("retry_after_seconds", retryAfterSeconds),
		)
		response.WriteProblem(
			ctx,
			http.StatusTooManyRequests,
			RequestIDFromContext(ctx.Request.Context()),
			response.NewProblem(
				"rate_limit.exceeded",
				"Request could not be completed.",
				nil,
				nil,
			),
		)
	}
}

func resolveClientAddress(
	request *http.Request,
	trustedProxies config.TrustedProxySet,
) (netip.Addr, error) {
	peerAddress, err := parseRemoteAddress(request.RemoteAddr)
	if err != nil {
		return netip.Addr{}, errRemoteAddressInvalid
	}
	if !trustedProxies.Contains(peerAddress) {
		return peerAddress, nil
	}

	forwardedValues := request.Header.Values("X-Forwarded-For")
	if len(forwardedValues) == 0 {
		return peerAddress, nil
	}
	forwardedValue := strings.Join(forwardedValues, ",")
	if len(forwardedValue) == 0 || len(forwardedValue) > maxForwardedForBytes {
		return peerAddress, nil
	}

	rawAddresses := strings.Split(forwardedValue, ",")
	if len(rawAddresses) > maxForwardedForHops {
		return peerAddress, nil
	}

	addresses := make([]netip.Addr, len(rawAddresses))
	for index, rawAddress := range rawAddresses {
		address, parseErr := netip.ParseAddr(strings.TrimSpace(rawAddress))
		if parseErr != nil || address.Zone() != "" {
			return peerAddress, nil
		}
		addresses[index] = canonicalAddress(address)
	}

	for index := len(addresses) - 1; index >= 0; index-- {
		if index == 0 || !trustedProxies.Contains(addresses[index]) {
			return addresses[index], nil
		}
	}

	return peerAddress, nil
}

// ResolveClientAddress returns the canonical address selected with the same
// explicit trusted-proxy policy used by rate limiting.
func ResolveClientAddress(
	request *http.Request,
	trustedProxies config.TrustedProxySet,
) (string, error) {
	address, err := resolveClientAddress(request, trustedProxies)
	if err != nil {
		return "", err
	}
	return address.String(), nil
}

func parseRemoteAddress(remoteAddress string) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err == nil {
		address, parseErr := netip.ParseAddr(host)
		if parseErr == nil {
			return canonicalAddress(address), nil
		}
		return netip.Addr{}, errRemoteAddressInvalid
	}

	address, parseErr := netip.ParseAddr(remoteAddress)
	if parseErr != nil {
		return netip.Addr{}, errRemoteAddressInvalid
	}
	return canonicalAddress(address), nil
}

func canonicalAddress(address netip.Addr) netip.Addr {
	if address.Zone() != "" {
		address = address.WithZone("")
	}
	return address.Unmap()
}

func retryAfterSeconds(retryAfter time.Duration) int64 {
	seconds := int64(retryAfter / time.Second)
	if retryAfter%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		return 1
	}
	return seconds
}

func denialReasonName(reason ratelimit.DenialReason) string {
	switch reason {
	case ratelimit.DenialRateExceeded:
		return "rate_exceeded"
	case ratelimit.DenialCapacitySaturated:
		return "capacity_saturated"
	default:
		return "unspecified"
	}
}

func writeRateLimitInternalError(
	ctx *gin.Context,
	options RateLimitOptions,
	reason string,
) {
	ctx.Header("Cache-Control", "no-store")
	options.Logger.ErrorContext(
		ctx.Request.Context(),
		"rate limit unavailable",
		slog.String("policy", options.Policy),
		slog.String(
			"request_id",
			RequestIDFromContext(ctx.Request.Context()),
		),
		slog.String("reason", reason),
	)
	response.WriteProblem(
		ctx,
		http.StatusInternalServerError,
		RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(
			"system.internal_error",
			"Request could not be completed.",
			nil,
			nil,
		),
	)
}
