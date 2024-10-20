package stream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lhjnilsson/foreverbull/internal/environment"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

type Dependency string

const (
	DBDep              Dependency = "db"
	StreamDep          Dependency = "stream"
	ContainerEngineDep Dependency = "container_engine"
	StorageDep         Dependency = "storage"
)

type Handler interface {
	Process(ctx context.Context, message Message) error
}

type Stream interface {
	Unsubscribe() error
	Publish(ctx context.Context, message Message) error
	CommandSubscriber(component, method string, cb func(context.Context, Message) error) error
	RunOrchestration(ctx context.Context, orchestration *MessageOrchestration) error
}

func New() (*nats.Conn, nats.JetStreamContext, error) {
	natsConnect, err := nats.Connect(environment.GetNATSURL())
	if err != nil {
		return nil, nil, fmt.Errorf("error connecting to nats: %w", err)
	}

	natsJetstream, err := natsConnect.JetStream()
	if err != nil {
		return nil, nil, fmt.Errorf("error connecting to jetstream: %w", err)
	}

	_, err = natsJetstream.AddStream(&nats.StreamConfig{
		Name:     "foreverbull",
		Subjects: []string{"foreverbull.>"},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating stream: %w", err)
	}

	return natsConnect, natsJetstream, nil
}

type NATSStream struct {
	module string

	jt   nats.JetStreamContext
	subs []*nats.Subscription

	deps *dependencyContainer

	repository repository
}

func NewNATSStream(jetstream nats.JetStreamContext, module string,
	dependencies DependencyContainer, pool *pgxpool.Pool,
) (Stream, error) {
	cfg := &nats.ConsumerConfig{
		Name:       module,
		Durable:    module,
		MaxDeliver: 1,
		AckPolicy:  nats.AckExplicitPolicy,
	}

	_, err := jetstream.AddConsumer("foreverbull", cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating consumer: %w", err)
	}

	_, err = pool.Exec(context.Background(), table)
	if err != nil {
		return nil, fmt.Errorf("error creating table: %w", err)
	}

	return &NATSStream{
		module:     module,
		jt:         jetstream,
		deps:       dependencies.(*dependencyContainer),
		repository: NewRepository(pool),
	}, nil
}

func (ns *NATSStream) CommandSubscriber(component, method string, cb func(context.Context, Message) error) error {
	jtCb := func(natsMsg *nats.Msg) {
		// For now we just ack the message
		if err := natsMsg.Ack(); err != nil {
			log.Err(err).Msg("error acknowledging message")
		}

		go func(natsMsg *nats.Msg) {
			defer func() {
				if r := recover(); r != nil {
					log.Err(fmt.Errorf("panic: %v", r)).Stack().Str("component", component).Str("method", method).Msg("panic in command subscriber")
					fmt.Println("stacktrace from panic: \n" + string(debug.Stack()))
				}
			}()

			msg := &message{}

			err := json.Unmarshal(natsMsg.Data, &msg)
			if err != nil {
				log.Err(err).Msg("error unmarshalling message")
			}

			if msg.ID == nil {
				log.Error().Msg("message id is nil")
				return
			}

			ctx := context.Background()

			msg, err = ns.repository.UpdatePublishedAndGetMessage(ctx, *msg.ID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					log.Debug().Msg("message not found, probably already processed")
				} else {
					log.Err(err).Msg("error updating message status")
				}

				return
			}

			log := log.With().Str("id", *msg.ID).Str("component", component).Str("method", method).Logger()
			if msg.OrchestrationID != nil {
				log = log.With().Str("id", *msg.ID).Str("Orchestration", *msg.OrchestrationName).Str("OrchestrationID", *msg.OrchestrationID).Str("OrchestrationStep", *msg.OrchestrationStep).Logger()
			}

			log.Info().Msg("received command")

			msg.dependencyContainer = ns.deps

			err = cb(ctx, msg)
			if err != nil {
				log.Err(err).Msg("error executing command")

				err = ns.repository.UpdateMessageStatus(ctx, *msg.ID, MessageStatusError, err)
				if err != nil {
					log.Err(err).Msg("error updating message status")
					return
				}
			} else {
				log.Info().Msg("command completed successfully")

				err = ns.repository.UpdateMessageStatus(ctx, *msg.ID, MessageStatusComplete, nil)
				if err != nil {
					log.Err(err).Msg("error updating message status")
					return
				}
			}

			payload, err := json.Marshal(msg)
			if err != nil {
				log.Err(err).Msg("error marshalling message")
				return
			}

			_, err = ns.jt.Publish(fmt.Sprintf("foreverbull.%s.%s.%s.event", ns.module, component, method), payload)
			if err != nil {
				log.Err(err).Msg("error publishing event")
				return
			}
		}(natsMsg)
	}

	opts := []nats.SubOpt{
		nats.MaxDeliver(1),
		nats.Durable(fmt.Sprintf("foreverbull-%s-%s-%s", ns.module, component, method)),
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

	sub, err := ns.jt.Subscribe(fmt.Sprintf("foreverbull.%s.%s.%s.command", ns.module, component, method), jtCb, opts...)
	if err != nil {
		return fmt.Errorf("error subscribing to jetstream: %w", err)
	}

	ns.subs = append(ns.subs, sub)

	return nil
}

func (ns *NATSStream) Unsubscribe() error {
	for _, sub := range ns.subs {
		if !sub.IsValid() {
			continue
		}

		err := sub.Drain()
		if err != nil {
			return fmt.Errorf("error draining subscription: %w", err)
		}
	}

	return nil
}

func (ns *NATSStream) Publish(ctx context.Context, msg Message) error {
	m := msg.(*message)
	if m.ID == nil {
		if err := ns.repository.CreateMessage(ctx, m); err != nil {
			return fmt.Errorf("error creating message: %w", err)
		}
	}

	topic := fmt.Sprintf("foreverbull.%s.%s.%s.command", m.Module, m.Component, m.Method)

	payload, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("error marshalling message: %w", err)
	}

	err = ns.repository.UpdateMessageStatus(ctx, *m.ID, MessageStatusPublished, err)
	if err != nil {
		return fmt.Errorf("error updating message status: %w", err)
	}

	_, err = ns.jt.Publish(topic, payload)
	if err != nil {
		return fmt.Errorf("error publishing message: %w", err)
	}

	return nil
}

