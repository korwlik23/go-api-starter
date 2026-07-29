package http

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
)

var ErrInvalidRateLimitDependencies = errors.New(
	"invalid identity rate limit dependencies",
)

type RateLimitDependencies struct {
	Limiter        platformmiddleware.RateLimitDecider
	TrustedProxies config.TrustedProxySet
	ClientKey      platformmiddleware.ClientKeyer
	Logger         *slog.Logger
}

type RateLimits struct {
	Baseline gin.HandlerFunc
	Login    gin.HandlerFunc
	Recovery gin.HandlerFunc
	MFA      gin.HandlerFunc
}

func NewRateLimits(dependencies RateLimitDependencies) (RateLimits, error) {
	if dependencies.Limiter == nil ||
		dependencies.ClientKey == nil ||
		dependencies.Logger == nil {
		return RateLimits{}, ErrInvalidRateLimitDependencies
	}

	build := func(policy string) gin.HandlerFunc {
		return platformmiddleware.RateLimit(platformmiddleware.RateLimitOptions{
			Policy:         policy,
			Limiter:        dependencies.Limiter,
			TrustedProxies: dependencies.TrustedProxies,
			ClientKey:      dependencies.ClientKey,
			Logger:         dependencies.Logger,
		})
	}
	return RateLimits{
		Baseline: build("baseline"),
		Login:    build("login"),
		Recovery: build("recovery"),
		MFA:      build("mfa"),
	}, nil
}
