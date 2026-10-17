package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/grpc"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/internal/web"
	"github.com/lhjnilsson/foreverbull/pkg/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/finance"
	"github.com/lhjnilsson/foreverbull/pkg/service"
	"github.com/lhjnilsson/foreverbull/pkg/strategy"

	"go.uber.org/fx"
)

// The broker can start before Postgres in a fresh cluster, so a few failed
// pings are expected. About a minute of retries covers a cold start, and
// past that a wrong POSTGRES_URL should stop the process with an error that
// names the host instead of looping forever.
const (
	postgresPingAttempts  = 20
	postgresRetryInterval = 3 * time.Second
	postgresPingTimeout   = 5 * time.Second
)

func newPostgresPool() (*pgxpool.Pool, error) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, environment.GetPostgresURL())
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool from %s: %w", environment.PostgresURL, err)
	}

	if err := pingPostgres(ctx, pool, postgresPingAttempts, postgresRetryInterval); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

// pingPostgres pings at most attempts times, waiting interval between tries.
// The error names host and port only, so the password in the URL never
// reaches a log.
func pingPostgres(ctx context.Context, pool *pgxpool.Pool, attempts int, interval time.Duration) error {
	connConfig := pool.Config().ConnConfig
	address := net.JoinHostPort(connConfig.Host, strconv.Itoa(int(connConfig.Port)))

	var err error

	for attempt := 1; attempt <= attempts; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, postgresPingTimeout)
		err = pool.Ping(pingCtx)
		cancel()

		if err == nil {
			return nil
		}

		if attempt < attempts {
			if attempt == 1 {
				log.Printf(
					"postgres at %s is not reachable (attempt %d of %d), retrying in %s; if this persists, check %s and that Postgres is running",
					address, attempt, attempts, interval, environment.PostgresURL,
				)
			} else {
				log.Printf("postgres at %s is not reachable (attempt %d of %d), retrying in %s", address, attempt, attempts, interval)
			}
			time.Sleep(interval)
		}
	}

	return fmt.Errorf(
		"could not reach postgres at %s after %d attempts; check %s and that Postgres is running: %w",
		address, attempts, environment.PostgresURL, err,
	)
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
		web.Module,
		backtest.Module,
		finance.Module,
		service.Module,
		strategy.Module,
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
		fmt.Fprintf(os.Stderr, "foreverbull: %v\n", err)
		os.Exit(1)
	}
}