func (ns *NATSStream) RunOrchestration(ctx context.Context, orchestration *MessageOrchestration) error {
	for _, step := range orchestration.Steps {
		for _, cmd := range step.Commands {
			msg, isMsg := cmd.(*message)
			if msg.OrchestrationID == nil {
				return fmt.Errorf("orchestration id is nil")
			}

			if msg.OrchestrationStep == nil {
				return errors.New("orchestration step is nil")
			}

			if msg.OrchestrationStepNumber == nil {
				return errors.New("orchestration step number is nil")
			}

			if !isMsg {
				return fmt.Errorf("command is not a message")
			}

			err := ns.repository.CreateMessage(ctx, msg)
			if err != nil {
				return err
			}
		}
	}

	if orchestration.FallbackStep == nil {
		return fmt.Errorf("orchestration must have fallback step")
	}

	for _, cmd := range orchestration.FallbackStep.Commands {
		msg, isMsg := cmd.(*message)
		if msg.OrchestrationID == nil {
			return fmt.Errorf("orchestration id is nil")
		}

		if msg.OrchestrationStep == nil {
			return errors.New("orchestration step is nil")
		}

		if !isMsg {
			return fmt.Errorf("command is not a message")
		}

		err := ns.repository.CreateMessage(ctx, msg)
		if err != nil {
			return fmt.Errorf("error creating message: %w", err)
		}
	}

	commands, err := ns.repository.GetNextOrchestrationCommands(ctx, orchestration.OrchestrationID, -1)
	if err != nil {
		return fmt.Errorf("error getting latest unpublished orchestration step commands: %w", err)
	}

	for _, cmd := range *commands {
		err = ns.Publish(ctx, &cmd)
		if err != nil {
			return fmt.Errorf("error publishing command: %w", err)
		}
	}

	return nil
}
