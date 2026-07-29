package app

import (
	"context"
	"errors"

	"github.com/korwlik23/go-api-starter/internal/platform/config"
)

type configLoader func() (config.Config, error)
type configValidator func(config.Config) error
type runtimeBuilder func(
	context.Context,
	config.Config,
) (HTTPServer, func() error, error)

func runApplication(
	ctx context.Context,
	load configLoader,
	validate configValidator,
	build runtimeBuilder,
) error {
	if ctx == nil || load == nil || validate == nil || build == nil {
		return errors.New("app: invalid platform composition")
	}
	cfg, err := load()
	if err != nil {
		return err
	}
	if err := validate(cfg); err != nil {
		return err
	}
	server, closeDependencies, err := build(ctx, cfg)
	if err != nil {
		return err
	}
	if server == nil {
		if closeDependencies != nil {
			_ = closeDependencies()
		}
		return errHTTPServerRequired
	}

	runErr := server.Run(ctx)
	if closeDependencies == nil {
		return runErr
	}
	return errors.Join(runErr, closeDependencies())
}
