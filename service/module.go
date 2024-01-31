package service

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/config"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/service/container"
	"github.com/lhjnilsson/foreverbull/service/internal/api"
	containerImpl "github.com/lhjnilsson/foreverbull/service/internal/container"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/service/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/service/internal/stream/dependency"
	"github.com/nats-io/nats.go"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const Stream = "service"

type ServiceStream stream.Stream

type ServiceAPI struct {
	*gin.RouterGroup
}

var Module = fx.Options(
	fx.Provide(
		containerImpl.New,
		func(gin *gin.Engine) *ServiceAPI {
			return &ServiceAPI{gin.Group("/service/api")}
		},
		func(jt nats.JetStreamContext, config *config.Config, log *zap.Logger, conn *pgxpool.Pool, container container.Container) (ServiceStream, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingelton(stream.ConfigDep, config)
			dc.AddSingelton(stream.DBDep, conn)
			dc.AddSingelton(dependency.ContainerDep, container)
			return stream.NewNATSStream(jt, Stream, config.NATS_DELIVERY_POLICY, log, dc, conn)
		},
	),
	fx.Invoke(
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
		},
		func(serviceAPI *ServiceAPI, pgxpool *pgxpool.Pool, log *zap.Logger, stream ServiceStream, container container.Container) error {
			serviceAPI.Use(
				internalHTTP.OrchestrationMiddleware(api.OrchestrationDependency, stream),
				internalHTTP.TransactionMiddleware(api.TXDependency, pgxpool),
				func(ctx *gin.Context) {
					ctx.Set(api.LoggingDependency, log)
					ctx.Set(api.ContainerDependency, container)
					ctx.Next()
				},
			)
			serviceAPI.GET("/services", api.ListServices)
			serviceAPI.POST("/services", api.CreateService)
			serviceAPI.GET("/services/:name", api.GetService)
			serviceAPI.DELETE("/services/:name", api.DeleteService)
			serviceAPI.GET("/services/:name/image", api.GetServiceImage)
			//serviceAPI.POST("/services/:name/image", api.UpdateServiceImage)

			serviceAPI.GET("/instances", api.ListInstances)
			serviceAPI.GET("/instances/:instanceID", api.GetInstance)
			serviceAPI.PATCH("/instances/:instanceID", api.PatchInstance)
			return nil
		},
		func(lc fx.Lifecycle, s ServiceStream, config *config.Config, log *zap.Logger, container container.Container, conn *pgxpool.Pool) error {
			lc.Append(
				fx.Hook{
					OnStart: func(ctx context.Context) error {
						err := s.CommandSubscriber("service", "start", command.ServiceStart)
						if err != nil {
							return fmt.Errorf("error subscribing to service.start: %w", err)
						}
						err = s.CommandSubscriber("instance", "interview", command.InstanceInterview)
						if err != nil {
							return fmt.Errorf("error subscribing to instance.interview: %w", err)
						}
						err = s.CommandSubscriber("instance", "sanity_check", command.InstanceSanityCheck)
						if err != nil {
							return fmt.Errorf("error subscribing to instance.sanity_check: %w", err)
						}
						err = s.CommandSubscriber("instance", "stop", command.InstanceStop)
						if err != nil {
							return fmt.Errorf("error subscribing to instance.stop: %w", err)
						}
						err = s.CommandSubscriber("service", "status", command.UpdateServiceStatus)
						if err != nil {
							return fmt.Errorf("error subscribing to service.status: %w", err)
						}
						return nil
					},
					OnStop: func(ctx context.Context) error {
						return s.Unsubscribe()
					},
				},
			)
			return nil
		},
	),
)
