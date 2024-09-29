package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/service/container"
	containerImpl "github.com/lhjnilsson/foreverbull/pkg/service/internal/container"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/stream/command"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/stream/dependency"
	"github.com/nats-io/nats.go"
	"go.uber.org/fx"
)

const Stream = "service"

type ServiceStream stream.Stream

var Module = fx.Options(
	fx.Provide(
		containerImpl.NewImageRegistry,
		containerImpl.NewContainerRegistry,
		func(jt nats.JetStreamContext, conn *pgxpool.Pool, container container.Container) (ServiceStream, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			dc.AddSingleton(dependency.ContainerDep, container)
			return stream.NewNATSStream(jt, Stream, dc, conn)
		},
	),
	fx.Invoke(
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
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
