package finance

import (
	"context"
	"fmt"

	"github.com/gin-contrib/logger"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	apiDef "github.com/lhjnilsson/foreverbull/finance/api"
	"github.com/lhjnilsson/foreverbull/finance/internal/api"
	"github.com/lhjnilsson/foreverbull/finance/internal/repository"
	"github.com/lhjnilsson/foreverbull/finance/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/finance/internal/stream/dependency"
	"github.com/lhjnilsson/foreverbull/finance/internal/suppliers/marketdata"
	"github.com/lhjnilsson/foreverbull/finance/internal/suppliers/trading"
	"github.com/lhjnilsson/foreverbull/finance/supplier"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
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
		func() (apiDef.Client, error) {
			return apiDef.NewClient()
		},
		func(jt nats.JetStreamContext, conn *pgxpool.Pool, md supplier.Marketdata) (FinanceStream, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			dc.AddSingleton(dependency.MarketDataDep, md)
			s, err := stream.NewNATSStream(jt, Stream, dc, conn)
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
		func(financeAPI *FinanceAPI, pgxpool *pgxpool.Pool, marketdata supplier.Marketdata, trading supplier.Trading) error {
			financeAPI.Use(
				logger.SetLogger(logger.WithLogger(func(ctx *gin.Context, l zerolog.Logger) zerolog.Logger {
					return log.Logger
				}),
				),
				func(ctx *gin.Context) {
					ctx.Set(api.TradingDependency, trading)
					ctx.Next()
				},
			)
			financeAPI.GET("/portfolio", api.GetPortfolio)
			return nil
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
