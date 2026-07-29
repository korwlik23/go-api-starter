package security_test

import (
	"bytes"
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	adminapi "github.com/korwlik23/go-api-starter/internal/generated/admin"
	mfaapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/mfa"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	identityhttp "github.com/korwlik23/go-api-starter/internal/modules/identity/transport/http"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/stretchr/testify/require"
)

func TestMFAChallengeHTTPReturnsPendingOnlyProjection(t *testing.T) {
	t.Parallel()

	handler, err := identityhttp.NewMFAHandler(
		&mfaVerifierStub{},
		&mfaVerifierStub{},
		func(*gin.Context, string) {},
	)
	require.NoError(t, err)
	router := mfaPendingTestRouter(t, handler)
	request := httptest.NewRequest(
		nethttp.MethodGet,
		"/api/v1/auth/mfa/challenge",
		nil,
	)
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "pending-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Contains(t, response.Body.String(), `"challenge_id":"pending-session"`)
	require.Contains(t, response.Body.String(), `"status":"mfa_pending"`)
	require.NotContains(t, response.Body.String(), "pending-token")
}

func TestMFAVerifyHTTPRotatesPendingSession(t *testing.T) {
	t.Parallel()

	verifier := &mfaVerifierStub{result: mfaapp.VerificationResult{
		SessionToken: "authenticated-token",
		CSRFToken:    "authenticated-csrf",
	}}
	handler, err := identityhttp.NewMFAHandler(
		verifier,
		verifier,
		func(ctx *gin.Context, token string) {
			ctx.Header("X-Test-Session", token)
		},
	)
	require.NoError(t, err)
	router := mfaPendingTestRouter(t, handler)
	request := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/auth/mfa/verify",
		bytes.NewBufferString(`{"method":"totp","code":"123456"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "pending-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusOK, response.Code)
	require.Equal(t, "authenticated-token", response.Header().Get("X-Test-Session"))
	require.Contains(t, response.Body.String(), `"status":"authenticated"`)
	require.Contains(t, response.Body.String(), `"csrf_token":"authenticated-csrf"`)
	require.Equal(t, "123456", verifier.command.Code)
	require.Equal(t, "user-id", verifier.command.UserID)
	require.Equal(t, int64(7), verifier.command.AuthorizationVersion)
}

func TestMFAVerifyHTTPAcceptsOneTimeRecoveryCode(t *testing.T) {
	t.Parallel()

	verifier := &mfaVerifierStub{result: mfaapp.VerificationResult{
		SessionToken: "authenticated-token",
		CSRFToken:    "authenticated-csrf",
	}}
	handler, err := identityhttp.NewMFAHandler(
		verifier,
		verifier,
		func(*gin.Context, string) {},
	)
	require.NoError(t, err)
	router := mfaPendingTestRouter(t, handler)
	request := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/auth/mfa/verify",
		bytes.NewBufferString(
			`{"method":"recovery_code","code":"one-time-code"}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&nethttp.Cookie{Name: "session", Value: "pending-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, nethttp.StatusOK, response.Code)
	require.Equal(t, "one-time-code", verifier.recoveryCommand.Code)
	require.Empty(t, verifier.command.Code)
}

func mfaPendingTestRouter(
	t *testing.T,
	handler *identityhttp.MFAHandler,
) *gin.Engine {
	t.Helper()
	now := time.Date(2026, time.July, 30, 11, 0, 0, 0, time.UTC)
	codec, err := platformcrypto.NewTokenCodec(
		bytes.Repeat([]byte{0x42}, 32),
		"identity.session.v1",
	)
	require.NoError(t, err)
	userID := "user-id"
	version := int64(7)
	authentication, err := platformmiddleware.NewSessionAuthentication(
		platformmiddleware.SessionAuthenticationConfig{
			CookieName: "session",
			Sessions: loginSessionReaderStub{session: identitydomain.Session{
				ID:                   "pending-session",
				TokenHash:            identitydomain.TokenHash(codec.Hash("pending-token")),
				State:                identitydomain.SessionStateMFAPending,
				UserID:               &userID,
				AuthorizationVersion: &version,
				IdleExpiresAt:        now.Add(time.Minute),
				AbsoluteExpiresAt:    now.Add(time.Hour),
			}},
			AuthorizationVersions: fixedAuthorizationVersionReader{version: version},
			SessionTokens:         codec,
			Now:                   func() time.Time { return now },
		},
	)
	require.NoError(t, err)
	router := gin.New()
	router.Use(authentication)
	router.GET("/api/v1/auth/mfa/challenge", handler.Challenge)
	router.POST("/api/v1/auth/mfa/verify", handler.Verify)
	return router
}

type fixedAuthorizationVersionReader struct {
	version int64
}

func (reader fixedAuthorizationVersionReader) CurrentAuthorizationVersion(
	context.Context,
	string,
) (int64, error) {
	return reader.version, nil
}

type mfaVerifierStub struct {
	result          mfaapp.VerificationResult
	command         mfaapp.VerifyTOTPCommand
	recoveryCommand mfaapp.VerifyRecoveryCodeCommand
	err             error
}

func (stub *mfaVerifierStub) VerifyRecoveryCode(
	_ context.Context,
	command mfaapp.VerifyRecoveryCodeCommand,
) (mfaapp.VerificationResult, error) {
	stub.recoveryCommand = command
	return stub.result, stub.err
}

func (stub *mfaVerifierStub) VerifyTOTP(
	_ context.Context,
	command mfaapp.VerifyTOTPCommand,
) (mfaapp.VerificationResult, error) {
	stub.command = command
	return stub.result, stub.err
}

var _ = adminapi.MFAVerificationRequestMethodTotp
