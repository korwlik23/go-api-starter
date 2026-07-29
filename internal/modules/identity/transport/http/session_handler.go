package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

var ErrInvalidSessionHandler = errors.New("invalid session handler dependencies")

type sessionRevoker interface {
	Execute(ctx context.Context, rawToken string) error
}

type SessionTokenReader func(ctx *gin.Context) (string, error)
type SessionCookieClearer func(ctx *gin.Context)

type SessionHandler struct {
	revoker     sessionRevoker
	readToken   SessionTokenReader
	clearCookie SessionCookieClearer
}

func NewSessionHandler(
	revoker sessionRevoker,
	readToken SessionTokenReader,
	clearCookie SessionCookieClearer,
) (*SessionHandler, error) {
	if revoker == nil || readToken == nil || clearCookie == nil {
		return nil, ErrInvalidSessionHandler
	}
	return &SessionHandler{
		revoker:     revoker,
		readToken:   readToken,
		clearCookie: clearCookie,
	}, nil
}

func (handler *SessionHandler) Logout(ctx *gin.Context) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State == identitydomain.SessionStateAnonymous {
		writeLoginProblem(
			ctx,
			nethttp.StatusUnauthorized,
			"authentication_required",
			"Authentication is required.",
			nil,
		)
		return
	}
	rawToken, err := handler.readToken(ctx)
	if err != nil || rawToken == "" {
		writeLoginProblem(
			ctx,
			nethttp.StatusUnauthorized,
			"authentication_required",
			"Authentication is required.",
			nil,
		)
		return
	}
	if err := handler.revoker.Execute(ctx.Request.Context(), rawToken); err != nil {
		writeLoginProblem(
			ctx,
			nethttp.StatusInternalServerError,
			"internal_error",
			"The request could not be completed.",
			err,
		)
		return
	}
	handler.clearCookie(ctx)
	ctx.Header("Cache-Control", "no-store")
	ctx.Status(nethttp.StatusNoContent)
}
