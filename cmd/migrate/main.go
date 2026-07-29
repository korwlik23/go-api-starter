package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/korwlik23/go-api-starter/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := app.Migrate(ctx, os.LookupEnv); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
}
