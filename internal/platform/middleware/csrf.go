package middleware

import (
	"context"
	"errors"
	nethttp "net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

const CSRFTokenHeader = "X-CSRF-Token"

var ErrInvalidCSRFConfig = errors.New("invalid CSRF middleware configuration")

type csrfSessionStore interface {
	FindByTokenHash(
		ctx context.Context,
		hash identitydomain.TokenHash,
	) (identitydomain.Session, error)
}

type csrfTokenCodec interface {
	Hash(raw string) [32]byte
	Matches(raw string, expected [32]byte) bool
}

type CSRFConfig struct {
	TrustedOrigin string
	CookieName    string
	Sessions      csrfSessionStore
	SessionTokens csrfTokenCodec
	CSRFTokens    csrfTokenCodec
}

func NewCSRF(config CSRFConfig) (gin.HandlerFunc, error) {
	trustedOrigin, err := parseOrigin(config.TrustedOrigin)
	if err != nil ||
		strings.TrimSpace(config.CookieName) == "" ||
		config.Sessions == nil ||
		config.SessionTokens == nil ||
		config.CSRFTokens == nil {
		return nil, ErrInvalidCSRFConfig
	}

	return func(ctx *gin.Context) {
		if isSafeMethod(ctx.Request.Method) {
			ctx.Next()
			return
		}
		if !hasTrustedRequestSource(ctx.Request, trustedOrigin) {
			writeCSRFRejected(ctx, nil)
			return
		}

		sessionToken, cookieErr := ctx.Cookie(config.CookieName)
		csrfToken := ctx.GetHeader(CSRFTokenHeader)
		if cookieErr != nil || sessionToken == "" || csrfToken == "" {
			writeCSRFRejected(ctx, nil)
			return
		}

		sessionHash := identitydomain.TokenHash(config.SessionTokens.Hash(sessionToken))
		session, err := config.Sessions.FindByTokenHash(ctx.Request.Context(), sessionHash)
		if err != nil ||
			session.RevokedAt != nil ||
			!config.CSRFTokens.Matches(csrfToken, [32]byte(session.CSRFTokenHash)) {
			writeCSRFRejected(ctx, err)
			return
		}

		ctx.Next()
	}, nil
}

type requestOrigin struct {
	scheme string
	host   string
}

func parseOrigin(value string) (requestOrigin, error) {
	parsed, err := url.Parse(value)
	if err != nil ||
		parsed.Scheme == "" ||
		parsed.Host == "" ||
		parsed.User != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		(parsed.Path != "" && parsed.Path != "/") ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return requestOrigin{}, ErrInvalidCSRFConfig
	}
	return requestOrigin{scheme: parsed.Scheme, host: parsed.Host}, nil
}

func hasTrustedRequestSource(request *nethttp.Request, trusted requestOrigin) bool {
	originValues := request.Header.Values("Origin")
	if len(originValues) > 0 {
		if len(originValues) != 1 {
			return false
		}
		origin, err := parseOrigin(originValues[0])
		return err == nil && origin == trusted
	}

	refererValues := request.Header.Values("Referer")
	if len(refererValues) != 1 {
		return false
	}
	referer, err := url.Parse(refererValues[0])
	if err != nil ||
		referer.Scheme == "" ||
		referer.Host == "" ||
		referer.User != nil ||
		(referer.Scheme != "http" && referer.Scheme != "https") {
		return false
	}
	return requestOrigin{scheme: referer.Scheme, host: referer.Host} == trusted
}

func isSafeMethod(method string) bool {
	switch method {
	case nethttp.MethodGet, nethttp.MethodHead, nethttp.MethodOptions, nethttp.MethodTrace:
		return true
	default:
		return false
	}
}

func writeCSRFRejected(ctx *gin.Context, cause error) {
	response.WriteProblem(
		ctx,
		nethttp.StatusForbidden,
		RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(
			"csrf_invalid",
			"The CSRF token is missing or invalid.",
			nil,
			cause,
		),
	)
}

var _ csrfTokenCodec = (*platformcrypto.TokenCodec)(nil)
