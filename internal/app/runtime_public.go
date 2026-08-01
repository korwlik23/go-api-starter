package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/korwlik23/go-api-starter/internal/modules/localization/adapters/bundle"
	localizationgorm "github.com/korwlik23/go-api-starter/internal/modules/localization/adapters/gorm"
	localizationapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application"
	catalogapp "github.com/korwlik23/go-api-starter/internal/modules/localization/application/catalog"
	localizationhttp "github.com/korwlik23/go-api-starter/internal/modules/localization/transport/http"
	operationsgorm "github.com/korwlik23/go-api-starter/internal/modules/operations/adapters/gorm"
	"github.com/korwlik23/go-api-starter/internal/platform/config"
	"github.com/korwlik23/go-api-starter/internal/platform/database"
	"github.com/korwlik23/go-api-starter/internal/platform/httpserver"
	platformmiddleware "github.com/korwlik23/go-api-starter/internal/platform/middleware"
	"github.com/korwlik23/go-api-starter/internal/platform/ratelimit"
	"github.com/korwlik23/go-api-starter/internal/platform/telemetry"
)

const maximumRateLimitClients = 10_000

func buildPublicRuntime(
	ctx context.Context,
	cfg config.Config,
) (HTTPServer, func() error, error) {
	db, err := database.Open(ctx, cfg.Database)
	if err != nil {
		return nil, nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, database.ErrUnavailable
	}
	closeDatabase := sqlDB.Close
	fail := func(cause error) (HTTPServer, func() error, error) {
		_ = closeDatabase()
		return nil, nil, cause
	}
	moduleRegistry, err := Modules()
	if err != nil {
		return fail(err)
	}
	moduleRepository, err := operationsgorm.NewModuleStateRepository(
		db,
		cfg.Database.MigrationLockTimeout,
	)
	if err != nil {
		return fail(err)
	}
	moduleProbe, err := newModuleProjectionProbe(
		moduleRegistry,
		enabledModuleConfiguration(moduleRegistry),
		moduleRepository,
	)
	if err != nil {
		return fail(err)
	}
	if err := moduleProbe(ctx); err != nil {
		return fail(err)
	}
	authorization, err := buildAuthorizationRuntime(db, moduleRepository)
	if err != nil {
		return fail(err)
	}

	localeRepository, err := localizationgorm.NewLocaleRepository(db)
	if err != nil {
		return fail(err)
	}
	catalogRepository, err := localizationgorm.NewCatalogRepository(db, time.Now)
	if err != nil {
		return fail(err)
	}
	listLocales, err := localizationapp.NewListLocales(localeRepository)
	if err != nil {
		return fail(err)
	}
	mergeCatalog, err := catalogapp.NewMerge(catalogRepository, bundle.Repository{})
	if err != nil {
		return fail(err)
	}
	localeHandler, err := localizationhttp.NewLocaleHandler(listLocales)
	if err != nil {
		return fail(err)
	}
	catalogHandler, err := localizationhttp.NewCatalogReadHandler(mergeCatalog)
	if err != nil {
		return fail(err)
	}

	trustedProxies, err := config.ParseTrustedProxyCIDRs(cfg.HTTP.TrustedProxyCIDRs)
	if err != nil {
		return fail(err)
	}
	corsOrigins := make([]string, len(cfg.HTTP.CORSAllowedOrigins))
	for index := range cfg.HTTP.CORSAllowedOrigins {
		corsOrigins[index] = cfg.HTTP.CORSAllowedOrigins[index].String()
	}
	cors, err := platformmiddleware.NewCORS(corsOrigins)
	if err != nil {
		return fail(err)
	}
	limiter, err := ratelimit.NewLimiter(
		map[string]ratelimit.Policy{
			"baseline": {
				Capacity: cfg.RateLimits.Baseline.Capacity,
				Window:   cfg.RateLimits.Baseline.Window,
			},
			"login": {
				Capacity: cfg.RateLimits.Login.Capacity,
				Window:   cfg.RateLimits.Login.Window,
			},
			"recovery": {
				Capacity: cfg.RateLimits.Recovery.Capacity,
				Window:   cfg.RateLimits.Recovery.Window,
			},
			"mfa": {
				Capacity: cfg.RateLimits.MFA.Capacity,
				Window:   cfg.RateLimits.MFA.Window,
			},
		},
		ratelimit.Options{
			Clock:               time.Now,
			MaxEntriesPerPolicy: maximumRateLimitClients,
		},
	)
	if err != nil {
		return fail(err)
	}
	var clientKeySecret [32]byte
	if _, err := rand.Read(clientKeySecret[:]); err != nil {
		return fail(fmt.Errorf("app: client key entropy unavailable"))
	}
	clientKey, err := platformmiddleware.NewHMACClientKeyer(clientKeySecret[:])
	if err != nil {
		return fail(err)
	}
	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
		return fail(fmt.Errorf("app: invalid log level"))
	}
	logger := telemetry.NewLogger(os.Stdout, telemetry.LoggerOptions{
		Service: cfg.App.Name, Environment: string(cfg.App.Environment), Level: level,
	})
	identity, err := buildIdentityRuntime(
		db,
		cfg,
		trustedProxies,
		limiter,
		clientKey,
		logger,
	)
	if err != nil {
		return fail(err)
	}

	readiness := httpserver.NewReadinessGate(httpserver.Readiness{
		Timeout: cfg.Database.ConnectTimeout,
		Probes: []httpserver.Probe{
			{Name: "database", Critical: true, Check: sqlDB.PingContext},
			{Name: "module_projection", Critical: true, Check: moduleProbe},
		},
	})
	router := httpserver.NewRouter(readiness)
	if trustedProxies.Enabled() {
		if err := router.SetTrustedProxies(trustedProxies.Strings()); err != nil {
			return fail(fmt.Errorf("app: invalid trusted proxy configuration"))
		}
	}
	router.Use(platformmiddleware.RequestID(nil), cors, identity.rateLimits.Baseline)
	RegisterPublicSystemRoutes(
		router.Group("/api/v1"),
		localeHandler,
		catalogHandler,
	)
	RegisterIdentityRoutes(router.Group("/api/v1"), identity)
	RegisterAuthorizationRoutes(router.Group("/api/v1"), authorization)
	server := httpserver.NewServer(httpserver.ServerConfig{
		Address: cfg.HTTP.Address, ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ShutdownTimeout: cfg.HTTP.ShutdownTimeout,
	}, router, readiness)
	return server, closeDatabase, nil
}
