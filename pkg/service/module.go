package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/stream"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/repository"
	"github.com/lhjnilsson/foreverbull/pkg/service/internal/stream/command"
	"go.uber.org/fx"
)

const StreamName = "service"

type Subscriber stream.Subscriber

var Module = fx.Options( //nolint: gochecknoglobals
	fx.Provide(
		func(eventStream stream.Stream, conn *pgxpool.Pool) (Subscriber, error) {
			dc := stream.NewDependencyContainer()
			dc.AddSingleton(stream.DBDep, conn)
			sub, err := eventStream.NewSubscriber(StreamName, dc)
			if err != nil {
				return nil, fmt.Errorf("failed to create subscriber: %w", err)
			}
			return sub, nil
		},
	),
	fx.Invoke(
		func(conn *pgxpool.Pool) error {
			return repository.CreateTables(context.Background(), conn)
		},
		registerServiceHTTP,
		func(lc fx.Lifecycle, sub Subscriber) error {
			lc.Append(
				fx.Hook{
					OnStart: func(_ context.Context) error {
						err := sub.Subscribe("service", "start", command.ServiceStart)
						if err != nil {
							return fmt.Errorf("error subscribing to service.start: %w", err)
						}
						err = sub.Subscribe("instance", "interview", command.InstanceInterview)
						if err != nil {
							return fmt.Errorf("error subscribing to instance.interview: %w", err)
						}
						err = sub.Subscribe("instance", "sanity_check", command.InstanceSanityCheck)
						if err != nil {
							return fmt.Errorf("error subscribing to instance.sanity_check: %w", err)
						}
						err = sub.Subscribe("instance", "stop", command.InstanceStop)
						if err != nil {
							return fmt.Errorf("error subscribing to instance.stop: %w", err)
						}
						err = sub.Subscribe("service", "status", command.UpdateServiceStatus)
						if err != nil {
							return fmt.Errorf("error subscribing to service.status: %w", err)
						}
						return nil
					},
					OnStop: func(_ context.Context) error {
						err := sub.Unsubscribe()
						if err != nil {
							return fmt.Errorf("error unsubscribing: %w", err)
						}
						return nil
					},
				},
			)
			return nil
		},
	),
)

type serviceHTTPParams struct {
	fx.In

	Mux  *http.ServeMux `optional:"true"`
	Conn *pgxpool.Pool
}

// registerServiceHTTP mounts service reads when the HTTP mux is in the process.
// The image is not a path segment. Module tests without the mux do not listen.
func registerServiceHTTP(p serviceHTTPParams) {
	if p.Mux == nil || p.Conn == nil {
		return
	}
	RegisterHTTP(p.Mux, NewStore(p.Conn))
}
