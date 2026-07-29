package http

import (
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutesExposesTheIdentityContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes := router.Group("/api/v1")
	routes.Use(func(ctx *gin.Context) { ctx.Next() })

	RegisterRoutes(routes, RouteHandlers{
		CSRF:            &CSRFHandler{},
		Login:           &LoginHandler{},
		MFA:             &MFAHandler{},
		MFAConfirmation: &MFAConfirmationHandler{},
		MFAEnrollment:   &MFAEnrollmentHandler{},
		Recovery:        &RecoveryHandler{},
		Session:         &SessionHandler{},
		SessionQuery:    &SessionQueryHandler{},
	}, RateLimits{
		Login:    func(ctx *gin.Context) { ctx.Next() },
		Recovery: func(ctx *gin.Context) { ctx.Next() },
		MFA:      func(ctx *gin.Context) { ctx.Next() },
	})

	got := make([]string, 0, len(router.Routes()))
	for _, route := range router.Routes() {
		got = append(got, route.Method+" "+route.Path)
	}
	want := []string{
		"DELETE /api/v1/auth/sessions/:session_id",
		"GET /api/v1/auth/csrf",
		"GET /api/v1/auth/mfa/challenge",
		"GET /api/v1/auth/session",
		"GET /api/v1/auth/sessions",
		"POST /api/v1/auth/login",
		"POST /api/v1/auth/logout",
		"POST /api/v1/auth/mfa/enrollment",
		"POST /api/v1/auth/mfa/enrollment/:enrollment_id/confirmation",
		"POST /api/v1/auth/mfa/verify",
		"POST /api/v1/auth/password/recovery",
		"POST /api/v1/auth/password/reset",
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("registered routes = %#v, want %#v", got, want)
	}
}
