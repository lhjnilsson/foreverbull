package main

import (
	"context"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest"
	"github.com/lhjnilsson/foreverbull/finance"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/log"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service"
	"github.com/lhjnilsson/foreverbull/strategy"

	"go.uber.org/fx"
)

var CoreModules = fx.Options(
	fx.Provide(
		environment.Setup(),
		func() (*pgxpool.Pool, error) {
			return pgxpool.New(context.TODO(), environment.GetPostgresURL())
		},
		storage.NewMinioStorage,
		log.NewLogger,
		stream.NewJetstream,
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
			app().Run()
			return nil
		},
	}
	if err := cli.Run(os.Args); err != nil {
		panic(err)
	}
}
