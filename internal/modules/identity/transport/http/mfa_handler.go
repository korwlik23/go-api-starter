package http

import (
	"context"
	"errors"
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	mfaapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/mfa"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

var ErrInvalidMFAHandler = errors.New("invalid MFA handler dependencies")

type mfaVerifier interface {
	VerifyTOTP(
		ctx context.Context,
		command mfaapp.VerifyTOTPCommand,
	) (mfaapp.VerificationResult, error)
}

type mfaRecoveryVerifier interface {
	VerifyRecoveryCode(
		ctx context.Context,
		command mfaapp.VerifyRecoveryCodeCommand,
	) (mfaapp.VerificationResult, error)
}

type MFAHandler struct {
	verifier         mfaVerifier
	recoveryVerifier mfaRecoveryVerifier
	writeCookie      SessionCookieWriter
}

func NewMFAHandler(
	verifier mfaVerifier,
	recoveryVerifier mfaRecoveryVerifier,
	writeCookie SessionCookieWriter,
) (*MFAHandler, error) {
	if verifier == nil || recoveryVerifier == nil || writeCookie == nil {
		return nil, ErrInvalidMFAHandler
	}
	return &MFAHandler{
		verifier:         verifier,
		recoveryVerifier: recoveryVerifier,
		writeCookie:      writeCookie,
	}, nil
}

func (handler *MFAHandler) Challenge(ctx *gin.Context) {
	session, ok := mfaPendingSession(ctx)
	if !ok {
		return
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(nethttp.StatusOK, adminapi.MFAChallengeResponse{
		ChallengeId: session.ID,
		Methods: []adminapi.MFAChallengeResponseMethods{
			adminapi.MFAChallengeResponseMethodsTotp,
			adminapi.MFAChallengeResponseMethodsRecoveryCode,
		},
		Status: adminapi.MFAChallengeResponseStatusMfaPending,
	})
}

func (handler *MFAHandler) Verify(ctx *gin.Context) {
	session, ok := mfaPendingSession(ctx)
	if !ok {
		return
	}
	var request adminapi.MFAVerificationRequest
	if err := decodeStrictJSON(ctx, &request); err != nil ||
		request.Code == nil ||
		*request.Code == "" {
		writeLoginProblem(
			ctx,
			nethttp.StatusBadRequest,
			"request_invalid",
			"The MFA verification request is invalid.",
			err,
		)
		return
	}
	var result mfaapp.VerificationResult
	var err error
	switch request.Method {
	case adminapi.MFAVerificationRequestMethodTotp:
		result, err = handler.verifier.VerifyTOTP(
			ctx.Request.Context(),
			mfaapp.VerifyTOTPCommand{
				PendingSessionHash:   session.TokenHash,
				UserID:               *session.UserID,
				AuthorizationVersion: *session.AuthorizationVersion,
				Code:                 *request.Code,
			},
		)
	case adminapi.MFAVerificationRequestMethodRecoveryCode:
		result, err = handler.recoveryVerifier.VerifyRecoveryCode(
			ctx.Request.Context(),
			mfaapp.VerifyRecoveryCodeCommand{
				PendingSessionHash:   session.TokenHash,
				UserID:               *session.UserID,
				AuthorizationVersion: *session.AuthorizationVersion,
				Code:                 *request.Code,
			},
		)
	default:
		writeLoginProblem(
			ctx,
			nethttp.StatusBadRequest,
			"request_invalid",
			"The MFA verification request is invalid.",
			nil,
		)
		return
	}
	if errors.Is(err, mfaapp.ErrInvalidMFAVerification) {
		writeLoginProblem(
			ctx,
			nethttp.StatusForbidden,
			"mfa_verification_failed",
			"The MFA code is invalid or expired.",
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
	ctx.JSON(nethttp.StatusOK, adminapi.MFAVerificationResponse{
		CsrfToken: result.CSRFToken,
		Status:    adminapi.MFAVerificationResponseStatusAuthenticated,
	})
}

func mfaPendingSession(
	ctx *gin.Context,
) (identitydomain.Session, bool) {
	session, ok := platformmiddleware.SessionFromContext(ctx.Request.Context())
	if !ok {
		writeLoginProblem(
			ctx,
			nethttp.StatusUnauthorized,
			"authentication_required",
			"Authentication is required.",
			nil,
		)
		return identitydomain.Session{}, false
	}
	if session.State != identitydomain.SessionStateMFAPending ||
		session.UserID == nil ||
		session.AuthorizationVersion == nil {
		writeLoginProblem(
			ctx,
			nethttp.StatusForbidden,
			"session_state_forbidden",
			"The current session cannot access this resource.",
			nil,
		)
		return identitydomain.Session{}, false
	}
	return session, true
}
