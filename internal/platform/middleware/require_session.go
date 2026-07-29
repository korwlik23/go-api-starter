package middleware

import (
	nethttp "net/http"

	"github.com/gin-gonic/gin"
	identitydomain "github.com/korwlik23/go-api-starter/internal/modules/identity/domain"
	"github.com/korwlik23/go-api-starter/internal/platform/response"
)

func RequireSession(allowedStates ...identitydomain.SessionState) gin.HandlerFunc {
	allowed := make(map[identitydomain.SessionState]struct{}, len(allowedStates))
	for _, state := range allowedStates {
		allowed[state] = struct{}{}
	}

	return func(ctx *gin.Context) {
		session, ok := SessionFromContext(ctx.Request.Context())
		if !ok {
			writeAuthenticationRequired(ctx)
			return
		}
		if len(allowed) > 0 {
			if _, accepted := allowed[session.State]; !accepted {
				response.WriteProblem(
					ctx,
					nethttp.StatusForbidden,
					RequestIDFromContext(ctx.Request.Context()),
					response.NewProblem(
						"session_state_forbidden",
						"The current session cannot access this resource.",
						nil,
						nil,
					),
				)
				return
			}
		}
		ctx.Next()
	}
}

func writeAuthenticationRequired(ctx *gin.Context) {
	response.WriteProblem(
		ctx,
		nethttp.StatusUnauthorized,
		RequestIDFromContext(ctx.Request.Context()),
		response.NewProblem(
			"authentication_required",
			"Authentication is required.",
			nil,
			nil,
		),
	)
}
