package strategy

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/strategy/internal/api"
	"github.com/lhjnilsson/foreverbull/strategy/internal/repository"
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
		func(jt nats.JetStreamContext, conn *pgxpool.Pool) (StrategyStream, error) {
			dc := stream.NewDependencyContainer()
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
				internalHTTP.TransactionMiddleware(api.TXDependency, pgxpool),
				func(ctx *gin.Context) {
					ctx.Next()
				},
			)
			strategyAPI.GET("/strategies", api.ListStrategies)
			strategyAPI.POST("/strategies", api.CreateStrategy)
			strategyAPI.GET("/strategies/:name", api.GetStrategy)
			strategyAPI.PATCH("/strategies/:name", api.PatchStrategy)
			strategyAPI.DELETE("/strategies/:name", api.DeleteStrategy)

			strategyAPI.GET("/executions", api.ListExecutions)
			strategyAPI.POST("/executions", api.CreateExecution)
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
	),
)
