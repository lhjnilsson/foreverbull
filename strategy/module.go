package strategy

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/finance/supplier"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	serviceAPI "github.com/lhjnilsson/foreverbull/service/api"
	"github.com/lhjnilsson/foreverbull/strategy/internal/api"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
	"github.com/lhjnilsson/foreverbull/strategy/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/strategy/internal/stream/dependency"
	"github.com/nats-io/nats.go"
	"go.uber.org/fx"
)

const Stream = "strategy"

type StrategyStream stream.Stream

type StrategyAPI struct {
	*gin.RouterGroup
}

var Module = fx.Options(
	fx.Provide(
		func(jt nats.JetStreamContext, conn *pgxpool.Pool, serviceAPI serviceAPI.Client, trading supplier.Trading) (StrategyStream, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			dc.AddSingleton(dependency.ServiceAPI, serviceAPI)
			dc.AddSingleton(dependency.Trading, trading)
			dc.AddMethod(dependency.ExecutionRunner, dependency.GetExecution)
			dc.AddMethod(dependency.WorkerPool, dependency.GetWorkerPool)
			s, err := stream.NewNATSStream(jt, Stream, dc, conn)
			if err != nil {
				return nil, fmt.Errorf("failed to create stream: %w", err)
			}
			return s, nil
		},
		func(gin *gin.Engine) *StrategyAPI {
			return &StrategyAPI{gin.Group("/strategy/api")}
		},
	),
	fx.Invoke(
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
		},
		func(strategyAPI *StrategyAPI, pgxpool *pgxpool.Pool, stream StrategyStream) error {
			strategyAPI.Use(
				internalHTTP.OrchestrationMiddleware(api.OrchestrationDependency, stream),
				internalHTTP.TransactionMiddleware(api.TXDependency, pgxpool),
				func(ctx *gin.Context) {
					ctx.Next()
				},
			)
			strategyAPI.GET("/strategies", api.ListStrategies)
			strategyAPI.POST("/strategies", api.CreateStrategy)
			strategyAPI.GET("/strategies/:name", api.GetStrategy)
			strategyAPI.DELETE("/strategies/:name", api.DeleteStrategy)

			strategyAPI.GET("/executions", api.ListExecutions)
			strategyAPI.POST("/executions", api.CreateExecution)
			strategyAPI.GET("/executions/:id", api.GetExecution)
			return nil
		},
		func(lc fx.Lifecycle, s StrategyStream, conn *pgxpool.Pool) error {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					return nil
				},
				OnStop: func(ctx context.Context) error {
					return s.Unsubscribe()
				},
			})
			return nil
		},
		func(lc fx.Lifecycle, s StrategyStream, conn *pgxpool.Pool) error {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					err := s.CommandSubscriber("execution", "status", command.UpdateExecutionStatus)
					if err != nil {
						return fmt.Errorf("failed to subscribe to execution status: %w", err)
					}
					err = s.CommandSubscriber("execution", "run", command.RunExecution)
					if err != nil {
						return fmt.Errorf("failed to subscribe to execution run: %w", err)
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
