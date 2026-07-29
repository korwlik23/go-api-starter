package http

import (
	"context"
	"errors"
	nethttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	authapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/auth"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

var ErrInvalidRecoveryHandler = errors.New("invalid recovery handler dependencies")

type recoveryRequester interface {
	Execute(ctx context.Context, email string) error
}

type passwordResetter interface {
	Execute(
		ctx context.Context,
		command authapp.ResetPasswordCommand,
	) (authapp.ResetPasswordResult, error)
}

type RecoveryHandler struct {
	requester   recoveryRequester
	resetter    passwordResetter
	writeCookie SessionCookieWriter
}

func NewRecoveryHandler(
	requester recoveryRequester,
	resetter passwordResetter,
	writeCookie SessionCookieWriter,
) (*RecoveryHandler, error) {
	if requester == nil || resetter == nil || writeCookie == nil {
		return nil, ErrInvalidRecoveryHandler
	}
	return &RecoveryHandler{
		requester:   requester,
		resetter:    resetter,
		writeCookie: writeCookie,
	}, nil
}

func (handler *RecoveryHandler) Request(ctx *gin.Context) {
	var request adminapi.RecoveryRequest
	if err := decodeStrictJSON(ctx, &request); err != nil ||
		strings.TrimSpace(string(request.Email)) == "" {
		writeLoginProblem(
			ctx,
			nethttp.StatusBadRequest,
			"request_invalid",
			"The request body is invalid.",
			err,
		)
		return
	}
	if err := handler.requester.Execute(
		ctx.Request.Context(),
		string(request.Email),
	); err != nil {
		writeLoginProblem(
			ctx,
			nethttp.StatusInternalServerError,
			"internal_error",
			"The request could not be completed.",
			err,
		)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusAccepted, adminapi.RecoveryAcceptedResponse{
		Status: adminapi.Accepted,
	})
}

func (handler *RecoveryHandler) Reset(ctx *gin.Context) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok || session.State != identitydomain.SessionStateAnonymous {
		writeLoginProblem(
			ctx,
			nethttp.StatusUnauthorized,
			"authentication_required",
			"An anonymous session is required.",
			nil,
		)
		return
	}

	var request adminapi.PasswordResetRequest
	if err := decodeStrictJSON(ctx, &request); err != nil ||
		request.Token == nil ||
		request.NewPassword == nil ||
		*request.Token == "" ||
		*request.NewPassword == "" {
		writeLoginProblem(
			ctx,
			nethttp.StatusBadRequest,
			"request_invalid",
			"The request body is invalid.",
			err,
		)
		return
	}
	result, err := handler.resetter.Execute(
		ctx.Request.Context(),
		authapp.ResetPasswordCommand{
			RawToken:           *request.Token,
			NewPassword:        *request.NewPassword,
			CurrentSessionHash: session.TokenHash,
		},
	)
	if errors.Is(err, authapp.ErrRecoveryTokenInvalid) ||
		errors.Is(err, ports.ErrOneTimeTokenConflict) ||
		errors.Is(err, ports.ErrOneTimeTokenNotFound) {
		writeLoginProblem(
			ctx,
			nethttp.StatusBadRequest,
			"recovery_token_invalid",
			"The recovery token is invalid or expired.",
			nil,
		)
		return
	}
	if err != nil {
		writeLoginProblem(
			ctx,
			nethttp.StatusInternalServerError,
			"internal_error",
			"The request could not be completed.",
			err,
		)
		return
	}

	handler.writeCookie(ctx, result.SessionToken)
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Referrer-Policy", "no-referrer")
	ctx.JSON(nethttp.StatusOK, adminapi.PasswordResetResponse{
		CsrfToken: result.CSRFToken,
		Status:    adminapi.PasswordReset,
	})
}
