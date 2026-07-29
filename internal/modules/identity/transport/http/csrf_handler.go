package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	sessionapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/session"
	"github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidCSRFHandler = errors.New("invalid CSRF handler dependencies")

type anonymousSessionCreator interface {
	Execute(ctx context.Context) (sessionapp.CreateAnonymousResult, error)
}

type SessionCookieWriter func(ctx *gin.Context, token string)

type CSRFHandler struct {
	creator     anonymousSessionCreator
	writeCookie SessionCookieWriter
}

func NewCSRFHandler(
	creator anonymousSessionCreator,
	writeCookie SessionCookieWriter,
) (*CSRFHandler, error) {
	if creator == nil || writeCookie == nil {
		return nil, ErrInvalidCSRFHandler
	}
	return &CSRFHandler{creator: creator, writeCookie: writeCookie}, nil
}

func (handler *CSRFHandler) GetToken(ctx *gin.Context) {
	result, err := handler.creator.Execute(ctx.Request.Context())
	if err != nil {
		response.WriteProblem(
			ctx,
			nethttp.StatusInternalServerError,
			middleware.RequestIDFromContext(ctx.Request.Context()),
			response.NewProblem(
				"internal_error",
				"The request could not be completed.",
				nil,
				err,
			),
		)
		return
	}

	handler.writeCookie(ctx, result.SessionToken)
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusOK, adminapi.CSRFTokenResponse{CsrfToken: result.CSRFToken})
}
