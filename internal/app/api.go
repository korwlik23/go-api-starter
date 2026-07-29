package app

import (
	"context"
	"os"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
)

// Run owns the production API lifecycle from validated process configuration
// through graceful server shutdown and dependency cleanup.
func Run(ctx context.Context) error {
	return runApplication(
		ctx,
		func() (config.Config, error) {
			return config.Load(os.LookupEnv)
		},
		config.Validate,
		buildPublicRuntime,
	)
}
