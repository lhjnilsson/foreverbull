package finance

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/servicer"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/pkg/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	"github.com/lhjnilsson/foreverbull/pkg/finance/supplier"
	"github.com/nats-io/nats.go"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

const Stream = "finance"

type FinanceStream stream.Stream

var Module = fx.Options(
	fx.Provide(

		func() (supplier.Marketdata, supplier.Trading, error) {
			if environment.GetAlpacaAPIKey() == "" || environment.GetAlpacaAPISecret() == "" {
				marketData, err := marketdata.NewYahooClient()
				if err != nil {
					return nil, nil, fmt.Errorf("failed to create Yahoo client: %w", err)
				}
				return marketData, nil, nil
			} else {
				marketData, err := marketdata.NewAlpacaClient()
				if err != nil {
					return nil, nil, fmt.Errorf("failed to create Alpaca client: %w", err)
				}
				t, err := trading.NewAlpacaClient()
				if err != nil {
					return nil, nil, fmt.Errorf("failed to create Alpaca client: %w", err)
				}
				return marketData, t, nil
			}
		},
		func(jt nats.JetStreamContext, conn *pgxpool.Pool, marketData supplier.Marketdata) (FinanceStream, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			dc.AddSingleton(dependency.MarketDataDep, marketData)
			s, err := stream.NewNATSStream(jt, Stream, dc, conn)
			if err != nil {
				return nil, fmt.Errorf("failed to create stream: %w", err)
			}
			return s, nil
		},
	),
	fx.Invoke(
		func(s *grpc.Server, pgx *pgxpool.Pool, md supplier.Marketdata) error {
			server := servicer.NewFinanceServer(pgx, md)
			pb.RegisterFinanceServer(s, server)
			return nil
		},
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
		},
		func(lc fx.Lifecycle, s FinanceStream, pgxpool *pgxpool.Pool, marketdata supplier.Marketdata) error {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					err := s.CommandSubscriber("marketdata", "ingest", command.Ingest)
					if err != nil {
						return fmt.Errorf("failed to subscribe to ingest command: %w", err)
					}
					return nil
				},
				OnStop: func(ctx context.Context) error {
					return s.Unsubscribe()
				},
			})
			return nil
		},
	),
)
