package middleware

import (
	"context"
	"errors"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidSessionAuthenticationConfig = errors.New(
	"invalid session authentication middleware configuration",
)

type sessionAuthenticationStore interface {
	FindByTokenHash(
		ctx context.Context,
		hash identitydomain.TokenHash,
	) (identitydomain.Session, error)
}

type sessionAuthenticationTokenCodec interface {
	Hash(raw string) [32]byte
}

type SessionAuthenticationConfig struct {
	CookieName    string
	Sessions      sessionAuthenticationStore
	SessionTokens sessionAuthenticationTokenCodec
	Now           func() time.Time
}

type sessionContextKey struct{}

func NewSessionAuthentication(
	config SessionAuthenticationConfig,
) (gin.HandlerFunc, error) {
	if strings.TrimSpace(config.CookieName) == "" ||
		config.Sessions == nil ||
		config.SessionTokens == nil ||
		config.Now == nil {
		return nil, ErrInvalidSessionAuthenticationConfig
	}
	if err := (&nethttp.Cookie{Name: config.CookieName, Value: "probe"}).Valid(); err != nil {
		return nil, ErrInvalidSessionAuthenticationConfig
	}

	return func(ctx *gin.Context) {
		rawToken, err := ctx.Cookie(config.CookieName)
		if err != nil || rawToken == "" {
			ctx.Next()
			return
		}

		hash := identitydomain.TokenHash(config.SessionTokens.Hash(rawToken))
		session, err := config.Sessions.FindByTokenHash(ctx.Request.Context(), hash)
		if errors.Is(err, ports.ErrSessionNotFound) {
			ctx.Next()
			return
		}
		if err != nil {
			response.WriteProblem(
				ctx,
				nethttp.StatusInternalServerError,
				RequestIDFromContext(ctx.Request.Context()),
				response.NewProblem(
					"session_lookup_failed",
					"The session could not be checked.",
					nil,
					err,
				),
			)
			return
		}
		if session.ValidateAt(config.Now()) != nil {
			ctx.Next()
			return
		}

		requestContext := context.WithValue(
			ctx.Request.Context(),
			sessionContextKey{},
			session,
		)
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Next()
	}, nil
}

func SessionFromContext(ctx context.Context) (identitydomain.Session, bool) {
	session, ok := ctx.Value(sessionContextKey{}).(identitydomain.Session)
	return session, ok
}
