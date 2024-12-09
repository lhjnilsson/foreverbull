package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/grpc"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy"

	"go.uber.org/fx"
)

const (
	PostgresRetryInterval = 3
)

var CoreModules = fx.Options( //nolint: gochecknoglobals
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
				log.Printf("failed to ping postgres, retrying in %d seconds", PostgresRetryInterval)
				time.Sleep(time.Second * time.Duration(PostgresRetryInterval))
			}
		},
		container.NewEngine,
		func() (storage.Storage, error) {
			client, err := storage.NewMinioStorage(context.TODO())
			if err != nil {
				return nil, fmt.Errorf("failed to create minio client: %w", err)
			}
			return client, nil
		},
		stream.New,
	),
)

func app() *fx.App {
	return fx.New(
		CoreModules,
		grpc.Module,
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
		Action: func(_ *cli.Context) error {
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
