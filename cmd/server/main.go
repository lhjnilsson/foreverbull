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

	"go.uber.org/fx"
)

const postgresRetryInterval = 3 * time.Second

func newPostgresPool() (*pgxpool.Pool, error) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	for {
		if err := pool.Ping(ctx); err == nil {
			return pool, nil
		}

		log.Printf("failed to ping postgres, retrying in %s", postgresRetryInterval)
		time.Sleep(postgresRetryInterval)
	}
}

func newObjectStorage() (storage.Storage, error) {
	client, err := storage.NewMinioStorage(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return client, nil
}

var coreModules = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		newPostgresPool,
		container.NewEngine,
		newObjectStorage,
	),
)

func app() *fx.App {
	return fx.New(
		coreModules,
		grpc.Module,
		backtest.Module,
		finance.Module,
		service.Module,
		stream.Module,
	)
}

func main() {
	cliApp := &cli.App{
		Name: "foreverbull",
		Action: func(_ *cli.Context) error {
			if err := environment.Setup(); err != nil {
				return fmt.Errorf("failed to setup environment: %w", err)
			}
			app().Run()
			return nil
		},
	}
	if err := cliApp.Run(os.Args); err != nil {
		panic(err)
	}
}
