package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mfaapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/mfa"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/stretchr/testify/require"
)

func TestMFAConfirmationHTTPReturnsRecoveryCodesOnlyAfterSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := "018f0000-0000-7000-8000-000000000001"
	now := time.Date(2026, time.July, 30, 8, 0, 0, 0, time.UTC)
	version := int64(1)
	codec, err := platformcrypto.NewTokenCodec(
		bytes.Repeat([]byte{0x42}, 32),
		"identity.session.v1",
	)
	require.NoError(t, err)
	session := identitydomain.Session{
		ID:                   "session-id",
		TokenHash:            identitydomain.TokenHash(codec.Hash("session-token")),
		State:                identitydomain.SessionStateAuthenticated,
		UserID:               &userID,
		AuthorizationVersion: &version,
		IdleExpiresAt:        now.Add(time.Hour),
		AbsoluteExpiresAt:    now.Add(2 * time.Hour),
	}
	authentication, err := platformmiddleware.NewSessionAuthentication(
		platformmiddleware.SessionAuthenticationConfig{
			CookieName:            "session",
			Sessions:              mfaSessionReaderStub{session: session},
			AuthorizationVersions: mfaAuthorizationVersionStub{version: version},
			SessionTokens:         codec,
			Now:                   func() time.Time { return now },
		},
	)
	require.NoError(t, err)
	confirmer := &mfaConfirmerStub{result: mfaapp.ConfirmationResult{
		RecoveryCodes: []string{"first", "second"},
	}}
	handler, err := NewMFAConfirmationHandler(confirmer)
	require.NoError(t, err)
	router := gin.New()
	router.Use(authentication)
	router.POST(
		"/api/v1/auth/mfa/enrollment/:enrollment_id/confirmation",
		handler.Confirm,
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/mfa/enrollment/enrollment-id/confirmation",
		bytes.NewBufferString(`{"code":"123456"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.Equal(t, userID, confirmer.command.UserID)
	require.Equal(t, "enrollment-id", confirmer.command.EnrollmentID)
	require.NotContains(t, response.Body.String(), "123456")
	require.JSONEq(t, `{
		"status":"confirmed",
		"recovery_codes":["first","second"]
	}`, response.Body.String())
}

type mfaConfirmerStub struct {
	command mfaapp.ConfirmCommand
	result  mfaapp.ConfirmationResult
	err     error
}

func (stub *mfaConfirmerStub) Confirm(
	_ context.Context,
	command mfaapp.ConfirmCommand,
) (mfaapp.ConfirmationResult, error) {
	stub.command = command
	return stub.result, stub.err
}
