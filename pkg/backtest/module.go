package backtest

import (
	"context"
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
	internalBacktest "github.com/lhjnilsson/foreverbull/pkg/backtest/internal/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/servicer"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/pb"
	"github.com/nats-io/nats.go"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

const Stream = "backtest"

type BacktestStream stream.Stream
type DependecyContainer stream.DependencyContainer

var Module = fx.Options(
	fx.Provide(
		func(ce container.Engine) (engine.Engine, error) {
			return internalBacktest.NewZiplineEngine(context.Background(), nil, nil)
		},
		func(conn *pgxpool.Pool, st storage.Storage, ce container.Engine) (DependecyContainer, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			dc.AddSingleton(stream.StorageDep, st)
			dc.AddSingleton(stream.ContainerEngineDep, ce)
			// dc.AddMethod(dependency.GetEngineKey, dependency.GetEngine)
			return dc, nil
		},
		func(jt nats.JetStreamContext, conn *pgxpool.Pool, st storage.Storage, dc DependecyContainer) (BacktestStream, error) {
			s, err := stream.NewNATSStream(jt, Stream, dc, conn)
			if err != nil {
				return nil, fmt.Errorf("failed to create stream: %w", err)
			}
			return s, nil
		},
	),
	fx.Invoke(
		func(g *grpc.Server, pgx *pgxpool.Pool, s BacktestStream, st storage.Storage) error {
			backtestServer := servicer.NewBacktestServer(pgx, s)
			pb.RegisterBacktestServicerServer(g, backtestServer)
			ingestionServer := servicer.NewIngestionServer(s, st, pgx)
			pb.RegisterIngestionServicerServer(g, ingestionServer)
			return nil
		},
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.TODO(), conn)
		},
		func(lc fx.Lifecycle, s BacktestStream, conn *pgxpool.Pool, ce container.Engine, dc DependecyContainer) error {
			var backtestContainer container.Container
			var backtestEngine engine.Engine
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					var err error
					backtestContainer, err = ce.Start(ctx, environment.GetBacktestImage(), "")
					if err != nil {
						return fmt.Errorf("error starting container: %v", err)
					}

					for i := 0; i < 30; i++ {
						health, err := backtestContainer.GetHealth()
						if err != nil {
							return fmt.Errorf("error getting container health: %v", err)
						}
						if health == types.Healthy {
							break
						} else if health == types.Unhealthy {
							return fmt.Errorf("container is unhealthy")
						}
						time.Sleep(time.Second / 3)
					}
					backtestEngine, err = backtest.NewZiplineEngine(ctx, backtestContainer, nil)
					if err != nil {
						return fmt.Errorf("error creating zipline engine: %v", err)
					}
					returnEngine := func(ctx context.Context, msg stream.Message) (interface{}, error) {
						return backtestEngine, nil
					}
					dc.AddMethod(dependency.GetEngineKey, returnEngine)

					err = s.CommandSubscriber("ingest", "ingest", command.Ingest)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.ingest: %w", err)
					}
					err = s.CommandSubscriber("session", "run", command.SessionRun)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.start: %w", err)
					}
					return nil
				},
				OnStop: func(ctx context.Context) error {
					if err := backtestEngine.Stop(ctx); err != nil {
						return fmt.Errorf("error stopping zipline engine: %v", err)
					}
					if err := backtestContainer.Stop(); err != nil {
						return fmt.Errorf("error stopping container: %v", err)
					}
					return s.Unsubscribe()
				},
			})
			return nil
		},
	),
)
