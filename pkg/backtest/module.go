package backtest

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/backtest/service"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

const StreamName = "backtest"

type (
	Subscriber          stream.Subscriber
	DependencyContainer stream.DependencyContainer
)

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func(conn *pgxpool.Pool, st storage.Storage, ce container.Engine, _stream stream.Stream) (Subscriber, DependencyContainer, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			dc.AddSingleton(stream.StorageDep, st)
			dc.AddSingleton(stream.ContainerEngineDep, ce)

			sub, err := _stream.NewSubscriber(StreamName, dc)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create subscriber: %w", err)
			}
			return sub, dc, nil
		},
		func(ce container.Engine) (engine.Engine, error) {
			container, err := ce.Get(context.Background(), environment.GetZiplineContainerName())
			if err != nil {
				return nil, fmt.Errorf("failed to get container for zipline: %w", err)
			}
			health, err := container.GetHealth()
			if err != nil {
				return nil, fmt.Errorf("failed to get health for zipline container: %w", err)
			}
			if health != types.Healthy {
				return nil, errors.New("zipline container is unhealthy")
			}
			engine, err := backtest.NewZiplineEngine(context.Background(), container, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create zipline engine: %w", err)
			}
			return engine, nil
		},
	),
	fx.Invoke(
		func(g *grpc.Server, pgx *pgxpool.Pool, s stream.Stream, st storage.Storage) error {
			backtestServer := api.NewBacktestServer(pgx, s)
			servicePb.RegisterBacktestServicerServer(g, backtestServer)
			return nil
		},
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.TODO(), conn)
		},
		func(lc fx.Lifecycle, sub Subscriber, backtestEngine engine.Engine, dependencies DependencyContainer) error {
			var backtestContainer container.Container
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					returnEngine := func(ctx context.Context, msg stream.Message) (interface{}, error) {
						return backtestEngine, nil
					}
					dependencies.AddMethod(dependency.GetEngineKey, returnEngine)

					err := sub.Subscribe("ingest", "ingest", command.Ingest)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.ingest: %w", err)
					}
					err = sub.Subscribe("session", "run", command.SessionRun)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.start: %w", err)
					}
					return nil
				},
				OnStop: func(ctx context.Context) error {
					if err := backtestContainer.Stop(); err != nil {
						return fmt.Errorf("error stopping container: %w", err)
					}
					return sub.Unsubscribe()
				},
			})
			return nil
		},
	),
)
