package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	identityapp "github.com/korwlik23/go-api-starter/internal/modules/identity/application/mfa"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	platformcrypto "github.com/korwlik23/go-api-starter/internal/platform/crypto"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/stretchr/testify/require"
)

func TestMFAEnrollmentHTTPRequiresAuthenticatedSessionAndReturnsProvisioningURI(
	t *testing.T,
) {
	gin.SetMode(gin.TestMode)
	userID := "018f0000-0000-7000-8000-000000000001"
	enroller := &mfaEnrollerStub{result: identityapp.EnrollmentResult{
		EnrollmentID:    "enrollment-id",
		ProvisioningURI: "otpauth://totp/starter:user@example.com?secret=secret",
	}}
	handler, err := NewMFAEnrollmentHandler(
		enroller,
		&mfaUserReaderStub{user: identitydomain.User{
			ID:    userID,
			Email: "user@example.com",
		}},
	)
	require.NoError(t, err)

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
	router := gin.New()
	router.Use(authentication)
	router.POST("/api/v1/auth/mfa/enrollment", handler.Enroll)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/mfa/enrollment",
		nil,
	)
	request.AddCookie(&http.Cookie{Name: "session", Value: "session-token"})
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, userID, enroller.userID)
	require.Equal(t, "user@example.com", enroller.email)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.JSONEq(t, `{
		"enrollment_id":"enrollment-id",
		"provisioning_uri":"otpauth://totp/starter:user@example.com?secret=secret"
	}`, recorder.Body.String())
}

type mfaEnrollerStub struct {
	result identityapp.EnrollmentResult
	userID string
	email  string
	err    error
}

func (stub *mfaEnrollerStub) Enroll(
	_ context.Context,
	userID string,
	email string,
) (identityapp.EnrollmentResult, error) {
	stub.userID = userID
	stub.email = email
	return stub.result, stub.err
}

type mfaUserReaderStub struct {
	user identitydomain.User
	err  error
}

func (stub *mfaUserReaderStub) FindByID(
	_ context.Context,
	_ string,
) (identitydomain.User, error) {
	return stub.user, stub.err
}

type mfaSessionReaderStub struct {
	session identitydomain.Session
}

func (stub mfaSessionReaderStub) FindByTokenHash(
	context.Context,
	identitydomain.TokenHash,
) (identitydomain.Session, error) {
	return stub.session, nil
}

type mfaAuthorizationVersionStub struct {
	version int64
}

func (stub mfaAuthorizationVersionStub) CurrentAuthorizationVersion(
	context.Context,
	string,
) (int64, error) {
	return stub.version, nil
}
