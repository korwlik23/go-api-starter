package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	accountsports "github.com/korwlik23/go-api-starter/internal/modules/accounts/ports"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidCurrentAccountHandler = errors.New(
	"invalid current account handler dependencies",
)

type accountResolver interface {
	Execute(
		ctx context.Context,
		userID string,
	) (accountsports.ResolvedAccount, error)
}

type CurrentAccountHandler struct {
	resolver accountResolver
}

func NewCurrentAccountHandler(
	resolver accountResolver,
) (*CurrentAccountHandler, error) {
	if resolver == nil {
		return nil, ErrInvalidCurrentAccountHandler
	}
	return &CurrentAccountHandler{resolver: resolver}, nil
}

func (handler *CurrentAccountHandler) Get(ctx *gin.Context) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok ||
		session.State != identitydomain.SessionStateAuthenticated ||
		session.UserID == nil {
		writeAccountProblem(ctx, nethttp.StatusUnauthorized, nil)
		return
	}
	resolved, err := handler.resolver.Execute(
		ctx.Request.Context(),
		*session.UserID,
	)
	if err != nil {
		writeAccountProblem(ctx, nethttp.StatusForbidden, err)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusOK, adminapi.CurrentAccountResponse{
		Id:   resolved.Account.ID,
		Name: resolved.Account.Slug,
		Slug: resolved.Account.Slug,
	})
}

func writeAccountProblem(
	ctx *gin.Context,
	status int,
	cause error,
) {
	response.WriteProblem(
		ctx,
		status,
		platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(
			"account_resolution_failed",
			"The current account could not be resolved.",
			nil,
			cause,
		),
	)
}
