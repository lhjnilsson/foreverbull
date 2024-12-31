package backtest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/servicer"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"github.com/nats-io/nats.go"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

const StreamName = "backtest"

type (
	Stream             stream.Stream
	DependecyContainer stream.DependencyContainer
)

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func(conn *pgxpool.Pool, st storage.Storage, ce container.Engine) (DependecyContainer, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			dc.AddSingleton(stream.StorageDep, st)
			dc.AddSingleton(stream.ContainerEngineDep, ce)
			// dc.AddMethod(dependency.GetEngineKey, dependency.GetEngine)
			return dc, nil
		},
		func(jt nats.JetStreamContext, conn *pgxpool.Pool, dc DependecyContainer) (Stream, error) {
			s, err := stream.NewNATSStream(jt, StreamName, dc, conn)
			if err != nil {
				return nil, fmt.Errorf("failed to create stream: %w", err)
			}
			return s, nil
		},
	),
	fx.Invoke(
		func(g *grpc.Server, pgx *pgxpool.Pool, s Stream, st storage.Storage) error {
			backtestServer := servicer.NewBacktestServer(pgx, s)
			pb.RegisterBacktestServicerServer(g, backtestServer)
			ingestionServer := servicer.NewIngestionServer(s, st, pgx)
			pb.RegisterIngestionServicerServer(g, ingestionServer)
			return nil
		},
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.TODO(), conn)
		},
		func(lc fx.Lifecycle, backtestStream Stream, containers container.Engine, dependencies DependecyContainer) error {
			var backtestContainer container.Container
			var backtestEngine engine.Engine
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					var err error
					backtestContainer, err = containers.Start(ctx, environment.GetBacktestImage(), "")
					if err != nil {
						return fmt.Errorf("error starting container: %w", err)
					}

					for range 30 {
						health, err := backtestContainer.GetHealth()
						if err != nil {
							return fmt.Errorf("error getting container health: %w", err)
						}
						if health == types.Healthy {
							break
						} else if health == types.Unhealthy {
							return errors.New("container is unhealthy")
						}
						time.Sleep(time.Second / 3) //nolint: gomnd
					}
					backtestEngine, err = backtest.NewZiplineEngine(ctx, backtestContainer, nil)
					if err != nil {
						return fmt.Errorf("error creating zipline engine: %w", err)
					}
					returnEngine := func(ctx context.Context, msg stream.Message) (interface{}, error) {
						return backtestEngine, nil
					}
					dependencies.AddMethod(dependency.GetEngineKey, returnEngine)

					err = backtestStream.CommandSubscriber("ingest", "ingest", command.Ingest)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.ingest: %w", err)
					}
					err = backtestStream.CommandSubscriber("session", "run", command.SessionRun)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.start: %w", err)
					}
					err = backtestStream.CommandSubscriber("status", "update", command.UpdateIngestionStatus)
					if err != nil {
						return fmt.Errorf("error subscribing to ingestion.update: %w", err)
					}
					return nil
				},
				OnStop: func(ctx context.Context) error {
					if err := backtestContainer.Stop(); err != nil {
						return fmt.Errorf("error stopping container: %w", err)
					}
					return backtestStream.Unsubscribe()
				},
			})
			return nil
		},
	),
)
