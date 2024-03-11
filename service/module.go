package service

import (
	"context"
	"fmt"

	"github.com/gin-contrib/logger"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	internalHTTP "github.com/lhjnilsson/foreverbull/internal/http"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	apiDef "github.com/lhjnilsson/foreverbull/service/api"
	"github.com/lhjnilsson/foreverbull/service/container"
	"github.com/lhjnilsson/foreverbull/service/internal/api"
	containerImpl "github.com/lhjnilsson/foreverbull/service/internal/container"
	"github.com/lhjnilsson/foreverbull/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/service/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/service/internal/stream/dependency"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

const Stream = "service"

type ServiceStream stream.Stream

type ServiceAPI struct {
	*gin.RouterGroup
}

var Module = fx.Options(
	fx.Provide(
		containerImpl.NewImageRegistry,
		containerImpl.NewContainerRegistry,
		func(gin *gin.Engine) *ServiceAPI {
			return &ServiceAPI{gin.Group("/service/api")}
		},
		func(jt nats.JetStreamContext, conn *pgxpool.Pool, container container.Container) (ServiceStream, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			dc.AddSingleton(dependency.ContainerDep, container)
			return stream.NewNATSStream(jt, Stream, dc, conn)
		},
		func() (apiDef.Client, error) {
			return apiDef.NewClient()
		},
	),
	fx.Invoke(
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
		},
		func(serviceAPI *ServiceAPI, pgxpool *pgxpool.Pool, stream ServiceStream, container container.Container, image container.Image) error {
			serviceAPI.Use(
				logger.SetLogger(logger.WithLogger(func(ctx *gin.Context, l zerolog.Logger) zerolog.Logger {
					return log.Logger
				}),
				),
				internalHTTP.OrchestrationMiddleware(api.OrchestrationDependency, stream),
				internalHTTP.TransactionMiddleware(api.TXDependency, pgxpool),
				func(ctx *gin.Context) {
					ctx.Set(api.ContainerDependency, container)
					ctx.Set(api.ImageDependency, image)
					ctx.Next()
				},
			)
			serviceAPI.GET("/services", api.ListServices)
			serviceAPI.POST("/services", api.CreateService)
			serviceAPI.GET("/services/*image", api.GetService)
			serviceAPI.DELETE("/services/*image", api.DeleteService)

			serviceAPI.GET("/instances", api.ListInstances)
			serviceAPI.GET("/instances/:instanceID", api.GetInstance)
			serviceAPI.PATCH("/instances/:instanceID", api.PatchInstance)

			serviceAPI.GET("/images/*name", api.GetImage)
			serviceAPI.POST("/images/*name", api.PullImage)
			return nil
		},
		func(lc fx.Lifecycle, s ServiceStream, container container.Container, conn *pgxpool.Pool) error {
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
						err := s.Unsubscribe()
						if err != nil {
							return fmt.Errorf("error unsubscribing: %w", err)
						}
						err = container.StopAll(ctx, true)
						if err != nil {
							return fmt.Errorf("error stopping all containers: %w", err)
						}
						return nil
					},
				},
			)
			return nil
		},
	),
)
