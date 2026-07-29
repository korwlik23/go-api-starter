package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	mfaapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/mfa"
)

var ErrInvalidMFAConfirmationHandler = errors.New(
	"invalid MFA confirmation handler dependencies",
)

type mfaConfirmer interface {
	Confirm(
		ctx context.Context,
		command mfaapp.ConfirmCommand,
	) (mfaapp.ConfirmationResult, error)
}

type MFAConfirmationHandler struct {
	confirmer mfaConfirmer
}

func NewMFAConfirmationHandler(
	confirmer mfaConfirmer,
) (*MFAConfirmationHandler, error) {
	if confirmer == nil {
		return nil, ErrInvalidMFAConfirmationHandler
	}
	return &MFAConfirmationHandler{confirmer: confirmer}, nil
}

func (handler *MFAConfirmationHandler) Confirm(ctx *gin.Context) {
	session, ok := authenticatedSession(ctx)
	if !ok {
		return
	}
	var request adminapi.MFAEnrollmentConfirmationRequest
	if err := decodeStrictJSON(ctx, &request); err != nil ||
		request.Code == nil ||
		*request.Code == "" ||
		ctx.Param("enrollment_id") == "" {
		writeLoginProblem(
			ctx,
			nethttp.StatusBadRequest,
			"request_invalid",
			"The MFA confirmation request is invalid.",
			err,
		)
		return
	}
	result, err := handler.confirmer.Confirm(
		ctx.Request.Context(),
		mfaapp.ConfirmCommand{
			UserID:       *session.UserID,
			EnrollmentID: ctx.Param("enrollment_id"),
			Code:         *request.Code,
		},
	)
	if errors.Is(err, mfaapp.ErrInvalidMFAConfirmation) {
		writeLoginProblem(
			ctx,
			nethttp.StatusBadRequest,
			"mfa_confirmation_invalid",
			"The MFA confirmation code is invalid or expired.",
			nil,
		)
		return
	}
	if err != nil {
		writeLoginProblem(
			ctx,
			nethttp.StatusConflict,
			"mfa_confirmation_conflict",
			"The MFA enrollment could not be confirmed.",
			err,
		)
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusOK, adminapi.MFAEnrollmentConfirmationResponse{
		RecoveryCodes: result.RecoveryCodes,
		Status:        adminapi.Confirmed,
	})
}
