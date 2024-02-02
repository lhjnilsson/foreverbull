package backtest

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/backtest/internal/stream/dependency"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/nats-io/nats.go"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const Stream = "backtest"

type BacktestStream stream.Stream
type BacktestAPI struct {
	*gin.RouterGroup
}

var Module = fx.Options(
	fx.Provide(
		func(jt nats.JetStreamContext, log *zap.Logger, conn *pgxpool.Pool) (BacktestStream, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingelton(stream.DBDep, conn)
			dc.AddSingelton(stream.LoggerDep, log)
			httpClient := dependency.GetHTTPClient()
			dc.AddSingelton(dependency.GetHTTPClientKey, httpClient)
			dc.AddMethod(dependency.GetBacktestEngineKey, dependency.GetBacktestEngine)
			dc.AddMethod(dependency.GetBacktestSessionKey, dependency.GetBacktestSession)
			s, err := stream.NewNATSStream(jt, Stream, log, dc, conn)
			if err != nil {
				return nil, fmt.Errorf("failed to create stream: %w", err)
			}
			return s, nil
		},
		func(gin *gin.Engine) *BacktestAPI {
			return &BacktestAPI{gin.Group("/backtest/api")}
		},
	),
	fx.Invoke(
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.TODO(), conn)
		},
		func(backtestAPI *BacktestAPI, pgxpool *pgxpool.Pool, log *zap.Logger, stream BacktestStream, storage storage.BlobStorage) error {
			backtestAPI.Use(
				internalHTTP.OrchestrationMiddleware(api.OrchestrationDependency, stream),
				internalHTTP.TransactionMiddleware(api.TXDependency, pgxpool),
				func(ctx *gin.Context) {
					ctx.Set(api.LoggingDependency, log)
					ctx.Set(api.StorageDependency, storage)
					ctx.Next()
				},
			)
			backtestAPI.GET("/backtests", api.ListBacktests)
			backtestAPI.POST("/backtests", api.CreateBacktest)
			backtestAPI.GET("/backtests/:name", api.GetBacktest)
			backtestAPI.PUT("/backtests/:name", api.UpdateBacktest)
			backtestAPI.DELETE("/backtests/:name", api.DeleteBacktest)

			backtestAPI.GET("/sessions", api.ListSessions)
			backtestAPI.POST("/sessions", api.CreateSession)
			backtestAPI.GET("/sessions/:id", api.GetSession)

			backtestAPI.GET("/executions", api.ListExecutions)
			backtestAPI.GET("/executions/:id", api.GetExecution)
			backtestAPI.GET("/executions/:id/orders", api.GetExecutionOrders)
			backtestAPI.GET("/executions/:id/portfolio", api.GetExecutionPortfolio)
			backtestAPI.GET("/executions/:id/periods", api.GetExecutionPeriods)
			backtestAPI.GET("/executions/:id/periods/metrics", api.GetExecutionPeriodMetrics)
			backtestAPI.GET("/executions/:id/periods/metrics/:metric", api.GetExecutionPeriodMetric)
			backtestAPI.GET("/executions/:id/dataframe", api.GetExecutionDataframe)
			return nil
		},
		func(storage storage.BlobStorage) error {
			return storage.VerifyBuckets(context.TODO())
		},
		func(lc fx.Lifecycle, s BacktestStream, log *zap.Logger, conn *pgxpool.Pool) error {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					err := s.CommandSubscriber("backtest", "ingest", command.BacktestIngest)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.ingest: %w", err)
					}
					err = s.CommandSubscriber("session", "run", command.SessionRun)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.start: %w", err)
					}
					err = s.CommandSubscriber("backtest", "status", command.UpdateBacktestStatus)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.status: %w", err)
					}
					err = s.CommandSubscriber("session", "status", command.UpdateSessionStatus)
					if err != nil {
						return fmt.Errorf("error subscribing to session.status: %w", err)
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
