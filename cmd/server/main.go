package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

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
			pool, err := pgxpool.New(context.TODO(), environment.GetPostgresURL())
			if err != nil {
				return nil, fmt.Errorf("failed to create postgres pool: %w", err)
			}
			for {
				err = pool.Ping(context.TODO())
				if err == nil {
					return pool, nil
				}
				log.Printf("failed to ping postgres, retrying in %d seconds", 3)
				time.Sleep(time.Second * time.Duration(3))
			}
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
