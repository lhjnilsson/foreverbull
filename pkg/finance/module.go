package finance

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	servicePb "github.com/lhjnilsson/foreverbull/pkg/pb/finance/service"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

const StreamName = "finance"

type Subscriber stream.Subscriber

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func() (supplier.Marketdata, error) {
			switch environment.GetMarketDataProvider() {
			case "alpaca":
				marketData, err := marketdata.NewAlpacaClient()
				if err != nil {
					return nil, fmt.Errorf("failed to create Alpaca client: %w", err)
				}
				return marketData, nil
			case "yahoo":
				marketData, err := marketdata.NewYahooClient()
				if err != nil {
					return nil, fmt.Errorf("failed to create Yahoo client: %w", err)
				}
				return marketData, nil
			default:
				return nil, fmt.Errorf("unknown market data provider: %s", environment.GetMarketDataProvider())
			}
		},
		func() (supplier.Trading, error) {
			if !environment.GetTradingEnabled() {
				return nil, nil
			}

			switch environment.GetTradingProvider() {
			case "alpaca":
				t, err := trading.NewAlpacaClient()
				if err != nil {
					return nil, fmt.Errorf("failed to create Alpaca client: %w", err)
				}
				return t, nil
			default:
				return nil, fmt.Errorf("unknown trading provider: %s", environment.GetTradingProvider())
			}
		},
		func(conn *pgxpool.Pool, marketData supplier.Marketdata, trading supplier.Trading, eventStream stream.Stream) (Subscriber, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			dc.AddSingleton(dependency.MarketDataDep, marketData)
			dc.AddSingleton(dependency.TradingDep, trading)
			sub, err := eventStream.NewSubscriber(StreamName, dc)
			if err != nil {
				return nil, fmt.Errorf("failed to create subscriber: %w", err)
			}
			return sub, nil
		},
	),
	fx.Invoke(
		func(s *grpc.Server, pgx *pgxpool.Pool, eventStream stream.Stream, md supplier.Marketdata, t supplier.Trading) error {
			marketdataServer := api.NewMarketdataServer(pgx, md)
			servicePb.RegisterMarketdataServer(s, marketdataServer)
			tradingServer := api.NewTradingServer(pgx, eventStream, t)
			servicePb.RegisterTradingServicerServer(s, tradingServer)
			return nil
		},
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
		},
		func(lc fx.Lifecycle, sub Subscriber) error {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					err := sub.Subscribe("marketdata", "ingest", command.Ingest)
					if err != nil {
						return fmt.Errorf("failed to subscribe to ingest command: %w", err)
					}
					err = sub.Subscribe("trading", "run", command.SessionRun)
					if err != nil {
						return fmt.Errorf("failed to subscribe to run command: %w", err)
					}
					return nil
				},
				OnStop: func(_ context.Context) error {
					return sub.Unsubscribe()
				},
			})
			return nil
		},
	),
)
