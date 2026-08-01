package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProductionSessionCookieUsesHostPrefixAndSecureAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cookie, err := NewSessionCookie(SessionCookieConfig{
		Name:       "go_lang_starter_session",
		Production: true,
	})
	require.NoError(t, err)

	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(nethttp.MethodGet, "/", nil)
	cookie.Write(ctx, "opaque-token")

	result := response.Result().Cookies()
	require.Len(t, result, 1)
	require.Equal(t, "__Host-go_lang_starter_session", result[0].Name)
	require.Equal(t, "opaque-token", result[0].Value)
	require.Equal(t, "/", result[0].Path)
	require.Empty(t, result[0].Domain)
	require.True(t, result[0].HttpOnly)
	require.True(t, result[0].Secure)
	require.Equal(t, nethttp.SameSiteLaxMode, result[0].SameSite)
	require.NotContains(t, strings.ToLower(response.Header().Get("Set-Cookie")), "domain=")
}

func TestDevelopmentSessionCookieSupportsLocalHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cookie, err := NewSessionCookie(SessionCookieConfig{
		Name:       "go_lang_starter_session",
		Production: false,
		Secure:     false,
	})
	require.NoError(t, err)

	response := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(response)
	ctx.Request = httptest.NewRequest(nethttp.MethodGet, "/", nil)
	cookie.Write(ctx, "opaque-token")

	result := response.Result().Cookies()
	require.Len(t, result, 1)
	require.Equal(t, "go_lang_starter_session", result[0].Name)
	require.False(t, result[0].Secure)
	require.True(t, result[0].HttpOnly)
	require.Equal(t, nethttp.SameSiteLaxMode, result[0].SameSite)
}

func TestSessionCookieExposesTheEffectiveEnvironmentName(t *testing.T) {
	development, err := NewSessionCookie(SessionCookieConfig{
		Name: "go_lang_starter_session",
	})
	require.NoError(t, err)
	require.Equal(t, "go_lang_starter_session", development.Name())

	production, err := NewSessionCookie(SessionCookieConfig{
		Name:       "go_lang_starter_session",
		Production: true,
	})
	require.NoError(t, err)
	require.Equal(t, "__Host-go_lang_starter_session", production.Name())
}

func TestSessionCookieRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{"", "__Host-existing-prefix", "contains space"} {
		_, err := NewSessionCookie(SessionCookieConfig{Name: name})
		require.ErrorIs(t, err, ErrInvalidSessionCookie)
	}
}

func TestSessionCookieClearExpiresTheSameSecureScope(t *testing.T) {
	t.Parallel()

	cookie, err := NewSessionCookie(SessionCookieConfig{
		Name:       "session",
		Production: true,
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	cookie.Clear(ctx)

	result := recorder.Result().Cookies()
	require.Len(t, result, 1)
	require.Equal(t, "__Host-session", result[0].Name)
	require.Empty(t, result[0].Value)
	require.Equal(t, -1, result[0].MaxAge)
	require.True(t, result[0].HttpOnly)
	require.True(t, result[0].Secure)
	require.Equal(t, "/", result[0].Path)
	require.Equal(t, nethttp.SameSiteLaxMode, result[0].SameSite)
}
