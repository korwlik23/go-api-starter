package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

const RequestIDHeader = "X-Request-ID"

const maxRequestIDLength = 128

type RequestIDGenerator func() string

type requestIDContextKey struct{}

// RequestID accepts a constrained caller-provided request ID or generates a
// fresh opaque ID, then makes the selected value available to downstream code.
func RequestID(generator RequestIDGenerator) gin.HandlerFunc {
	if generator == nil {
		generator = generateRequestID
	}

	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader(RequestIDHeader)
		if !isValidRequestID(requestID) {
			requestID = generator()
			if !isValidRequestID(requestID) {
				panic("request ID generator returned an invalid value")
			}
		}

		requestContext := context.WithValue(
			ctx.Request.Context(),
			requestIDContextKey{},
			requestID,
		)
		ctx.Request = ctx.Request.WithContext(requestContext)
		ctx.Header(RequestIDHeader, requestID)
		ctx.Next()
	}
}

func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

func isValidRequestID(requestID string) bool {
	if len(requestID) == 0 || len(requestID) > maxRequestIDLength {
		return false
	}
	if !isASCIIAlphaNumeric(requestID[0]) {
		return false
	}

	for index := 1; index < len(requestID); index++ {
		character := requestID[index]
		if isASCIIAlphaNumeric(character) {
			continue
		}
		switch character {
		case '-', '_', '.', ':':
			continue
		default:
			return false
		}
	}
	return true
}

func isASCIIAlphaNumeric(character byte) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9'
}

func generateRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("request ID entropy unavailable")
	}
	return hex.EncodeToString(value[:])
}
