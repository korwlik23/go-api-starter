package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	mfaapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/mfa"
	"github.com/korwlik23/go-api-starter/internal/modules/identity/ports"
)

var ErrInvalidMFAEnrollmentHandler = errors.New(
	"invalid MFA enrollment handler dependencies",
)

type mfaEnroller interface {
	Enroll(
		ctx context.Context,
		userID string,
		email string,
	) (mfaapp.EnrollmentResult, error)
}

type MFAEnrollmentHandler struct {
	enroller mfaEnroller
	users    ports.UserByIDReader
}

func NewMFAEnrollmentHandler(
	enroller mfaEnroller,
	users ports.UserByIDReader,
) (*MFAEnrollmentHandler, error) {
	if enroller == nil || users == nil {
		return nil, ErrInvalidMFAEnrollmentHandler
	}
	return &MFAEnrollmentHandler{enroller: enroller, users: users}, nil
}

func (handler *MFAEnrollmentHandler) Enroll(ctx *gin.Context) {
	session, ok := authenticatedSession(ctx)
	if !ok {
		return
	}
	user, err := handler.users.FindByID(
		ctx.Request.Context(),
		*session.UserID,
	)
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
	result, err := handler.enroller.Enroll(
		ctx.Request.Context(),
		user.ID,
		user.Email,
	)
	if err != nil {
		writeLoginProblem(
			ctx,
			nethttp.StatusConflict,
			"mfa_enrollment_conflict",
			"MFA enrollment could not be created.",
			err,
		)
		return
	}

	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusCreated, adminapi.MFAEnrollmentResponse{
		EnrollmentId:    result.EnrollmentID,
		ProvisioningUri: result.ProvisioningURI,
	})
}
