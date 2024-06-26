package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy"

	"go.uber.org/fx"
)

var CoreModules = fx.Options(
	fx.Provide(
		func() (*pgxpool.Pool, error) {
			return pgxpool.New(context.TODO(), environment.GetPostgresURL())
		},
		storage.NewMinioStorage,
		stream.New,
		http.NewEngine,
	),
	fx.Invoke(
		http.NewLifeCycleRouter,
	),
)

func app() *fx.App {
	return fx.New(
		CoreModules,
		backtest.Module,
		finance.Module,
		service.Module,
		strategy.Module,
		stream.OrchestrationLifecycle,
	)
}

func main() {
	cli := &cli.App{
		Name: "foreverbull",
		Action: func(c *cli.Context) error {
			if err := environment.Setup(); err != nil {
				return fmt.Errorf("failed to setup environment: %w", err)
			}
			app().Run()
			return nil
		},
	}
	if err := cli.Run(os.Args); err != nil {
		panic(err)
	}
}
