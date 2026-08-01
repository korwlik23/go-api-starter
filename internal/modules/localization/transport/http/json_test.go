package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDecodeStrictJSONRejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/locales", strings.NewReader(`{"code":"fr","unexpected":true}`))

	var payload struct {
		Code string `json:"code"`
	}
	err := decodeStrictJSON(ctx, &payload)

	require.Error(t, err)
}

func TestDecodeStrictJSONRejectsTrailingValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/locales", strings.NewReader(`{"code":"fr"}{"code":"th"}`))

	var payload struct {
		Code string `json:"code"`
	}
	err := decodeStrictJSON(ctx, &payload)

	require.Error(t, err)
}
