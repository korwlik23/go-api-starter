package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders adds browser-side hardening that is safe for JSON API
// responses and health endpoints. HSTS is intentionally not emitted here:
// TLS is terminated by the edge proxy in production, while emitting HSTS for
// the local HTTP profile would unexpectedly pin localhost to HTTPS.
func SecurityHeaders() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("X-Content-Type-Options", "nosniff")
		ctx.Header("X-Frame-Options", "DENY")
		ctx.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		ctx.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		ctx.Next()
	}
}
