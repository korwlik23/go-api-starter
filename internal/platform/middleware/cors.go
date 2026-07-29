package middleware

import (
	"errors"
	nethttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	corsAllowedMethods = "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowedHeaders = "Content-Type, X-CSRF-Token, X-Request-ID"
)

var ErrInvalidCORSConfig = errors.New("invalid CORS configuration")

func NewCORS(allowedOrigins []string) (gin.HandlerFunc, error) {
	if len(allowedOrigins) == 0 {
		return nil, ErrInvalidCORSConfig
	}

	allowed := make(map[requestOrigin]string, len(allowedOrigins))
	for _, configured := range allowedOrigins {
		if configured == "*" {
			return nil, ErrInvalidCORSConfig
		}
		origin, err := parseOrigin(configured)
		if err != nil {
			return nil, ErrInvalidCORSConfig
		}
		canonical := origin.scheme + "://" + origin.host
		if _, duplicate := allowed[origin]; duplicate {
			return nil, ErrInvalidCORSConfig
		}
		allowed[origin] = canonical
	}

	return func(ctx *gin.Context) {
		ctx.Writer.Header().Add("Vary", "Origin")

		originValues := ctx.Request.Header.Values("Origin")
		requestOrigin, originAllowed := allowedRequestOrigin(originValues, allowed)
		isPreflight := ctx.Request.Method == nethttp.MethodOptions &&
			ctx.GetHeader("Access-Control-Request-Method") != ""

		if isPreflight {
			if !originAllowed || !validPreflightRequest(ctx.Request) {
				ctx.AbortWithStatus(nethttp.StatusForbidden)
				return
			}
			writeCORSHeaders(ctx, requestOrigin)
			ctx.Header("Access-Control-Allow-Methods", corsAllowedMethods)
			ctx.Header("Access-Control-Allow-Headers", corsAllowedHeaders)
			ctx.AbortWithStatus(nethttp.StatusNoContent)
			return
		}

		if originAllowed {
			writeCORSHeaders(ctx, requestOrigin)
		}
		ctx.Next()
	}, nil
}

func allowedRequestOrigin(
	values []string,
	allowed map[requestOrigin]string,
) (string, bool) {
	if len(values) != 1 {
		return "", false
	}
	origin, err := parseOrigin(values[0])
	if err != nil {
		return "", false
	}
	canonical, ok := allowed[origin]
	return canonical, ok
}

func validPreflightRequest(request *nethttp.Request) bool {
	method := request.Header.Get("Access-Control-Request-Method")
	switch method {
	case nethttp.MethodGet,
		nethttp.MethodHead,
		nethttp.MethodPost,
		nethttp.MethodPut,
		nethttp.MethodPatch,
		nethttp.MethodDelete,
		nethttp.MethodOptions:
	default:
		return false
	}

	allowedHeaders := map[string]struct{}{
		"content-type": {},
		"x-csrf-token": {},
		"x-request-id": {},
	}
	for _, requested := range strings.Split(
		request.Header.Get("Access-Control-Request-Headers"),
		",",
	) {
		header := strings.ToLower(strings.TrimSpace(requested))
		if header == "" {
			continue
		}
		if _, ok := allowedHeaders[header]; !ok {
			return false
		}
	}
	return true
}

func writeCORSHeaders(ctx *gin.Context, origin string) {
	ctx.Header("Access-Control-Allow-Origin", origin)
	ctx.Header("Access-Control-Allow-Credentials", "true")
}
