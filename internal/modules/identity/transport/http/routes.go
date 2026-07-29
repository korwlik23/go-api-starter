package http

import "github.com/gin-gonic/gin"

type RouteHandlers struct {
	CSRF          *CSRFHandler
	Login         *LoginHandler
	MFA           *MFAHandler
	MFAEnrollment *MFAEnrollmentHandler
	Recovery      *RecoveryHandler
	Session       *SessionHandler
	SessionQuery  *SessionQueryHandler
}

func RegisterRoutes(
	router *gin.RouterGroup,
	handlers RouteHandlers,
	rateLimits RateLimits,
) {
	router.GET("/auth/csrf", handlers.CSRF.GetToken)
	router.POST("/auth/login", rateLimits.Login, handlers.Login.Login)
	router.POST("/auth/logout", handlers.Session.Logout)
	router.POST(
		"/auth/mfa/enrollment",
		rateLimits.MFA,
		handlers.MFAEnrollment.Enroll,
	)
	router.GET("/auth/mfa/challenge", handlers.MFA.Challenge)
	router.POST("/auth/mfa/verify", rateLimits.MFA, handlers.MFA.Verify)
	router.POST(
		"/auth/password/recovery",
		rateLimits.Recovery,
		handlers.Recovery.Request,
	)
	router.POST(
		"/auth/password/reset",
		rateLimits.Recovery,
		handlers.Recovery.Reset,
	)
	router.GET("/auth/session", handlers.SessionQuery.Current)
	router.GET("/auth/sessions", handlers.SessionQuery.List)
	router.DELETE("/auth/sessions/:session_id", handlers.SessionQuery.Revoke)
}

func RegisterCSRFRoutes(router *gin.RouterGroup, handler *CSRFHandler) {
	router.GET("/auth/csrf", handler.GetToken)
}
