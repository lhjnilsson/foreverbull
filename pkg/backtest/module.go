package backtest

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/storage"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/api"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/backtest/internal/stream/dependency"
	serviceAPI "github.com/lhjnilsson/foreverbull/pkg/service/api"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

const Stream = "backtest"

type BacktestStream stream.Stream
type BacktestAPI struct {
	*gin.RouterGroup
}

var Module = fx.Options(
	fx.Provide(
		func(jt nats.JetStreamContext, conn *pgxpool.Pool) (BacktestStream, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			serviceAPI, err := serviceAPI.NewClient()
			if err != nil {
				return nil, fmt.Errorf("failed to create service api client: %w", err)
			}
			dc.AddSingleton(dependency.GetServiceAPI, serviceAPI)
			httpClient := dependency.GetHTTPClient()
			dc.AddSingleton(dependency.GetHTTPClientKey, httpClient)
			dc.AddMethod(dependency.GetIngestEngineKey, dependency.GetIngestEngine)
			dc.AddMethod(dependency.GetBacktestSessionKey, dependency.GetBacktestSession)
			s, err := stream.NewNATSStream(jt, Stream, dc, conn)
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
		func(backtestAPI *BacktestAPI, pgxpool *pgxpool.Pool, stream BacktestStream, storage storage.BlobStorage) error {
			backtestAPI.Use(
				internalHTTP.OrchestrationMiddleware(api.OrchestrationDependency, stream),
				internalHTTP.TransactionMiddleware(api.TXDependency, pgxpool),
				func(ctx *gin.Context) {
					ctx.Set(api.StorageDependency, storage)
					ctx.Next()
				},
			)
			backtestAPI.POST("/ingestion", api.CreateIngestion)
			backtestAPI.GET("/ingestion", api.GetIngestion)

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
			backtestAPI.GET("/executions/:id/periods", api.GetExecutionPeriods)
			backtestAPI.GET("/executions/:id/periods/metrics", api.GetExecutionPeriodMetrics)
			backtestAPI.GET("/executions/:id/periods/metrics/:metric", api.GetExecutionPeriodMetric)
			backtestAPI.GET("/executions/:id/dataframe", api.GetExecutionDataframe)
			return nil
		},
		func(storage storage.BlobStorage) error {
			return storage.VerifyBuckets(context.TODO())
		},
		func(lc fx.Lifecycle, serviceAPI serviceAPI.Client) error {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					image := environment.GetBacktestImage()
					_, err := serviceAPI.GetImage(ctx, image)
					if err != nil {
						log.Info().Str("image", image).Msg("not able to find locally, pulling backtest image")
						_, pullErr := serviceAPI.DownloadImage(ctx, image)
						if pullErr != nil {
							return fmt.Errorf("error pulling backtest image: %w", pullErr)
						}
						log.Info().Str("image", image).Msg("backtest image pulled")
					}
					return nil
				},
				OnStop: func(ctx context.Context) error {
					return nil
				},
			})
			return nil
		},
		func(lc fx.Lifecycle, s BacktestStream, conn *pgxpool.Pool) error {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					err := s.CommandSubscriber("ingest", "ingest", command.Ingest)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.ingest: %w", err)
					}
					err = s.CommandSubscriber("session", "run", command.SessionRun)
					if err != nil {
						return fmt.Errorf("error subscribing to backtest.start: %w", err)
					}
					err = s.CommandSubscriber("ingest", "status", command.UpdateIngestStatus)
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
