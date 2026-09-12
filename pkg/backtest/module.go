package backtest

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/container"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/engine"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/backtest"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	financeapi "github.com/lhjnilsson/foreverbull/pkg/finance/api"
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
		func(ce container.Engine) engine.Engine {
			return backtest.NewLazyEngine(ce)
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
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					returnEngine := func(ctx context.Context, msg stream.Message) (interface{}, error) {
						if lazy, ok := backtestEngine.(*backtest.LazyEngine); ok {
							if err := lazy.Init(ctx); err != nil {
								return nil, fmt.Errorf("failed to initialize zipline engine: %w", err)
							}
						}
						return backtestEngine, nil
					}
					mdclient, err := financeapi.NewMarketdataClient()
					if err != nil {
						return fmt.Errorf("error creating marketdata client: %w", err)
					}
					dependencies.AddMethod(dependency.GetEngineKey, returnEngine)
					dependencies.AddSingleton(dependency.GetETFScraper, dependency.NewETFScaper())
					dependencies.AddSingleton(dependency.GetMarketdataClient, mdclient)

					err = sub.Subscribe("ingest", "ingest", command.Ingest)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.ingest: %w", err)
					}
					err = sub.Subscribe("session", "run", command.SessionRun)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.start: %w", err)
					}
					err = sub.Subscribe("create", "from_etf", command.CreateBacktestFromETF)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.create: %w", err)
					}
					err = sub.Subscribe("create", "from_backtest", command.CreateBacktestFromBacktest)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.create: %w", err)
					}
					err = sub.Subscribe("download", "download", command.DownloadFinancialData)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.download: %w", err)
					}
					return nil
				},
				OnStop: func(ctx context.Context) error {
					return sub.Unsubscribe()
				},
			})
			return nil
		},
	),
)
