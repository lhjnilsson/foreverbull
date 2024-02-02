package finance

import (
	"context"
	"fmt"

	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/nats-io/nats.go"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/finance/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/finance/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/finance/supplier"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const Stream = "finance"

type FinanceStream stream.Stream
type FinanceAPI struct {
	*gin.RouterGroup
}

var Module = fx.Options(
	fx.Provide(
		func() (supplier.Marketdata, supplier.Trading, error) {
			md, err := marketdata.NewAlpacaClient()
			if err != nil {
				return nil, nil, err
			}
			t, err := trading.NewAlpacaClient()
			if err != nil {
				return nil, nil, err
			}
			return md, t, nil
		},
		func(jt nats.JetStreamContext, log *zap.Logger, conn *pgxpool.Pool, md supplier.Marketdata) (FinanceStream, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingelton(stream.DBDep, conn)
			dc.AddSingelton(dependency.MarketDataDep, md)
			s, err := stream.NewNATSStream(jt, Stream, log, dc, conn)
			if err != nil {
				return nil, fmt.Errorf("failed to create stream: %w", err)
			}
			return s, nil
		},
		func(gin *gin.Engine) *FinanceAPI {
			return &FinanceAPI{gin.Group("/finance/api")}
		},
	),
	fx.Invoke(
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
		},
		func(financeAPI *FinanceAPI, pgxpool *pgxpool.Pool, log *zap.Logger, marketdata supplier.Marketdata) error {
			return nil
		},
		func(lc fx.Lifecycle, s FinanceStream, log *zap.Logger, pgxpool *pgxpool.Pool, marketdata supplier.Marketdata) error {
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
