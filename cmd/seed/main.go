package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/korwlik23/go-api-starter/internal/app"
	"github.com/korwlik23/go-api-starter/internal/seed"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	seedConfig, err := seed.ParseConfig(os.LookupEnv)
	if err != nil {
		slog.Error("seed configuration failed", "error", err)
		os.Exit(1)
	}
	if seedConfig.Password == "" {
		slog.Error("seed configuration failed", "error", "SEED_PASSWORD or BOOTSTRAP_PASSWORD is required")
		os.Exit(1)
	}
	ids := seed.NewIDs()
	ids.Account = seedConfig.AccountID
	ids.OwnerUser = ids.User("owner")
	overrides := map[string]string{
		"BOOTSTRAP_EMAIL":                         seedConfig.Email,
		"BOOTSTRAP_PASSWORD":                      seedConfig.Password,
		"BOOTSTRAP_ACCOUNT_SLUG":                  seedConfig.AccountSlug,
		"BOOTSTRAP_ACCOUNT_ROLE_NAME":             "Owner",
		"BOOTSTRAP_SYSTEM_ROLE_NAME":              "Owner System",
		"BOOTSTRAP_ACCOUNT_ID":                    seedConfig.AccountID,
		"BOOTSTRAP_USER_ID":                       ids.OwnerUser,
		"BOOTSTRAP_MEMBERSHIP_ID":                 ids.Membership("owner"),
		"BOOTSTRAP_ACCOUNT_ROLE_ID":               ids.Role("owner"),
		"BOOTSTRAP_SYSTEM_ROLE_ID":                ids.SystemRole("owner"),
		"BOOTSTRAP_MEMBERSHIP_ROLE_ASSIGNMENT_ID": ids.Assignment("account/owner"),
		"BOOTSTRAP_SYSTEM_ROLE_ASSIGNMENT_ID":     ids.Assignment("system/owner"),
	}
	lookup := func(key string) (string, bool) {
		if value, ok := overrides[key]; ok {
			return value, true
		}
		return os.LookupEnv(key)
	}
	bootstrapResult, err := app.RunBootstrap(ctx, lookup)
	if err != nil {
		slog.Error("seed foundation failed", "error", err)
		os.Exit(1)
	}
	result, err := seed.Run(ctx, os.LookupEnv)
	if err != nil {
		slog.Error("seed demo failed", "error", err)
		os.Exit(1)
	}
	slog.Info("seed complete", "profile", result.Profile, "account_id", bootstrapResult.AccountID)
}
