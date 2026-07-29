package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	nethttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	authapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/auth"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

var ErrInvalidLoginHandler = errors.New("invalid login handler dependencies")

type loginExecutor interface {
	Execute(
		ctx context.Context,
		command authapp.LoginCommand,
	) (authapp.LoginResult, error)
}

type ClientAddressResolver func(request *nethttp.Request) (string, error)

type LoginHandler struct {
	executor      loginExecutor
	writeCookie   SessionCookieWriter
	clientAddress ClientAddressResolver
}

func NewLoginHandler(
	executor loginExecutor,
	writeCookie SessionCookieWriter,
	clientAddress ClientAddressResolver,
) (*LoginHandler, error) {
	if executor == nil || writeCookie == nil || clientAddress == nil {
		return nil, ErrInvalidLoginHandler
	}
	return &LoginHandler{
		executor:      executor,
		writeCookie:   writeCookie,
		clientAddress: clientAddress,
	}, nil
}

func (handler *LoginHandler) Login(ctx *gin.Context) {
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

	var request adminapi.LoginRequest
	if err := decodeStrictJSON(ctx, &request); err != nil ||
		request.Password == nil ||
		strings.TrimSpace(string(request.Email)) == "" ||
		*request.Password == "" {
		writeLoginProblem(
			ctx,
			nethttp.StatusBadRequest,
			"request_invalid",
			"The request body is invalid.",
			err,
		)
		return
	}
	clientAddress, err := handler.clientAddress(ctx.Request)
	if err != nil || clientAddress == "" {
		writeLoginProblem(
			ctx,
			nethttp.StatusInternalServerError,
			"internal_error",
			"The request could not be completed.",
			err,
		)
		return
	}

	result, err := handler.executor.Execute(
		ctx.Request.Context(),
		authapp.LoginCommand{
			Email:              string(request.Email),
			Password:           *request.Password,
			ClientIP:           clientAddress,
			CurrentSessionHash: session.TokenHash,
		},
	)
	if errors.Is(err, authapp.ErrInvalidCredentials) {
		writeLoginProblem(
			ctx,
			nethttp.StatusUnauthorized,
			"credentials_invalid",
			"The email or password is invalid.",
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
	switch result.Status {
	case authapp.LoginStatusAuthenticated:
		ctx.JSON(nethttp.StatusOK, adminapi.LoginAuthenticatedResponse{
			CsrfToken: result.CSRFToken,
			Status:    adminapi.LoginAuthenticatedResponseStatusAuthenticated,
		})
	case authapp.LoginStatusMFAPending:
		ctx.JSON(nethttp.StatusAccepted, adminapi.LoginMFAPendingResponse{
			ChallengeId: result.ChallengeID,
			CsrfToken:   result.CSRFToken,
			Status:      adminapi.LoginMFAPendingResponseStatusMfaPending,
		})
	default:
		writeLoginProblem(
			ctx,
			nethttp.StatusInternalServerError,
			"internal_error",
			"The request could not be completed.",
			authapp.ErrInvalidLoginFlow,
		)
	}
}

func decodeStrictJSON(ctx *gin.Context, destination any) error {
	decoder := json.NewDecoder(ctx.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func writeLoginProblem(
	ctx *gin.Context,
	status int,
	code string,
	message string,
	cause error,
) {
	response.WriteProblem(
		ctx,
		status,
		platformmiddleware.RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(code, message, nil, cause),
	)
}
