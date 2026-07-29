package middleware

import (
	"context"
	"errors"
	nethttp "net/http"
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
	CookieName    string
	Sessions      csrfSessionStore
	SessionTokens csrfTokenCodec
	CSRFTokens    csrfTokenCodec
}

func NewCSRF(config CSRFConfig) (gin.HandlerFunc, error) {
	if strings.TrimSpace(config.CookieName) == "" ||
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
