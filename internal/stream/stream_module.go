package stream

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/lhjnilsson/foreverbull/internal/stream/pb"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
	"google.golang.org/protobuf/proto"
)

type stateSubscriber struct {
	repository *Repository
	jt         nats.JetStreamContext

	sub *nats.Subscription
}

func (s *stateSubscriber) msgHandler(msg *nats.Msg) {
	state := &pb.State{}
	err := proto.Unmarshal(msg.Data, state)
	if err != nil {
		log.Err(err).Msg("error unmarshalling message")
		return
	}

	messageId := strings.Split(msg.Subject, ".")[4]
	err = s.repository.CreateMessageState(context.Background(), messageId, state)
	if err != nil {
		log.Err(err).Msg("error creating message state")
	}
}

func (s *stateSubscriber) Start(ctx context.Context) error {
	var err error

	opts := []nats.SubOpt{
		nats.MaxDeliver(1),
		nats.Durable("foreverbull-state-listener"),
	}
	deliverPolicy := environment.GetNATSDeliveryPolicy()

	switch deliverPolicy {
	case "all":
		opts = append(opts, nats.DeliverAll())
	case "last":
		opts = append(opts, nats.DeliverLast())
	default:
		return fmt.Errorf("unknown delivery policy: %s", environment.GetNATSDeliveryPolicy())
	}

	s.sub, err = s.jt.Subscribe("foreverbull.*.*.*.*.state", s.msgHandler, opts...)
	if err != nil {
		return fmt.Errorf("error subscribing to jetstream: %w", err)
	}
	return nil
}

func (s *stateSubscriber) Stop(ctx context.Context) error {
	if err := s.sub.Unsubscribe(); err != nil {
		return fmt.Errorf("error unsubscribing from jetstream for orchestration: %w", err)
	}

	return nil
}

var Module = fx.Options(
	fx.Provide(
		func() (Stream, error) {
			return NewNATSStream()
		},
		func(_ Stream, conn *pgxpool.Pool) (*stateSubscriber, error) {
			ctx := context.Background()
			repo := NewRepository(conn)
			if err := CreateTables(ctx, conn); err != nil {
				return nil, fmt.Errorf("error creating tables: %w", err)
			}
			natsConn, err := nats.Connect(environment.GetNATSURL())
			if err != nil {
				return nil, fmt.Errorf("error connecting to nats: %w", err)
			}

			jt, err := natsConn.JetStream()
			if err != nil {
				return nil, fmt.Errorf("error connecting to jetstream: %w", err)
			}
			return &stateSubscriber{repository: repo, jt: jt}, nil
		},
	),
	fx.Invoke(
		func(lc fx.Lifecycle, sub *stateSubscriber) error {
			lc.Append(
				fx.Hook{
					OnStart: func(ctx context.Context) error {
						return sub.Start(ctx)
					},
					OnStop: func(ctx context.Context) error {
						return sub.Stop(ctx)
					},
				},
			)
			return nil
		},
	),
)
