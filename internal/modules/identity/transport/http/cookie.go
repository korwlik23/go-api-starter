package http

import (
	"errors"
	nethttp "net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var ErrInvalidSessionCookie = errors.New("invalid session cookie configuration")

type SessionCookieConfig struct {
	Name       string
	Production bool
	Secure     bool
}

type SessionCookie struct {
	name   string
	secure bool
}

func NewSessionCookie(config SessionCookieConfig) (*SessionCookie, error) {
	if config.Name == "" || strings.HasPrefix(config.Name, "__Host-") {
		return nil, ErrInvalidSessionCookie
	}
	probe := nethttp.Cookie{Name: config.Name, Value: "probe"}
	if err := probe.Valid(); err != nil {
		return nil, ErrInvalidSessionCookie
	}

	name := config.Name
	secure := config.Secure
	if config.Production {
		name = "__Host-" + name
		secure = true
	}

	return &SessionCookie{name: name, secure: secure}, nil
}

func (cookie *SessionCookie) Write(ctx *gin.Context, token string) {
	nethttp.SetCookie(ctx.Writer, &nethttp.Cookie{
		Name:     cookie.name,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   cookie.secure,
		SameSite: nethttp.SameSiteLaxMode,
	})
}
